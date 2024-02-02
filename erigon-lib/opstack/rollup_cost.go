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

// This file implements OP stack L1 cost calculation, based on op-geth
// https://github.com/ethereum-optimism/op-geth/commit/a290ca164a36c80a8d106d88bd482b6f82220bef

package opstack

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/chain"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/fixedgas"
	"github.com/ledgerwatch/erigon-lib/types"
	"github.com/ledgerwatch/log/v3"
)

const (
	// The two 4-byte Ecotone fee scalar values are packed into the same storage slot as the 8-byte
	// sequence number and have the following Solidity offsets within the slot. Note that Solidity
	// offsets correspond to the last byte of the value in the slot, counting backwards from the
	// end of the slot. For example, The 8-byte sequence number has offset 0, and is therefore
	// stored as big-endian format in bytes [24:32] of the slot.
	BaseFeeScalarSlotOffset     = 12 // bytes [16:20] of the slot
	BlobBaseFeeScalarSlotOffset = 8  // bytes [20:24] of the slot

	// scalarSectionStart is the beginning of the scalar values segment in the slot
	// array. baseFeeScalar is in the first four bytes of the segment, blobBaseFeeScalar the next
	// four.
	scalarSectionStart = 32 - BaseFeeScalarSlotOffset - 4

	LegacyL1InfoBytes  = 4 + 32*8
	EcotoneL1InfoBytes = 164
)

func init() {
	if BlobBaseFeeScalarSlotOffset != BaseFeeScalarSlotOffset-4 {
		panic("this code assumes the scalars are at adjacent positions in the scalars slot")
	}
}

var (
	// BedrockL1AttributesSelector is the function selector indicating Bedrock style L1 gas
	// attributes.
	BedrockL1AttributesSelector = []byte{0x01, 0x5d, 0x8e, 0xb9}
	// EcotoneL1AttributesSelector is the selector indicating Ecotone style L1 gas attributes.
	EcotoneL1AttributesSelector = []byte{0x44, 0x0a, 0x5e, 0x20}

	// L1BlockAddr is the address of the L1Block contract which stores the L1 gas attributes.
	L1BlockAddr = libcommon.HexToAddress("0x4200000000000000000000000000000000000015")

	L1BaseFeeSlot = libcommon.BigToHash(big.NewInt(1))
	OverheadSlot  = libcommon.BigToHash(big.NewInt(5))
	ScalarSlot    = libcommon.BigToHash(big.NewInt(6))

	// L2BlobBaseFeeSlot was added with the Ecotone upgrade and stores the blobBaseFee L1 gas
	// attribute.
	L1BlobBaseFeeSlot = libcommon.BigToHash(big.NewInt(7))
	// L1FeeScalarsSlot as of the Ecotone upgrade stores the 32-bit baseFeeScalar and
	// blobBaseFeeScalar L1 gas attributes at offsets `BaseFeeScalarSlotOffset` and
	// `BlobBaseFeeScalarSlotOffset` respectively.
	L1FeeScalarsSlot = libcommon.BigToHash(big.NewInt(3))

	oneMillion     = uint256.NewInt(1_000_000)
	ecotoneDivisor = uint256.NewInt(1_000_000 * 16)
	sixteen        = uint256.NewInt(16)

	emptyScalars = make([]byte, 8)
)

type StateGetter interface {
	GetState(addr libcommon.Address, key *libcommon.Hash, value *uint256.Int)
}

// L1CostFunc is used in the state transition to determine the data availability fee charged to the
// sender of non-Deposit transactions.  It returns nil if no data availability fee is charged.
type L1CostFunc func(rcd types.RollupCostData, blockTime uint64) *uint256.Int

// l1CostFunc is an internal version of L1CostFunc that also returns the gasUsed for use in
// receipts.
type l1CostFunc func(rcd types.RollupCostData) (fee, gasUsed *uint256.Int)

// NewL1CostFunc returns a function used for calculating data availability fees, or nil if this is
// not an op-stack chain.
func NewL1CostFunc(config *chain.Config, statedb StateGetter) L1CostFunc {
	if config.Optimism == nil {
		return nil
	}
	forBlock := ^uint64(0)
	var cachedFunc l1CostFunc
	return func(rollupCostData types.RollupCostData, blockTime uint64) *uint256.Int {
		if rollupCostData == (types.RollupCostData{}) {
			return nil // Do not charge if there is no rollup cost-data (e.g. RPC call or deposit).
		}
		if forBlock != blockTime {
			if forBlock != ^uint64(0) {
				// best practice is not to re-use l1 cost funcs across different blocks, but we
				// make it work just in case.
				log.Info("l1 cost func re-used for different L1 block", "oldTime", forBlock, "newTime", blockTime)
			}
			forBlock = blockTime
			// Note: the various state variables below are not initialized from the DB until this
			// point to allow deposit transactions from the block to be processed first by state
			// transition.  This behavior is consensus critical!
			if !config.IsOptimismEcotone(blockTime) {
				cachedFunc = newL1CostFuncBedrock(config, statedb, blockTime)
			} else {
				var l1BlobBaseFee, l1FeeScalarsInt uint256.Int
				statedb.GetState(L1BlockAddr, &L1BlobBaseFeeSlot, &l1BlobBaseFee)
				statedb.GetState(L1BlockAddr, &L1FeeScalarsSlot, &l1FeeScalarsInt)
				l1FeeScalars := l1FeeScalarsInt.Bytes32()

				// Edge case: the very first Ecotone block requires we use the Bedrock cost
				// function. We detect this scenario by checking if the Ecotone parameters are
				// unset.  Not here we rely on assumption that the scalar parameters are adjacent
				// in the buffer and baseFeeScalar comes first.
				if l1BlobBaseFee.BitLen() == 0 &&
					bytes.Equal(emptyScalars, l1FeeScalars[scalarSectionStart:scalarSectionStart+8]) {
					log.Info("using bedrock l1 cost func for first Ecotone block", "time", blockTime)
					cachedFunc = newL1CostFuncBedrock(config, statedb, blockTime)
				} else {
					var l1BaseFee uint256.Int
					statedb.GetState(L1BlockAddr, &L1BaseFeeSlot, &l1BaseFee)
					offset := scalarSectionStart
					l1BaseFeeScalar := new(uint256.Int).SetBytes(l1FeeScalars[offset : offset+4])
					l1BlobBaseFeeScalar := new(uint256.Int).SetBytes(l1FeeScalars[offset+4 : offset+8])
					cachedFunc = newL1CostFuncEcotone(&l1BaseFee, &l1BlobBaseFee, l1BaseFeeScalar, l1BlobBaseFeeScalar)
				}
			}
		}
		fee, _ := cachedFunc(rollupCostData)
		return fee
	}
}

// newL1CostFuncBedrock returns an L1 cost function suitable for Bedrock, Regolith, and the first
// block only of the Ecotone upgrade.
func newL1CostFuncBedrock(config *chain.Config, statedb StateGetter, blockTime uint64) l1CostFunc {
	var l1BaseFee, overhead, scalar uint256.Int
	statedb.GetState(L1BlockAddr, &L1BaseFeeSlot, &l1BaseFee)
	statedb.GetState(L1BlockAddr, &OverheadSlot, &overhead)
	statedb.GetState(L1BlockAddr, &ScalarSlot, &scalar)
	isRegolith := config.IsRegolith(blockTime)
	return newL1CostFuncBedrockHelper(&l1BaseFee, &overhead, &scalar, isRegolith)
}

// newL1CostFuncBedrockHelper is lower level version of newL1CostFuncBedrock that expects already
// extracted parameters
func newL1CostFuncBedrockHelper(l1BaseFee, overhead, scalar *uint256.Int, isRegolith bool) l1CostFunc {
	return func(rollupCostData types.RollupCostData) (fee, gasUsed *uint256.Int) {
		if rollupCostData == (types.RollupCostData{}) {
			return nil, nil // Do not charge if there is no rollup cost-data (e.g. RPC call or deposit)
		}
		gas := rollupCostData.Zeroes * fixedgas.TxDataZeroGas
		if isRegolith {
			gas += rollupCostData.Ones * fixedgas.TxDataNonZeroGasEIP2028
		} else {
			gas += (rollupCostData.Ones + 68) * fixedgas.TxDataNonZeroGasEIP2028
		}
		gasWithOverhead := uint256.NewInt(gas)
		gasWithOverhead.Add(gasWithOverhead, overhead)
		l1Cost := l1CostHelper(gasWithOverhead, l1BaseFee, scalar)
		return l1Cost, gasWithOverhead
	}
}

// newL1CostFuncEcotone returns an l1 cost function suitable for the Ecotone upgrade except for the
// very first block of the upgrade.
func newL1CostFuncEcotone(l1BaseFee, l1BlobBaseFee, l1BaseFeeScalar, l1BlobBaseFeeScalar *uint256.Int) l1CostFunc {
	return func(costData types.RollupCostData) (fee, calldataGasUsed *uint256.Int) {
		calldataGas := (costData.Zeroes * fixedgas.TxDataZeroGas) + (costData.Ones * fixedgas.TxDataNonZeroGasEIP2028)
		calldataGasUsed = new(uint256.Int).SetUint64(calldataGas)

		// Ecotone L1 cost function:
		//
		//   (calldataGas/16)*(l1BaseFee*16*l1BaseFeeScalar + l1BlobBaseFee*l1BlobBaseFeeScalar)/1e6
		//
		// We divide "calldataGas" by 16 to change from units of calldata gas to "estimated # of bytes when
		// compressed". Known as "compressedTxSize" in the spec.
		//
		// Function is actually computed as follows for better precision under integer arithmetic:
		//
		//   calldataGas*(l1BaseFee*16*l1BaseFeeScalar + l1BlobBaseFee*l1BlobBaseFeeScalar)/16e6

		calldataCostPerByte := new(uint256.Int).Set(l1BaseFee)
		calldataCostPerByte = calldataCostPerByte.Mul(calldataCostPerByte, sixteen)
		calldataCostPerByte = calldataCostPerByte.Mul(calldataCostPerByte, l1BaseFeeScalar)

		blobCostPerByte := new(uint256.Int).Set(l1BlobBaseFee)
		blobCostPerByte = blobCostPerByte.Mul(blobCostPerByte, l1BlobBaseFeeScalar)

		fee = new(uint256.Int).Add(calldataCostPerByte, blobCostPerByte)
		fee = fee.Mul(fee, calldataGasUsed)
		fee = fee.Div(fee, ecotoneDivisor)

		return fee, calldataGasUsed
	}
}

// ExtractL1GasParams extracts the gas parameters necessary to compute gas costs from L1 block info
// calldata.
func ExtractL1GasParams(config *chain.Config, time uint64, data []byte) (l1BaseFee *uint256.Int, costFunc l1CostFunc, feeScalar *big.Float, err error) {
	if config.IsEcotone(time) {
		// edge case: for the very first Ecotone block we still need to use the Bedrock
		// function. We detect this edge case by seeing if the function selector is the old one
		if len(data) >= 4 && !bytes.Equal(data[0:4], BedrockL1AttributesSelector) {
			l1BaseFee, costFunc, err = extractL1GasParamsEcotone(data)
			return
		}
	}
	return extractL1GasParamsLegacy(config.IsRegolith(time), data)
}

func extractL1GasParamsLegacy(isRegolith bool, data []byte) (l1BaseFee *uint256.Int, costFunc l1CostFunc, feeScalar *big.Float, err error) {
	// data consists of func selector followed by 7 ABI-encoded parameters (32 bytes each)
	if len(data) < LegacyL1InfoBytes {
		return nil, nil, nil, fmt.Errorf("expected at least %d L1 info bytes, got %d", LegacyL1InfoBytes, len(data))
	}
	data = data[4:]                                          // trim function selector
	l1BaseFee = new(uint256.Int).SetBytes(data[32*2 : 32*3]) // arg index 2
	overhead := new(uint256.Int).SetBytes(data[32*6 : 32*7]) // arg index 6
	scalar := new(uint256.Int).SetBytes(data[32*7 : 32*8])   // arg index 7
	fscalar := new(big.Float).SetInt(scalar.ToBig())         // legacy: format fee scalar as big Float
	fdivisor := new(big.Float).SetUint64(1_000_000)          // 10**6, i.e. 6 decimals
	feeScalar = new(big.Float).Quo(fscalar, fdivisor)
	costFunc = newL1CostFuncBedrockHelper(l1BaseFee, overhead, scalar, isRegolith)
	return l1BaseFee, costFunc, feeScalar, nil
}

// extractEcotoneL1GasParams extracts the gas parameters necessary to compute gas from L1 attribute
// info calldata after the Ecotone upgrade, but not for the very first Ecotone block.
func extractL1GasParamsEcotone(data []byte) (l1BaseFee *uint256.Int, costFunc l1CostFunc, err error) {
	if len(data) != EcotoneL1InfoBytes {
		return nil, nil, fmt.Errorf("expected 164 L1 info bytes, got %d", len(data))
	}
	// data layout assumed for Ecotone:
	// offset type varname
	// 0      <selector>
	// 4     uint32 _baseFeeScalar
	// 8     uint32 _blobBaseFeeScalar
	// 12    uint64 _sequenceNumber,
	// 20    uint64 _timestamp,
	// 28    uint64 _l1BlockNumber
	// 36    uint256 _baseFee,
	// 68    uint256 _blobBaseFee,
	// 100    bytes32 _hash,
	// 132   bytes32 _batcherHash,
	l1BaseFee = new(uint256.Int).SetBytes(data[36:68])
	l1BlobBaseFee := new(uint256.Int).SetBytes(data[68:100])
	l1BaseFeeScalar := new(uint256.Int).SetBytes(data[4:8])
	l1BlobBaseFeeScalar := new(uint256.Int).SetBytes(data[8:12])
	costFunc = newL1CostFuncEcotone(l1BaseFee, l1BlobBaseFee, l1BaseFeeScalar, l1BlobBaseFeeScalar)
	return
}

// L1Cost computes the the data availability fee for transactions in blocks prior to the Ecotone
// upgrade. It is used by e2e tests so must remain exported.
func L1Cost(rollupDataGas uint64, l1BaseFee, overhead, scalar *uint256.Int) *uint256.Int {
	l1GasUsed := uint256.NewInt(rollupDataGas)
	l1GasUsed.Add(l1GasUsed, overhead)
	return l1CostHelper(l1GasUsed, l1BaseFee, scalar)
}

func l1CostHelper(gasWithOverhead, l1BaseFee, scalar *uint256.Int) *uint256.Int {
	fee := new(uint256.Int).Set(gasWithOverhead)
	fee.Mul(fee, l1BaseFee).Mul(fee, scalar).Div(fee, oneMillion)
	return fee
}

func L1CostFnForTxPool(data []byte) (types.L1CostFn, error) {
	var costFunc l1CostFunc
	var err error
	if len(data) == EcotoneL1InfoBytes {
		_, costFunc, err = extractL1GasParamsEcotone(data)
		if err != nil {
			return nil, err
		}
	} else {
		// This method is used for txpool that is needed for OP sequencer.
		// Assume that isRegolith will be always true for new block.
		_, costFunc, _, err = extractL1GasParamsLegacy(true, data)
		if err != nil {
			return nil, err
		}
	}
	return func(tx *types.TxSlot) *uint256.Int {
		fee, _ := costFunc(tx.RollupCostData)
		return fee
	}, nil
}
