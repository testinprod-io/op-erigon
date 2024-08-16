// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package gasprice

import (
	"context"
	"math/big"
	"testing"

	"github.com/erigontech/erigon-lib/chain"
	libcommon "github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon/core"
	"github.com/erigontech/erigon/core/types"
	"github.com/erigontech/erigon/crypto"
	"github.com/erigontech/erigon/eth/gasprice/gaspricecfg"
	"github.com/erigontech/erigon/event"
	"github.com/erigontech/erigon/params"
	"github.com/erigontech/erigon/rpc"
	"github.com/holiman/uint256"
)

const (
	blockGasLimit = params.TxGas * 3
)

type testTxData struct {
	priorityFee int64
	gasLimit    uint64
}

type testCache struct {
	latestHash  libcommon.Hash
	latestPrice *big.Int
}

// GetLatest implements Cache.
func (c *testCache) GetLatest() (libcommon.Hash, *big.Int) {
	return c.latestHash, c.latestPrice
}

// SetLatest implements Cache.
func (c *testCache) SetLatest(hash libcommon.Hash, price *big.Int) {
	c.latestHash = hash
	c.latestPrice = price
}

type opTestBackend struct {
	block    *types.Block
	receipts []*types.Receipt
}

func (b *opTestBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	panic("not implemented")
}

func (b *opTestBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	return b.block, nil
}

func (b *opTestBackend) GetReceipts(ctx context.Context, block *types.Block) (types.Receipts, error) {
	return b.receipts, nil
}

func (b *opTestBackend) PendingBlockAndReceipts() (*types.Block, types.Receipts) {
	panic("not implemented")
}

func (b *opTestBackend) ChainConfig() *chain.Config {
	return params.OptimismTestConfig
}

func (b *opTestBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return nil
}

func newOpTestBackend(t *testing.T, txs []testTxData) *opTestBackend {
	var (
		key, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		signer = types.LatestSigner(params.TestChainConfig)
	)
	// only the most recent block is considered for optimism priority fee suggestions, so this is
	// where we add the test transactions
	ts := []types.Transaction{}
	rs := []*types.Receipt{}
	header := types.Header{}
	header.GasLimit = blockGasLimit
	var nonce uint64
	for _, tx := range txs {
		txdata := &types.DynamicFeeTransaction{
			ChainID:  uint256.MustFromBig(params.TestChainConfig.ChainID),
			FeeCap:   uint256.MustFromBig(big.NewInt(100 * params.GWei)),
			Tip:      uint256.MustFromBig(big.NewInt(tx.priorityFee)),
			CommonTx: types.CommonTx{},
		}
		t := types.MustSignNewTx(key, *signer, txdata)
		ts = append(ts, t)
		r := types.Receipt{}
		r.GasUsed = tx.gasLimit
		header.GasUsed += r.GasUsed
		rs = append(rs, &r)
		nonce++
	}
	// hasher := trie.NewStackTrie(nil)
	b := types.NewBlock(&header, ts, nil, rs, nil, nil)
	return &opTestBackend{block: b, receipts: rs}
}

func TestSuggestOptimismPriorityFee(t *testing.T) {
	minSuggestion := new(big.Int).SetUint64(1e8 * params.Wei)
	var cases = []struct {
		txdata []testTxData
		want   *big.Int
	}{
		{
			// block well under capacity, expect min priority fee suggestion
			txdata: []testTxData{{params.GWei, 21000}},
			want:   minSuggestion,
		},
		{
			// 2 txs, still under capacity, expect min priority fee suggestion
			txdata: []testTxData{{params.GWei, 21000}, {params.GWei, 21000}},
			want:   minSuggestion,
		},
		{
			// 2 txs w same priority fee (1 gwei), but second tx puts it right over capacity
			txdata: []testTxData{{params.GWei, 21000}, {params.GWei, 21001}},
			want:   big.NewInt(1100000000), // 10 percent over 1 gwei, the median
		},
		{
			// 3 txs, full block. return 10% over the median tx (10 gwei * 10% == 11 gwei)
			txdata: []testTxData{{10 * params.GWei, 21000}, {1 * params.GWei, 21000}, {100 * params.GWei, 21000}},
			want:   big.NewInt(11 * params.GWei),
		},
	}
	for i, c := range cases {
		backend := newOpTestBackend(t, c.txdata)
		oracle := NewOracle(backend, gaspricecfg.Config{MinSuggestedPriorityFee: minSuggestion}, &testCache{}, nil)
		got := oracle.SuggestOptimismPriorityFee(context.Background(), backend.block.Header(), backend.block.Hash())
		if got.Cmp(c.want) != 0 {
			t.Errorf("Gas price mismatch for test case %d: want %d, got %d", i, c.want, got)
		}
	}
}
