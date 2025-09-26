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
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/erigontech/erigon-lib/chain"
	"github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/common/fixedgas"
	"github.com/erigontech/erigon-lib/log/v3"
	"github.com/holiman/uint256"
)

const (
	// The two 4-byte Ecotone fee scalar values are packed into the same storage slot as the 8-byte
	// sequence number and have the following Solidity offsets within the slot. Note that Solidity
	// offsets correspond to the last byte of the value in the slot, counting backwards from the
	// end of the slot. For example, The 8-byte sequence number has offset 0, and is therefore
	// stored as big-endian format in bytes [24:32) of the slot.
	BaseFeeScalarSlotOffset     = 12 // bytes [16:20) of the slot
	BlobBaseFeeScalarSlotOffset = 8  // bytes [20:24) of the slot

	// scalarSectionStart is the beginning of the scalar values segment in the slot
	// array. baseFeeScalar is in the first four bytes of the segment, blobBaseFeeScalar the next
	// four.
	scalarSectionStart = 32 - BaseFeeScalarSlotOffset - 4

	PreEcotoneL1InfoBytes  = 4 + 32*8
	PostEcotoneL1InfoBytes = 164
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
	// IsthmusL1AttributesSelector is the selector indicating Isthmus style L1 gas attributes.
	IsthmusL1AttributesSelector = []byte{0x09, 0x89, 0x99, 0xbe}

	// L1BlockAddr is the address of the L1Block contract which stores the L1 gas attributes.
	L1BlockAddr = common.HexToAddress("0x4200000000000000000000000000000000000015")

	L1BaseFeeSlot = common.BigToHash(big.NewInt(1))
	OverheadSlot  = common.BigToHash(big.NewInt(5))
	ScalarSlot    = common.BigToHash(big.NewInt(6))

	// L1BlobBaseFeeSlot was added with the Ecotone upgrade and stores the blobBaseFee L1 gas
	// attribute.
	L1BlobBaseFeeSlot = common.BigToHash(big.NewInt(7))
	// L1FeeScalarsSlot as of the Ecotone upgrade stores the 32-bit baseFeeScalar and
	// blobBaseFeeScalar L1 gas attributes at offsets `BaseFeeScalarSlotOffset` and
	// `BlobBaseFeeScalarSlotOffset` respectively.
	L1FeeScalarsSlot = common.BigToHash(big.NewInt(3))

	oneMillion     = uint256.NewInt(1_000_000)
	ecotoneDivisor = uint256.NewInt(1_000_000 * 16)
	fjordDivisor   = uint256.NewInt(1_000_000_000_000)
	sixteen        = uint256.NewInt(16)

	// Do not use L1CostIntercept because it is negative and not fits in uint256
	L1CostInterceptNeg = uint256.NewInt(42_585_600)
	L1CostFastlzCoef   = uint256.NewInt(836_500)

	MinTransactionSize       = uint256.NewInt(100)
	MinTransactionSizeScaled = new(uint256.Int).Mul(MinTransactionSize, uint256.NewInt(1e6))

	emptyScalars = make([]byte, 8)

	// OperatorFeeParamsSlot stores the operatorFeeScalar and operatorFeeConstant L1 gas
	// attributes
	OperatorFeeParamsSlot = common.BigToHash(big.NewInt(8))
)

type StateGetter interface {
	GetState(addr common.Address, key *common.Hash, value *uint256.Int) error
}

// RollupCostData is a transaction structure that caches data for quickly computing the data
// availability costs for the transaction.
type RollupCostData struct {
	Zeroes, Ones uint64
	FastLzSize   uint64
}

type L1CostFn func(rcd RollupCostData) *uint256.Int

// L1CostFunc is used in the state transition to determine the data availability fee charged to the
// sender of non-Deposit transactions.  It returns nil if no data availability fee is charged.
type L1CostFunc func(rcd RollupCostData, blockTime uint64) *uint256.Int

// l1CostFunc is an internal version of L1CostFunc that also returns the gasUsed for use in
// receipts.
type l1CostFunc func(rcd RollupCostData) (fee, gasUsed *uint256.Int)

// OperatorCostFunc is used in the state transition to determine the operator fee charged to the
// sender of non-Deposit transactions. It returns 0 if no operator fee is charged.
type OperatorCostFunc func(gasUsed uint64, blockTime uint64) *uint256.Int

// operatorCostFunc is an internal version of OperatorCostFunc that is used for caching.
type operatorCostFunc func(gasUsed uint64) *uint256.Int

// A RollupTransaction provides all the input data needed to compute the total rollup cost.
type RollupTransaction interface {
	RollupCostData() RollupCostData
	Gas() uint64
}

// TotalRollupCostFunc is used in the transaction pool to determine the total rollup cost,
// including both the data availability fee and the operator fee. It returns nil if both costs are nil.
type TotalRollupCostFunc func(tx RollupTransaction, blockTime uint64) *uint256.Int

// NewL1CostFunc returns a function used for calculating data availability fees, or nil if this is
// not an op-stack chain.
func NewL1CostFunc(config *chain.Config, statedb StateGetter) L1CostFunc {
	if config.Optimism == nil {
		return nil
	}
	forBlock := ^uint64(0)
	var cachedFunc l1CostFunc
	selectFunc := func(blockTime uint64) l1CostFunc {
		if !config.IsOptimismEcotone(blockTime) {
			return newL1CostFuncPreEcotone(config, statedb, blockTime)
		}

		// Note: the various state variables below are not initialized from the DB until this
		// point to allow deposit transactions from the block to be processed first by state
		// transition.  This behavior is consensus critical!
		var l1FeeScalarsInt, l1BlobBaseFee, l1BaseFee uint256.Int
		statedb.GetState(L1BlockAddr, &L1FeeScalarsSlot, &l1FeeScalarsInt)
		l1FeeScalars := l1FeeScalarsInt.Bytes32()
		statedb.GetState(L1BlockAddr, &L1BlobBaseFeeSlot, &l1BlobBaseFee)
		statedb.GetState(L1BlockAddr, &L1BaseFeeSlot, &l1BaseFee)

		// Edge case: the very first Ecotone block requires we use the Bedrock cost
		// function. We detect this scenario by checking if the Ecotone parameters are
		// unset. Note here we rely on assumption that the scalar parameters are adjacent
		// in the buffer and l1BaseFeeScalar comes first. We need to check this prior to
		// other forks, as the first block of Fjord and Ecotone could be the same block.
		firstEcotoneBlock := l1BlobBaseFee.BitLen() == 0 &&
			bytes.Equal(emptyScalars, l1FeeScalars[scalarSectionStart:scalarSectionStart+8])
		if firstEcotoneBlock {
			log.Info("using pre-ecotone l1 cost func for first Ecotone block", "time", blockTime)
			return newL1CostFuncPreEcotone(config, statedb, blockTime)
		}

		l1BaseFeeScalar, l1BlobBaseFeeScalar := extractEcotoneFeeParams(l1FeeScalars[:])

		if config.IsOptimismFjord(blockTime) {
			return NewL1CostFuncFjord(
				&l1BaseFee,
				&l1BlobBaseFee,
				l1BaseFeeScalar,
				l1BlobBaseFeeScalar,
			)
		} else {
			return newL1CostFuncEcotone(&l1BaseFee, &l1BlobBaseFee, l1BaseFeeScalar, l1BlobBaseFeeScalar)
		}
	}

	return func(rollupCostData RollupCostData, blockTime uint64) *uint256.Int {
		if rollupCostData == (RollupCostData{}) {
			return nil // Do not charge if there is no rollup cost-data (e.g. RPC call or deposit).
		}
		if forBlock != blockTime {
			cachedFunc = selectFunc(blockTime)
		}
		fee, _ := cachedFunc(rollupCostData)
		return fee
	}
}

// newL1CostFuncPreEcotone returns an L1 cost function suitable for Bedrock, Regolith, and the first
// block only of the Ecotone upgrade.
func newL1CostFuncPreEcotone(config *chain.Config, statedb StateGetter, blockTime uint64) l1CostFunc {
	var l1BaseFee, overhead, scalar uint256.Int
	statedb.GetState(L1BlockAddr, &L1BaseFeeSlot, &l1BaseFee)
	statedb.GetState(L1BlockAddr, &OverheadSlot, &overhead)
	statedb.GetState(L1BlockAddr, &ScalarSlot, &scalar)
	isRegolith := config.IsRegolith(blockTime)
	return newL1CostFuncPreEcotoneHelper(&l1BaseFee, &overhead, &scalar, isRegolith)
}

// newL1CostFuncPreEcotoneHelper is lower level version of newL1CostFuncPreEcotone that expects already
// extracted parameters
func newL1CostFuncPreEcotoneHelper(l1BaseFee, overhead, scalar *uint256.Int, isRegolith bool) l1CostFunc {
	return func(rollupCostData RollupCostData) (fee, gasUsed *uint256.Int) {
		if rollupCostData == (RollupCostData{}) {
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
		l1Cost := l1CostPreEcotoneHelper(gasWithOverhead, l1BaseFee, scalar)
		return l1Cost, gasWithOverhead
	}
}

// newL1CostFuncEcotone returns an l1 cost function suitable for the Ecotone upgrade except for the
// very first block of the upgrade.
func newL1CostFuncEcotone(l1BaseFee, l1BlobBaseFee, l1BaseFeeScalar, l1BlobBaseFeeScalar *uint256.Int) l1CostFunc {
	return func(costData RollupCostData) (fee, calldataGasUsed *uint256.Int) {
		calldataGasUsed = bedrockCalldataGasUsed(costData)

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

// extractL1GasParams extracts the gas parameters necessary to compute gas costs from L1 block info
func ExtractL1GasParams(config *chain.Config, time uint64, data []byte) (gasParams, error) {
	if config.IsOptimismIsthmus(time) && len(data) >= 4 && !bytes.Equal(data[0:4], EcotoneL1AttributesSelector) {
		// edge case: for the very first Isthmus block we still need to use the Ecotone
		// function. We detect this edge case by seeing if the function selector is the old one
		// If so, fall through to the pre-isthmus format
		p, err := extractL1GasParamsPostIsthmus(data)
		if err != nil {
			return gasParams{}, err
		}
		p.CostFunc = NewL1CostFuncFjord(
			p.L1BaseFee,
			p.L1BlobBaseFee,
			new(uint256.Int).SetUint64(uint64(*p.L1BaseFeeScalar)),
			new(uint256.Int).SetUint64(uint64(*p.L1BlobBaseFeeScalar)),
		)
		return p, nil
	} else if config.IsEcotone(time) && len(data) >= 4 && !bytes.Equal(data[0:4], BedrockL1AttributesSelector) {
		// edge case: for the very first Ecotone block we still need to use the Bedrock
		// function. We detect this edge case by seeing if the function selector is the old one
		// If so, fall through to the pre-ecotone format
		// Both Ecotone and Fjord use the same function selector
		p, err := extractL1GasParamsPostEcotone(data)
		if err != nil {
			return gasParams{}, err
		}

		if config.IsFjord(time) {
			p.CostFunc = NewL1CostFuncFjord(
				p.L1BaseFee,
				p.L1BlobBaseFee,
				new(uint256.Int).SetUint64(uint64(*p.L1BaseFeeScalar)),
				new(uint256.Int).SetUint64(uint64(*p.L1BlobBaseFeeScalar)),
			)
		} else {
			p.CostFunc = newL1CostFuncEcotone(
				p.L1BaseFee,
				p.L1BlobBaseFee,
				new(uint256.Int).SetUint64(uint64(*p.L1BaseFeeScalar)),
				new(uint256.Int).SetUint64(uint64(*p.L1BlobBaseFeeScalar)),
			)
		}
		return p, nil
	}
	return extractL1GasParamsPreEcotone(config, time, data)
}

func extractL1InfoPreEcotone(data []byte) (l1BaseFee, overhead, scalar *uint256.Int, feeScalar *big.Float, err error) {
	// data consists of func selector followed by 7 ABI-encoded parameters (32 bytes each)
	if len(data) < PreEcotoneL1InfoBytes {
		return nil, nil, nil, nil, fmt.Errorf("expected at least %d L1 info bytes, got %d", PreEcotoneL1InfoBytes, len(data))
	}
	data = data[4:]                                          // trim function selector
	l1BaseFee = new(uint256.Int).SetBytes(data[32*2 : 32*3]) // arg index 2
	overhead = new(uint256.Int).SetBytes(data[32*6 : 32*7])  // arg index 6
	scalar = new(uint256.Int).SetBytes(data[32*7 : 32*8])    // arg index 7
	feeScalar = intToScaledFloat(scalar)                     // legacy: format fee scalar as big Float
	return
}

func extractL1GasParamsPreEcotone(config *chain.Config, time uint64, data []byte) (gasParams, error) {
	l1BaseFee, overhead, scalar, feeScalar, err := extractL1InfoPreEcotone(data)
	if err != nil {
		return gasParams{}, err
	}
	costFunc := newL1CostFuncPreEcotoneHelper(l1BaseFee, overhead, scalar, config.IsRegolith(time))
	return gasParams{
		L1BaseFee: l1BaseFee,
		CostFunc:  costFunc,
		FeeScalar: feeScalar,
	}, nil
}

func extractL1InfoPostEcotone(data []byte) (l1BaseFee, l1BlobBaseFee *uint256.Int, l1BaseFeeScalar, l1BlobBaseFeeScalar uint32, err error) {
	if len(data) != PostEcotoneL1InfoBytes {
		return nil, nil, 0, 0, fmt.Errorf("expected 164 L1 info bytes, got %d", len(data))
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
	l1BlobBaseFee = new(uint256.Int).SetBytes(data[68:100])
	l1BaseFeeScalar = binary.BigEndian.Uint32(data[4:8])
	l1BlobBaseFeeScalar = binary.BigEndian.Uint32(data[8:12])
	return
}

// extractL1GasParamsPostEcotone extracts the gas parameters necessary to compute gas from L1 attribute
// info calldata after the Ecotone upgrade, but not for the very first Ecotone block.
func extractL1GasParamsPostEcotone(data []byte) (gasParams, error) {
	l1BaseFee, l1BlobBaseFee, l1BaseFeeScalar, l1BlobBaseFeeScalar, err := extractL1InfoPostEcotone(data)
	if err != nil {
		return gasParams{}, err
	}
	return gasParams{
		L1BaseFee:           l1BaseFee,
		L1BlobBaseFee:       l1BlobBaseFee,
		L1BaseFeeScalar:     &l1BaseFeeScalar,
		L1BlobBaseFeeScalar: &l1BlobBaseFeeScalar,
	}, nil
}

func extractL1InfoPostIsthmus(data []byte) (l1BaseFee, l1BlobBaseFee *uint256.Int, l1BaseFeeScalar, l1BlobBaseFeeScalar, operatorFeeScalar uint32, operatorFeeConstant uint64, err error) {
	if len(data) != 176 {
		return nil, nil, 0, 0, 0, 0, fmt.Errorf("expected 176 L1 info bytes, got %d", len(data))
	}
	// data layout assumed for Isthmus:
	// offset type varname
	// 0     <selector>
	// 4     uint32 _basefeeScalar
	// 8     uint32 _blobBaseFeeScalar
	// 12    uint64 _sequenceNumber,
	// 20    uint64 _timestamp,
	// 28    uint64 _l1BlockNumber
	// 36    uint256 _basefee,
	// 68    uint256 _blobBaseFee,
	// 100   bytes32 _hash,
	// 132   bytes32 _batcherHash,
	// 164   uint32  _operatorFeeScalar
	// 168   uint64  _operatorFeeConstant
	l1BaseFee = new(uint256.Int).SetBytes(data[36:68])
	l1BlobBaseFee = new(uint256.Int).SetBytes(data[68:100])
	l1BaseFeeScalar = binary.BigEndian.Uint32(data[4:8])
	l1BlobBaseFeeScalar = binary.BigEndian.Uint32(data[8:12])
	operatorFeeScalar = binary.BigEndian.Uint32(data[164:168])
	operatorFeeConstant = binary.BigEndian.Uint64(data[168:176])
	return
}

// extractL1GasParamsPostIsthmus extracts the gas parameters necessary to compute gas from L1 attribute
// info calldata after the Isthmus upgrade, but not for the very first Isthmus block.
func extractL1GasParamsPostIsthmus(data []byte) (gasParams, error) {
	l1BaseFee, l1BlobBaseFee, l1BaseFeeScalar, l1BlobBaseFeeScalar, operatorFeeScalar, operatorFeeConstant, err := extractL1InfoPostIsthmus(data)
	if err != nil {
		return gasParams{}, err
	}

	return gasParams{
		L1BaseFee:           l1BaseFee,
		L1BlobBaseFee:       l1BlobBaseFee,
		L1BaseFeeScalar:     &l1BaseFeeScalar,
		L1BlobBaseFeeScalar: &l1BlobBaseFeeScalar,
		OperatorFeeScalar:   &operatorFeeScalar,
		OperatorFeeConstant: &operatorFeeConstant,
	}, nil
}

type gasParams struct {
	L1BaseFee           *uint256.Int
	L1BlobBaseFee       *uint256.Int
	CostFunc            l1CostFunc
	FeeScalar           *big.Float // pre-ecotone
	L1BaseFeeScalar     *uint32    // post-ecotone
	L1BlobBaseFeeScalar *uint32    // post-ecotone
	OperatorFeeScalar   *uint32    // post-Isthmus
	OperatorFeeConstant *uint64    // post-Isthmus
}

// intToScaledFloat returns scalar/10e6 as a float
func intToScaledFloat(scalar *uint256.Int) *big.Float {
	fscalar := new(big.Float).SetInt(scalar.ToBig())
	fdivisor := new(big.Float).SetUint64(1_000_000) // 10**6, i.e. 6 decimals
	return new(big.Float).Quo(fscalar, fdivisor)
}

// L1CostPreEcotone computes the the data availability fee for transactions in blocks prior to the Ecotone
// upgrade. It is used by e2e tests so must remain exported.
func L1CostPreEcotone(rollupDataGas uint64, l1BaseFee, overhead, scalar *uint256.Int) *uint256.Int {
	l1GasUsed := uint256.NewInt(rollupDataGas)
	l1GasUsed.Add(l1GasUsed, overhead)
	return l1CostPreEcotoneHelper(l1GasUsed, l1BaseFee, scalar)
}

func l1CostPreEcotoneHelper(gasWithOverhead, l1BaseFee, scalar *uint256.Int) *uint256.Int {
	fee := new(uint256.Int).Set(gasWithOverhead)
	fee.Mul(fee, l1BaseFee).Mul(fee, scalar).Div(fee, oneMillion)
	return fee
}

// NewL1CostFuncFjord returns an l1 cost function suitable for the Fjord upgrade
func NewL1CostFuncFjord(l1BaseFee, l1BlobBaseFee, baseFeeScalar, blobFeeScalar *uint256.Int) l1CostFunc {
	return func(costData RollupCostData) (fee, calldataGasUsed *uint256.Int) {
		// Fjord L1 cost function:
		//l1FeeScaled = baseFeeScalar*l1BaseFee*16 + blobFeeScalar*l1BlobBaseFee
		//estimatedSize = max(minTransactionSize, intercept + fastlzCoef*fastlzSize)
		//l1Cost = estimatedSize * l1FeeScaled / 1e12

		scaledL1BaseFee := new(uint256.Int).Mul(baseFeeScalar, l1BaseFee)
		calldataCostPerByte := new(uint256.Int).Mul(scaledL1BaseFee, sixteen)
		blobCostPerByte := new(uint256.Int).Mul(blobFeeScalar, l1BlobBaseFee)
		l1FeeScaled := new(uint256.Int).Add(calldataCostPerByte, blobCostPerByte)

		fastLzSize := new(uint256.Int).SetUint64(costData.FastLzSize)

		// Check L1CostIntercept + L1CostFastlzCoef * fastLzSize >= 0, or
		//       L1CostFastlzCoef * fastLzSize >= L1CostInterceptNeg
		estimatedSize := new(uint256.Int)
		temp := new(uint256.Int).Mul(L1CostFastlzCoef, fastLzSize)
		if temp.Cmp(L1CostInterceptNeg) < 0 {
			// estimatedSize is negative. fall back to MinTransactionSizeScaled
			estimatedSize.Set(MinTransactionSizeScaled)
		} else {
			// we can safely evaulate avoiding underflow
			estimatedSize = new(uint256.Int).Sub(temp, L1CostInterceptNeg)
			if estimatedSize.Cmp(MinTransactionSizeScaled) < 0 {
				estimatedSize.Set(MinTransactionSizeScaled)
			}
		}

		l1CostScaled := new(uint256.Int).Mul(estimatedSize, l1FeeScaled)
		l1Cost := new(uint256.Int).Div(l1CostScaled, fjordDivisor)

		calldataGasUsed = new(uint256.Int).Mul(estimatedSize, new(uint256.Int).SetUint64(fixedgas.TxDataNonZeroGasEIP2028))
		calldataGasUsed.Div(calldataGasUsed, uint256.NewInt(1e6))

		return l1Cost, calldataGasUsed
	}
}

func extractEcotoneFeeParams(l1FeeParams []byte) (l1BaseFeeScalar, l1BlobBaseFeeScalar *uint256.Int) {
	offset := scalarSectionStart
	l1BaseFeeScalar = new(uint256.Int).SetBytes(l1FeeParams[offset : offset+4])
	l1BlobBaseFeeScalar = new(uint256.Int).SetBytes(l1FeeParams[offset+4 : offset+8])
	return
}

func bedrockCalldataGasUsed(costData RollupCostData) (calldataGasUsed *uint256.Int) {
	calldataGas := (costData.Zeroes * fixedgas.TxDataZeroGas) + (costData.Ones * fixedgas.TxDataNonZeroGasEIP2028)
	return new(uint256.Int).SetUint64(calldataGas)
}

// FlzCompressLen returns the length of the data after compression through FastLZ, based on
// https://github.com/Vectorized/solady/blob/5315d937d79b335c668896d7533ac603adac5315/js/solady.js
func FlzCompressLen(ib []byte) uint32 {
	n := uint32(0)
	ht := make([]uint32, 8192)
	u24 := func(i uint32) uint32 {
		return uint32(ib[i]) | (uint32(ib[i+1]) << 8) | (uint32(ib[i+2]) << 16)
	}
	cmp := func(p uint32, q uint32, e uint32) uint32 {
		l := uint32(0)
		for e -= q; l < e; l++ {
			if ib[p+l] != ib[q+l] {
				e = 0
			}
		}
		return l
	}
	literals := func(r uint32) {
		n += 0x21 * (r / 0x20)
		r %= 0x20
		if r != 0 {
			n += r + 1
		}
	}
	match := func(l uint32) {
		l--
		n += 3 * (l / 262)
		if l%262 >= 6 {
			n += 3
		} else {
			n += 2
		}
	}
	hash := func(v uint32) uint32 {
		return ((2654435769 * v) >> 19) & 0x1fff
	}
	setNextHash := func(ip uint32) uint32 {
		ht[hash(u24(ip))] = ip
		return ip + 1
	}
	a := uint32(0)
	ipLimit := uint32(len(ib)) - 13
	if len(ib) < 13 {
		ipLimit = 0
	}
	for ip := a + 2; ip < ipLimit; {
		r := uint32(0) //nolint:all
		d := uint32(0) //nolint:all
		for {
			s := u24(ip)
			h := hash(s)
			r = ht[h]
			ht[h] = ip
			d = ip - r
			if ip >= ipLimit {
				break
			}
			ip++
			if d <= 0x1fff && s == u24(r) {
				break
			}
		}
		if ip >= ipLimit {
			break
		}
		ip--
		if ip > a {
			literals(ip - a)
		}
		l := cmp(r+3, ip+3, ipLimit+9)
		match(l)
		ip = setNextHash(setNextHash(ip + l))
		a = ip
	}
	literals(uint32(len(ib)) - a)
	return n
}

func ExtractOperatorFeeParams(operatorFeeParams common.Hash) (operatorFeeScalar, operatorFeeConstant *uint256.Int) {
	operatorFeeScalar = new(uint256.Int).SetBytes(operatorFeeParams[20:24])
	operatorFeeConstant = new(uint256.Int).SetBytes(operatorFeeParams[24:32])
	return
}

// NewOperatorCostFunc returns a function used for calculating operator fees, or nil if this is
// not an op-stack chain.
func NewOperatorCostFunc(config *chain.Config, statedb StateGetter) OperatorCostFunc {
	if config.Optimism == nil {
		return nil
	}
	forBlock := ^uint64(0)
	var cachedFunc operatorCostFunc

	selectFunc := func(blockTime uint64) operatorCostFunc {
		if !config.IsOptimismIsthmus(blockTime) {
			return func(gas uint64) *uint256.Int {
				return uint256.NewInt(0)
			}
		}
		var operatorFeeParamsInt uint256.Int
		statedb.GetState(L1BlockAddr, &OperatorFeeParamsSlot, &operatorFeeParamsInt)
		if operatorFeeParamsInt.IsZero() {
			return func(gas uint64) *uint256.Int {
				return uint256.NewInt(0)
			}
		}
		operatorFeeParams := common.Hash(operatorFeeParamsInt.Bytes32())
		operatorFeeScalar, operatorFeeConstant := ExtractOperatorFeeParams(operatorFeeParams)

		return newOperatorCostFunc(operatorFeeScalar, operatorFeeConstant)
	}

	return func(gas uint64, blockTime uint64) *uint256.Int {
		if forBlock != blockTime {
			forBlock = blockTime
			cachedFunc = selectFunc(blockTime)
		}

		return cachedFunc(gas)
	}
}

func newOperatorCostFunc(operatorFeeScalar *uint256.Int, operatorFeeConstant *uint256.Int) operatorCostFunc {
	return func(gas uint64) *uint256.Int {
		fee := new(uint256.Int).SetUint64(gas)
		fee = fee.Mul(fee, operatorFeeScalar)
		fee = fee.Div(fee, oneMillion)
		fee = fee.Add(fee, operatorFeeConstant)

		return fee
	}
}

// NewTotalRollupCostFunc return a TotalRollupCostFunc that computes the total rollup cost, consisting
// of both, the data availability cost and the operator cost.
func NewTotalRollupCostFunc(config *chain.Config, statedb StateGetter) TotalRollupCostFunc {
	if !config.IsOptimism() {
		return nil
	}
	l1CostFunc := NewL1CostFunc(config, statedb)
	operatorCostFunc := NewOperatorCostFunc(config, statedb)

	return func(tx RollupTransaction, blockTime uint64) *uint256.Int {
		// proper caching is happening inside the individual cost functions
		l1Cost := l1CostFunc(tx.RollupCostData(), blockTime)
		operatorCost := operatorCostFunc(tx.Gas(), blockTime)
		if l1Cost == nil && operatorCost == nil {
			return nil
		}

		var totalCost *uint256.Int
		var overflow bool
		if l1Cost != nil {
			totalCost = l1Cost
		} else {
			totalCost = new(uint256.Int)
		}

		// Note, the operator cost currently always returns a non-nil value, so we wouldn't
		// need the nil-check here. But we keep it for future-proofing.
		if operatorCost != nil {
			_, overflow = totalCost.AddOverflow(totalCost, operatorCost)
			if overflow {
				panic("overflow in total rollup cost: operatorCost")
			}
		}
		return totalCost
	}
}
