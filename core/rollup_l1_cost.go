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

package core

import (
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon/core/vm"
	"math/big"

	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/params"
)

var big10 = uint256.NewInt(10)

var (
	L1BaseFeeSlot = common.BigToHash(big.NewInt(1))
	OverheadSlot  = common.BigToHash(big.NewInt(3))
	ScalarSlot    = common.BigToHash(big.NewInt(4))
	DecimalsSlot  = common.BigToHash(big.NewInt(5))
)

var (
	OVM_GasPriceOracleAddr = common.HexToAddress("0x420000000000000000000000000000000000000F")
	L1BlockAddr            = common.HexToAddress("0x4200000000000000000000000000000000000015")
)

// NewL1CostFunc returns a function used for calculating L1 fee cost.
// This depends on the oracles because gas costs can change over time.
// It returns nil if there is no applicable cost function.
func NewL1CostFunc(config *params.ChainConfig, statedb *state.IntraBlockState) vm.L1CostFunc {
	cacheBlockNum := ^uint64(0)
	var l1BaseFee, overhead, scalar, decimals, divisor uint256.Int
	return func(blockNum uint64, msg vm.RollupMessage) *uint256.Int {
		rollupDataGas := msg.RollupDataGas() // Only fake txs for RPC view-calls are 0.
		if config.Optimism == nil || msg.Nonce() == types.DepositsNonce || rollupDataGas == 0 {
			return nil
		}
		if blockNum != cacheBlockNum {
			statedb.GetState(L1BlockAddr, &L1BaseFeeSlot, &l1BaseFee)
			statedb.GetState(OVM_GasPriceOracleAddr, &OverheadSlot, &overhead)
			statedb.GetState(OVM_GasPriceOracleAddr, &ScalarSlot, &scalar)
			statedb.GetState(OVM_GasPriceOracleAddr, &DecimalsSlot, &decimals)
			divisor.Exp(big10, &decimals)
			cacheBlockNum = blockNum
		}
		var l1GasUsed uint256.Int
		l1GasUsed.SetUint64(rollupDataGas)
		l1GasUsed.Add(&l1GasUsed, &overhead)
		l1Cost := l1GasUsed.Mul(&l1GasUsed, &l1BaseFee)
		l1Cost = l1Cost.Mul(l1Cost, &scalar)
		l1Cost = l1Cost.Div(l1Cost, &divisor)
		return l1Cost
	}
}
