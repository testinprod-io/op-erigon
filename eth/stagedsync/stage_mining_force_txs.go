package stagedsync

import (
	"fmt"
	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/log/v3"
	"github.com/erigontech/erigon/core/types"
)

func SpawnMiningForceTxsStage(s *StageState, tx kv.RwTx, cfg MiningCreateBlockCfg, quit <-chan struct{}) (err error) {
	var forceTxs []types.Transaction
	if cfg.blockBuilderParameters != nil {
		log.Info("stage running - force txs, with params",
			"txs", len(cfg.blockBuilderParameters.Transactions),
			"notxpool", cfg.blockBuilderParameters.NoTxPool)
		for i, otx := range cfg.blockBuilderParameters.Transactions {
			tx, err := types.UnmarshalTransactionFromBinary(otx, false)
			if err != nil {
				return fmt.Errorf("tx %d is invalid: %v", i, err)
			}
			forceTxs = append(forceTxs, tx)
		}
	} else {
		log.Info("stage running - force txs, nil params")
	}
	cfg.miner.MiningBlock.ForceTxs = types.NewTransactionsFixedOrder(forceTxs)
	return nil
}
