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
	"context"

	"github.com/erigontech/erigon-lib/log/v3"

	remote "github.com/erigontech/erigon-lib/gointerfaces/remoteproto"
	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/wrap"
	"github.com/erigontech/erigon/core/types"
	"github.com/erigontech/erigon/eth/stagedsync/stages"
)

type ChainEventNotifier interface {
	OnNewHeader(newHeadersRlp [][]byte)
	OnNewPendingLogs(types.Logs)
	OnLogs([]*remote.SubscribeLogsReply)
	HasLogSubsriptions() bool
}

func MiningStages(
	ctx context.Context,
	createBlockCfg MiningCreateBlockCfg,
<<<<<<< HEAD
	borHeimdallCfg BorHeimdallCfg, //nolint:gocritic
=======
	borHeimdallCfg BorHeimdallCfg,
	executeBlockCfg ExecuteBlockCfg,
	sendersCfg SendersCfg,
>>>>>>> v3.0.0-alpha1
	execCfg MiningExecCfg,
	finish MiningFinishCfg,
) []*Stage {
	return []*Stage{
		{
			ID:          stages.MiningCreateBlock,
<<<<<<< HEAD
			Description: "Mining: add force-txs",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, txc wrap.TxContainer, logger log.Logger) error {
				return SpawnMiningForceTxsStage(s, txc.Tx, createBlockCfg, ctx.Done())
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, txc wrap.TxContainer, logger log.Logger) error {
				return nil
			},
			Prune: func(firstCycle bool, u *PruneState, tx kv.RwTx, logger log.Logger) error { return nil },
		},
		{
			ID:          stages.MiningCreateBlock,
			Description: "Mining: construct new block from tx pool",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, txc wrap.TxContainer, logger log.Logger) error {
=======
			Description: "Mining: construct new block from txn pool",
			Forward: func(badBlockUnwind bool, s *StageState, u Unwinder, txc wrap.TxContainer, logger log.Logger) error {
>>>>>>> v3.0.0-alpha1
				return SpawnMiningCreateBlockStage(s, txc.Tx, createBlockCfg, ctx.Done(), logger)
			},
			Unwind: func(u *UnwindState, s *StageState, txc wrap.TxContainer, logger log.Logger) error {
				return nil
			},
			Prune: func(u *PruneState, tx kv.RwTx, logger log.Logger) error { return nil },
		},
		{
			ID:          stages.MiningBorHeimdall,
			Description: "Download Bor-specific data from Heimdall",
			Forward: func(badBlockUnwind bool, s *StageState, u Unwinder, txc wrap.TxContainer, logger log.Logger) error {
				if badBlockUnwind {
					return nil
				}
				return MiningBorHeimdallForward(ctx, borHeimdallCfg, s, u, txc.Tx, logger)
			},
			Unwind: func(u *UnwindState, s *StageState, txc wrap.TxContainer, logger log.Logger) error {
				return BorHeimdallUnwind(u, ctx, s, txc.Tx, borHeimdallCfg)
			},
			Prune: func(p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return BorHeimdallPrune(p, ctx, tx, borHeimdallCfg)
			},
		},
		{
			ID:          stages.MiningExecution,
			Description: "Mining: execute new block from txn pool",
			Forward: func(badBlockUnwind bool, s *StageState, u Unwinder, txc wrap.TxContainer, logger log.Logger) error {
				return SpawnMiningExecStage(s, txc, execCfg, sendersCfg, executeBlockCfg, ctx, logger)
			},
<<<<<<< HEAD
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, txc wrap.TxContainer, logger log.Logger) error {
				return UnwindMiningExecutionStage(u, s, txc.Tx, ctx, execCfg, logger)
			},
			Prune: func(firstCycle bool, u *PruneState, tx kv.RwTx, logger log.Logger) error { return nil },
		},
		{
			ID:          stages.HashState,
			Description: "Hash the key in the state",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, txc wrap.TxContainer, logger log.Logger) error {
				return SpawnHashStateStage(s, txc.Tx, hashStateCfg, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, txc wrap.TxContainer, logger log.Logger) error {
				return UnwindHashStateStage(u, s, txc.Tx, hashStateCfg, ctx, logger)
			},
			Prune: func(firstCycle bool, u *PruneState, tx kv.RwTx, logger log.Logger) error { return nil },
		},
		{
			ID:          stages.IntermediateHashes,
			Description: "Generate intermediate hashes and computing state root",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, txc wrap.TxContainer, logger log.Logger) error {
				stateRoot, err := SpawnIntermediateHashesStage(s, u, txc.Tx, trieCfg, ctx, logger)
				if err != nil {
					return err
				}
				createBlockCfg.miner.MiningBlock.Header.Root = stateRoot
				return nil
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, txc wrap.TxContainer, logger log.Logger) error {
				return UnwindIntermediateHashesStage(u, s, txc.Tx, trieCfg, ctx, logger)
			},
			Prune: func(firstCycle bool, u *PruneState, tx kv.RwTx, logger log.Logger) error { return nil },
=======
			Unwind: func(u *UnwindState, s *StageState, txc wrap.TxContainer, logger log.Logger) error {
				return nil
			},
			Prune: func(u *PruneState, tx kv.RwTx, logger log.Logger) error { return nil },
>>>>>>> v3.0.0-alpha1
		},
		{
			ID:          stages.MiningFinish,
			Description: "Mining: create and propagate valid block",
			Forward: func(badBlockUnwind bool, s *StageState, u Unwinder, txc wrap.TxContainer, logger log.Logger) error {
				return SpawnMiningFinishStage(s, txc.Tx, finish, ctx.Done(), logger)
			},
			Unwind: func(u *UnwindState, s *StageState, txc wrap.TxContainer, logger log.Logger) error {
				return nil
			},
			Prune: func(u *PruneState, tx kv.RwTx, logger log.Logger) error { return nil },
		},
	}
}
