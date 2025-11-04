package stagedsync

import (
	"fmt"

	"github.com/erigontech/erigon-lib/chain"
	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/log/v3"
	"github.com/erigontech/erigon-lib/opstack"
	"github.com/erigontech/erigon/consensus"
	"github.com/erigontech/erigon/core/types"
	"github.com/erigontech/erigon/turbo/builder"
	"github.com/erigontech/erigon/turbo/services"
)

type MiningFinishCfg struct {
	db                    kv.RwDB
	chainConfig           chain.Config
	engine                consensus.Engine
	sealCancel            chan struct{}
	miningState           MiningState
	blockReader           services.FullBlockReader
	latestBlockBuiltStore *builder.LatestBlockBuiltStore
}

func StageMiningFinishCfg(
	db kv.RwDB,
	chainConfig chain.Config,
	engine consensus.Engine,
	miningState MiningState,
	sealCancel chan struct{},
	blockReader services.FullBlockReader,
	latestBlockBuiltStore *builder.LatestBlockBuiltStore,
) MiningFinishCfg {
	return MiningFinishCfg{
		db:                    db,
		chainConfig:           chainConfig,
		engine:                engine,
		miningState:           miningState,
		sealCancel:            sealCancel,
		blockReader:           blockReader,
		latestBlockBuiltStore: latestBlockBuiltStore,
	}
}

func SpawnMiningFinishStage(s *StageState, tx kv.RwTx, cfg MiningFinishCfg, quit <-chan struct{}, logger log.Logger) error {
	logPrefix := s.LogPrefix()
	current := cfg.miningState.MiningBlock

	// Short circuit when receiving duplicate result caused by resubmitting.
	//if w.chain.HasBlock(block.Hash(), block.NumberU64()) {
	//	continue
	//}

	// Store DA footprint in BlobGasUsed header field if it hasn't already been set yet.
	// Builder code may already calculate it during block building to avoid recalculating it here.
	header := current.Header
	if cfg.chainConfig.IsJovian(header.Time) && (header.BlobGasUsed == nil || *header.BlobGasUsed == 0) {
		daFootprint, err := CalcDAFootprint(current.Txs)
		if err != nil {
			return fmt.Errorf("error calculating DA footprint: %w", err)
		}
		header.BlobGasUsed = &daFootprint
	}

	block := types.NewBlockForAsembling(current.Header, current.Txs, current.Uncles, current.Receipts, current.Withdrawals, cfg.chainConfig.IsOptimismIsthmus(current.Header.Time))
	blockWithReceipts := &types.BlockWithReceipts{Block: block, Receipts: current.Receipts, Requests: current.Requests}
	*current = MiningBlock{} // hack to clean global data

	//sealHash := engine.SealHash(block.Header())
	// Reject duplicate sealing work due to resubmitting.
	//if sealHash == prev {
	//	s.Done()
	//	return nil
	//}
	//prev = sealHash
	cfg.latestBlockBuiltStore.AddBlockBuilt(block)

	// Tests may set pre-calculated nonce
	if block.NonceU64() != 0 {
		// Note: To propose a new signer for Clique consensus, the block nonce should be set to 0xFFFFFFFFFFFFFFFF.
		if cfg.engine.Type() != chain.CliqueConsensus {
			cfg.miningState.MiningResultCh <- blockWithReceipts
			return nil
		}
	}

	cfg.miningState.PendingResultCh <- block

	if block.Transactions().Len() > 0 {
		logger.Info(fmt.Sprintf("[%s] block ready for seal", logPrefix),
			"block", block.NumberU64(),
			"transactions", block.Transactions().Len(),
			"gasUsed", block.GasUsed(),
			"gasLimit", block.GasLimit(),
			"difficulty", block.Difficulty(),
		)
	}
	// interrupt aborts the in-flight sealing task.
	select {
	case cfg.sealCancel <- struct{}{}:
	default:
		logger.Trace("No in-flight sealing task.")
	}
	chain := ChainReader{Cfg: cfg.chainConfig, Db: tx, BlockReader: cfg.blockReader, Logger: logger}
	if err := cfg.engine.Seal(chain, blockWithReceipts, cfg.miningState.MiningResultCh, cfg.sealCancel); err != nil {
		logger.Warn("Block sealing failed", "err", err)
	}

	return nil
}

// CalcDAFootprint calculates the total DA footprint of a block for an OP Stack chain.
// Jovian introduces a DA footprint block limit which is stored in the BlobGasUsed header field and that is taken
// into account during base fee updates.
// CalcDAFootprint must not be called for pre-Jovian blocks.
func CalcDAFootprint(txs types.Transactions) (uint64, error) {
	if txs.Len() == 0 || txs[0].Type() != types.DepositTxType {
		return 0, fmt.Errorf("missing deposit transaction")
	}

	// First Jovian block doesn't set the DA footprint gas scalar yet and
	// it must not have user transactions.
	data := txs[0].GetData()
	if len(data) == opstack.IsthmusL1AttributesLen {
		if txs[len(txs)-1].Type() != types.DepositTxType {
			// sufficient to check last transaction because deposits precede non-deposit txs
			return 0, fmt.Errorf("unexpected non-deposit transactions in Jovian activation block")
		}
		return 0, nil
	} // ExtractDAFootprintGasScalar catches all invalid lengths

	daFootprintGasScalar, err := opstack.ExtractDAFootprintGasScalar(data)
	if err != nil {
		return 0, err
	}
	var daFootprint uint64
	for _, tx := range txs {
		if tx.Type() == types.DepositTxType {
			continue
		}
		daFootprint += tx.RollupCostData().EstimatedDASize().Uint64() * uint64(daFootprintGasScalar)
	}
	return daFootprint, nil
}
