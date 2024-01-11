package stagedsync

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ledgerwatch/erigon-lib/common/length"
	"github.com/ledgerwatch/erigon-lib/etl"
	"github.com/ledgerwatch/erigon-lib/kv/temporal/historyv2"
	"github.com/ledgerwatch/erigon/common/changeset"
	"github.com/ledgerwatch/erigon/common/dbutils"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/log/v3"
	"golang.org/x/net/context"

	"github.com/ledgerwatch/erigon-lib/chain"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	types2 "github.com/ledgerwatch/erigon-lib/types"

	"github.com/ledgerwatch/erigon/consensus"
	"github.com/ledgerwatch/erigon/consensus/misc"
	"github.com/ledgerwatch/erigon/core"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/core/systemcontracts"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/core/types/accounts"
	"github.com/ledgerwatch/erigon/core/vm"
	"github.com/ledgerwatch/erigon/eth/stagedsync/stages"
	"github.com/ledgerwatch/erigon/params"
	"github.com/ledgerwatch/erigon/turbo/services"
)

type MiningExecCfg struct {
	db          kv.RwDB
	miningState MiningState
	notifier    ChainEventNotifier
	chainConfig chain.Config
	engine      consensus.Engine
	blockReader services.FullBlockReader
	vmConfig    *vm.Config
	tmpdir      string
	interrupt   *int32
	payloadId   uint64
	txPool2     TxPoolForMining
	txPool2DB   kv.RoDB
	noTxPool    bool
}

type TxPoolForMining interface {
	YieldBest(n uint16, txs *types2.TxsRlp, tx kv.Tx, onTopOf, availableGas, availableDataGas uint64, toSkip mapset.Set[[32]byte]) (bool, int, error)
}

func StageMiningExecCfg(
	db kv.RwDB, miningState MiningState,
	notifier ChainEventNotifier, chainConfig chain.Config,
	engine consensus.Engine, vmConfig *vm.Config,
	tmpdir string, interrupt *int32, payloadId uint64,
	txPool2 TxPoolForMining, txPool2DB kv.RoDB,
	noTxPool bool,
	blockReader services.FullBlockReader,
) MiningExecCfg {
	return MiningExecCfg{
		db:          db,
		miningState: miningState,
		notifier:    notifier,
		chainConfig: chainConfig,
		engine:      engine,
		blockReader: blockReader,
		vmConfig:    vmConfig,
		tmpdir:      tmpdir,
		interrupt:   interrupt,
		payloadId:   payloadId,
		txPool2:     txPool2,
		txPool2DB:   txPool2DB,
		noTxPool:    noTxPool,
	}
}

// SpawnMiningExecStage
// TODO:
// - resubmitAdjustCh - variable is not implemented
func SpawnMiningExecStage(s *StageState, tx kv.RwTx, cfg MiningExecCfg, quit <-chan struct{}, logger log.Logger) error {
	cfg.vmConfig.NoReceipts = false
	chainID, _ := uint256.FromBig(cfg.chainConfig.ChainID)
	logPrefix := s.LogPrefix()
	current := cfg.miningState.MiningBlock
	txs := current.PreparedTxs
	forceTxs := current.ForceTxs
	noempty := true

	stateReader := state.NewPlainStateReader(tx)
	ibs := state.New(stateReader)
	stateWriter := state.NewPlainStateWriter(tx, tx, current.Header.Number.Uint64())
	if cfg.chainConfig.DAOForkBlock != nil && cfg.chainConfig.DAOForkBlock.Cmp(current.Header.Number) == 0 {
		misc.ApplyDAOHardFork(ibs)
	}

	// Optimism Canyon
	misc.EnsureCreate2Deployer(&cfg.chainConfig, current.Header.Time, ibs)

	systemcontracts.UpgradeBuildInSystemContract(&cfg.chainConfig, current.Header.Number, ibs, logger)

	// Create an empty block based on temporary copied state for
	// sealing in advance without waiting block execution finished.
	if !noempty {
		logger.Info("Commit an empty block", "number", current.Header.Number)
		return nil
	}

	getHeader := func(hash libcommon.Hash, number uint64) *types.Header { return rawdb.ReadHeader(tx, hash, number) }

	// Short circuit if there is no available pending transactions.
	// But if we disable empty precommit already, ignore it. Since
	// empty block is necessary to keep the liveness of the network.
	if noempty {
		if !forceTxs.Empty() {
			// forceTxs is sent by Optimism consensus client, and all force txs must be included in the payload.
			// Therefore, interrupts to block building must not be handled while force txs are being processed.
			// So do not pass cfg.interrupt
			logs, _, err := addTransactionsToMiningBlock(logPrefix, current, cfg.chainConfig, cfg.vmConfig, getHeader, cfg.engine, forceTxs, cfg.miningState.MiningConfig.Etherbase, ibs, quit, nil, cfg.payloadId, true, logger)
			if err != nil {
				return err
			}
			NotifyPendingLogs(logPrefix, cfg.notifier, logs, logger)
		}
		if txs != nil && !txs.Empty() {
			logs, _, err := addTransactionsToMiningBlock(logPrefix, current, cfg.chainConfig, cfg.vmConfig, getHeader, cfg.engine, txs, cfg.miningState.MiningConfig.Etherbase, ibs, quit, cfg.interrupt, cfg.payloadId, false, logger)
			if err != nil {
				return err
			}
			NotifyPendingLogs(logPrefix, cfg.notifier, logs, logger)
		} else if !cfg.noTxPool {

			yielded := mapset.NewSet[[32]byte]()
			simulationTx := memdb.NewMemoryBatch(tx, cfg.tmpdir)
			defer simulationTx.Rollback()
			executionAt, err := s.ExecutionAt(tx)
			if err != nil {
				return err
			}

			for {
				txs, y, err := getNextTransactions(cfg, chainID, current.Header, 50, executionAt, simulationTx, yielded, logger)
				if err != nil {
					return err
				}

				if !txs.Empty() {
					logs, stop, err := addTransactionsToMiningBlock(logPrefix, current, cfg.chainConfig, cfg.vmConfig, getHeader, cfg.engine, txs, cfg.miningState.MiningConfig.Etherbase, ibs, quit, cfg.interrupt, cfg.payloadId, false, logger)
					if err != nil {
						return err
					}
					NotifyPendingLogs(logPrefix, cfg.notifier, logs, logger)
					if stop {
						break
					}
				} else {
					break
				}

				// if we yielded less than the count we wanted, assume the txpool has run dry now and stop to save another loop
				if y < 50 {
					break
				}
			}
		}
	}

	logger.Debug("SpawnMiningExecStage", "block txn", current.Txs.Len(), "payload", cfg.payloadId)
	if current.Uncles == nil {
		current.Uncles = []*types.Header{}
	}
	if current.Txs == nil {
		current.Txs = []types.Transaction{}
	}
	if current.Receipts == nil {
		current.Receipts = types.Receipts{}
	}

	var err error
	_, current.Txs, current.Receipts, err = core.FinalizeBlockExecution(cfg.engine, stateReader, current.Header, current.Txs, current.Uncles, stateWriter, &cfg.chainConfig, ibs, current.Receipts, current.Withdrawals, ChainReaderImpl{config: &cfg.chainConfig, tx: tx, blockReader: cfg.blockReader}, true)
	if err != nil {
		return err
	}
	logger.Debug("FinalizeBlockExecution", "current txn", current.Txs.Len(), "current receipt", current.Receipts.Len(), "payload", cfg.payloadId)

	// hack: pretend that we are real execution stage - next stages will rely on this progress
	if err := stages.SaveStageProgress(tx, stages.Execution, current.Header.Number.Uint64()); err != nil {
		return err
	}
	return nil
}

func getNextTransactions(
	cfg MiningExecCfg,
	chainID *uint256.Int,
	header *types.Header,
	amount uint16,
	executionAt uint64,
	simulationTx *memdb.MemoryMutation,
	alreadyYielded mapset.Set[[32]byte],
	logger log.Logger,
) (types.TransactionsStream, int, error) {
	txSlots := types2.TxsRlp{}
	var onTime bool
	count := 0
	if err := cfg.txPool2DB.View(context.Background(), func(poolTx kv.Tx) error {
		var err error
		counter := 0
		for !onTime && counter < 1000 {
			remainingGas := header.GasLimit - header.GasUsed
			remainingDataGas := uint64(0)
			if header.DataGasUsed != nil {
				remainingDataGas = chain.MaxDataGasPerBlock - *header.DataGasUsed
			}
			if onTime, count, err = cfg.txPool2.YieldBest(amount, &txSlots, poolTx, executionAt, remainingGas, remainingDataGas, alreadyYielded); err != nil {
				return err
			}
			time.Sleep(1 * time.Millisecond)
			counter++
		}
		return nil
	}); err != nil {
		return nil, 0, err
	}

	var txs []types.Transaction //nolint:prealloc
	for i := range txSlots.Txs {
		transaction, err := types.DecodeWrappedTransaction(txSlots.Txs[i])
		if err == io.EOF {
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		if !transaction.GetChainID().IsZero() && transaction.GetChainID().Cmp(chainID) != 0 {
			continue
		}

		var sender libcommon.Address
		copy(sender[:], txSlots.Senders.At(i))

		// Check if tx nonce is too low
		txs = append(txs, transaction)
		txs[len(txs)-1].SetSender(sender)
	}

	blockNum := executionAt + 1
	txs, err := filterBadTransactions(txs, cfg.chainConfig, blockNum, header.BaseFee, simulationTx, logger)
	if err != nil {
		return nil, 0, err
	}

	return types.NewTransactionsFixedOrder(txs), count, nil
}

func filterBadTransactions(transactions []types.Transaction, config chain.Config, blockNumber uint64, baseFee *big.Int, simulationTx *memdb.MemoryMutation, logger log.Logger) ([]types.Transaction, error) {
	initialCnt := len(transactions)
	var filtered []types.Transaction
	gasBailout := false

	missedTxs := 0
	noSenderCnt := 0
	noAccountCnt := 0
	nonceTooLowCnt := 0
	notEOACnt := 0
	feeTooLowCnt := 0
	balanceTooLowCnt := 0
	overflowCnt := 0
	for len(transactions) > 0 && missedTxs != len(transactions) {
		transaction := transactions[0]
		sender, ok := transaction.GetSender()
		if !ok {
			transactions = transactions[1:]
			noSenderCnt++
			continue
		}
		var account accounts.Account
		ok, err := rawdb.ReadAccount(simulationTx, sender, &account)
		if err != nil {
			return nil, err
		}
		if !ok {
			transactions = transactions[1:]
			noAccountCnt++
			continue
		}
		// Check transaction nonce
		if account.Nonce > transaction.GetNonce() {
			transactions = transactions[1:]
			nonceTooLowCnt++
			continue
		}
		if account.Nonce < transaction.GetNonce() {
			missedTxs++
			transactions = append(transactions[1:], transaction)
			continue
		}
		missedTxs = 0

		// Make sure the sender is an EOA (EIP-3607)
		if !account.IsEmptyCodeHash() {
			transactions = transactions[1:]
			notEOACnt++
			continue
		}

		if config.IsLondon(blockNumber) {
			baseFee256 := uint256.NewInt(0)
			if overflow := baseFee256.SetFromBig(baseFee); overflow {
				return nil, fmt.Errorf("bad baseFee %s", baseFee)
			}
			// Make sure the transaction gasFeeCap is greater than the block's baseFee.
			if !transaction.GetFeeCap().IsZero() || !transaction.GetTip().IsZero() {
				if err := core.CheckEip1559TxGasFeeCap(sender, transaction.GetFeeCap(), transaction.GetTip(), baseFee256, false /* isFree */); err != nil {
					transactions = transactions[1:]
					feeTooLowCnt++
					continue
				}
			}
		}
		txnGas := transaction.GetGas()
		txnPrice := transaction.GetPrice()
		value := transaction.GetValue()
		accountBalance := account.Balance

		want := uint256.NewInt(0)
		want.SetUint64(txnGas)
		want, overflow := want.MulOverflow(want, txnPrice)
		if overflow {
			transactions = transactions[1:]
			overflowCnt++
			continue
		}

		if transaction.GetFeeCap() != nil {
			want.SetUint64(txnGas)
			want, overflow = want.MulOverflow(want, transaction.GetFeeCap())
			if overflow {
				transactions = transactions[1:]
				overflowCnt++
				continue
			}
			want, overflow = want.AddOverflow(want, value)
			if overflow {
				transactions = transactions[1:]
				overflowCnt++
				continue
			}
		}

		if accountBalance.Cmp(want) < 0 {
			if !gasBailout {
				transactions = transactions[1:]
				balanceTooLowCnt++
				continue
			}
		}
		// Updates account in the simulation
		account.Nonce++
		account.Balance.Sub(&account.Balance, want)
		accountBuffer := make([]byte, account.EncodingLengthForStorage())
		account.EncodeForStorage(accountBuffer)
		if err := simulationTx.Put(kv.PlainState, sender[:], accountBuffer); err != nil {
			return nil, err
		}
		// Mark transaction as valid
		filtered = append(filtered, transaction)
		transactions = transactions[1:]
	}
	logger.Debug("Filtration", "initial", initialCnt, "no sender", noSenderCnt, "no account", noAccountCnt, "nonce too low", nonceTooLowCnt, "nonceTooHigh", missedTxs, "sender not EOA", notEOACnt, "fee too low", feeTooLowCnt, "overflow", overflowCnt, "balance too low", balanceTooLowCnt, "filtered", len(filtered))
	return filtered, nil
}

func addTransactionsToMiningBlock(logPrefix string, current *MiningBlock, chainConfig chain.Config, vmConfig *vm.Config, getHeader func(hash libcommon.Hash, number uint64) *types.Header,
	engine consensus.Engine, txs types.TransactionsStream, coinbase libcommon.Address, ibs *state.IntraBlockState, quit <-chan struct{},
	interrupt *int32, payloadId uint64, allowDeposits bool, logger log.Logger) (types.Logs, bool, error) {
	header := current.Header
	tcount := 0
	gasPool := new(core.GasPool).AddGas(header.GasLimit - header.GasUsed)
	signer := types.MakeSigner(&chainConfig, header.Number.Uint64(), header.Time)

	var coalescedLogs types.Logs
	noop := state.NewNoopWriter()

	var miningCommitTx = func(txn types.Transaction, coinbase libcommon.Address, vmConfig *vm.Config, chainConfig chain.Config, ibs *state.IntraBlockState, current *MiningBlock) ([]*types.Log, error) {
		ibs.SetTxContext(txn.Hash(), libcommon.Hash{}, tcount)
		gasSnap := gasPool.Gas()
		dataGasSnap := gasPool.DataGas()
		snap := ibs.Snapshot()
		logger.Debug("addTransactionsToMiningBlock", "txn hash", txn.Hash())
		receipt, _, err := core.ApplyTransaction(&chainConfig, core.GetHashFn(header, getHeader), engine, &coinbase, gasPool, ibs, noop, header, txn, &header.GasUsed, header.DataGasUsed, *vmConfig)
		if err != nil {
			ibs.RevertToSnapshot(snap)
			gasPool = new(core.GasPool).AddGas(gasSnap).AddDataGas(dataGasSnap) // restore gasPool as well as ibs
			return nil, err
		}

		current.Txs = append(current.Txs, txn)
		current.Receipts = append(current.Receipts, receipt)
		return receipt.Logs, nil
	}

	var stopped *time.Ticker
	defer func() {
		if stopped != nil {
			stopped.Stop()
		}
	}()

	done := false

LOOP:
	for {
		// see if we need to stop now
		if stopped != nil {
			select {
			case <-stopped.C:
				done = true
				break LOOP
			default:
			}
		}

		if err := libcommon.Stopped(quit); err != nil {
			return nil, true, err
		}

		if interrupt != nil && atomic.LoadInt32(interrupt) != 0 && stopped == nil {
			logger.Debug("Transaction adding was requested to stop", "payload", payloadId)
			// ensure we run for at least 500ms after the request to stop comes in from GetPayload
			stopped = time.NewTicker(500 * time.Millisecond)
		}
		// If we don't have enough gas for any further transactions then we're done
		if gasPool.Gas() < params.TxGas {
			logger.Debug(fmt.Sprintf("[%s] Not enough gas for further transactions", logPrefix), "have", gasPool, "want", params.TxGas)
			done = true
			break
		}
		// Retrieve the next transaction and abort if all done
		txn := txs.Peek()
		if txn == nil {
			break
		}

		// We use the eip155 signer regardless of the env hf.
		from, err := txn.Sender(*signer)
		if err != nil {
			logger.Warn(fmt.Sprintf("[%s] Could not recover transaction sender", logPrefix), "hash", txn.Hash(), "err", err)
			txs.Pop()
			continue
		}

		// Check whether the txn is replay protected. If we're not in the EIP155 (Spurious Dragon) hf
		// phase, start ignoring the sender until we do.
		if txn.Protected() && !chainConfig.IsSpuriousDragon(header.Number.Uint64()) {
			logger.Debug(fmt.Sprintf("[%s] Ignoring replay protected transaction", logPrefix), "hash", txn.Hash(), "eip155", chainConfig.SpuriousDragonBlock)

			txs.Pop()
			continue
		}

		// Start executing the transaction
		if !allowDeposits && txn.Type() == types.DepositTxType {
			log.Warn(fmt.Sprintf("[%s] Ignoring deposit tx that made its way through mempool", logPrefix), "hash", txn.Hash())
			txs.Pop()
			continue
		}
		logs, err := miningCommitTx(txn, coinbase, vmConfig, chainConfig, ibs, current)

		if errors.Is(err, core.ErrGasLimitReached) {
			// Pop the env out-of-gas transaction without shifting in the next from the account
			logger.Debug(fmt.Sprintf("[%s] Gas limit exceeded for env block", logPrefix), "hash", txn.Hash(), "sender", from)
			txs.Pop()
		} else if errors.Is(err, core.ErrNonceTooLow) {
			// New head notification data race between the transaction pool and miner, shift
			logger.Debug(fmt.Sprintf("[%s] Skipping transaction with low nonce", logPrefix), "hash", txn.Hash(), "sender", from, "nonce", txn.GetNonce())
			txs.Shift()
		} else if errors.Is(err, core.ErrNonceTooHigh) {
			// Reorg notification data race between the transaction pool and miner, skip account =
			logger.Debug(fmt.Sprintf("[%s] Skipping transaction with high nonce", logPrefix), "hash", txn.Hash(), "sender", from, "nonce", txn.GetNonce())
			txs.Pop()
		} else if err == nil {
			// Everything ok, collect the logs and shift in the next transaction from the same account
			logger.Debug(fmt.Sprintf("[%s] addTransactionsToMiningBlock Successful", logPrefix), "sender", from, "nonce", txn.GetNonce(), "payload", payloadId)
			coalescedLogs = append(coalescedLogs, logs...)
			tcount++
			txs.Shift()
		} else {
			// Strange error, discard the transaction and get the next in line (note, the
			// nonce-too-high clause will prevent us from executing in vain).
			logger.Debug(fmt.Sprintf("[%s] Skipping transaction", logPrefix), "hash", txn.Hash(), "sender", from, "err", err)
			txs.Shift()
		}
	}

	/*
		// Notify resubmit loop to decrease resubmitting interval if env interval is larger
		// than the user-specified one.
		if interrupt != nil {
			w.resubmitAdjustCh <- &intervalAdjust{inc: false}
		}
	*/
	return coalescedLogs, done, nil

}

func NotifyPendingLogs(logPrefix string, notifier ChainEventNotifier, logs types.Logs, logger log.Logger) {
	if len(logs) == 0 {
		return
	}

	if notifier == nil {
		logger.Debug(fmt.Sprintf("[%s] rpc notifier is not set, rpc daemon won't be updated about pending logs", logPrefix))
		return
	}
	notifier.OnNewPendingLogs(logs)
}

// implemented by tweaking UnwindExecutionStage
func UnwindMiningExecutionStage(u *UnwindState, s *StageState, tx kv.RwTx, ctx context.Context, cfg MiningExecCfg, logger log.Logger) (err error) {
	if u.UnwindPoint >= s.BlockNumber {
		return nil
	}
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = cfg.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}
	logPrefix := u.LogPrefix()
	log.Info(fmt.Sprintf("[%s] Unwind Mining Execution", logPrefix), "from", s.BlockNumber, "to", u.UnwindPoint)

	if err = unwindMiningExecutionStage(u, s, tx, ctx, cfg, logger); err != nil {
		return err
	}
	if err = u.Done(tx); err != nil {
		return err
	}

	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func unwindMiningExecutionStage(u *UnwindState, s *StageState, tx kv.RwTx, ctx context.Context, cfg MiningExecCfg, logger log.Logger) error {
	logPrefix := s.LogPrefix()
	stateBucket := kv.PlainState
	storageKeyLength := length.Addr + length.Incarnation + length.Hash

	changes := etl.NewCollector(logPrefix, cfg.tmpdir, etl.NewOldestEntryBuffer(etl.BufferOptimalSize), logger)
	defer changes.Close()
	errRewind := changeset.RewindData(tx, s.BlockNumber, u.UnwindPoint, changes, ctx.Done())
	if errRewind != nil {
		return fmt.Errorf("getting rewind data: %w", errRewind)
	}

	if err := changes.Load(tx, stateBucket, func(k, v []byte, table etl.CurrentTableReader, next etl.LoadNextFunc) error {
		if len(k) == 20 {
			if len(v) > 0 {
				var acc accounts.Account
				if err := acc.DecodeForStorage(v); err != nil {
					return err
				}

				// Fetch the code hash
				recoverCodeHashPlain(&acc, tx, k)
				var address libcommon.Address
				copy(address[:], k)

				// cleanup contract code bucket
				original, err := state.NewPlainStateReader(tx).ReadAccountData(address)
				if err != nil {
					return fmt.Errorf("read account for %x: %w", address, err)
				}
				if original != nil {
					// clean up all the code incarnations original incarnation and the new one
					for incarnation := original.Incarnation; incarnation > acc.Incarnation && incarnation > 0; incarnation-- {
						err = tx.Delete(kv.PlainContractCode, dbutils.PlainGenerateStoragePrefix(address[:], incarnation))
						if err != nil {
							return fmt.Errorf("writeAccountPlain for %x: %w", address, err)
						}
					}
				}

				newV := make([]byte, acc.EncodingLengthForStorage())
				acc.EncodeForStorage(newV)
				if err := next(k, k, newV); err != nil {
					return err
				}
			} else {
				if err := next(k, k, nil); err != nil {
					return err
				}
			}
			return nil
		}
		if len(v) > 0 {
			if err := next(k, k[:storageKeyLength], v); err != nil {
				return err
			}
		} else {
			if err := next(k, k[:storageKeyLength], nil); err != nil {
				return err
			}
		}
		return nil

	}, etl.TransformArgs{Quit: ctx.Done()}); err != nil {
		return err
	}

	if err := historyv2.Truncate(tx, u.UnwindPoint+1); err != nil {
		return err
	}

	// we do not have to delete receipt, epoch, callTraceSet because
	// mining stage does not write anything to db.
	return nil
}
