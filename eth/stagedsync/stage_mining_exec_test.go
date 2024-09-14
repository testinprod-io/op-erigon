package stagedsync

import (
	"context"
	"github.com/erigontech/erigon-lib/common/datadir"
	"github.com/erigontech/erigon-lib/kv/temporal/temporaltest"
	"github.com/erigontech/erigon-lib/wrap"
	"testing"

	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/log/v3"
	"github.com/erigontech/erigon/eth/stagedsync/stages"
	"github.com/stretchr/testify/require"
)

func TestMiningExec(t *testing.T) {
	logger := log.New()

	ctx := context.Background()
	dirs1 := datadir.New(t.TempDir())
	db1, _ := temporaltest.NewTestDB(t, dirs1)

	dirs2 := datadir.New(t.TempDir())
	db2, _ := temporaltest.NewTestDB(t, dirs2)
	cfg := MiningExecCfg{}

	t.Run("UnwindMiningExecutionStagePlainStatic", func(t *testing.T) {
		require := require.New(t)
		tx1, _ := db1.BeginRw(context.Background())
		tx2, _ := db2.BeginRw(context.Background())

		defer func() {
			tx1.Rollback()
			tx2.Rollback()
		}()

		before, after, writer := apply(tx1, logger)
		generateBlocks2(t, 1, 25, writer, before, after, staticCodeStaticIncarnations)
		before2, after2, writer2 := apply(tx2, logger)
		generateBlocks2(t, 1, 50, writer2, before2, after2, staticCodeStaticIncarnations)

		err := stages.SaveStageProgress(tx2, stages.MiningExecution, 50)
		require.NoError(err)

		u := &UnwindState{ID: stages.MiningExecution, UnwindPoint: 25}
		s := &StageState{ID: stages.MiningExecution, BlockNumber: 50}

		err = UnwindMiningExecutionStage(u, s, wrap.TxContainer{Tx: tx2}, ctx, cfg, logger)
		require.NoError(err)

		compareCurrentState(t, tx1, tx2, kv.PlainState, kv.PlainContractCode, kv.ContractCode)
	})
	t.Run("UnwindMiningExecutionStagePlainWithIncarnationChanges", func(t *testing.T) {
		require := require.New(t)
		tx1, _ := db1.BeginRw(context.Background())
		tx2, _ := db2.BeginRw(context.Background())
		defer func() {
			tx1.Rollback()
			tx2.Rollback()
		}()

		before1, after1, writer1 := apply(tx1, logger)
		before2, after2, writer2 := apply(tx2, logger)
		generateBlocks2(t, 1, 25, writer1, before1, after1, changeCodeWithIncarnations)
		generateBlocks2(t, 1, 50, writer2, before2, after2, changeCodeWithIncarnations)

		err := stages.SaveStageProgress(tx2, stages.MiningExecution, 50)
		require.NoError(err)

		u := &UnwindState{ID: stages.MiningExecution, UnwindPoint: 25}
		s := &StageState{ID: stages.MiningExecution, BlockNumber: 50}
		err = UnwindMiningExecutionStage(u, s, wrap.TxContainer{Tx: tx2}, ctx, cfg, logger)
		require.NoError(err)

		compareCurrentState(t, tx1, tx2, kv.PlainState, kv.PlainContractCode)
	})
	t.Run("UnwindMiningExecutionStagePlainWithCodeChanges", func(t *testing.T) {
		t.Skip("not supported yet, to be restored")
		require := require.New(t)
		tx1, _ := db1.BeginRw(context.Background())
		tx2, _ := db2.BeginRw(context.Background())
		defer func() {
			tx1.Rollback()
			tx2.Rollback()
		}()

		before1, after1, writer1 := apply(tx1, logger)
		before2, after2, writer2 := apply(tx2, logger)
		generateBlocks2(t, 1, 25, writer1, before1, after1, changeCodeIndepenentlyOfIncarnations)
		generateBlocks2(t, 1, 50, writer2, before2, after2, changeCodeIndepenentlyOfIncarnations)

		err := stages.SaveStageProgress(tx2, stages.MiningExecution, 50)
		if err != nil {
			t.Errorf("error while saving progress: %v", err)
		}
		u := &UnwindState{ID: stages.MiningExecution, UnwindPoint: 25}
		s := &StageState{ID: stages.MiningExecution, BlockNumber: 50}
		err = UnwindMiningExecutionStage(u, s, wrap.TxContainer{Tx: tx2}, ctx, cfg, logger)
		require.NoError(err)

		compareCurrentState(t, tx1, tx2, kv.PlainState, kv.PlainContractCode)
	})
}
