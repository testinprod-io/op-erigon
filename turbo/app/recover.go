package app

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/cmd/utils"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth"
	"github.com/ledgerwatch/erigon/eth/stagedsync"
	"github.com/ledgerwatch/erigon/ethdb/prune"
	"github.com/ledgerwatch/erigon/params"
	"github.com/ledgerwatch/erigon/params/networkname"
	turboNode "github.com/ledgerwatch/erigon/turbo/node"
	"github.com/ledgerwatch/log/v3"
	"github.com/urfave/cli/v2"
)

const (
	recoverBatchSize = 2500
)

// chainID derived when v = 0
var overflowedChainID = uint256.NewInt(0x7fffffffffffffee)

var recoverSendersCommand = cli.Command{
	Action:    MigrateFlags(recoverSenders),
	Name:      "recover-senders",
	Usage:     "Recover senders",
	ArgsUsage: "<blockNumFirst> <blockNumLast>",
	Flags: []cli.Flag{
		&utils.DataDirFlag,
		&utils.ChainFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The recover command recovers Senders table.`,
}

var recoverLogIndexCommand = cli.Command{
	Action:    MigrateFlags(recoverLogIndex),
	Name:      "recover-log-index",
	Usage:     "Recover log index",
	ArgsUsage: "<blockNumFirst> <blockNumLast>",
	Flags: []cli.Flag{
		&utils.DataDirFlag,
		&utils.ChainFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The recover command recovers LogTopicIndex table and LogAddressIndex table.`,
}

var recoverRegenesisCommand = cli.Command{
	Action: MigrateFlags(recoverRegenesis),
	Name:   "recover-regenesis",
	Usage:  "Recovery for regenesis",
	Flags: []cli.Flag{
		&utils.DataDirFlag,
		&utils.ChainFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The recover command corrects chain config and genesis for bedrock regenesis.`,
}

var recoverIntermediateHashCommand = cli.Command{
	Action:    MigrateFlags(recoverIntermediateHash),
	Name:      "recover-intermediatehash",
	Usage:     "Recovery for intermediatehash",
	ArgsUsage: "<filename>",
	Flags: []cli.Flag{
		&utils.DataDirFlag,
		&utils.ChainFlag,
	},
	Category: "BLOCKCHAIN COMMANDS",
	Description: `
The recover command recovers TrieStorage table and TrieAccount table`,
}

func recoverSenders(ctx *cli.Context) error {
	if ctx.NArg() < 2 {
		utils.Fatalf("This command requires an argument.")
	}

	first, ferr := strconv.ParseInt(ctx.Args().Get(0), 10, 64)
	last, lerr := strconv.ParseInt(ctx.Args().Get(1), 10, 64)
	if ferr != nil || lerr != nil {
		utils.Fatalf("Recover error in parsing parameters: block number not an integer\n")
	}
	if first < 0 || last < 0 {
		utils.Fatalf("Recover error: block number must be greater than 0\n")
	}

	nodeCfg := turboNode.NewNodConfigUrfave(ctx)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg)

	stack := makeConfigNode(nodeCfg)
	defer stack.Close()

	ethereum, err := eth.New(stack, ethCfg)
	if err != nil {
		return err
	}
	err = ethereum.Init(stack, ethCfg)
	if err != nil {
		return err
	}

	if err := RecoverSenders(ethereum, uint64(first), uint64(last)); err != nil {
		return err
	}

	return nil
}

func RecoverSenders(ethereum *eth.Ethereum, first, last uint64) error {
	// Watch for Ctrl-C while the import is running.
	// If a signal is received, the import will stop at the next batch.
	interrupt := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	defer close(interrupt)
	go func() {
		if _, ok := <-interrupt; ok {
			log.Info("Interrupted during recovery, stopping at next batch")
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

	signer := types.LatestSignerForChainID(ethereum.ChainConfig().ChainID)
	log.Info("Recovering Senders")
	n := first
	startTime, reportedTime := time.Now(), time.Now()
	for batch := 0; ; batch++ {
		if checkInterrupt() {
			return fmt.Errorf("interrupted")
		}
		start := n
		end := n + recoverBatchSize - 1
		if end > last {
			end = last
		}
		if err := RecoverSendersBatch(ethereum, signer, start, end); err != nil {
			return err
		}
		if end == last {
			break
		}
		n += recoverBatchSize

		if time.Since(reportedTime) >= 8*time.Second {
			log.Info("Recovering senders", "recovered", start, "elapsed", time.Duration(time.Since(startTime)))
			reportedTime = time.Now()
		}
	}
	return nil
}

func RecoverSendersBatch(ethereum *eth.Ethereum, signer *types.Signer, start, end uint64) error {
	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for nr := start; nr <= end; nr++ {
		block, err := rawdb.ReadBlockByNumber(tx, nr)
		if err != nil {
			return err
		}
		txs := block.Transactions()
		if len(txs) == 0 {
			log.Info("Recovery: transactions not found", "blockNum", nr)
			continue
		}

		var sendersRecovered []common.Address
		for _, txn := range txs {
			from, err := txn.Sender(*signer)
			if err != nil {
				if txn.GetChainID().Eq(overflowedChainID) {
					from = common.Address{}
				} else {
					return err
				}
			}
			sendersRecovered = append(sendersRecovered, from)
		}
		// sanity check senders included in transactions
		sendersFromTxs := block.Body().SendersFromTxs()
		for i := 0; i < len(txs); i++ {
			if sendersFromTxs[i] != sendersRecovered[i] {
				log.Error("Recovery mismatch", "blockNum", nr)
				return fmt.Errorf("recovery mismatch")
			}
		}

		if err := rawdb.WriteSenders(tx, block.Hash(), nr, sendersRecovered); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func recoverLogIndex(ctx *cli.Context) error {
	if ctx.NArg() < 2 {
		utils.Fatalf("This command requires an argument.")
	}

	first, ferr := strconv.ParseInt(ctx.Args().Get(0), 10, 64)
	last, lerr := strconv.ParseInt(ctx.Args().Get(1), 10, 64)
	if ferr != nil || lerr != nil {
		utils.Fatalf("Recover error in parsing parameters: block number not an integer\n")
	}
	if first < 0 || last < 0 {
		utils.Fatalf("Recover error: block number must be greater than 0\n")
	}

	nodeCfg := turboNode.NewNodConfigUrfave(ctx)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg)

	stack := makeConfigNode(nodeCfg)
	defer stack.Close()

	ethereum, err := eth.New(stack, ethCfg)
	if err != nil {
		return err
	}
	err = ethereum.Init(stack, ethCfg)
	if err != nil {
		return err
	}

	if err := RecoverLogIndex(ethereum, uint64(first), uint64(last)); err != nil {
		return err
	}

	return nil
}

func RecoverLogIndex(ethereum *eth.Ethereum, first, last uint64) error {
	// Watch for Ctrl-C while the import is running.
	// If a signal is received, the import will stop at the next batch.
	interrupt := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	defer close(interrupt)
	go func() {
		if _, ok := <-interrupt; ok {
			log.Info("Interrupted during recovery, stopping at next batch")
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

	log.Info("Recovering Log Index")
	n := first
	startTime, reportedTime := time.Now(), time.Now()
	for batch := 0; ; batch++ {
		if checkInterrupt() {
			return fmt.Errorf("interrupted")
		}
		start := n
		end := n + recoverBatchSize - 1
		if end > last {
			end = last
		}
		if err := RecoverLogIndexBatch(ethereum, start, end); err != nil {
			return err
		}
		if end == last {
			break
		}
		n += recoverBatchSize

		if time.Since(reportedTime) >= 8*time.Second {
			log.Info("Recovering Log Index", "recovered", start, "elapsed", time.Duration(time.Since(startTime)))
			reportedTime = time.Now()
		}
	}
	return nil
}

func RecoverLogIndexBatch(ethereum *eth.Ethereum, start, end uint64) error {
	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// receipt sanity check before log recovery
	for nr := start; nr <= end; nr++ {
		block, err := rawdb.ReadBlockByNumber(tx, nr)
		if err != nil {
			return err
		}
		receipts := rawdb.ReadRawReceipts(tx, nr)
		receipts.ProcessFieldsForValidation(block)
		receiptHash := types.DeriveSha(receipts)
		if receiptHash != block.ReceiptHash() {
			log.Error("receipt root mismatch", "blockNum", nr, "receiptHash", receiptHash.Hex(), "newReceiptHash", block.ReceiptHash().Hex())
			return fmt.Errorf("receipt trie root mismatch. blockNum = %d", nr)
		}
	}

	pm := prune.DefaultMode
	dirs := ethereum.Dirs()
	logPrefix := ""
	ctx := context.Background()

	cfg := stagedsync.StageLogIndexCfg(db, pm, dirs.Tmp)
	if err = stagedsync.PromoteLogIndex(logPrefix, tx, start, end, cfg, ctx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func recoverRegenesis(ctx *cli.Context) error {
	nodeCfg := turboNode.NewNodConfigUrfave(ctx)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg)

	stack := makeConfigNode(nodeCfg)
	defer stack.Close()

	ethereum, err := eth.New(stack, ethCfg)
	if err != nil {
		return err
	}

	if err := RecoverRegenesis(ethereum); err != nil {
		return err
	}

	return nil
}

func RecoverRegenesis(ethereum *eth.Ethereum) error {
	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	genesisHeader := rawdb.ReadHeaderByNumber(tx, 0)
	genesisHash := genesisHeader.Hash()
	genesisBody, _, _ := rawdb.ReadBody(tx, genesisHash, 0)
	if genesisBody == nil {
		return errors.New("genesis block body nil")
	}
	config, err := rawdb.ReadChainConfig(tx, genesisHash)
	if err != nil {
		return err
	}
	if err := rawdb.DeleteChainConfig(tx, genesisHash); err != nil {
		return err
	}

	var targetGenesisHash libcommon.Hash
	switch config.ChainName {
	case networkname.OptimismGoerliChainName:
		genesisHeader.Root = params.OptimismGoerliStateRoot
		targetGenesisHash = params.OptimismGoerliGenesisHash
	case networkname.OptimismMainnetChainName:
		genesisHeader.Root = params.OptimismMainnetStateRoot
		targetGenesisHash = params.OptimismMainnetGenesisHash
	default:
		return fmt.Errorf("%s chain not supported for regenesis", config.ChainName)
	}
	newGenesisHash := genesisHeader.Hash()
	if newGenesisHash != targetGenesisHash {
		return fmt.Errorf("regenesis header hash mismatch")
	}

	// body did not change
	if err := rawdb.WriteBody(tx, newGenesisHash, 0, genesisBody); err != nil {
		return err
	}
	// update new header and hash->number mapping
	rawdb.WriteHeader(tx, genesisHeader)
	if err := rawdb.WriteCanonicalHash(tx, newGenesisHash, 0); err != nil {
		return err
	}
	if err := rawdb.WriteChainConfig(tx, newGenesisHash, config); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Info("Successfully wrote regenesis state", "hash", newGenesisHash.Hex(), "root", genesisHeader.Root.Hex())

	return nil
}

func recoverIntermediateHash(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		utils.Fatalf("This command requires an argument.")
	}

	nodeCfg := turboNode.NewNodConfigUrfave(ctx)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg)

	stack := makeConfigNode(nodeCfg)
	defer stack.Close()

	ethereum, err := eth.New(stack, ethCfg)
	if err != nil {
		return err
	}
	fn := ctx.Args().First()

	if err := RecoverIntermediateHash(ethereum, fn); err != nil {
		return err
	}

	return nil
}

func RecoverIntermediateHash(ethereum *eth.Ethereum, fn string) error {
	log.Info("Recovering Intermediate Hash")
	db := ethereum.ChainDB()
	tx, err := db.BeginRw(ethereum.SentryCtx())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	fh, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer fh.Close()

	var (
		method          string
		bucketIndicator string
		khex            string
		vhex            string
	)

	idx := 0
	quit := StatusReporter("Recover Intermediate Hash", &idx)

	reader := bufio.NewReader(fh)
	delimiter := byte('\n')

	trieAccountCursor, err := tx.RwCursor(kv.TrieOfAccounts)
	if err != nil {
		return err
	}
	trieStorageCursor, err := tx.RwCursor(kv.TrieOfStorage)
	if err != nil {
		return err
	}

	cursorFunc := func(b string) (kv.RwCursor, error) {
		switch b {
		case "t":
			// TrieAccount's last byte
			return trieAccountCursor, nil
		case "e":
			// TrieStorage's last byte
			return trieStorageCursor, nil
		default:
			return nil, fmt.Errorf("invalid cursor indicator: %s", b)
		}
	}

	for {
		idx += 1
		line, err := reader.ReadBytes(delimiter)
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return err
		}
		_, err = fmt.Sscanf(string(line), "%s %s %s %s", &method, &bucketIndicator, &khex, &vhex)
		if err != nil {
			return err
		}
		// log.Info(method)
		// log.Info(bucketIndicator)
		// log.Info(khex)
		// log.Info(vhex)

		k, err := hex.DecodeString(khex)
		if err != nil {
			return err
		}
		v, err := hex.DecodeString(vhex)
		if err != nil {
			return err
		}
		cursor, err := cursorFunc(bucketIndicator)
		if err != nil {
			return err
		}
		switch method {
		case "a":
			if err := cursor.Append(k, v); err != nil {
				return err
			}
		case "ad":
			if err := cursor.(kv.RwCursorDupSort).AppendDup(k, v); err != nil {
				return err
			}
		case "d":
			if err := cursor.Delete(k); err != nil {
				return err
			}
		case "p":
			if err := cursor.Put(k, v); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid method %s", method)
		}
	}
	close(quit)

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
