package engineapi

import (
	"errors"
	"github.com/erigontech/erigon-lib/chain"
	"github.com/erigontech/erigon-lib/common/hexutil"
	"github.com/erigontech/erigon/core/types"
	"github.com/erigontech/erigon/turbo/engineapi/engine_types"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func preCanyon() *chain.Config {
	cfg := new(chain.Config)
	// Mark as an Optimism chain so IsOptimismFoo is true when FooTime is active
	cfg.Optimism = &chain.OptimismConfig{}
	return cfg
}

func postCanyon() *chain.Config {
	cfg := preCanyon()
	cfg.CanyonTime = new(big.Int)
	return cfg
}

func postHolocene() *chain.Config {
	cfg := postCanyon()
	cfg.HoloceneTime = new(big.Int)
	return cfg
}

func postIsthmus() *chain.Config {
	cfg := postHolocene()
	cfg.IsthmusTime = new(big.Int)
	return cfg
}

var valid1559Params = []byte{0, 1, 2, 3, 4, 5, 6, 7}
var validExtraData = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8}
var emptyWithdrawals = make([]*types.Withdrawal, 0)
var validJovianExtraData = append(append([]byte{1}, valid1559Params...), make([]byte, 8)...) // version=1, 8 bytes params, 8 byte minBaseFee

func TestCheckOptimismPayload(t *testing.T) {
	tests := []struct {
		name     string
		params   engine_types.ExecutionPayload
		cfg      *chain.Config
		expected error
	}{
		{
			name: "valid payload pre-Canyon",
			params: engine_types.ExecutionPayload{
				ExtraData: []byte{},
			},
			cfg:      preCanyon(),
			expected: nil,
		},
		{
			name: "valid payload post-Canyon",
			params: engine_types.ExecutionPayload{
				ExtraData:   []byte{},
				Withdrawals: emptyWithdrawals,
			},
			cfg: postCanyon(),
		},
		{
			name: "invalid empty withdrawals post-Canyon",
			params: engine_types.ExecutionPayload{
				ExtraData:   []byte{},
				Withdrawals: make([]*types.Withdrawal, 1),
			},
			cfg:      postCanyon(),
			expected: errors.New("non-empty withdrawals post-Canyon"),
		},
		{
			name: "non-nil withdrawalsRoot pre-Isthmus",
			params: engine_types.ExecutionPayload{
				ExtraData:       []byte{},
				Withdrawals:     emptyWithdrawals,
				WithdrawalsRoot: &types.EmptyRootHash,
			},
			cfg:      postCanyon(),
			expected: errors.New("non-nil withdrawalsRoot pre-Isthmus"),
		},
		{
			name: "invalid non-empty extraData pre-Holocene",
			params: engine_types.ExecutionPayload{
				ExtraData:   []byte{1, 2, 3},
				Withdrawals: emptyWithdrawals,
			},
			cfg:      postCanyon(),
			expected: errors.New("extraData must be empty before Holocene"),
		},
		{
			name: "invalid extraData post-Holocene",
			params: engine_types.ExecutionPayload{
				ExtraData:   []byte{1, 2, 3},
				Withdrawals: emptyWithdrawals,
			},
			cfg:      postHolocene(),
			expected: errors.New("holocene extraData should be 9 bytes, got 3"),
		},
		{
			name: "valid payload post-Holocene with extraData",
			params: engine_types.ExecutionPayload{
				ExtraData:   validExtraData,
				Withdrawals: emptyWithdrawals,
			},
			cfg:      postHolocene(),
			expected: nil,
		},
		{
			name: "invalid non-nil withdrawalsRoot post-Holocene",
			params: engine_types.ExecutionPayload{
				ExtraData:       validExtraData,
				Withdrawals:     emptyWithdrawals,
				WithdrawalsRoot: &types.EmptyRootHash,
			},
			cfg:      postHolocene(),
			expected: errors.New("non-nil withdrawalsRoot pre-Isthmus"),
		},
		{
			name: "valid payload post-Isthmus",
			params: engine_types.ExecutionPayload{
				ExtraData:       validExtraData,
				Withdrawals:     emptyWithdrawals,
				WithdrawalsRoot: &types.EmptyRootHash,
			},
			cfg:      postIsthmus(),
			expected: nil,
		},
		{
			name: "invalid nil withdrawals root post-isthmus",
			params: engine_types.ExecutionPayload{
				Withdrawals: emptyWithdrawals,
				ExtraData:   validExtraData,
			},
			cfg:      postIsthmus(),
			expected: errors.New("nil withdrawalsRoot post-Isthmus"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkOptimismPayload(&test.params, test.cfg)
			if test.expected == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, test.expected.Error())
			}
		})
	}
}

func TestCheckOptimismPayloadAttributes(t *testing.T) {
	tests := []struct {
		name              string
		payloadAttributes *engine_types.PayloadAttributes
		cfg               *chain.Config
		expected          error
		shouldPanic       bool
	}{
		{
			name:              "nil payload attributes panic",
			payloadAttributes: nil,
			cfg:               preCanyon(),
			shouldPanic:       true,
		},
		{
			name: "valid payload attributes pre-Canyon",
			payloadAttributes: &engine_types.PayloadAttributes{
				GasLimit: new(hexutil.Uint64),
			},
			cfg:      preCanyon(),
			expected: nil,
		},
		{
			name: "invalid nil gasLimit",
			payloadAttributes: &engine_types.PayloadAttributes{
				GasLimit: nil,
			},
			cfg:      preCanyon(),
			expected: errors.New("gasLimit parameter is required"),
		},
		{
			name: "invalid non-empty withdrawals post-Canyon",
			payloadAttributes: &engine_types.PayloadAttributes{
				GasLimit:      new(hexutil.Uint64),
				EIP1559Params: valid1559Params,
				Withdrawals:   make([]*types.Withdrawal, 1),
			},
			cfg:      postCanyon(),
			expected: errors.New("non-empty withdrawals post-Canyon"),
		},
		{
			name: "invalid non-empty eip1559Params pre-Holocene",
			payloadAttributes: &engine_types.PayloadAttributes{
				GasLimit:      new(hexutil.Uint64),
				EIP1559Params: valid1559Params,
			},
			cfg:      postCanyon(),
			expected: errors.New("non-empty eip155Params pre-Holocene"),
		},
		{
			name: "invalid eip1559Params post-Holocene",
			payloadAttributes: &engine_types.PayloadAttributes{
				GasLimit:      new(hexutil.Uint64),
				EIP1559Params: append(valid1559Params, 77),
			},
			cfg:      postHolocene(),
			expected: errors.New("holocene eip-1559 params should be 8 bytes, got 9"),
		},
		{
			name: "valid payload attributes post-Holocene",
			payloadAttributes: &engine_types.PayloadAttributes{
				GasLimit:      new(hexutil.Uint64),
				EIP1559Params: valid1559Params,
			},
			cfg:      postHolocene(),
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.shouldPanic {
				require.Panics(t, func() {
					checkOptimismPayloadAttributes(test.payloadAttributes, test.cfg)
				})
			} else {
				err := checkOptimismPayloadAttributes(test.payloadAttributes, test.cfg)
				if test.expected == nil {
					require.NoError(t, err)
				} else {
					require.EqualError(t, err, test.expected.Error())
				}
			}
		})
	}
}
