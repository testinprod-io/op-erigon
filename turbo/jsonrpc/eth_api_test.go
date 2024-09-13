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

package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	libcommon "github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/common/hexutil"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/erigontech/erigon-lib/common"

	"github.com/erigontech/erigon-lib/kv/kvcache"
	"github.com/erigontech/erigon/core"
	"github.com/erigontech/erigon/core/types"
	"github.com/erigontech/erigon/rpc"
	"github.com/erigontech/erigon/rpc/rpccfg"
	"github.com/erigontech/erigon/turbo/adapter/ethapi"
	"github.com/erigontech/erigon/turbo/stages/mock"

	"github.com/erigontech/erigon-lib/log/v3"

	"github.com/erigontech/erigon/cmd/rpcdaemon/rpcdaemontest"
)

func newBaseApiForTest(m *mock.MockSentry) *BaseAPI {
	stateCache := kvcache.New(kvcache.DefaultCoherentConfig)

	return NewBaseApi(nil, stateCache, m.BlockReader, false, rpccfg.DefaultEvmCallTimeout, m.Engine, m.Dirs, nil, nil, nil)
}

func TestGetBalanceChangesInBlock(t *testing.T) {
	assert := assert.New(t)
	myBlockNum := rpc.BlockNumberOrHashWithNumber(0)
	m, _, _ := rpcdaemontest.CreateTestSentry(t)
	db := m.DB
	api := NewErigonAPI(newBaseApiForTest(m), db, nil)
	balances, err := api.GetBalanceChangesInBlock(context.Background(), myBlockNum)
	if err != nil {
		t.Errorf("calling GetBalanceChangesInBlock resulted in an error: %v", err)
	}
	expected := map[common.Address]*hexutil.Big{
		common.HexToAddress("0x0D3ab14BBaD3D99F4203bd7a11aCB94882050E7e"): (*hexutil.Big)(uint256.NewInt(200000000000000000).ToBig()),
		common.HexToAddress("0x703c4b2bD70c169f5717101CaeE543299Fc946C7"): (*hexutil.Big)(uint256.NewInt(300000000000000000).ToBig()),
		common.HexToAddress("0x71562b71999873DB5b286dF957af199Ec94617F7"): (*hexutil.Big)(uint256.NewInt(9000000000000000000).ToBig()),
	}
	assert.Equal(len(expected), len(balances))
	for i := range balances {
		assert.Contains(expected, i, "%s is not expected to be present in the output.", i)
		assert.Equal(balances[i], expected[i], "the value for %s is expected to be %v, but got %v.", i, expected[i], balances[i])
	}
}

func TestGetTransactionReceipt(t *testing.T) {
	m, _, _ := rpcdaemontest.CreateTestSentry(t)
	db := m.DB
	stateCache := kvcache.New(kvcache.DefaultCoherentConfig)
	api := NewEthAPI(NewBaseApi(nil, stateCache, m.BlockReader, false, rpccfg.DefaultEvmCallTimeout, m.Engine, m.Dirs, nil, nil, nil), db, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())
	// Call GetTransactionReceipt for transaction which is not in the database
	if _, err := api.GetTransactionReceipt(context.Background(), common.Hash{}); err != nil {
		t.Errorf("calling GetTransactionReceipt with empty hash: %v", err)
	}
}

func TestGetTransactionReceiptUnprotected(t *testing.T) {
	m, _, _ := rpcdaemontest.CreateTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())
	// Call GetTransactionReceipt for un-protected transaction
	if _, err := api.GetTransactionReceipt(context.Background(), common.HexToHash("0x3f3cb8a0e13ed2481f97f53f7095b9cbc78b6ffb779f2d3e565146371a8830ea")); err != nil {
		t.Errorf("calling GetTransactionReceipt for unprotected tx: %v", err)
	}
}

// EIP-1898 test cases

func TestGetStorageAt_ByBlockNumber_WithRequireCanonicalDefault(t *testing.T) {
	assert := assert.New(t)
	m, _, _ := rpcdaemontest.CreateTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())
	addr := common.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")

	result, err := api.GetStorageAt(context.Background(), addr, "0x0", rpc.BlockNumberOrHashWithNumber(0))
	if err != nil {
		t.Errorf("calling GetStorageAt: %v", err)
	}

	assert.Equal(common.HexToHash("0x0").String(), result)
}

func TestGetStorageAt_ByBlockHash_WithRequireCanonicalDefault(t *testing.T) {
	assert := assert.New(t)
	m, _, _ := rpcdaemontest.CreateTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())
	addr := common.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")

	result, err := api.GetStorageAt(context.Background(), addr, "0x0", rpc.BlockNumberOrHashWithHash(m.Genesis.Hash(), false))
	if err != nil {
		t.Errorf("calling GetStorageAt: %v", err)
	}

	assert.Equal(common.HexToHash("0x0").String(), result)
}

func TestGetStorageAt_ByBlockHash_WithRequireCanonicalTrue(t *testing.T) {
	assert := assert.New(t)
	m, _, _ := rpcdaemontest.CreateTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())
	addr := common.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")

	result, err := api.GetStorageAt(context.Background(), addr, "0x0", rpc.BlockNumberOrHashWithHash(m.Genesis.Hash(), true))
	if err != nil {
		t.Errorf("calling GetStorageAt: %v", err)
	}

	assert.Equal(common.HexToHash("0x0").String(), result)
}

func TestGetStorageAt_ByBlockHash_WithRequireCanonicalDefault_BlockNotFoundError(t *testing.T) {
	m, _, _ := rpcdaemontest.CreateTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())
	addr := common.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")

	offChain, err := core.GenerateChain(m.ChainConfig, m.Genesis, m.Engine, m.DB, 1, func(i int, block *core.BlockGen) {
	})
	if err != nil {
		t.Fatal(err)
	}
	offChainBlock := offChain.Blocks[0]

	if _, err := api.GetStorageAt(context.Background(), addr, "0x0", rpc.BlockNumberOrHashWithHash(offChainBlock.Hash(), false)); err != nil {
		if fmt.Sprintf("%v", err) != fmt.Sprintf("block %s not found", offChainBlock.Hash().String()[2:]) {
			t.Errorf("wrong error: %v", err)
		}
	} else {
		t.Error("error expected")
	}
}

func TestGetStorageAt_ByBlockHash_WithRequireCanonicalTrue_BlockNotFoundError(t *testing.T) {
	m, _, _ := rpcdaemontest.CreateTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())
	addr := common.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")

	offChain, err := core.GenerateChain(m.ChainConfig, m.Genesis, m.Engine, m.DB, 1, func(i int, block *core.BlockGen) {
	})
	if err != nil {
		t.Fatal(err)
	}
	offChainBlock := offChain.Blocks[0]

	if _, err := api.GetStorageAt(context.Background(), addr, "0x0", rpc.BlockNumberOrHashWithHash(offChainBlock.Hash(), true)); err != nil {
		if fmt.Sprintf("%v", err) != fmt.Sprintf("block %s not found", offChainBlock.Hash().String()[2:]) {
			t.Errorf("wrong error: %v", err)
		}
	} else {
		t.Error("error expected")
	}
}

func TestGetStorageAt_ByBlockHash_WithRequireCanonicalDefault_NonCanonicalBlock(t *testing.T) {
	assert := assert.New(t)
	m, _, orphanedChain := rpcdaemontest.CreateTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())
	addr := common.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")

	orphanedBlock := orphanedChain[0].Blocks[0]

	result, err := api.GetStorageAt(context.Background(), addr, "0x0", rpc.BlockNumberOrHashWithHash(orphanedBlock.Hash(), false))
	if err != nil {
		if fmt.Sprintf("%v", err) != fmt.Sprintf("hash %s is not currently canonical", orphanedBlock.Hash().String()[2:]) {
			t.Errorf("wrong error: %v", err)
		}
	} else {
		t.Error("error expected")
	}

	assert.Equal(common.HexToHash("0x0").String(), result)
}

func TestGetStorageAt_ByBlockHash_WithRequireCanonicalTrue_NonCanonicalBlock(t *testing.T) {
	m, _, orphanedChain := rpcdaemontest.CreateTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())
	addr := common.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")

	orphanedBlock := orphanedChain[0].Blocks[0]

	if _, err := api.GetStorageAt(context.Background(), addr, "0x0", rpc.BlockNumberOrHashWithHash(orphanedBlock.Hash(), true)); err != nil {
		if fmt.Sprintf("%v", err) != fmt.Sprintf("hash %s is not currently canonical", orphanedBlock.Hash().String()[2:]) {
			t.Errorf("wrong error: %v", err)
		}
	} else {
		t.Error("error expected")
	}
}

func TestCall_ByBlockHash_WithRequireCanonicalDefault_NonCanonicalBlock(t *testing.T) {
	m, _, orphanedChain := rpcdaemontest.CreateTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())
	from := common.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")
	to := common.HexToAddress("0x0d3ab14bbad3d99f4203bd7a11acb94882050e7e")

	orphanedBlock := orphanedChain[0].Blocks[0]

	if _, err := api.Call(context.Background(), ethapi.CallArgs{
		From: &from,
		To:   &to,
	}, rpc.BlockNumberOrHashWithHash(orphanedBlock.Hash(), false), nil); err != nil {
		if fmt.Sprintf("%v", err) != fmt.Sprintf("hash %s is not currently canonical", orphanedBlock.Hash().String()[2:]) {
			/* Not sure. Here https://github.com/ethereum/EIPs/blob/master/EIPS/eip-1898.md it is not explicitly said that
			   eth_call should only work with canonical blocks.
			   But since there is no point in changing the state of non-canonical block, it ignores RequireCanonical. */
			t.Errorf("wrong error: %v", err)
		}
	} else {
		t.Error("error expected")
	}
}

func TestCall_ByBlockHash_WithRequireCanonicalTrue_NonCanonicalBlock(t *testing.T) {
	m, _, orphanedChain := rpcdaemontest.CreateTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())
	from := common.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")
	to := common.HexToAddress("0x0d3ab14bbad3d99f4203bd7a11acb94882050e7e")

	orphanedBlock := orphanedChain[0].Blocks[0]

	if _, err := api.Call(context.Background(), ethapi.CallArgs{
		From: &from,
		To:   &to,
	}, rpc.BlockNumberOrHashWithHash(orphanedBlock.Hash(), true), nil); err != nil {
		if fmt.Sprintf("%v", err) != fmt.Sprintf("hash %s is not currently canonical", orphanedBlock.Hash().String()[2:]) {
			t.Errorf("wrong error: %v", err)
		}
	} else {
		t.Error("error expected")
	}
}

func TestNewRPCTransactionDepositTx(t *testing.T) {
	tx := &types.DepositTx{
		SourceHash:          common.Hash{1},
		From:                common.Address{1},
		IsSystemTransaction: true,
		Mint:                uint256.NewInt(34),
		Value:               uint256.NewInt(1337),
	}
	nonce := uint64(12)
	depositNonce := &nonce
	receipt := &types.Receipt{DepositNonce: depositNonce}
	got := NewRPCTransaction(tx, common.Hash{}, uint64(12), uint64(1), big.NewInt(0), receipt)
	// Should provide zero values for unused fields that are required in other transactions
	require.Equal(t, got.GasPrice, (*hexutil.Big)(big.NewInt(0)), "NewRPCTransaction().GasPrice = %v, want 0x0", got.GasPrice)
	require.Equal(t, got.V.Uint64(), uint64(0), "NewRPCTransaction().V = %v, want 0x0", got.V)
	require.Equal(t, got.R.Uint64(), uint64(0), "NewRPCTransaction().R = %v, want 0x0", got.R)
	require.Equal(t, got.S.Uint64(), uint64(0), "NewRPCTransaction().S = %v, want 0x0", got.S)

	// Should include deposit tx specific fields
	require.Equal(t, got.SourceHash, &tx.SourceHash, "NewRPCTransaction().SourceHash = %v, want %v", got.SourceHash, tx.SourceHash)
	require.Equal(t, got.IsSystemTx, &tx.IsSystemTransaction, "NewRPCTransaction().IsSystemTx = %v, want %v", got.IsSystemTx, tx.IsSystemTransaction)
	require.Equal(t, got.Mint, (*hexutil.Big)(tx.Mint.ToBig()), "NewRPCTransaction().Mint = %v, want %v", got.Mint, tx.Mint.ToBig())
	require.Equal(t, got.Nonce, (hexutil.Uint64)(nonce), "NewRPCTransaction().Nonce = %v, want %v", got.Nonce, nonce)
}

func TestNewRPCTransactionDepositTxWithVersion(t *testing.T) {
	tx := &types.DepositTx{
		SourceHash:          common.Hash{1},
		From:                common.Address{1},
		IsSystemTransaction: true,
		Mint:                uint256.NewInt(34),
		Value:               uint256.NewInt(1337),
	}
	nonce := uint64(7)
	version := types.CanyonDepositReceiptVersion
	receipt := &types.Receipt{
		DepositNonce:          &nonce,
		DepositReceiptVersion: &version,
	}
	got := NewRPCTransaction(tx, libcommon.Hash{}, uint64(12), uint64(1), big.NewInt(0), receipt)
	// Should provide zero values for unused fields that are required in other transactions
	require.Equal(t, got.GasPrice, (*hexutil.Big)(big.NewInt(0)), "NewRPCTransaction().GasPrice = %v, want 0x0", got.GasPrice)
	require.Equal(t, got.V.Uint64(), uint64(0), "NewRPCTransaction().V = %v, want 0x0", got.V)
	require.Equal(t, got.R.Uint64(), uint64(0), "NewRPCTransaction().R = %v, want 0x0", got.R)
	require.Equal(t, got.S.Uint64(), uint64(0), "NewRPCTransaction().S = %v, want 0x0", got.S)

	// Should include versioned deposit tx specific fields
	require.Equal(t, got.SourceHash, &tx.SourceHash, "NewRPCTransaction().SourceHash = %v, want %v", got.SourceHash, tx.SourceHash)
	require.Equal(t, got.IsSystemTx, &tx.IsSystemTransaction, "NewRPCTransaction().IsSystemTx = %v, want %v", got.IsSystemTx, tx.IsSystemTransaction)
	require.Equal(t, got.Mint, (*hexutil.Big)(tx.Mint.ToBig()), "NewRPCTransaction().Mint = %v, want %v", got.Mint, tx.Mint.ToBig())
	require.Equal(t, got.Nonce, (hexutil.Uint64)(nonce), "NewRPCTransaction().Nonce = %v, want %v", got.Nonce, nonce)
	require.Equal(t, *got.DepositReceiptVersion, (hexutil.Uint64(version)), "NewRPCTransaction().DepositReceiptVersion = %v, want %v", *got.DepositReceiptVersion, version)

	// Make sure json marshal/unmarshal of the rpc tx preserves the receipt version
	b, err := json.Marshal(got)
	require.NoError(t, err, "marshalling failed: %w", err)
	parsed := make(map[string]interface{})
	err = json.Unmarshal(b, &parsed)
	require.NoError(t, err, "unmarshalling failed: %w", err)
	require.Equal(t, "0x1", parsed["depositReceiptVersion"])
}

func TestNewRPCTransactionOmitIsSystemTxFalse(t *testing.T) {
	tx := &types.DepositTx{
		IsSystemTransaction: false,
		From:                common.Address{1},
		Value:               uint256.NewInt(1337),
	}
	got := NewRPCTransaction(tx, common.Hash{}, uint64(12), uint64(1), big.NewInt(0), nil)

	require.Nil(t, got.IsSystemTx, "should omit IsSystemTx when false")
}

func TestUnmarshalRpcDepositTx(t *testing.T) {
	version := hexutil.Uint64(types.CanyonDepositReceiptVersion)
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
				tx.V = (*hexutil.Big)(common.Big0)
				tx.R = (*hexutil.Big)(common.Big0)
				tx.S = (*hexutil.Big)(common.Big0)
				tx.GasPrice = (*hexutil.Big)(common.Big0)
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
		{
			name: "Non-nil deposit receipt version",
			modifier: func(tx *RPCTransaction) {
				tx.DepositReceiptVersion = &version
			},
			valid: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &types.DepositTx{
				SourceHash:          common.Hash{1},
				From:                common.Address{1},
				IsSystemTransaction: true,
				Mint:                uint256.NewInt(34),
				Value:               uint256.NewInt(1337),
			}
			rpcTx := NewRPCTransaction(tx, common.Hash{}, uint64(12), uint64(1), big.NewInt(0), nil)
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
