package app

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/cmd/utils"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth"
	turboNode "github.com/ledgerwatch/erigon/turbo/node"
	"github.com/ledgerwatch/log/v3"
	"github.com/urfave/cli/v2"
)

const (
	recoverBatchSize = 2500
)

var nullAddress = common.HexToAddress("0x0000000000000000000000000000000000000000")

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

func recoverSenders(ctx *cli.Context) error {
	if ctx.NArg() != 2 {
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

	logger := log.New(ctx)

	nodeCfg := turboNode.NewNodConfigUrfave(ctx)
	ethCfg := turboNode.NewEthConfigUrfave(ctx, nodeCfg)

	stack := makeConfigNode(nodeCfg)
	defer stack.Close()

	ethereum, err := eth.New(stack, ethCfg, logger)
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
					from = nullAddress
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
