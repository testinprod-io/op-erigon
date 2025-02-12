package stagedsync

import (
	"context"
	"testing"

	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/kv/memdb"
	"github.com/erigontech/erigon-lib/log/v3"
	"github.com/erigontech/erigon/eth/stagedsync/stages"
	"github.com/stretchr/testify/require"
)

func TestMiningExec(t *testing.T) {
	logger := log.New()
	ctx, db1, db2 := context.Background(), memdb.NewTestDB(t), memdb.NewTestDB(t)
	cfg := MiningExecCfg{}

	t.Run("UnwindMiningExecutionStagePlainStatic", func(t *testing.T) {
		require, tx1, tx2 := require.New(t), memdb.BeginRw(t, db1), memdb.BeginRw(t, db2)

		generateBlocks(t, 1, 25, plainWriterGen(tx1), staticCodeStaticIncarnations)
		generateBlocks(t, 1, 50, plainWriterGen(tx2), staticCodeStaticIncarnations)

		err := stages.SaveStageProgress(tx2, stages.MiningExecution, 50)
		require.NoError(err)

		u := &UnwindState{ID: stages.MiningExecution, UnwindPoint: 25}
		s := &StageState{ID: stages.MiningExecution, BlockNumber: 50}
		err = UnwindMiningExecutionStage(u, s, tx2, ctx, cfg, logger)
		require.NoError(err)

		compareCurrentState(t, newAgg(t, logger), tx1, tx2, kv.PlainState, kv.PlainContractCode, kv.ContractTEVMCode)
	})
	t.Run("UnwindMiningExecutionStagePlainWithIncarnationChanges", func(t *testing.T) {
		require, tx1, tx2 := require.New(t), memdb.BeginRw(t, db1), memdb.BeginRw(t, db2)

		generateBlocks(t, 1, 25, plainWriterGen(tx1), changeCodeWithIncarnations)
		generateBlocks(t, 1, 50, plainWriterGen(tx2), changeCodeWithIncarnations)

		err := stages.SaveStageProgress(tx2, stages.MiningExecution, 50)
		require.NoError(err)

		u := &UnwindState{ID: stages.MiningExecution, UnwindPoint: 25}
		s := &StageState{ID: stages.MiningExecution, BlockNumber: 50}
		err = UnwindMiningExecutionStage(u, s, tx2, ctx, cfg, logger)
		require.NoError(err)

		compareCurrentState(t, newAgg(t, logger), tx1, tx2, kv.PlainState, kv.PlainContractCode)
	})
	t.Run("UnwindMiningExecutionStagePlainWithCodeChanges", func(t *testing.T) {
		t.Skip("not supported yet, to be restored")
		require, tx1, tx2 := require.New(t), memdb.BeginRw(t, db1), memdb.BeginRw(t, db2)

		generateBlocks(t, 1, 25, plainWriterGen(tx1), changeCodeIndepenentlyOfIncarnations)
		generateBlocks(t, 1, 50, plainWriterGen(tx2), changeCodeIndepenentlyOfIncarnations)

		err := stages.SaveStageProgress(tx2, stages.MiningExecution, 50)
		if err != nil {
			t.Errorf("error while saving progress: %v", err)
		}
		u := &UnwindState{ID: stages.MiningExecution, UnwindPoint: 25}
		s := &StageState{ID: stages.MiningExecution, BlockNumber: 50}
		err = UnwindMiningExecutionStage(u, s, tx2, ctx, cfg, logger)
		require.NoError(err)

		compareCurrentState(t, newAgg(t, logger), tx1, tx2, kv.PlainState, kv.PlainContractCode)
	})
}
