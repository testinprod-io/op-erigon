package engineapi

import (
	"errors"
	"github.com/erigontech/erigon-lib/chain"
	"github.com/erigontech/erigon/consensus/misc"
	"github.com/erigontech/erigon/turbo/engineapi/engine_types"
)

// checkOptimismPayload performs Optimism-specific checks on the payload data (called during [(*EngineServer).newPayload]).
func checkOptimismPayload(params *engine_types.ExecutionPayload, cfg *chain.Config) error {
	// (non)-nil withdrawals is already checked by Shanghai rules.
	// Canyon - empty withdrawals
	if cfg.IsCanyon(params.Timestamp.Uint64()) {
		if len(params.Withdrawals) != 0 {
			return errors.New("non-empty withdrawals post-Canyon")
		}
	}

	// Holocene - extraData
	if cfg.IsHolocene(params.Timestamp.Uint64()) {
		if err := misc.ValidateHoloceneExtraData(params.ExtraData); err != nil {
			return err
		}
	} else if len(params.ExtraData) > 0 { // pre-Holocene
		return errors.New("extraData must be empty before Holocene")
	}

	// Isthmus - withdrawalsRoot
	if cfg.IsIsthmus(params.Timestamp.Uint64()) {
		if params.WithdrawalsRoot == nil {
			return errors.New("nil withdrawalsRoot post-Isthmus")
		}
	} else if params.WithdrawalsRoot != nil { // pre-Isthmus
		return errors.New("non-nil withdrawalsRoot pre-Isthmus")
	}

	return nil
}

// checkOptimismPayloadAttributes performs Optimism-specific checks on the payload attributes (called during [(*EngineServer).forkchoiceUpdated].
// Will panic if payloadAttributes is nil.
func checkOptimismPayloadAttributes(payloadAttributes *engine_types.PayloadAttributes, cfg *chain.Config) error {
	if payloadAttributes.GasLimit == nil {
		return errors.New("gasLimit parameter is required")
	}

	// (non)-nil withdrawals is already checked by Shanghai rules.
	// Canyon - empty withdrawals
	if cfg.IsCanyon(payloadAttributes.Timestamp.Uint64()) {
		if len(payloadAttributes.Withdrawals) != 0 {
			return errors.New("non-empty withdrawals post-Canyon")
		}
	}

	// Holocene - extraData
	if cfg.IsHolocene(payloadAttributes.Timestamp.Uint64()) {
		if err := misc.ValidateHolocene1559Params(payloadAttributes.EIP1559Params); err != nil {
			return err
		}
	} else if len(payloadAttributes.EIP1559Params) != 0 { // pre-Holocene
		return errors.New("non-empty eip155Params pre-Holocene")
	}

	// Note: PayloadAttributes don't contain the Isthmus withdrawalsRoot, it's set during block assembly.

	return nil
}
