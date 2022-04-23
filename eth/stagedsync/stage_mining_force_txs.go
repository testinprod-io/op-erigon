package stagedsync

import (
	"fmt"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/core/types"
)

func SpawnMiningForceTxsStage(s *StageState, tx kv.RwTx, cfg MiningCreateBlockCfg, quit <-chan struct{}) (err error) {
	var forceTxs []types.Transaction
	for i, otx := range cfg.blockProposerParameters.Transactions {
		tx, err := types.UnmarshalTransactionFromBinary(otx)
		if err != nil {
			return fmt.Errorf("tx %d is invalid: %v", i, err)
		}
		forceTxs = append(forceTxs, tx)
	}
	cfg.miner.MiningBlock.ForceTxs = types.NewTransactionsFixedOrder(forceTxs)
	return nil
}
