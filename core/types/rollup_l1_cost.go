// Copyright 2022 The go-ethereum Authors
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

package types

import (
	"github.com/holiman/uint256"
	"math/big"

	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/params"
)

type RollupMessage interface {
	RollupDataGas() uint64
	IsDepositTx() bool
}

type StateGetter interface {
	GetState(addr common.Address, key *common.Hash, value *uint256.Int)
}

// L1CostFunc is used in the state transition to determine the cost of a rollup message.
// Returns nil if there is no cost.
type L1CostFunc func(blockNum uint64, msg RollupMessage) *uint256.Int

var (
	L1BaseFeeSlot = common.BigToHash(big.NewInt(1))
	OverheadSlot  = common.BigToHash(big.NewInt(5))
	ScalarSlot    = common.BigToHash(big.NewInt(6))
)

var L1BlockAddr = common.HexToAddress("0x4200000000000000000000000000000000000015")

// NewL1CostFunc returns a function used for calculating L1 fee cost.
// This depends on the oracles because gas costs can change over time.
// It returns nil if there is no applicable cost function.
func NewL1CostFunc(config *params.ChainConfig, statedb StateGetter) L1CostFunc {
	cacheBlockNum := ^uint64(0)
	var l1BaseFee, overhead, scalar uint256.Int
	return func(blockNum uint64, msg RollupMessage) *uint256.Int {
		rollupDataGas := msg.RollupDataGas() // Only fake txs for RPC view-calls are 0.
		if config.Optimism == nil || msg.IsDepositTx() || rollupDataGas == 0 {
			return nil
		}
		if blockNum != cacheBlockNum {
			statedb.GetState(L1BlockAddr, &L1BaseFeeSlot, &l1BaseFee)
			statedb.GetState(L1BlockAddr, &OverheadSlot, &overhead)
			statedb.GetState(L1BlockAddr, &ScalarSlot, &scalar)
			cacheBlockNum = blockNum
		}
		return L1Cost(rollupDataGas, &l1BaseFee, &overhead, &scalar)
	}
}

func L1Cost(rollupDataGas uint64, l1BaseFee, overhead, scalar *uint256.Int) *uint256.Int {
	l1GasUsed := new(uint256.Int).SetUint64(rollupDataGas)
	l1GasUsed = l1GasUsed.Add(l1GasUsed, overhead)
	l1Cost := l1GasUsed.Mul(l1GasUsed, l1BaseFee)
	l1Cost = l1Cost.Mul(l1Cost, scalar)
	return l1Cost.Div(l1Cost, uint256.NewInt(1_000_000))
}
