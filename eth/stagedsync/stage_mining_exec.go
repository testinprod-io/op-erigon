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

package stagedsync

import (
	"errors"
	"fmt"
	"github.com/erigontech/erigon-lib/kv/rawdbv3"
	"io"
	"math/big"
	"sync/atomic"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/holiman/uint256"
	"golang.org/x/net/context"

	"github.com/erigontech/erigon-lib/kv/membatchwithdb"
	"github.com/erigontech/erigon-lib/log/v3"
	libstate "github.com/erigontech/erigon-lib/state"
	"github.com/erigontech/erigon-lib/wrap"

	"github.com/erigontech/erigon-lib/chain"
	libcommon "github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/common/metrics"
	"github.com/erigontech/erigon-lib/kv"
	types2 "github.com/erigontech/erigon-lib/types"
	"github.com/erigontech/erigon/consensus"
	"github.com/erigontech/erigon/core"
	"github.com/erigontech/erigon/core/rawdb"
	"github.com/erigontech/erigon/core/state"
	"github.com/erigontech/erigon/core/types"
	"github.com/erigontech/erigon/core/types/accounts"
	"github.com/erigontech/erigon/core/vm"
	"github.com/erigontech/erigon/eth/stagedsync/stages"
	"github.com/erigontech/erigon/params"
	"github.com/erigontech/erigon/turbo/services"
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
	txPool      TxPoolForMining
	txPoolDB    kv.RoDB
	noTxPool    bool
}

type TxPoolForMining interface {
	YieldBest(n uint16, txs *types2.TxsRlp, tx kv.Tx, onTopOf, availableGas, availableBlobGas uint64, toSkip mapset.Set[[32]byte]) (bool, int, error)
}

func StageMiningExecCfg(
	db kv.RwDB, miningState MiningState,
	notifier ChainEventNotifier, chainConfig chain.Config,
	engine consensus.Engine, vmConfig *vm.Config,
	tmpdir string, interrupt *int32, payloadId uint64,
	txPool TxPoolForMining, txPoolDB kv.RoDB,
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
		txPool:      txPool,
		txPoolDB:    txPoolDB,
		noTxPool:    noTxPool,
	}
}

// SpawnMiningExecStage
// TODO:
// - resubmitAdjustCh - variable is not implemented
func SpawnMiningExecStage(s *StageState, txc wrap.TxContainer, cfg MiningExecCfg, sendersCfg SendersCfg, execCfg ExecuteBlockCfg, ctx context.Context, logger log.Logger) error {
	cfg.vmConfig.NoReceipts = false
	chainID, _ := uint256.FromBig(cfg.chainConfig.ChainID)
	logPrefix := s.LogPrefix()
	current := cfg.miningState.MiningBlock
	txs := current.PreparedTxs
	forceTxs := current.ForceTxs
	noempty := true
	var domains *libstate.SharedDomains
	var (
		stateReader state.StateReader
	)
	stateReader = state.NewReaderV4(txc.Doms)
	ibs := state.New(stateReader)
	// Clique consensus needs forced author in the evm context
	if cfg.chainConfig.Consensus == chain.CliqueConsensus {
		execCfg.author = &cfg.miningState.MiningConfig.Etherbase
	}

	// Create an empty block based on temporary copied state for
	// sealing in advance without waiting block execution finished.
	if !noempty {
		logger.Info("Commit an empty block", "number", current.Header.Number)
		return nil
	}
	getHeader := func(hash libcommon.Hash, number uint64) *types.Header { return rawdb.ReadHeader(txc.Tx, hash, number) }

	// Short circuit if there is no available pending transactions.
	// But if we disable empty precommit already, ignore it. Since
	// empty block is necessary to keep the liveness of the network.
	if noempty {
		if !forceTxs.Empty() {
			// forceTxs is sent by Optimism consensus client, and all force txs must be included in the payload.
			// Therefore, interrupts to block building must not be handled while force txs are being processed.
			// So do not pass cfg.interrupt
			logs, _, err := addTransactionsToMiningBlock(logPrefix, current, cfg.chainConfig, cfg.vmConfig, getHeader, cfg.engine, forceTxs, cfg.miningState.MiningConfig.Etherbase, ibs, ctx, nil, cfg.payloadId, true, logger)
			if err != nil {
				return err
			}
			NotifyPendingLogs(logPrefix, cfg.notifier, logs, logger)
		}
		if txs != nil && !txs.Empty() {
			logs, _, err := addTransactionsToMiningBlock(logPrefix, current, cfg.chainConfig, cfg.vmConfig, getHeader, cfg.engine, txs, cfg.miningState.MiningConfig.Etherbase, ibs, ctx, cfg.interrupt, cfg.payloadId, false, logger)
			if err != nil {
				return err
			}
			NotifyPendingLogs(logPrefix, cfg.notifier, logs, logger)
		} else if !cfg.noTxPool {

			yielded := mapset.NewSet[[32]byte]()
			var simStateReader state.StateReader
			var simStateWriter state.StateWriter
			m := membatchwithdb.NewMemoryBatch(txc.Tx, cfg.tmpdir, logger)
			defer m.Rollback()
			var err error
			domains, err = libstate.NewSharedDomains(m, logger)
			if err != nil {
				return err
			}
			defer domains.Close()
			simStateReader = state.NewReaderV4(domains)
			simStateWriter = state.NewWriterV4(domains)

			executionAt, err := s.ExecutionAt(txc.Tx)
			if err != nil {
				return err
			}

			for {
				txs, y, err := getNextTransactions(cfg, chainID, current.Header, 50, executionAt, yielded, simStateReader, simStateWriter, logger)
				if err != nil {
					return err
				}

				if !txs.Empty() {
					logs, stop, err := addTransactionsToMiningBlock(logPrefix, current, cfg.chainConfig, cfg.vmConfig, getHeader, cfg.engine, txs, cfg.miningState.MiningConfig.Etherbase, ibs, ctx, cfg.interrupt, cfg.payloadId, false, logger)
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

			metrics.UpdateBlockProducerProductionDelay(current.ParentHeaderTime, current.Header.Number.Uint64(), logger)
		}
	}

	logger.Debug("SpawnMiningExecStage", "block", current.Header.Number, "txn", current.Txs.Len(), "payload", cfg.payloadId)
	if current.Uncles == nil {
		current.Uncles = []*types.Header{}
	}
	if current.Txs == nil {
		current.Txs = []types.Transaction{}
	}
	if current.Receipts == nil {
		current.Receipts = types.Receipts{}
	}
	chainReader := ChainReaderImpl{config: &cfg.chainConfig, tx: txc.Tx, blockReader: cfg.blockReader, logger: logger}

	if err := cfg.engine.Prepare(chainReader, current.Header, ibs); err != nil {
		return err
	}

	var err error
	var block *types.Block
	block, current.Txs, current.Receipts, err = core.FinalizeBlockExecution(cfg.engine, stateReader, current.Header, current.Txs, current.Uncles, &state.NoopWriter{}, &cfg.chainConfig, ibs, current.Receipts, current.Withdrawals, current.Requests, chainReader, true, logger)
	if err != nil {
		return fmt.Errorf("cannot finalize block execution: %s", err)
	}
	current.Requests = block.Requests()

	// Simulate the block execution to get the final state root
	if err := rawdb.WriteHeader(txc.Tx, block.Header()); err != nil {
		return fmt.Errorf("cannot write header: %s", err)
	}
	blockHeight := block.NumberU64()

	if err := rawdb.WriteCanonicalHash(txc.Tx, block.Hash(), blockHeight); err != nil {
		return fmt.Errorf("cannot write canonical hash: %s", err)
	}
	if err := rawdb.WriteHeadHeaderHash(txc.Tx, block.Hash()); err != nil {
		return err
	}
	if _, err = rawdb.WriteRawBodyIfNotExists(txc.Tx, block.Hash(), blockHeight, block.RawBody()); err != nil {
		return fmt.Errorf("cannot write body: %s", err)
	}
	if err := rawdb.AppendCanonicalTxNums(txc.Tx, blockHeight); err != nil {
		return err
	}
	if err := stages.SaveStageProgress(txc.Tx, kv.Headers, blockHeight); err != nil {
		return err
	}
	if err := stages.SaveStageProgress(txc.Tx, stages.Bodies, blockHeight); err != nil {
		return err
	}
	senderS := &StageState{state: s.state, ID: stages.Senders, BlockNumber: blockHeight - 1}
	if err := SpawnRecoverSendersStage(sendersCfg, senderS, nil, txc.Tx, blockHeight, ctx, logger); err != nil {
		return err
	}

	// This flag will skip checking the state root
	execCfg.blockProduction = true
	execS := &StageState{state: s.state, ID: stages.Execution, BlockNumber: blockHeight - 1}
	if err := ExecBlockV3(execS, nil, txc, blockHeight, context.Background(), execCfg, false, logger); err != nil {
		return err
	}

	rh, err := txc.Doms.ComputeCommitment(ctx, true, blockHeight, s.LogPrefix())
	if err != nil {
		return fmt.Errorf("StateV3.Apply: %w", err)
	}
	current.Header.Root = libcommon.BytesToHash(rh)

	logger.Info("FinalizeBlockExecution", "block", current.Header.Number, "txn", current.Txs.Len(), "gas", current.Header.GasUsed, "receipt", current.Receipts.Len(), "payload", cfg.payloadId)

	return nil
}

func getNextTransactions(
	cfg MiningExecCfg,
	chainID *uint256.Int,
	header *types.Header,
	amount uint16,
	executionAt uint64,
	alreadyYielded mapset.Set[[32]byte],
	simStateReader state.StateReader,
	simStateWriter state.StateWriter,
	logger log.Logger,
) (types.TransactionsStream, int, error) {
	txSlots := types2.TxsRlp{}
	count := 0
	if err := cfg.txPoolDB.View(context.Background(), func(poolTx kv.Tx) error {
		var err error

		remainingGas := header.GasLimit - header.GasUsed
		remainingBlobGas := uint64(0)
		if header.BlobGasUsed != nil {
			remainingBlobGas = cfg.chainConfig.GetMaxBlobGasPerBlock() - *header.BlobGasUsed
		}

		if _, count, err = cfg.txPool.YieldBest(amount, &txSlots, poolTx, executionAt, remainingGas, remainingBlobGas, alreadyYielded); err != nil {
			return err
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

		// Check if txn nonce is too low
		txs = append(txs, transaction)
		txs[len(txs)-1].SetSender(sender)
	}

	blockNum := executionAt + 1
	txs, err := filterBadTransactions(txs, cfg.chainConfig, blockNum, header.BaseFee, simStateReader, simStateWriter, logger)
	if err != nil {
		return nil, 0, err
	}

	return types.NewTransactionsFixedOrder(txs), count, nil
}

func filterBadTransactions(transactions []types.Transaction, config chain.Config, blockNumber uint64, baseFee *big.Int, simStateReader state.StateReader, simStateWriter state.StateWriter, logger log.Logger) ([]types.Transaction, error) {
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
		account, err := simStateReader.ReadAccountData(sender)
		if err != nil {
			return nil, err
		}
		if account == nil {
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

		newAccount := new(accounts.Account)
		*newAccount = *account
		// Updates account in the simulation
		newAccount.Nonce++
		newAccount.Balance.Sub(&account.Balance, want)
		if err := simStateWriter.UpdateAccountData(sender, account, newAccount); err != nil {
			return nil, err
		}
		// Mark transaction as valid
		filtered = append(filtered, transaction)
		transactions = transactions[1:]
	}
	logger.Info("Filtration", "initial", initialCnt, "no sender", noSenderCnt, "no account", noAccountCnt, "nonce too low", nonceTooLowCnt, "nonceTooHigh", missedTxs, "sender not EOA", notEOACnt, "fee too low", feeTooLowCnt, "overflow", overflowCnt, "balance too low", balanceTooLowCnt, "filtered", len(filtered))
	return filtered, nil
}

func addTransactionsToMiningBlock(logPrefix string, current *MiningBlock, chainConfig chain.Config, vmConfig *vm.Config, getHeader func(hash libcommon.Hash, number uint64) *types.Header,
	engine consensus.Engine, txs types.TransactionsStream, coinbase libcommon.Address, ibs *state.IntraBlockState, ctx context.Context,
	interrupt *int32, payloadId uint64, allowDeposits bool, logger log.Logger) (types.Logs, bool, error) {
	header := current.Header
	tcount := 0
	gasPool := new(core.GasPool).AddGas(header.GasLimit - header.GasUsed)
	if header.BlobGasUsed != nil {
		gasPool.AddBlobGas(chainConfig.GetMaxBlobGasPerBlock() - *header.BlobGasUsed)
	}
	signer := types.MakeSigner(&chainConfig, header.Number.Uint64(), header.Time)

	var coalescedLogs types.Logs
	noop := state.NewNoopWriter()

	var miningCommitTx = func(txn types.Transaction, coinbase libcommon.Address, vmConfig *vm.Config, chainConfig chain.Config, ibs *state.IntraBlockState, current *MiningBlock) ([]*types.Log, error) {
		ibs.SetTxContext(txn.Hash(), libcommon.Hash{}, tcount)
		gasSnap := gasPool.Gas()
		blobGasSnap := gasPool.BlobGas()
		snap := ibs.Snapshot()
		receipt, _, err := core.ApplyTransaction(&chainConfig, core.GetHashFn(header, getHeader), engine, &coinbase, gasPool, ibs, noop, header, txn, &header.GasUsed, header.BlobGasUsed, *vmConfig)
		if err != nil {
			ibs.RevertToSnapshot(snap)
			gasPool = new(core.GasPool).AddGas(gasSnap).AddBlobGas(blobGasSnap) // restore gasPool as well as ibs
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

		if err := libcommon.Stopped(ctx.Done()); err != nil {
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
			logger.Trace(fmt.Sprintf("[%s] Added transaction", logPrefix), "hash", txn.Hash(), "sender", from, "nonce", txn.GetNonce(), "payload", payloadId)
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
func UnwindMiningExecutionStage(u *UnwindState, s *StageState, txc wrap.TxContainer, ctx context.Context, cfg MiningExecCfg, logger log.Logger) (err error) {
	if u.UnwindPoint >= s.BlockNumber {
		return nil
	}
	useExternalTx := txc.Tx != nil
	if !useExternalTx {
		txc.Tx, err = cfg.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer txc.Tx.Rollback()
	}
	logPrefix := u.LogPrefix()
	log.Info(fmt.Sprintf("[%s] Unwind Mining Execution", logPrefix), "from", s.BlockNumber, "to", u.UnwindPoint)

	unwindToLimit, ok, err := txc.Tx.(libstate.HasAggTx).AggTx().(*libstate.AggregatorRoTx).CanUnwindBeforeBlockNum(u.UnwindPoint, txc.Tx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %d < %d", ErrTooDeepUnwind, u.UnwindPoint, unwindToLimit)
	}

	if err = unwindMiningExecutionStage(u, s, txc, ctx, cfg, logger); err != nil {
		return err
	}
	if err = u.Done(txc.Tx); err != nil {
		return err
	}

	if !useExternalTx {
		if err = txc.Tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func unwindMiningExecutionStage(u *UnwindState, s *StageState, txc wrap.TxContainer, ctx context.Context, cfg MiningExecCfg, logger log.Logger) error {
	if s.BlockNumber-u.UnwindPoint < stateStreamLimit {
		_, err := cfg.blockReader.CanonicalHash(ctx, txc.Tx, u.UnwindPoint)
		if err != nil {
			return fmt.Errorf("read canonical hash of unwind point: %w", err)
		}
		_, err = cfg.blockReader.RawTransactions(ctx, txc.Tx, u.UnwindPoint, s.BlockNumber)
		if err != nil {
			return err
		}
	}

	return unwindMiningExec3(u, s, txc, ctx, logger)
}

func unwindMiningExec3(u *UnwindState, s *StageState, txc wrap.TxContainer, ctx context.Context, logger log.Logger) error {
	var domains *libstate.SharedDomains
	if txc.Doms == nil {
		domains, err := libstate.NewSharedDomains(txc.Tx, logger)
		if err != nil {
			return err
		}
		defer domains.Close()
	} else {
		domains = txc.Doms
	}
	rs := state.NewStateV3(domains, logger)
	// unwind all txs of u.UnwindPoint block. 1 txn in begin/end of block - system txs
	txNum, err := rawdbv3.TxNums.Min(txc.Tx, u.UnwindPoint+1)
	if err != nil {
		return err
	}
	t := time.Now()
	var changeset *[kv.DomainLen][]libstate.DomainEntryDiff
	for currentBlock := u.CurrentBlockNumber; currentBlock > u.UnwindPoint; currentBlock-- {
		currentHash, err := rawdb.ReadCanonicalHash(txc.Tx, currentBlock)
		if err != nil {
			return err
		}
		var ok bool
		var currentKeys [kv.DomainLen][]libstate.DomainEntryDiff
		currentKeys, ok, err = domains.GetDiffset(txc.Tx, currentHash, currentBlock)
		if !ok {
			return fmt.Errorf("domains.GetDiffset(%d, %s): not found", currentBlock, currentHash)
		}
		if err != nil {
			return err
		}
		if changeset == nil {
			changeset = &currentKeys
		} else {
			for i := range currentKeys {
				changeset[i] = libstate.MergeDiffSets(changeset[i], currentKeys[i])
			}
		}
	}
	if err := rs.Unwind(ctx, txc.Tx, u.UnwindPoint, txNum, nil, changeset); err != nil {
		return fmt.Errorf("StateV3.Unwind(%d->%d): %w, took %s", s.BlockNumber, u.UnwindPoint, err, time.Since(t))
	}
	// we do not have to delete receipt, epoch, callTraceSet because
	// mining stage does not write anything to db.
	return nil
}
