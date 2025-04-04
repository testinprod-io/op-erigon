// Copyright 2024 The Erigon Authors
// This file is part of Erigon.
//
// Erigon is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Erigon is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Erigon. If not, see <http://www.gnu.org/licenses/>.

package app

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erigontech/erigon-lib/common/datadir"
	"github.com/erigontech/erigon-lib/kv/order"
	libstate "github.com/erigontech/erigon-lib/state"
	"github.com/erigontech/erigon/core/tracing"
	"io"
	"math/big"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/erigontech/erigon-lib/chain"
	libcommon "github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/crypto"
	"github.com/erigontech/erigon-lib/kv/rawdbv3"
	"github.com/erigontech/erigon-lib/trie"
	"github.com/erigontech/erigon/core/state"
	stages2 "github.com/erigontech/erigon/eth/stagedsync/stages"
	"github.com/holiman/uint256"

	"github.com/urfave/cli/v2"

	"github.com/erigontech/erigon-lib/log/v3"

	"github.com/erigontech/erigon-lib/direct"
	execution "github.com/erigontech/erigon-lib/gointerfaces/executionproto"
	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/wrap"
	"github.com/erigontech/erigon/consensus/merge"
	"github.com/erigontech/erigon/turbo/execution/eth1/eth1_chain_reader"
	"github.com/erigontech/erigon/turbo/services"

	"github.com/erigontech/erigon-lib/rlp"
	"github.com/erigontech/erigon/cmd/utils"
	"github.com/erigontech/erigon/core"
	"github.com/erigontech/erigon/core/rawdb"
	"github.com/erigontech/erigon/core/types"
	"github.com/erigontech/erigon/eth"
	"github.com/erigontech/erigon/turbo/debug"
	turboNode "github.com/erigontech/erigon/turbo/node"
	"github.com/erigontech/erigon/turbo/stages"
)

const (
	importBatchSize = 2500
)

var importCommand = cli.Command{
	Action:    MigrateFlags(importChain),
	Name:      "import",
	Usage:     "Import a blockchain file",
	ArgsUsage: "<filename> (<filename 2> ... <filename N>) ",
	Flags: []cli.Flag{
		&utils.DataDirFlag,
		&utils.ChainFlag,
	},
	//Category: "BLOCKCHAIN COMMANDS",
	Description: `
The import command imports blocks from an RLP-encoded form. The form can be one file
with several RLP-encoded blocks, or several files can be used.

If only one file is used, import error will result in failure. If several files are used,
processing will proceed even if an individual RLP-file import failure occurs.`,
}

var importTotalDifficultyCommand = cli.Command{
	Action:    MigrateFlags(importTotalDifficulty),
	Name:      "import-totaldifficulty",
	Usage:     "Import a total difficulty file",
	ArgsUsage: "<filename> ",
	Flags: []cli.Flag{
		&utils.DataDirFlag,
		&utils.ChainFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The import command imports total difficulty from an RLP-encoded form.`,
}

var importStateCommand = cli.Command{
	Action:    MigrateFlags(importState),
	Name:      "import-state",
	Usage:     "Import a state file",
	ArgsUsage: "<filename> <blockNum>",
	Flags: []cli.Flag{
		&utils.DataDirFlag,
		&utils.ChainFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The import command imports state from a json form, causing regenesis.`,
}

func importChain(cliCtx *cli.Context) error {
	if cliCtx.NArg() < 1 {
		utils.Fatalf("This command requires an argument.")
	}
	logger, _, _, err := debug.Setup(cliCtx, true /* rootLogger */)
	if err != nil {
		return err
	}

	nodeCfg, err := turboNode.NewNodConfigUrfave(cliCtx, logger)
	if err != nil {
		return err
	}
	ethCfg := turboNode.NewEthConfigUrfave(cliCtx, nodeCfg, logger)

	stack := makeConfigNode(cliCtx.Context, nodeCfg, logger)
	defer stack.Close()

	ethereum, err := eth.New(cliCtx.Context, stack, ethCfg, logger)
	if err != nil {
		return err
	}
	err = ethereum.Init(stack, ethCfg, ethCfg.Genesis.Config)
	if err != nil {
		return err
	}

	if err := ImportChain(ethereum, ethereum.ChainDB(), cliCtx.Args().First(), logger); err != nil {
		logger.Info("Failed", "err", err)
		return err
	}

	logger.Info("Complete")
	return nil
}

func ImportChain(ethereum *eth.Ethereum, chainDB kv.RwDB, fn string, logger log.Logger) error {
	// Watch for Ctrl-C while the import is running.
	// If a signal is received, the import will stop at the next batch.
	interrupt := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	defer close(interrupt)
	go func() {
		if _, ok := <-interrupt; ok {
			logger.Info("Interrupted during import, stopping at next batch")
		}
		close(stop)
	}()
	checkInterrupt := func() bool {
		select {
		case <-stop:
			return true
		default:
			return false
		}
	}

	logger.Info("Importing blockchain", "file", fn)

	// Open the file handle and potentially unwrap the gzip stream
	fh, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer fh.Close()

	var reader io.Reader = fh
	if strings.HasSuffix(fn, ".gz") {
		if reader, err = gzip.NewReader(reader); err != nil {
			return err
		}
	}
	stream := rlp.NewStream(reader, 0)

	// Run actual the import.
	blocks := make(types.Blocks, importBatchSize)
	n := 0
	for batch := 0; ; batch++ {
		// Load a batch of RLP blocks.
		if checkInterrupt() {
			return errors.New("interrupted")
		}
		i := 0
		for ; i < importBatchSize; i++ {
			var b types.Block
			if err := stream.Decode(&b); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return fmt.Errorf("at block %d: %v", n, err)
			}
			// don't import first block
			if b.NumberU64() == 0 {
				i--
				continue
			}
			blocks[i] = &b
			n++
		}
		if i == 0 {
			break
		}
		// Import the batch.
		if checkInterrupt() {
			return errors.New("interrupted")
		}

		br, _ := ethereum.BlockIO()
		missing := missingBlocks(chainDB, blocks[:i], br)
		if len(missing) == 0 {
			logger.Info("Skipping batch as all blocks present", "batch", batch, "first", blocks[0].Hash(), "last", blocks[i-1].Hash())
			continue
		}

		// RLP decoding worked, try to insert into chain:
		missingChain := &core.ChainPack{
			Blocks:   missing,
			TopBlock: missing[len(missing)-1],
		}

		if err := InsertChainWithoutExecution(ethereum, missingChain, logger); err != nil {
			return err
		}
	}
	return nil
}

func ChainHasBlock(chainDB kv.RwDB, block *types.Block) bool {
	var chainHasBlock bool

	chainDB.View(context.Background(), func(tx kv.Tx) (err error) {
		chainHasBlock = rawdb.HasBlock(tx, block.Hash(), block.NumberU64())
		return nil
	})

	return chainHasBlock
}

func missingBlocks(chainDB kv.RwDB, blocks []*types.Block, blockReader services.FullBlockReader) []*types.Block {
	var headBlock *types.Block
	chainDB.View(context.Background(), func(tx kv.Tx) (err error) {
		headBlock, err = blockReader.CurrentBlock(tx)
		return err
	})

	for i, block := range blocks {
		// If we're behind the chain head, only check block, state is available at head
		if headBlock.NumberU64() > block.NumberU64() {
			if !ChainHasBlock(chainDB, block) {
				return blocks[i:]
			}
			continue
		}

		if !ChainHasBlock(chainDB, block) {
			return blocks[i:]
		}
	}

	return nil
}

func InsertChain(ethereum *eth.Ethereum, chain *core.ChainPack, logger log.Logger) error {
	sentryControlServer := ethereum.SentryControlServer()
	initialCycle, firstCycle := false, false
	for _, b := range chain.Blocks {
		sentryControlServer.Hd.AddMinedHeader(b.Header())
		sentryControlServer.Bd.AddToPrefetch(b.Header(), b.RawBody())
	}
	sentryControlServer.Hd.MarkAllVerified()
	blockReader, _ := ethereum.BlockIO()

	hook := stages.NewHook(ethereum.SentryCtx(), ethereum.ChainDB(), ethereum.Notifications(), ethereum.StagedSync(), blockReader, ethereum.ChainConfig(), logger, sentryControlServer.SetStatus)
	err := stages.StageLoopIteration(ethereum.SentryCtx(), ethereum.ChainDB(), wrap.TxContainer{}, ethereum.StagedSync(), initialCycle, firstCycle, logger, blockReader, hook)
	if err != nil {
		return err
	}

	return insertPosChain(ethereum, chain, logger)
}

func insertPosChain(ethereum *eth.Ethereum, chain *core.ChainPack, logger log.Logger) error {
	posBlockStart := 0
	for i, b := range chain.Blocks {
		if b.Header().Difficulty.Cmp(merge.ProofOfStakeDifficulty) == 0 {
			posBlockStart = i
			break
		}
	}

	if posBlockStart == chain.Length() {
		return nil
	}

	for i := posBlockStart; i < chain.Length(); i++ {
		if err := chain.Blocks[i].HashCheck(true); err != nil {
			return err
		}
	}

	chainRW := eth1_chain_reader.NewChainReaderEth1(ethereum.ChainConfig(), direct.NewExecutionClientDirect(ethereum.ExecutionModule()), uint64(time.Hour))

	ctx := context.Background()
	if err := chainRW.InsertBlocksAndWait(ctx, chain.Blocks); err != nil {
		return err
	}

	tipHash := chain.TopBlock.Hash()

	status, _, lvh, err := chainRW.UpdateForkChoice(ctx, tipHash, tipHash, tipHash)

	if err != nil {
		return err
	}

	ethereum.ChainDB().Update(ethereum.SentryCtx(), func(tx kv.RwTx) error {
		rawdb.WriteHeadBlockHash(tx, lvh)
		return nil
	})
	if status != execution.ExecutionStatus_Success {
		return fmt.Errorf("insertion failed for block %d, code: %s", chain.Blocks[chain.Length()-1].NumberU64(), status.String())
	}

	return nil
}

func InsertChainWithoutExecution(ethereum *eth.Ethereum, chain *core.ChainPack, logger log.Logger) error {
	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i := 0; i < chain.Length(); i++ {
		block := chain.Blocks[i]
		if err := WriteBlockWithoutExecution(ethereum, tx, block); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	return insertPosChain(ethereum, chain, logger)
}

// may rename to without execution
func WriteBlockWithoutExecution(ethereum *eth.Ethereum, tx kv.RwTx, block *types.Block) error {
	if err := rawdb.WriteBlock(tx, block); err != nil {
		return err
	}
	if err := rawdb.WriteHeaderNumber(tx, block.Hash(), block.NumberU64()); err != nil {
		return err
	}
	if err := rawdb.WriteCanonicalHash(tx, block.Hash(), block.NumberU64()); err != nil {
		return err
	}
	if err := rawdb.WriteHeadHeaderHash(tx, block.Hash()); err != nil {
		return err
	}
	rawdb.WriteForkchoiceHead(tx, block.Hash())
	rawdb.WriteForkchoiceSafe(tx, block.Hash())
	rawdb.WriteForkchoiceFinalized(tx, block.Hash())

	txNumMin, err := rawdbv3.TxNums.Min(tx, block.NumberU64())
	if err != nil {
		return err
	}
	rawdb.WriteTxLookupEntries(tx, block, txNumMin)

	if err := rawdbv3.TxNums.Append(tx, block.NumberU64(), uint64(block.Transactions().Len()+1)); err != nil {
		return err
	}

	// mark every stage as done
	for _, stage := range stages2.AllStages {
		if err := stages2.SaveStageProgress(tx, stage, block.NumberU64()); err != nil {
			return err
		}
	}

	txHash := types.DeriveSha(block.Transactions())
	if txHash != block.TxHash() {
		return errors.New("tx trie root mismatch. aborting")
	}

	return nil
}

//func importReceipts(ctx *cli.Context) error {
//	if ctx.NArg() < 1 {
//		utils.Fatalf("This command requires an argument.")
//	}
//
//	logger, _, _, err := debug.Setup(ctx, true /* rootLogger */)
//	if err != nil {
//		return err
//	}
//
//	nodeCfg, err := turboNode.NewNodConfigUrfave(ctx, logger)
//	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg, logger)
//
//	stack := makeConfigNode(ctx.Context, nodeCfg, logger)
//	defer stack.Close()
//
//	ethereum, err := eth.New(ctx.Context, stack, ethCfg, logger)
//	if err != nil {
//		return err
//	}
//	err = ethereum.Init(stack, ethCfg, ethCfg.Genesis.Config)
//	if err != nil {
//		return err
//	}
//
//	if err := ImportReceipts(ethereum, ethereum.ChainDB(), ctx.Args().First()); err != nil {
//		return err
//	}
//
//	return nil
//}

func importTotalDifficulty(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		utils.Fatalf("This command requires an argument.")
	}

	logger, _, _, err := debug.Setup(ctx, true /* rootLogger */)
	if err != nil {
		return err
	}

	nodeCfg, err := turboNode.NewNodConfigUrfave(ctx, logger)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg, logger)

	stack := makeConfigNode(ctx.Context, nodeCfg, logger)
	defer stack.Close()

	ethereum, err := eth.New(ctx.Context, stack, ethCfg, logger)
	if err != nil {
		return err
	}
	err = ethereum.Init(stack, ethCfg, ethCfg.Genesis.Config)
	if err != nil {
		return err
	}

	if err := ImportTotalDifficulty(ethereum, ethereum.ChainDB(), ctx.Args().First()); err != nil {
		logger.Info("Failed", "err", err)
		return err
	}

	logger.Info("Complete")
	return nil
}

func importState(ctx *cli.Context) error {
	if ctx.NArg() < 2 {
		utils.Fatalf("This command requires an argument.")
	}

	logger, _, _, err := debug.Setup(ctx, true /* rootLogger */)
	if err != nil {
		return err
	}

	nodeCfg, err := turboNode.NewNodConfigUrfave(ctx, logger)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg, logger)

	stack := makeConfigNode(ctx.Context, nodeCfg, logger)
	defer stack.Close()

	ethereum, err := eth.New(ctx.Context, stack, ethCfg, logger)
	if err != nil {
		return err
	}
	err = ethereum.Init(stack, ethCfg, ethCfg.Genesis.Config)
	if err != nil {
		return err
	}

	fn := ctx.Args().First()
	blockNum, err := strconv.ParseInt(ctx.Args().Get(1), 10, 64)
	if err != nil {
		utils.Fatalf("Export error in parsing parameters: block number not an integer\n")
	}

	// make sure state trie is empty before import
	if err := DbSanityCheck(ctx.Context, ethCfg.Dirs, ethereum, uint64(blockNum), true); err != nil {
		logger.Info("Failed DbSanityCheck", "err", err)
		return err
	}
	if err := ImportState(ctx.Context, ethereum, fn, uint64(blockNum), true, logger); err != nil {
		logger.Info("Failed ImportState", "err", err)
		return err
	}
	// storage trie sanity check will manually done using sanity-check command
	// if err := SanityCheckStorageTrie(ethereum, fn, uint64(blockNum), ethCfg.ImportStateStream); err != nil {
	// 	return err
	// }

	logger.Info("Complete")
	return nil
}

//func ImportReceipts(ethereum *eth.Ethereum, chainDB kv.RwDB, fn string) error {
//	// Watch for Ctrl-C while the import is running.
//	// If a signal is received, the import will stop at the next batch.
//	interrupt := make(chan os.Signal, 1)
//	stop := make(chan struct{})
//	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
//	defer signal.Stop(interrupt)
//	defer close(interrupt)
//	go func() {
//		if _, ok := <-interrupt; ok {
//			log.Info("Interrupted during import, stopping at next batch")
//		}
//		close(stop)
//	}()
//	checkInterrupt := func() bool {
//		select {
//		case <-stop:
//			return true
//		default:
//			return false
//		}
//	}
//
//	log.Info("Importing receipts", "file", fn)
//
//	// Open the file handle and potentially unwrap the gzip stream
//	fh, err := os.Open(fn)
//	if err != nil {
//		return err
//	}
//	defer fh.Close()
//
//	var reader io.Reader = fh
//	if strings.HasSuffix(fn, ".gz") {
//		if reader, err = gzip.NewReader(reader); err != nil {
//			return err
//		}
//	}
//	stream := rlp.NewStream(reader, 0)
//
//	// Run actual the import.
//	receiptsList := make(types.ReceiptsList, importBatchSize)
//
//	n := 0
//	quit := StatusReporter("Import receipts", &n)
//
//	for batch := 0; ; batch++ {
//		// Load a batch of RLP blocks.
//		if checkInterrupt() {
//			return fmt.Errorf("interrupted")
//		}
//		i := 0
//		for ; i < importBatchSize; i++ {
//			var hackreceipts types.HackReceipts
//			// hack assuming that default rlp will work
//			if err := stream.Decode(&hackreceipts); errors.Is(err, io.EOF) {
//				break
//			} else if err != nil {
//				return fmt.Errorf("at block %d: %v", n, err)
//			}
//			receipts, err := hackreceipts.ConvertToReceipts()
//			if err != nil {
//				return fmt.Errorf("at block %d: %v", n, err)
//			}
//			receiptsList[i] = receipts
//			n++
//		}
//		if i == 0 {
//			break
//		}
//		// Import the batch.
//		if checkInterrupt() {
//			return fmt.Errorf("interrupted")
//		}
//
//		missing := missingReceiptsList(chainDB, receiptsList[:i])
//		if len(missing) == 0 {
//			log.Info("Skipping batch as all receipts present", "batch", batch)
//			continue
//		}
//
//		if err := InsertReceipts(ethereum, missing); err != nil {
//			return err
//		}
//	}
//	close(quit)
//
//	return nil
//}

//func InsertReceipts(ethereum *eth.Ethereum, receiptsList []*types.Receipts) error {
//	db := ethereum.ChainDB()
//	tx, err := db.BeginRw(ethereum.SentryCtx())
//	if err != nil {
//		return err
//	}
//	defer tx.Rollback()
//
//	for _, receipts := range receiptsList {
//		if receipts.Len() == 0 {
//			continue
//		}
//		firstReceipt := []*types.Receipt(*receipts)[0]
//		blockNumber := firstReceipt.BlockNumber.Uint64()
//		block := rawdb.ReadBlock(tx, firstReceipt.BlockHash, blockNumber)
//
//		var receiptsVal types.Receipts = *receipts
//		err = rawdb.WriteReceipts(tx, blockNumber, receiptsVal)
//		if err != nil {
//			log.Error(err.Error())
//			break
//		}
//
//		receiptHash := types.DeriveSha(receipts)
//		if receiptHash != block.ReceiptHash() {
//			return errors.New("receipt trie root mismatch. aborting")
//		}
//
//	}
//	if err := tx.Commit(); err != nil {
//		return err
//	}
//
//	return nil
//}

func ImportTotalDifficulty(ethereum *eth.Ethereum, chainDB kv.RwDB, fn string) error {
	// Watch for Ctrl-C while the import is running.
	// If a signal is received, the import will stop at the next batch.
	interrupt := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	defer close(interrupt)
	go func() {
		if _, ok := <-interrupt; ok {
			log.Info("Interrupted during import, stopping at next batch")
		}
		close(stop)
	}()
	checkInterrupt := func() bool {
		select {
		case <-stop:
			return true
		default:
			return false
		}
	}

	log.Info("Importing total difficulty", "file", fn)

	// Open the file handle and potentially unwrap the gzip stream
	fh, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer fh.Close()

	var reader io.Reader = fh
	if strings.HasSuffix(fn, ".gz") {
		if reader, err = gzip.NewReader(reader); err != nil {
			return err
		}
	}
	stream := rlp.NewStream(reader, 0)

	n := 0
	quit := StatusReporter("Import total difficulty", &n)

	startNum := 0
	for batch := 0; ; batch++ {
		// Load a batch of RLP blocks.
		if checkInterrupt() {
			return fmt.Errorf("interrupted")
		}
		i := 0
		var difficultyList []*big.Int
		for ; i < importBatchSize; i++ {
			var td *big.Int
			if err := stream.Decode(&td); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return fmt.Errorf("at block %d: %v", n, err)
			}
			difficultyList = append(difficultyList, td)
			n++
		}
		if i == 0 {
			break
		}
		// Import the batch.
		if checkInterrupt() {
			return fmt.Errorf("interrupted")
		}
		if err := InsertTotalDifficulty(ethereum, difficultyList, uint64(startNum)); err != nil {
			return err
		}
		startNum += len(difficultyList)
	}
	close(quit)

	return nil
}

func InsertTotalDifficulty(ethereum *eth.Ethereum, difficultyList []*big.Int, number uint64) error {
	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, difficulty := range difficultyList {
		blockNum := number + uint64(i)
		header := rawdb.ReadHeaderByNumber(tx, blockNum)
		if header == nil {
			return fmt.Errorf("header not found")
		}
		block := rawdb.ReadBlock(tx, header.Hash(), blockNum)
		if block == nil {
			return errors.New("block not readable")
		}
		err = rawdb.WriteTd(tx, block.Hash(), blockNum, difficulty)
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// modified from l2geth's core/state/dump.go
type ImportAccount struct {
	Balance  string                    `json:"balance"`
	Nonce    uint64                    `json:"nonce"`
	Root     string                    `json:"root"`
	CodeHash string                    `json:"codeHash"`
	Code     string                    `json:"code,omitempty"`
	Storage  map[libcommon.Hash]string `json:"storage,omitempty"`
	Address  libcommon.Address         `json:"address,omitempty"`
}

type ImportAlloc map[libcommon.Address]ImportAccount

type ImportStreamHeader struct {
	Root libcommon.Hash `json:"root"`
}

func (ia *ImportAlloc) UnmarshalJson(data []byte) error {
	m := make(ImportAlloc)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*ia = make(ImportAlloc)
	for addr, a := range m {
		(*ia)[libcommon.Address(addr)] = a
	}
	return nil
}

func storeAccount(statedb *state.IntraBlockState, account *ImportAccount) error {
	address := account.Address
	balanceBigInt, ok := new(big.Int).SetString(account.Balance, 10)
	if !ok {
		return errors.New("balance bigint conversion failure")
	}
	balance, overflow := uint256.FromBig(balanceBigInt)
	if overflow {
		return errors.New("balance overflow")
	}
	err := statedb.AddBalance(address, balance, tracing.BalanceChangeUnspecified)
	if err != nil {
		return err
	}
	hexCode := strings.TrimPrefix(account.Code, "0x")
	code, err := hex.DecodeString(hexCode)
	if err != nil {
		return fmt.Errorf("code hexdecode failure, %s", hexCode)
	}
	hexCodeHash := strings.TrimPrefix(account.CodeHash, "0x")
	codeHash, err := hex.DecodeString(hexCodeHash)
	if err != nil {
		return fmt.Errorf("codehash hexdecode failure, %s", hexCodeHash)
	}
	tempCodeHash := crypto.Keccak256(code)
	if !bytes.Equal(tempCodeHash, codeHash) {
		return fmt.Errorf("codehash mismatch, expected %x, got %x", codeHash, tempCodeHash)
	}
	err = statedb.SetCode(address, code)
	if err != nil {
		return err
	}
	err = statedb.SetNonce(address, account.Nonce)
	if err != nil {
		return err
	}
	for key, hexValue := range account.Storage {
		key := key
		value, err := hex.DecodeString(hexValue)
		if err != nil {
			return errors.New("value hexdecode failure")
		}
		val := uint256.NewInt(0).SetBytes(value)
		err = statedb.SetState(address, &key, *val)
		if err != nil {
			return err
		}
	}

	if len(account.Code) > 0 || len(account.Storage) > 0 {
		err = statedb.SetIncarnation(address, state.FirstContractIncarnation)
		if err != nil {
			return err
		}
	}
	return nil
}

func ImportState(ctx context.Context, ethereum *eth.Ethereum, fn string, blockNumber uint64, stream bool, logger log.Logger) error {
	logger.Info("Importing state", "file", fn, "stream", stream)
	logger.Info("Importing state for block number", "blockNumber", blockNumber)
	fh, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer fh.Close()

	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	domains, err := libstate.NewSharedDomains(tx, logger)
	if err != nil {
		return err
	}
	domains.SetBlockNum(blockNumber)
	r, w := state.NewReaderV3(domains), state.NewWriterV4(domains)
	statedb := state.New(r)

	idx := 0
	quit := StatusReporter("Import state", &idx)

	if stream {
		reader := bufio.NewReader(fh)
		delimiter := byte('\n')
		var importStreamHeader ImportStreamHeader
		headerConsumed := false
		for {
			line, err := reader.ReadBytes(delimiter)
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return err
			}
			if !headerConsumed {
				if err := json.Unmarshal(line, &importStreamHeader); err != nil {
					return err
				}
				log.Info("state root from header", "root", importStreamHeader.Root.Hex())
				headerConsumed = true
				continue
			}
			var account ImportAccount
			if err := json.Unmarshal(line, &account); err != nil {
				return err
			}
			idx += 1
			if err := storeAccount(statedb, &account); err != nil {
				return err
			}
		}
	} else {
		decoder := json.NewDecoder(fh)
		ia := make(ImportAlloc)
		if err := decoder.Decode(&ia); err != nil {
			return err
		}
		for address, account := range ia {
			idx += 1
			account.Address = address
			//nolint:all
			if err := storeAccount(statedb, &account); err != nil {
				return err
			}
		}
	}
	close(quit)

	if err := statedb.FinalizeTx(&chain.Rules{}, w); err != nil {
		return err
	}

	broot, err := domains.ComputeCommitment(ctx, true, blockNumber, "import")
	root := libcommon.BytesToHash(broot)
	if err != nil {
		log.Info("root calculation failed")
		return err
	}
	log.Info("newly calculated root", "root", root.Hex())

	if err = domains.Flush(ctx, tx); err != nil {
		return err
	}

	startTime := time.Now()
	if err := statedb.CommitBlock(&chain.Rules{}, w); err != nil {
		return fmt.Errorf("cannot write state: %w", err)
	}
	log.Info("commit block", "elapsed", time.Duration(time.Since(startTime)))

	startTime = time.Now()
	if err := w.WriteChangeSets(); err != nil {
		return fmt.Errorf("cannot write change sets: %w", err)
	}
	log.Info("write change sets", "elapsed", time.Duration(time.Since(startTime)))

	startTime = time.Now()
	if err := w.WriteHistory(); err != nil {
		return fmt.Errorf("cannot write history: %w", err)
	}
	log.Info("write history", "elapsed", time.Duration(time.Since(startTime)))

	blockHash, err := rawdb.ReadCanonicalHash(tx, blockNumber)
	if err != nil {
		return err
	}

	header := rawdb.ReadHeader(tx, blockHash, blockNumber)
	log.Info("state root stored at blockheader", "root", header.Root.Hex())

	if bytes.Equal(root.Bytes(), header.Root.Bytes()) {
		log.Info("state root consistent with block header's state root")
	} else {
		return fmt.Errorf("state trie root mismatch, expected %x, got %x", header.Root, root)
	}

	// first bedrock block does not have tx, so no tx receipt
	//if err != rawdb.WriteReceipts(tx, blockNumber, nil) {
	//	return err
	//}
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func SanityCheckStorageTrie(ctx context.Context, dirs datadir.Dirs, ethereum *eth.Ethereum, fn string, blockNumber uint64, stream bool, logger log.Logger) error {
	logger.Info("Sanity check storage trie", "file", fn, "stream", stream)
	logger.Info("Sanity check storage trie for block number", "blockNumber", blockNumber)
	fh, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer fh.Close()

	db := ethereum.ChainDB().(kv.TemporalRwDB)

	//agg, err := libstate.NewAggregator2(ctx, dirs, config3.DefaultStepSize, db, log.New())
	//tdb, err := temporal.New(db, agg)
	//if err != nil {
	//	return err
	//}
	tx, err := db.BeginTemporalRw(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	domains, err := libstate.NewSharedDomains(tx, logger)
	if err != nil {
		return err
	}
	domains.SetBlockNum(blockNumber)

	idx := 0
	quit := StatusReporter("Sanity check storage trie", &idx)

	if stream {
		reader := bufio.NewReader(fh)
		delimiter := byte('\n')
		var importStreamHeader ImportStreamHeader
		headerConsumed := false
		for {
			line, err := reader.ReadBytes(delimiter)
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return err
			}
			if !headerConsumed {
				if err := json.Unmarshal(line, &importStreamHeader); err != nil {
					return err
				}
				log.Info("state root from header", "root", importStreamHeader.Root.Hex())
				headerConsumed = true
				continue
			}
			var account ImportAccount
			if err := json.Unmarshal(line, &account); err != nil {
				return err
			}
			idx += 1
			if err := sanityCheckStorageTrie(tx, &account, blockNumber); err != nil {
				return err
			}
		}
	} else {
		decoder := json.NewDecoder(fh)
		ia := make(ImportAlloc)
		if err := decoder.Decode(&ia); err != nil {
			return err
		}
		for address, account := range ia {
			idx += 1
			account.Address = address
			//nolint:all
			if err := sanityCheckStorageTrie(tx, &account, blockNumber); err != nil {
				return err
			}
		}
	}
	close(quit)

	return nil
}

func sanityCheckStorageTrie(ttx kv.TemporalRwTx, account *ImportAccount, blockNumber uint64) error {
	address := account.Address
	newStorageTrie := trie.New(emptyHash)

	txNum, err := rawdbv3.TxNums.Min(ttx, blockNumber+1)
	if err != nil {
		return err
	}

	nextAcc, _ := kv.NextSubtree(address[:])
	r, err := ttx.RangeAsOf(kv.StorageDomain, address[:], nextAcc, txNum, order.Asc, kv.Unlim)
	if err != nil {
		return fmt.Errorf("walking over storage for %x: %w", address, err)
	}
	defer r.Close()
	for r.HasNext() {
		k, vs, err := r.Next()
		if err != nil {
			return fmt.Errorf("walking over storage for %x: %w", address, err)
		}
		if len(vs) == 0 {
			continue // Skip deleted entries
		}
		loc := k[20:]
		h, _ := libcommon.HashData(loc)
		newStorageTrie.Update(h.Bytes(), libcommon.CopyBytes(vs))
	}
	r.Close()

	newStorageTrieRoot := newStorageTrie.Root()
	hexStorageRoot := strings.TrimPrefix(account.Root, "0x")
	storageRoot, err := hex.DecodeString(hexStorageRoot)

	if err != nil {
		return errors.New("storage root hexdecode failure")
	}
	if !bytes.Equal(newStorageTrieRoot, storageRoot) {
		return fmt.Errorf("storage root mismatch, expected %x, got %x", newStorageTrieRoot, storageRoot)
	}

	return nil
}
