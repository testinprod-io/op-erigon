package types

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/chain"
	"github.com/ledgerwatch/erigon/params"
	"github.com/stretchr/testify/require"
)

// This file is based on op-geth
// https://github.com/ethereum-optimism/op-geth/commit/e4177034f5bec308de5b9b53b0bf7b2d9381f4d3

func TestL1CostFunc(t *testing.T) {
	basefee := uint256.NewInt(1)
	overhead := uint256.NewInt(1)
	scalar := uint256.NewInt(1_000_000)

	costFunc0 := newL1CostFunc(basefee, overhead, scalar, false /*isRegolith*/)
	costFunc1 := newL1CostFunc(basefee, overhead, scalar, true)

	// emptyTx is a test tx defined in transaction_test.go
	c0, g0 := costFunc0(emptyTx.RollupCostData()) // pre-Regolith
	c1, g1 := costFunc1(emptyTx.RollupCostData())
	require.Equal(t, uint256.NewInt(1569), c0)
	require.Equal(t, uint256.NewInt(1569), g0) // gas-used == fee since scalars are all 1
	require.Equal(t, uint256.NewInt(481), c1)
	require.Equal(t, uint256.NewInt(481), g1)
}

func TestExtractGasParams(t *testing.T) {
	regolithTime := new(big.Int).SetUint64(1)
	config := &chain.Config{
		Optimism:     params.OptimismTestConfig.Optimism,
		RegolithTime: regolithTime,
	}

	selector := []byte{0x01, 0x5d, 0x8e, 0xb9}
	emptyU256 := make([]byte, 32)

	ignored := big.NewInt(1234)
	basefee := big.NewInt(1)
	overhead := big.NewInt(1)
	scalar := big.NewInt(1_000_000)

	data := []byte{}
	data = append(data, selector...)                      // selector
	data = append(data, ignored.FillBytes(emptyU256)...)  // arg 0
	data = append(data, ignored.FillBytes(emptyU256)...)  // arg 1
	data = append(data, basefee.FillBytes(emptyU256)...)  // arg 2
	data = append(data, ignored.FillBytes(emptyU256)...)  // arg 3
	data = append(data, ignored.FillBytes(emptyU256)...)  // arg 4
	data = append(data, ignored.FillBytes(emptyU256)...)  // arg 5
	data = append(data, overhead.FillBytes(emptyU256)...) // arg 6

	// try to extract from data which has not enough params, should get error.
	_, _, _, err := extractL1GasParams(config, regolithTime.Uint64(), data)
	require.Error(t, err)

	data = append(data, scalar.FillBytes(emptyU256)...) // arg 7

	// now it should succeed
	_, costFuncPreRegolith, _, err := extractL1GasParams(config, regolithTime.Uint64()-1, data)
	require.NoError(t, err)

	// Function should continue to succeed even with extra data (that just gets ignored) since we
	// have been testing the data size is at least the expected number of bytes instead of exactly
	// the expected number of bytes. It's unclear if this flexibility was intentional, but since
	// it's been in production we shouldn't change this behavior.
	data = append(data, ignored.FillBytes(emptyU256)...) // extra ignored arg
	_, costFuncRegolith, _, err := extractL1GasParams(config, regolithTime.Uint64(), data)
	require.NoError(t, err)

	c, _ := costFuncPreRegolith(emptyTx.RollupCostData())
	require.Equal(t, uint256.NewInt(1569), c)

	c, _ = costFuncRegolith(emptyTx.RollupCostData())
	require.Equal(t, uint256.NewInt(481), c)
}
