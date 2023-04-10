package ethapi

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/common/hexutil"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/stretchr/testify/require"
)

func TestNewRPCTransactionDepositTx(t *testing.T) {
	tx := &types.DepositTx{
		SourceHash:          libcommon.HexToHash("0x1234"),
		IsSystemTransaction: true,
		Mint:                uint256.NewInt(34),
		Value:               uint256.NewInt(1337),
	}
	nonce := uint64(7)
	depositNonce := &nonce
	got := newRPCTransaction(tx, libcommon.Hash{}, uint64(12), uint64(1), big.NewInt(0), depositNonce)
	// Should provide zero values for unused fields that are required in other transactions
	require.Equal(t, got.GasPrice, (*hexutil.Big)(big.NewInt(0)), "newRPCTransaction().GasPrice = %v, want 0x0", got.GasPrice)
	require.Equal(t, got.V, (*hexutil.Big)(big.NewInt(0)), "newRPCTransaction().V = %v, want 0x0", got.V)
	require.Equal(t, got.R, (*hexutil.Big)(big.NewInt(0)), "newRPCTransaction().R = %v, want 0x0", got.R)
	require.Equal(t, got.S, (*hexutil.Big)(big.NewInt(0)), "newRPCTransaction().S = %v, want 0x0", got.S)

	// Should include deposit tx specific fields
	require.Equal(t, *got.SourceHash, tx.SourceHash, "newRPCTransaction().SourceHash = %v, want %v", got.SourceHash, tx.SourceHash)
	require.Equal(t, *got.IsSystemTx, tx.IsSystemTransaction, "newRPCTransaction().IsSystemTransaction = %v, want %v", got.IsSystemTx, tx.IsSystemTransaction)
	require.Equal(t, got.Mint, (*hexutil.Big)(tx.Mint.ToBig()), "newRPCTransaction().Mint = %v, want %v", got.Mint, tx.Mint.ToBig())
	require.Equal(t, got.Nonce, (hexutil.Uint64)(nonce), "newRPCTransaction().Mint = %v, want %v", got.Nonce, nonce)
}

func TestNewRPCTransactionOmitIsSystemTxFalse(t *testing.T) {
	tx := &types.DepositTx{
		IsSystemTransaction: false,
		Value:               uint256.NewInt(1337),
	}
	got := newRPCTransaction(tx, libcommon.Hash{}, uint64(12), uint64(1), big.NewInt(0), nil)

	require.Nil(t, got.IsSystemTx, "should omit IsSystemTx when false")
}

func TestUnmarshalRpcDepositTx(t *testing.T) {
	tests := []struct {
		name     string
		modifier func(tx *RPCTransaction)
		valid    bool
	}{
		{
			name:     "Unmodified",
			modifier: func(tx *RPCTransaction) {},
			valid:    true,
		},
		{
			name: "Zero Values",
			modifier: func(tx *RPCTransaction) {
				tx.V = (*hexutil.Big)(libcommon.Big0)
				tx.R = (*hexutil.Big)(libcommon.Big0)
				tx.S = (*hexutil.Big)(libcommon.Big0)
				tx.GasPrice = (*hexutil.Big)(libcommon.Big0)
			},
			valid: true,
		},
		{
			name: "Nil Values",
			modifier: func(tx *RPCTransaction) {
				tx.V = nil
				tx.R = nil
				tx.S = nil
				tx.GasPrice = nil
			},
			valid: true,
		},
		{
			name: "Non-Zero GasPrice",
			modifier: func(tx *RPCTransaction) {
				tx.GasPrice = (*hexutil.Big)(big.NewInt(43))
			},
			valid: false,
		},
		{
			name: "Non-Zero V",
			modifier: func(tx *RPCTransaction) {
				tx.V = (*hexutil.Big)(big.NewInt(43))
			},
			valid: false,
		},
		{
			name: "Non-Zero R",
			modifier: func(tx *RPCTransaction) {
				tx.R = (*hexutil.Big)(big.NewInt(43))
			},
			valid: false,
		},
		{
			name: "Non-Zero S",
			modifier: func(tx *RPCTransaction) {
				tx.S = (*hexutil.Big)(big.NewInt(43))
			},
			valid: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &types.DepositTx{
				SourceHash:          libcommon.HexToHash("0x1234"),
				IsSystemTransaction: true,
				Mint:                uint256.NewInt(34),
				Value:               uint256.NewInt(1337),
			}
			rpcTx := newRPCTransaction(tx, libcommon.Hash{}, uint64(12), uint64(1), big.NewInt(0), nil)
			test.modifier(rpcTx)
			json, err := json.Marshal(rpcTx)
			require.NoError(t, err, "marshalling failed: %w", err)
			parsed := &types.DepositTx{}
			err = parsed.UnmarshalJSON(json)
			if test.valid {
				require.NoError(t, err, "unmarshal failed: %w", err)
			} else {
				require.Error(t, err, "unmarshal should have failed but did not")
			}
		})
	}
}
