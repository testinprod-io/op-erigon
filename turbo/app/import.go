package app

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/cmd/utils"
	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/core"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/eth"
	stageSync "github.com/ledgerwatch/erigon/eth/stagedsync/stages"
	"github.com/ledgerwatch/erigon/params"
	"github.com/ledgerwatch/erigon/rlp"
	turboNode "github.com/ledgerwatch/erigon/turbo/node"
	"github.com/ledgerwatch/erigon/turbo/stages"
	"github.com/ledgerwatch/erigon/turbo/trie"

	"github.com/ledgerwatch/log/v3"
	"github.com/urfave/cli"
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
		utils.DataDirFlag,
		utils.ChainFlag,
		utils.ImportExecutionFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The import command imports blocks from an RLP-encoded form. The form can be one file
with several RLP-encoded blocks, or several files can be used.

If only one file is used, import error will result in failure. If several files are used,
processing will proceed even if an individual RLP-file import failure occurs.`,
}

var importReceiptCommand = cli.Command{
	Action:    MigrateFlags(importReceipts),
	Name:      "import-receipts",
	Usage:     "Import a receipts file",
	ArgsUsage: "<filename> ",
	Flags: []cli.Flag{
		utils.DataDirFlag,
		utils.ChainFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The import command imports receipts from an RLP-encoded form.`,
}

var importStateCommand = cli.Command{
	Action:    MigrateFlags(importState),
	Name:      "import-state",
	Usage:     "Import a state file",
	ArgsUsage: "<filename> <blockNum>",
	Flags: []cli.Flag{
		utils.DataDirFlag,
		utils.ChainFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The import command imports state from a json form`,
}

var importDifficultyCommand = cli.Command{
	Action:    MigrateFlags(importDifficulty),
	Name:      "import-difficulty",
	Usage:     "Import a difficulty file",
	ArgsUsage: "<filename> ",
	Flags: []cli.Flag{
		utils.DataDirFlag,
		utils.ChainFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The import command imports difficulty from an RLP-encoded form.`,
}

func importReceipts(ctx *cli.Context) error {
	if len(ctx.Args()) < 1 {
		utils.Fatalf("This command requires an argument.")
	}

	logger := log.New(ctx)

	nodeCfg := turboNode.NewNodConfigUrfave(ctx)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg)

	stack := makeConfigNode(nodeCfg)
	defer stack.Close()

	ethereum, err := eth.New(stack, ethCfg, logger)
	if err != nil {
		return err
	}

	if err := ImportReceipts(ethereum, ethereum.ChainDB(), ctx.Args().First()); err != nil {
		return err
	}

	return nil
}

func importState(ctx *cli.Context) error {
	if ctx.NArg() < 2 {
		utils.Fatalf("This command requires an argument.")
	}

	logger := log.New(ctx)

	nodeCfg := turboNode.NewNodConfigUrfave(ctx)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg)

	stack := makeConfigNode(nodeCfg)
	defer stack.Close()

	ethereum, err := eth.New(stack, ethCfg, logger)
	if err != nil {
		return err
	}
	fn := ctx.Args().First()
	blockNum, err := strconv.ParseInt(ctx.Args().Get(1), 10, 64)
	if err != nil {
		utils.Fatalf("Export error in parsing parameters: block number not an integer\n")
	}

	if err := ImportState(ethereum, fn, uint64(blockNum)); err != nil {
		return err
	}

	if err := SanityCheckStorageTrie(ethereum, fn, uint64(blockNum)); err != nil {
		return err
	}

	return nil
}

func importDifficulty(ctx *cli.Context) error {
	if len(ctx.Args()) < 1 {
		utils.Fatalf("This command requires an argument.")
	}

	logger := log.New(ctx)

	nodeCfg := turboNode.NewNodConfigUrfave(ctx)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg)

	stack := makeConfigNode(nodeCfg)
	defer stack.Close()

	ethereum, err := eth.New(stack, ethCfg, logger)
	if err != nil {
		return err
	}

	if err := ImportDifficulty(ethereum, ethereum.ChainDB(), ctx.Args().First()); err != nil {
		return err
	}

	return nil
}

// modified from l2geth's core/state/dump.go
type ImportAccount struct {
	Balance  string                 `json:"balance"`
	Nonce    uint64                 `json:"nonce"`
	Root     string                 `json:"root"`
	CodeHash string                 `json:"codeHash"`
	Code     string                 `json:"code,omitempty"`
	Storage  map[common.Hash]string `json:"storage,omitempty"`
}

type ImportAlloc map[common.Address]ImportAccount

func (ia *ImportAlloc) UnmarshalJson(data []byte) error {
	m := make(ImportAlloc)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*ia = make(ImportAlloc)
	for addr, a := range m {
		(*ia)[common.Address(addr)] = a
	}
	return nil
}

func SanityCheckStorageTrie(ethereum *eth.Ethereum, fn string, blockNumber uint64) error {
	log.Info("Sanity check storage trie", "file", fn)
	log.Info("Sanity check storage trie for block number", "blockNumber", blockNumber)
	fh, err := os.Open(fn)
	if err != nil {
		return err
	}

	// TODO: make as json stream
	decoder := json.NewDecoder(fh)
	ia := make(ImportAlloc)

	if err := decoder.Decode(&ia); err != nil {
		return err
	}

	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	r := state.NewDbStateReader(tx)
	statedb := state.New(r)

	idx := 0
	for address, account := range ia {
		idx += 1
		// fmt.Println(idx, address.Hex())

		incarnation := statedb.GetIncarnation(address)
		newStorageTrie := trie.New(common.Hash{})
		if err := state.WalkAsOfStorage(tx,
			address,
			incarnation,
			common.Hash{}, /* startLocation */
			blockNumber + 1, /* do not know why adding one up, but it just works */
			func(_, loc, vs []byte) (bool, error) {
				h, _ := common.HashData(loc)
				newStorageTrie.Update(h.Bytes(), common.CopyBytes(vs))
				return true, nil
			}); err != nil {
			return fmt.Errorf("walking over storage for %x: %w", address, err)
		}
		newStorageTrieRoot := newStorageTrie.Root()
		hexStorageRoot := account.Root
		storageRoot, err := hex.DecodeString(hexStorageRoot)
		
		if err != nil {
		 	return errors.New("storage root hexdecode failure")
		}
		if !bytes.Equal(newStorageTrieRoot, storageRoot) {
			return fmt.Errorf("storage root mismatch, expected %x, got %x", newStorageTrieRoot, storageRoot)
		}	
	}
	
	return nil
}

func ImportState(ethereum *eth.Ethereum, fn string, blockNumber uint64) error {
	log.Info("Importing state", "file", fn)
	log.Info("Importing state for block number", "blockNumber", blockNumber)
	fh, err := os.Open(fn)
	if err != nil {
		return err
	}

	// TODO: make as json stream
	decoder := json.NewDecoder(fh)
	ia := make(ImportAlloc)

	if err := decoder.Decode(&ia); err != nil {
		return err
	}

	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	r, w := state.NewDbStateReader(tx), state.NewDbStateWriter(tx, blockNumber)
	//stateReader := state.NewPlainStateReader(tx)
	statedb := state.New(r)

	idx := 0
	for address, account := range ia {
		idx += 1
		fmt.Println(idx, address.Hex())
		balanceBigInt, ok := new(big.Int).SetString(account.Balance, 10)
		if !ok {
			return errors.New("balance bigint conversion failure")
		}
		balance, overflow := uint256.FromBig(balanceBigInt)
		if overflow {
			return errors.New("balance overflow")
		}
		statedb.AddBalance(address, balance)
		hexCode := account.Code
		code, err := hex.DecodeString(hexCode)
		if err != nil {
			return fmt.Errorf("code hexdecode failure, %s", hexCode)
		}
		hexCodeHash := account.CodeHash
		codeHash, err := hex.DecodeString(hexCodeHash)
		if err != nil {
			return fmt.Errorf("codehash hexdecode failure, %s", hexCodeHash)
		}
		tempCodeHash := crypto.Keccak256(code)
		if !bytes.Equal(tempCodeHash, codeHash) {
			return fmt.Errorf("codehash mismatch, expected %x, got %x", codeHash, tempCodeHash)
		}
		statedb.SetCode(address, code)
		statedb.SetNonce(address, account.Nonce)
		for key, hexValue := range account.Storage {
			key := key
			value, err := hex.DecodeString(hexValue)
			if err != nil {
				return errors.New("value hexdecode failure")
			}
			val := uint256.NewInt(0).SetBytes(value)
			statedb.SetState(address, &key, *val)
		}

		if len(account.Code) > 0 || len(account.Storage) > 0 {
			statedb.SetIncarnation(address, state.FirstContractIncarnation)
		}
	}

	if err := statedb.FinalizeTx(&params.Rules{}, w); err != nil {
		return err
	}

	root, err := trie.CalcRoot("genesis", tx)
	if err != nil {
		log.Info("root calculation failed")
	}
	log.Info("newly calculated root", "root", root.Hex())

	blockWriter := state.NewPlainStateWriter(tx, tx, blockNumber)
	if err := statedb.CommitBlock(&params.Rules{}, blockWriter); err != nil {
		return fmt.Errorf("cannot write state: %w", err)
	}
	if err := blockWriter.WriteChangeSets(); err != nil {
		return fmt.Errorf("cannot write change sets: %w", err)
	}
	if err := blockWriter.WriteHistory(); err != nil {
		return fmt.Errorf("cannot write history: %w", err)
	}

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

	// 4061224 block does not have tx, so no tx receipt
	if err != rawdb.WriteReceipts(tx, blockNumber, nil) {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func ImportReceipts(ethereum *eth.Ethereum, chainDB kv.RwDB, fn string) error {
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

	log.Info("Importing receipts", "file", fn)

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
	receiptsList := make(types.ReceiptsList, importBatchSize)
	n := 0
	for batch := 0; ; batch++ {
		// Load a batch of RLP blocks.
		if checkInterrupt() {
			return fmt.Errorf("interrupted")
		}
		i := 0
		for ; i < importBatchSize; i++ {
			var b types.HackReceipts
			if err := stream.Decode(&b); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return fmt.Errorf("at block %d: %v", n, err)
			}

			// hack assuming that default rlp will work
			var b2 types.Receipts
			for _, hackReceipt := range b {
				feeScalar := new(big.Float)
				feeScalar.SetString(hackReceipt.FeeScalar)
				receipt := types.Receipt{
					hackReceipt.Type,
					hackReceipt.PostState,
					hackReceipt.Status,
					hackReceipt.CumulativeGasUsed,
					hackReceipt.Bloom,
					hackReceipt.Logs,
					hackReceipt.TxHash,
					hackReceipt.ContractAddress,
					hackReceipt.GasUsed,
					hackReceipt.BlockHash,
					hackReceipt.BlockNumber,
					hackReceipt.TransactionIndex,
					hackReceipt.L1GasPrice,
					hackReceipt.L1GasUsed,
					hackReceipt.L1Fee,
					feeScalar,
				}
				b2 = append(b2, &receipt)
			}

			// log.Info("DEBUG print")
			// // for debug purposes
			// if err := json.NewEncoder(os.Stdout).Encode(b); err != nil {
			// 	log.Info("receipts print failed")
			// }
			// log.Info("DEBUG print done")

			receiptsList[i] = &b2
			n++
		}
		if i == 0 {
			break
		}
		// Import the batch.
		if checkInterrupt() {
			return fmt.Errorf("interrupted")
		}

		missing := missingReceiptsList(chainDB, receiptsList[:i])
		if len(missing) == 0 {
			log.Info("Skipping batch as all receipts present", "batch", batch)
			continue
		}

		if err := InsertReceipts(ethereum, missing); err != nil {
			return err
		}
	}

	return nil
}

func ImportDifficulty(ethereum *eth.Ethereum, chainDB kv.RwDB, fn string) error {
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

	log.Info("Importing difficulty", "file", fn)

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
		if err := InsertDifficulty(ethereum, difficultyList, uint64(startNum)); err != nil {
			return err
		}
		startNum += len(difficultyList)
	}
	return nil
}

func importChain(ctx *cli.Context) error {
	if len(ctx.Args()) < 1 {
		utils.Fatalf("This command requires an argument.")
	}

	logger := log.New(ctx)

	nodeCfg := turboNode.NewNodConfigUrfave(ctx)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg)

	stack := makeConfigNode(nodeCfg)
	defer stack.Close()

	ethereum, err := eth.New(stack, ethCfg, logger)
	if err != nil {
		return err
	}

	if err := ImportChain(ethereum, ethereum.ChainDB(), ctx.Args().First(), ethCfg.ImportExecution); err != nil {
		return err
	}

	return nil
}

func ImportChain(ethereum *eth.Ethereum, chainDB kv.RwDB, fn string, execute bool) error {
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

	log.Info("Importing blockchain", "file", fn)
	log.Info("Blockchain execution", "execute", execute)

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
			return fmt.Errorf("interrupted")
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
			return fmt.Errorf("interrupted")
		}

		missing := missingBlocks(chainDB, blocks[:i])
		if len(missing) == 0 {
			log.Info("Skipping batch as all blocks present", "batch", batch, "first", blocks[0].Hash(), "last", blocks[i-1].Hash())
			continue
		}

		// RLP decoding worked, try to insert into chain:
		missingChain := &core.ChainPack{
			Blocks:   missing,
			TopBlock: missing[len(missing)-1],
		}

		if execute {
			if err := InsertChain(ethereum, missingChain); err != nil {
				return err
			}
		} else {
			if err := InsertChainWithoutExecution(ethereum, missingChain); err != nil {
				return err
			}
		}
	}
	return nil
}

func ChainHasReceipt(chainDB kv.RwDB, hash common.Hash, number uint64) bool {
	var chainHasReceipt bool

	chainDB.View(context.Background(), func(tx kv.Tx) (err error) {
		chainHasReceipt = rawdb.HasReceipts(tx, hash, number)
		return nil
	})

	return chainHasReceipt
}

func ChainHasBlock(chainDB kv.RwDB, block *types.Block) bool {
	var chainHasBlock bool

	chainDB.View(context.Background(), func(tx kv.Tx) (err error) {
		chainHasBlock = rawdb.HasBlock(tx, block.Hash(), block.NumberU64())
		return nil
	})

	return chainHasBlock
}

func missingBlocks(chainDB kv.RwDB, blocks []*types.Block) []*types.Block {
	var headBlock *types.Block
	chainDB.View(context.Background(), func(tx kv.Tx) (err error) {
		hash := rawdb.ReadHeadHeaderHash(tx)
		number := rawdb.ReadHeaderNumber(tx, hash)
		headBlock = rawdb.ReadBlock(tx, hash, *number)
		return nil
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

func missingReceiptsList(chainDB kv.RwDB, receiptsList []*types.Receipts) []*types.Receipts {
	var headBlock *types.Block
	chainDB.View(context.Background(), func(tx kv.Tx) (err error) {
		hash := rawdb.ReadHeadHeaderHash(tx)
		number := rawdb.ReadHeaderNumber(tx, hash)
		headBlock = rawdb.ReadBlock(tx, hash, *number)
		return nil
	})

	for i, receipts := range receiptsList {
		if receipts.Len() == 0 {
			continue
		}
		firstReceipt := []*types.Receipt(*receipts)[0]
		blockHash := firstReceipt.BlockHash
		blockNumber := firstReceipt.BlockNumber.Uint64()
		if headBlock.NumberU64() >= blockNumber {
			if !ChainHasReceipt(chainDB, blockHash, blockNumber) {
				return receiptsList[i:]
			}
			continue
		}

		if !ChainHasReceipt(chainDB, blockHash, blockNumber) {
			return receiptsList[i:]
		}
	}
	return nil
}

func InsertChain(ethereum *eth.Ethereum, chain *core.ChainPack) error {
	sentryControlServer := ethereum.SentryControlServer()
	initialCycle := false
	highestSeenHeader := chain.TopBlock.NumberU64()

	for _, b := range chain.Blocks {
		sentryControlServer.Hd.AddMinedHeader(b.Header())
		sentryControlServer.Bd.AddMinedBlock(b)
	}

	sentryControlServer.Hd.MarkAllVerified()

	_, err := stages.StageLoopStep(ethereum.SentryCtx(), ethereum.ChainConfig(), ethereum.ChainDB(), ethereum.StagedSync(), highestSeenHeader, ethereum.Notifications(), initialCycle, sentryControlServer.UpdateHead, nil)
	if err != nil {
		return err
	}

	return nil
}

func InsertChainWithoutExecution(ethereum *eth.Ethereum, chain *core.ChainPack) error {
	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// log.Info("test", "length", chain.Length())
	// log.Info("test", "length", len(chain.Blocks))
	// log.Info("test", "length", len(chain.Receipts))

	for i := 0; i < chain.Length(); i++ {
		block := chain.Blocks[i]
		if i == 0 {
			log.Info("Write", "blockNum", block.Number().String())
		}
		WriteBlockWithoutExecution(ethereum, tx, block)
	}
	tx.Commit()

	return nil
}

// may rename to without execution
func WriteBlockWithoutExecution(ethereum *eth.Ethereum, tx kv.RwTx, block *types.Block) error {
	if err := rawdb.WriteTd(tx, block.Hash(), block.NumberU64(), block.Difficulty()); err != nil {
		return err
	}
	if err := rawdb.WriteBlock(tx, block); err != nil {
		return err
	}
	if err := rawdb.WriteHeaderNumber(tx, block.Hash(), block.NumberU64()); err != nil {
		return err
	}
	txNum := uint64(block.Transactions().Len())
	if err := rawdb.TxNums.Append(tx, block.NumberU64(), txNum); err != nil {
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

	rawdb.WriteTxLookupEntries(tx, block)

	// mark every stage as done
	for _, stage := range stageSync.AllStages {
		if err := stageSync.SaveStageProgress(tx, stage, block.NumberU64()); err != nil {
			return err
		}
	}

	txHash := types.DeriveSha(block.Transactions())
	if txHash != block.TxHash() {
		return errors.New("tx trie root mismatch. aborting")
	}

	return nil
}

func InsertReceipts(ethereum *eth.Ethereum, receiptsList []*types.Receipts) error {
	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, receipts := range receiptsList {
		if receipts.Len() == 0 {
			continue
		}
		firstReceipt := []*types.Receipt(*receipts)[0]
		blockNumber := firstReceipt.BlockNumber.Uint64()
		// log.Info("Write receipt", "block", blockNumber)
		block := rawdb.ReadBlock(tx, firstReceipt.BlockHash, blockNumber)

		var receiptsVal types.Receipts = *receipts
		err = rawdb.WriteReceipts(tx, blockNumber, receiptsVal)
		if err != nil {
			log.Error(err.Error())
			break
		}

		receiptHash := types.DeriveSha(receipts)
		if receiptHash != block.ReceiptHash() {
			return errors.New("receipt trie root mismatch. aborting")
		}

	}
	tx.Commit()

	return nil
}

func InsertDifficulty(ethereum *eth.Ethereum, difficultyList []*big.Int, number uint64) error {
	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, difficulty := range difficultyList {
		blockNum := number + uint64(i)
		block, err := rawdb.ReadBlockByNumber(tx, blockNum)
		if err != nil {
			return errors.New("block not readable")
		}
		err = rawdb.WriteTd(tx, block.Hash(), blockNum, difficulty)
		if err != nil {
			log.Error(err.Error())
			break
		}
	}
	tx.Commit()

	return nil
}
