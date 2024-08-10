// Copyright 2019 The go-ethereum Authors
// (original work)
// Copyright 2024 The Erigon Authors
// (modifications)
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

package types

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/ledgerwatch/erigon-lib/chain"
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"

	libcommon "github.com/erigontech/erigon-lib/common"

	"github.com/erigontech/erigon/common"
	"github.com/erigontech/erigon/common/u256"
	"github.com/erigontech/erigon/crypto"
	"github.com/erigontech/erigon/params"
	"github.com/erigontech/erigon/rlp"
)

var (
	ecotoneTestConfig = func() *chain.Config {
		conf := *params.OptimismTestConfig // copy the config
		conf.EcotoneTime = big.NewInt(0)
		return &conf
	}()
	depNonce1     = uint64(7)
	depNonce2     = uint64(8)
	blockNumber   = big.NewInt(5)
	blockTime     = uint64(10)
	blockHash     = libcommon.BytesToHash([]byte{0x03, 0x14})
	legacyReceipt = &Receipt{
		Status:            ReceiptStatusFailed,
		CumulativeGasUsed: 1,
		Logs: []*Log{
			{
				Address: libcommon.BytesToAddress([]byte{0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
			{
				Address: libcommon.BytesToAddress([]byte{0x01, 0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
		},
	}
	accessListReceipt = &Receipt{
		Status:            ReceiptStatusFailed,
		CumulativeGasUsed: 1,
		Logs: []*Log{
			{
				Address: libcommon.BytesToAddress([]byte{0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
			{
				Address: libcommon.BytesToAddress([]byte{0x01, 0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
		},
		Type: AccessListTxType,
	}
	eip1559Receipt = &Receipt{
		Status:            ReceiptStatusFailed,
		CumulativeGasUsed: 1,
		Logs: []*Log{
			{
				Address: libcommon.BytesToAddress([]byte{0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
			{
				Address: libcommon.BytesToAddress([]byte{0x01, 0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
		},
		Type: DynamicFeeTxType,
	}
	depositReceiptNoNonce = &Receipt{
		Status:            ReceiptStatusFailed,
		CumulativeGasUsed: 1,
		Logs: []*Log{
			{
				Address: libcommon.BytesToAddress([]byte{0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
			{
				Address: libcommon.BytesToAddress([]byte{0x01, 0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
		},
		Type: DepositTxType,
	}
	nonce                   = uint64(1234)
	depositReceiptWithNonce = &Receipt{
		Status:                ReceiptStatusFailed,
		CumulativeGasUsed:     1,
		DepositNonce:          &nonce,
		DepositReceiptVersion: nil,
		Logs: []*Log{
			{
				Address: libcommon.BytesToAddress([]byte{0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
			{
				Address: libcommon.BytesToAddress([]byte{0x01, 0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
		},
		Type: DepositTxType,
	}
	version                           = CanyonDepositReceiptVersion
	depositReceiptWithNonceAndVersion = &Receipt{
		Status:                ReceiptStatusFailed,
		CumulativeGasUsed:     1,
		DepositNonce:          &nonce,
		DepositReceiptVersion: &version,
		Logs: []*Log{
			{
				Address: libcommon.BytesToAddress([]byte{0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
			{
				Address: libcommon.BytesToAddress([]byte{0x01, 0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
			},
		},
		Type: DepositTxType,
	}
	basefee = uint256.NewInt(1000 * 1e6)
	scalar  = uint256.NewInt(7 * 1e6)

	// below are the expected cost func outcomes for the above parameter settings on the emptyTx
	// which is defined in transaction_test.go
	bedrockFee = uint256.NewInt(11326000000000)
	ecotoneFee = uint256.NewInt(960900) // (480/16)*(2*16*1000 + 3*10) == 960900

	bedrockGas = uint256.NewInt(1618)
	ecotoneGas = uint256.NewInt(480)
)

func TestDecodeEmptyTypedReceipt(t *testing.T) {
	t.Parallel()
	input := []byte{0x80}
	var r Receipt
	err := rlp.DecodeBytes(input, &r)
	if !errors.Is(err, rlp.EOL) {
		t.Fatal("wrong error:", err)
	}
}

func TestLegacyReceiptDecoding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		encode func(*Receipt) ([]byte, error)
	}{
		{
			"StoredReceiptRLP",
			encodeAsStoredReceiptRLP,
		},
		// Erigon: all the legacy formats are removed intentionally
	}

	tx := NewTransaction(1, libcommon.HexToAddress("0x1"), u256.Num1, 1, u256.Num1, nil)
	receipt := &Receipt{
		Status:            ReceiptStatusFailed,
		CumulativeGasUsed: 1,
		Logs: []*Log{
			{
				Address: libcommon.BytesToAddress([]byte{0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
				Index:   999,
			},
			{
				Address: libcommon.BytesToAddress([]byte{0x01, 0x11}),
				Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
				Data:    []byte{0x01, 0x00, 0xff},
				Index:   1000,
			},
		},
		TxHash:          tx.Hash(),
		ContractAddress: libcommon.BytesToAddress([]byte{0x01, 0x11, 0x11}),
		GasUsed:         111111,
	}
	receipt.Bloom = CreateBloom(Receipts{receipt})

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			enc, err := tc.encode(receipt)
			if err != nil {
				t.Fatalf("Error encoding receipt: %v", err)
			}
			var dec ReceiptForStorage
			if err := rlp.DecodeBytes(enc, &dec); err != nil {
				t.Fatalf("Error decoding RLP receipt: %v", err)
			}
			// Check whether all consensus fields are correct.
			if dec.Status != receipt.Status {
				t.Fatalf("Receipt status mismatch, want %v, have %v", receipt.Status, dec.Status)
			}
			if dec.CumulativeGasUsed != receipt.CumulativeGasUsed {
				t.Fatalf("Receipt CumulativeGasUsed mismatch, want %v, have %v", receipt.CumulativeGasUsed, dec.CumulativeGasUsed)
			}
			assert.Equal(t, uint32(receipt.Logs[0].Index), dec.FirstLogIndex)
			//if len(dec.Logs) != len(receipt.Logs) {
			//	t.Fatalf("Receipt log number mismatch, want %v, have %v", len(receipt.Logs), len(dec.Logs))
			//}
			//for i := 0; i < len(dec.Logs); i++ {
			//	if dec.Logs[i].Address != receipt.Logs[i].Address {
			//		t.Fatalf("Receipt log %d address mismatch, want %v, have %v", i, receipt.Logs[i].Address, dec.Logs[i].Address)
			//	}
			//	if !reflect.DeepEqual(dec.Logs[i].Topics, receipt.Logs[i].Topics) {
			//		t.Fatalf("Receipt log %d topics mismatch, want %v, have %v", i, receipt.Logs[i].Topics, dec.Logs[i].Topics)
			//	}
			//	if !bytes.Equal(dec.Logs[i].Data, receipt.Logs[i].Data) {
			//		t.Fatalf("Receipt log %d data mismatch, want %v, have %v", i, receipt.Logs[i].Data, dec.Logs[i].Data)
			//	}
			//}
		})
	}
}

func encodeAsStoredReceiptRLP(want *Receipt) ([]byte, error) {
	w := bytes.NewBuffer(nil)
	casted := ReceiptForStorage(*want)
	err := casted.EncodeRLP(w)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func diffDerivedFields(t *testing.T, receipts Receipts, txs Transactions, blockHash libcommon.Hash, blockNumber *big.Int) {
	logIndex := uint(0)
	for i := range receipts {
		if receipts[i].Type != txs[i].Type() {
			t.Errorf("receipts[%d].Type = %d, want %d", i, receipts[i].Type, txs[i].Type())
		}
		if receipts[i].TxHash != txs[i].Hash() {
			t.Errorf("receipts[%d].TxHash = %s, want %s", i, receipts[i].TxHash.String(), txs[i].Hash().String())
		}
		if receipts[i].BlockHash != blockHash {
			t.Errorf("receipts[%d].BlockHash = %s, want %s", i, receipts[i].BlockHash.String(), blockHash.String())
		}
		if receipts[i].BlockNumber.Cmp(blockNumber) != 0 {
			t.Errorf("receipts[%c].BlockNumber = %s, want %s", i, receipts[i].BlockNumber.String(), blockNumber.String())
		}
		if receipts[i].TransactionIndex != uint(i) {
			t.Errorf("receipts[%d].TransactionIndex = %d, want %d", i, receipts[i].TransactionIndex, i)
		}
		if receipts[i].GasUsed != txs[i].GetGas() {
			t.Errorf("receipts[%d].GasUsed = %d, want %d", i, receipts[i].GasUsed, txs[i].GetGas())
		}
		if txs[i].GetTo() != nil && receipts[i].ContractAddress != (libcommon.Address{}) {
			t.Errorf("receipts[%d].ContractAddress = %s, want %s", i, receipts[i].ContractAddress.String(), (libcommon.Address{}).String())
		}
		for j := range receipts[i].Logs {
			if receipts[i].Logs[j].BlockNumber != blockNumber.Uint64() {
				t.Errorf("receipts[%d].Logs[%d].BlockNumber = %d, want %d", i, j, receipts[i].Logs[j].BlockNumber, blockNumber.Uint64())
			}
			if receipts[i].Logs[j].BlockHash != blockHash {
				t.Errorf("receipts[%d].Logs[%d].BlockHash = %s, want %s", i, j, receipts[i].Logs[j].BlockHash.String(), blockHash.String())
			}
			if receipts[i].Logs[j].TxHash != txs[i].Hash() {
				t.Errorf("receipts[%d].Logs[%d].TxHash = %s, want %s", i, j, receipts[i].Logs[j].TxHash.String(), txs[i].Hash().String())
			}
			if receipts[i].Logs[j].TxHash != txs[i].Hash() {
				t.Errorf("receipts[%d].Logs[%d].TxHash = %s, want %s", i, j, receipts[i].Logs[j].TxHash.String(), txs[i].Hash().String())
			}
			if receipts[i].Logs[j].TxIndex != uint(i) {
				t.Errorf("receipts[%d].Logs[%d].TransactionIndex = %d, want %d", i, j, receipts[i].Logs[j].TxIndex, i)
			}
			if receipts[i].Logs[j].Index != logIndex {
				t.Errorf("receipts[%d].Logs[%d].Index = %d, want %d", i, j, receipts[i].Logs[j].Index, logIndex)
			}
			logIndex++
		}
	}
}

// Tests that receipt data can be correctly derived from the contextual infos
func TestDeriveFields(t *testing.T) {
	t.Parallel()
	// Create a few transactions to have receipts for
	to2 := libcommon.HexToAddress("0x2")
	to3 := libcommon.HexToAddress("0x3")
	txs := Transactions{
		&LegacyTx{
			CommonTx: CommonTx{
				Nonce: 1,
				Value: u256.Num1,
				Gas:   1,
			},
			GasPrice: u256.Num1,
		},
		&LegacyTx{
			CommonTx: CommonTx{
				To:    &to2,
				Nonce: 2,
				Value: u256.Num2,
				Gas:   2,
			},
			GasPrice: u256.Num2,
		},
		&AccessListTx{
			LegacyTx: LegacyTx{
				CommonTx: CommonTx{
					To:    &to3,
					Nonce: 3,
					Value: uint256.NewInt(3),
					Gas:   3,
				},
				GasPrice: uint256.NewInt(3),
			},
		},
		&DepositTx{
			Value: uint256.NewInt(3),
			Gas:   4,
		},
		&DepositTx{
			To:    nil, // contract creation
			Value: uint256.NewInt(6),
			Gas:   5,
		},
	}
	depNonce := uint64(7)
	depNonce2 := uint64(8)
	canyonDepositReceiptVersion := CanyonDepositReceiptVersion
	// Create the corresponding receipts
	receipts := Receipts{
		&Receipt{
			Status:            ReceiptStatusFailed,
			CumulativeGasUsed: 1,
			Logs: []*Log{
				{Address: libcommon.BytesToAddress([]byte{0x11})},
				{Address: libcommon.BytesToAddress([]byte{0x01, 0x11})},
			},
			TxHash:          txs[0].Hash(),
			ContractAddress: libcommon.BytesToAddress([]byte{0x01, 0x11, 0x11}),
			GasUsed:         1,
			FirstLogIndex:   0,
		},
		&Receipt{
			PostState:         libcommon.Hash{2}.Bytes(),
			CumulativeGasUsed: 3,
			Logs: []*Log{
				{Address: libcommon.BytesToAddress([]byte{0x22})},
				{Address: libcommon.BytesToAddress([]byte{0x02, 0x22})},
			},
			TxHash:          txs[1].Hash(),
			ContractAddress: libcommon.BytesToAddress([]byte{0x02, 0x22, 0x22}),
			GasUsed:         2,
			FirstLogIndex:   2,
		},
		&Receipt{
			Type:              AccessListTxType,
			PostState:         libcommon.Hash{3}.Bytes(),
			CumulativeGasUsed: 6,
			Logs: []*Log{
				{Address: libcommon.BytesToAddress([]byte{0x33})},
				{Address: libcommon.BytesToAddress([]byte{0x03, 0x33})},
			},
			TxHash:          txs[2].Hash(),
			ContractAddress: libcommon.BytesToAddress([]byte{0x03, 0x33, 0x33}),
			GasUsed:         3,
			FirstLogIndex:   4,
		},
		&Receipt{
			Type:              DepositTxType,
			PostState:         libcommon.Hash{3}.Bytes(),
			CumulativeGasUsed: 10,
			Logs: []*Log{
				{Address: libcommon.BytesToAddress([]byte{0x33})},
				{Address: libcommon.BytesToAddress([]byte{0x03, 0x33})},
			},
			TxHash:                txs[3].Hash(),
			ContractAddress:       libcommon.BytesToAddress([]byte{0x03, 0x33, 0x33}),
			GasUsed:               4,
			BlockHash:             libcommon.BytesToHash([]byte{0x03, 0x14}),
			BlockNumber:           big.NewInt(1),
			TransactionIndex:      7,
			DepositNonce:          &depNonce,
			DepositReceiptVersion: nil,
		},
		&Receipt{
			Type:              DepositTxType,
			PostState:         libcommon.Hash{5}.Bytes(),
			CumulativeGasUsed: 15,
			Logs: []*Log{
				{
					Address: libcommon.BytesToAddress([]byte{0x33}),
					Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
					// derived fields:
					BlockNumber: big.NewInt(1).Uint64(),
					TxHash:      txs[4].Hash(),
					TxIndex:     4,
					BlockHash:   libcommon.BytesToHash([]byte{0x03, 0x14}),
					Index:       4,
				},
				{
					Address: libcommon.BytesToAddress([]byte{0x03, 0x33}),
					Topics:  []libcommon.Hash{libcommon.HexToHash("dead"), libcommon.HexToHash("beef")},
					// derived fields:
					BlockNumber: big.NewInt(1).Uint64(),
					TxHash:      txs[4].Hash(),
					TxIndex:     4,
					BlockHash:   libcommon.BytesToHash([]byte{0x03, 0x14}),
					Index:       5,
				},
			},
			TxHash:                txs[4].Hash(),
			ContractAddress:       libcommon.HexToAddress("0x3bb898b4bbe24f68a4e9be46cfe72d1787fd74f4"),
			GasUsed:               5,
			BlockHash:             libcommon.BytesToHash([]byte{0x03, 0x14}),
			BlockNumber:           big.NewInt(1),
			TransactionIndex:      4,
			DepositNonce:          &depNonce2,
			DepositReceiptVersion: &canyonDepositReceiptVersion,
		},
	}

	nonces := []uint64{
		txs[0].GetNonce(),
		txs[1].GetNonce(),
		txs[2].GetNonce(),
		// Deposit tx should use deposit nonce
		*receipts[3].DepositNonce,
		*receipts[4].DepositNonce,
	}
	// Clear all the computed fields and re-derive them
	number := big.NewInt(1)
	hash := libcommon.BytesToHash([]byte{0x03, 0x14})
	time := uint64(0)

	t.Run("DeriveV1", func(t *testing.T) {
		clearComputedFieldsOnReceipts(t, receipts)
		if err := receipts.DeriveFields(hash, number.Uint64(), txs, []libcommon.Address{libcommon.BytesToAddress([]byte{0x0}), libcommon.BytesToAddress([]byte{0x0}), libcommon.BytesToAddress([]byte{0x0})}); err != nil {
			t.Fatalf("DeriveFields(...) = %v, want <nil>", err)
		}
		// Iterate over all the computed fields and check that they're correct
		signer := MakeSigner(params.TestChainConfig, number.Uint64(), 0)

		logIndex := uint(0)
		for i, r := range receipts {
			if r.Type != txs[i].Type() {
				t.Errorf("receipts[%d].Type = %d, want %d", i, r.Type, txs[i].Type())
			}
			if r.TxHash != txs[i].Hash() {
				t.Errorf("receipts[%d].TxHash = %s, want %s", i, r.TxHash.String(), txs[i].Hash().String())
			}
			if r.BlockHash != hash {
				t.Errorf("receipts[%d].BlockHash = %s, want %s", i, r.BlockHash.String(), hash.String())
			}
			if r.BlockNumber.Cmp(number) != 0 {
				t.Errorf("receipts[%c].BlockNumber = %s, want %s", i, r.BlockNumber.String(), number.String())
			}
			if r.TransactionIndex != uint(i) {
				t.Errorf("receipts[%d].TransactionIndex = %d, want %d", i, r.TransactionIndex, i)
			}
			if r.GasUsed != txs[i].GetGas() {
				t.Errorf("receipts[%d].GasUsed = %d, want %d", i, r.GasUsed, txs[i].GetGas())
			}
			if txs[i].GetTo() != nil && r.ContractAddress != (libcommon.Address{}) {
				t.Errorf("receipts[%d].ContractAddress = %s, want %s", i, r.ContractAddress.String(), (libcommon.Address{}).String())
			}
			from, _ := txs[i].Sender(*signer)
			contractAddress := crypto.CreateAddress(from, txs[i].GetNonce())
			if txs[i].GetTo() == nil && r.ContractAddress != contractAddress {
				t.Errorf("receipts[%d].ContractAddress = %s, want %s", i, r.ContractAddress.String(), contractAddress.String())
			}
			for j := range r.Logs {
				if r.Logs[j].BlockNumber != number.Uint64() {
					t.Errorf("receipts[%d].Logs[%d].BlockNumber = %d, want %d", i, j, r.Logs[j].BlockNumber, number.Uint64())
				}
				if r.Logs[j].BlockHash != hash {
					t.Errorf("receipts[%d].Logs[%d].BlockHash = %s, want %s", i, j, r.Logs[j].BlockHash.String(), hash.String())
				}
				if r.Logs[j].TxHash != txs[i].Hash() {
					t.Errorf("receipts[%d].Logs[%d].TxHash = %s, want %s", i, j, r.Logs[j].TxHash.String(), txs[i].Hash().String())
				}
				if r.Logs[j].TxHash != txs[i].Hash() {
					t.Errorf("receipts[%d].Logs[%d].TxHash = %s, want %s", i, j, r.Logs[j].TxHash.String(), txs[i].Hash().String())
				}
				if r.Logs[j].TxIndex != uint(i) {
					t.Errorf("receipts[%d].Logs[%d].TransactionIndex = %d, want %d", i, j, r.Logs[j].TxIndex, i)
				}
				if r.Logs[j].Index != logIndex {
					t.Errorf("receipts[%d].Logs[%d].Index = %d, want %d", i, j, r.Logs[j].Index, logIndex)
				}
				logIndex++
			}
		}
	})

	t.Run("DeriveV3", func(t *testing.T) {
		clearComputedFieldsOnReceipts(t, receipts)
		// Iterate over all the computed fields and check that they're correct
		signer := MakeSigner(params.TestChainConfig, number.Uint64(), 0)

		logIndex := uint(0)
		for i := range receipts {
			txs[i].SetSender(libcommon.BytesToAddress([]byte{0x0}))
			r, err := receipts.DeriveFieldsV3ForSingleReceipt(i, hash, number.Uint64(), txs[i])
			if err != nil {
				panic(err)
			}

			if r.Type != txs[i].Type() {
				t.Errorf("receipts[%d].Type = %d, want %d", i, r.Type, txs[i].Type())
			}
			if r.TxHash != txs[i].Hash() {
				t.Errorf("receipts[%d].TxHash = %s, want %s", i, r.TxHash.String(), txs[i].Hash().String())
			}
			if r.BlockHash != hash {
				t.Errorf("receipts[%d].BlockHash = %s, want %s", i, r.BlockHash.String(), hash.String())
			}
			if r.BlockNumber.Cmp(number) != 0 {
				t.Errorf("receipts[%c].BlockNumber = %s, want %s", i, r.BlockNumber.String(), number.String())
			}
			if r.TransactionIndex != uint(i) {
				t.Errorf("receipts[%d].TransactionIndex = %d, want %d", i, r.TransactionIndex, i)
			}
			if r.GasUsed != txs[i].GetGas() {
				t.Errorf("receipts[%d].GasUsed = %d, want %d", i, r.GasUsed, txs[i].GetGas())
			}
			if txs[i].GetTo() != nil && r.ContractAddress != (libcommon.Address{}) {
				t.Errorf("receipts[%d].ContractAddress = %s, want %s", i, r.ContractAddress.String(), (libcommon.Address{}).String())
			}
			from, _ := txs[i].Sender(*signer)
			contractAddress := crypto.CreateAddress(from, txs[i].GetNonce())
			if txs[i].GetTo() == nil && r.ContractAddress != contractAddress {
				t.Errorf("receipts[%d].ContractAddress = %s, want %s", i, r.ContractAddress.String(), contractAddress.String())
			}
			for j := range r.Logs {
				if r.Logs[j].BlockNumber != number.Uint64() {
					t.Errorf("receipts[%d].Logs[%d].BlockNumber = %d, want %d", i, j, r.Logs[j].BlockNumber, number.Uint64())
				}
				if r.Logs[j].BlockHash != hash {
					t.Errorf("receipts[%d].Logs[%d].BlockHash = %s, want %s", i, j, r.Logs[j].BlockHash.String(), hash.String())
				}
				if r.Logs[j].TxHash != txs[i].Hash() {
					t.Errorf("receipts[%d].Logs[%d].TxHash = %s, want %s", i, j, r.Logs[j].TxHash.String(), txs[i].Hash().String())
				}
				if r.Logs[j].TxHash != txs[i].Hash() {
					t.Errorf("receipts[%d].Logs[%d].TxHash = %s, want %s", i, j, r.Logs[j].TxHash.String(), txs[i].Hash().String())
				}
				if r.Logs[j].TxIndex != uint(i) {
					t.Errorf("receipts[%d].Logs[%d].TransactionIndex = %d, want %d", i, j, r.Logs[j].TxIndex, i)
				}
				if r.Logs[j].Index != logIndex {
					t.Errorf("receipts[%d].Logs[%d].Index = %d, want %d", i, j, r.Logs[j].Index, logIndex)
				}
				logIndex++
			}
		}
	})

}

// TestTypedReceiptEncodingDecoding reproduces a flaw that existed in the receipt
// rlp decoder, which failed due to a shadowing error.
func TestTypedReceiptEncodingDecoding(t *testing.T) {
	t.Parallel()
	var payload = common.FromHex("f9043eb9010c01f90108018262d4b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000c0b9010c01f901080182cd14b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000c0b9010d01f901090183013754b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000c0b9010d01f90109018301a194b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000c0")
	check := func(bundle []*Receipt) {
		t.Helper()
		for i, receipt := range bundle {
			if got, want := receipt.Type, uint8(1); got != want {
				t.Fatalf("bundle %d: got %x, want %x", i, got, want)
			}
		}
	}
	{
		var bundle []*Receipt
		if err := rlp.DecodeBytes(payload, &bundle); err != nil {
			t.Fatal(err)
		}
		check(bundle)
	}
	{
		var bundle []*Receipt
		r := bytes.NewReader(payload)
		s := rlp.NewStream(r, uint64(len(payload)))
		if err := s.Decode(&bundle); err != nil {
			t.Fatal(err)
		}
		check(bundle)
	}
}

func clearComputedFieldsOnReceipts(t *testing.T, receipts Receipts) {
	t.Helper()

	for _, receipt := range receipts {
		clearComputedFieldsOnReceipt(t, receipt)
	}
}

func clearComputedFieldsOnReceipt(t *testing.T, receipt *Receipt) {
	t.Helper()

	receipt.TxHash = libcommon.Hash{}
	receipt.BlockHash = libcommon.Hash{}
	receipt.BlockNumber = big.NewInt(math.MaxUint32)
	receipt.TransactionIndex = math.MaxUint32
	receipt.ContractAddress = libcommon.Address{}
	receipt.GasUsed = 0

	clearComputedFieldsOnLogs(t, receipt.Logs)
}

func clearComputedFieldsOnLogs(t *testing.T, logs []*Log) {
	t.Helper()

	for _, log := range logs {
		clearComputedFieldsOnLog(t, log)
	}
}

func clearComputedFieldsOnLog(t *testing.T, log *Log) {
	t.Helper()

	log.BlockNumber = math.MaxUint32
	log.BlockHash = libcommon.Hash{}
	log.TxHash = libcommon.Hash{}
	log.TxIndex = math.MaxUint32
	log.Index = math.MaxUint32
}

func TestBedrockDepositReceiptUnchanged(t *testing.T) {
	expectedRlp := common.FromHex("B9015a7EF90156A003000000000000000000000000000000000000000000000000000000000000000AB9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000F0D7940000000000000000000000000000000000000033C001D7940000000000000000000000000000000000000333C002")
	// Deposit receipt with no nonce
	receipt := &Receipt{
		Type:              DepositTxType,
		PostState:         libcommon.Hash{3}.Bytes(),
		CumulativeGasUsed: 10,
		Logs: []*Log{
			{Address: libcommon.BytesToAddress([]byte{0x33}), Data: []byte{1}, Topics: nil},
			{Address: libcommon.BytesToAddress([]byte{0x03, 0x33}), Data: []byte{2}, Topics: nil},
		},
		TxHash:          libcommon.Hash{},
		ContractAddress: libcommon.BytesToAddress([]byte{0x03, 0x33, 0x33}),
		GasUsed:         4,
	}

	encodedRlp, err := rlp.EncodeToBytes(receipt)
	require.NoError(t, err)
	require.Equal(t, expectedRlp, encodedRlp)

	// Consensus values should be unchanged after reparsing
	parsed := new(Receipt)
	err = rlp.DecodeBytes(encodedRlp, parsed)
	require.NoError(t, err)
	require.Equal(t, receipt.Status, parsed.Status)
	require.Equal(t, receipt.CumulativeGasUsed, parsed.CumulativeGasUsed)
	require.Equal(t, receipt.Bloom, parsed.Bloom)
	require.Equal(t, len(receipt.Logs), len(parsed.Logs))
	for i := 0; i < len(receipt.Logs); i++ {
		require.EqualValues(t, receipt.Logs[i], parsed.Logs[i])
	}
	// And still shouldn't have a nonce
	require.Nil(t, parsed.DepositNonce)
	// ..or a deposit nonce
	require.Nil(t, parsed.DepositReceiptVersion)
}

// Regolith did not include deposit nonce during receipt root construction.
// TestReceiptEncodeIndexBugIsEnshrined makes sure this difference is preserved for backwards
// compatibility purposes, but also that there is no discrepancy for the post-Canyon encoding.
func TestReceiptEncodeIndexBugIsEnshrined(t *testing.T) {
	// Check that a post-Regolith, pre-Canyon receipt produces no difference between
	// receipts having different depositNonce
	buf := new(bytes.Buffer)
	receipts := Receipts{depositReceiptWithNonce.Copy()}
	receipts.EncodeIndex(0, buf)
	indexBytesBefore := buf.Bytes()

	buf2 := new(bytes.Buffer)
	newDepositNonce := *receipts[0].DepositNonce + 1
	receipts[0].DepositNonce = &newDepositNonce
	receipts.EncodeIndex(0, buf2)
	indexBytesAfter := buf2.Bytes()

	require.Equal(t, indexBytesBefore, indexBytesAfter)

	// Confirm the buggy encoding is as expected, which means it should encode as if it had no
	// nonce specified (like that of a non-deposit receipt, whose encoding would differ only in the
	// type byte).
	buf3 := new(bytes.Buffer)
	receipts[0].Type = eip1559Receipt.Type
	receipts.EncodeIndex(0, buf3)
	indexBytesNoDeposit := buf3.Bytes()

	require.NotEqual(t, indexBytesBefore[0], indexBytesNoDeposit[0])
	require.Equal(t, indexBytesBefore[1:], indexBytesNoDeposit[1:])

	// Check that post-canyon changes the hash compared to pre-Canyon
	buf4 := new(bytes.Buffer)
	receipts = Receipts{depositReceiptWithNonceAndVersion.Copy()}
	receipts.EncodeIndex(0, buf4)
	indexBytesCanyon := buf4.Bytes()
	require.NotEqual(t, indexBytesBefore[1:], indexBytesCanyon[1:])

	// Check that bumping the nonce post-canyon changes the hash
	buf5 := new(bytes.Buffer)
	bumpedNonce := *depositReceiptWithNonceAndVersion.DepositNonce + 1
	receipts[0].DepositNonce = &bumpedNonce
	receipts.EncodeIndex(0, buf5)
	indexBytesCanyonBump := buf5.Bytes()
	require.NotEqual(t, indexBytesCanyon[1:], indexBytesCanyonBump[1:])
}

func TestRoundTripReceipt(t *testing.T) {
	tests := []struct {
		name string
		rcpt *Receipt
	}{
		{name: "Legacy", rcpt: legacyReceipt},
		{name: "AccessList", rcpt: accessListReceipt},
		{name: "EIP1559", rcpt: eip1559Receipt},
		{name: "DepositNoNonce", rcpt: depositReceiptNoNonce},
		{name: "DepositWithNonce", rcpt: depositReceiptWithNonce},
		{name: "DepositWithNonceAndVersion", rcpt: depositReceiptWithNonceAndVersion},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := rlp.EncodeToBytes(test.rcpt)
			require.NoError(t, err)

			d := &Receipt{}
			err = rlp.DecodeBytes(data, d)
			require.NoError(t, err)
			require.Equal(t, test.rcpt, d)
			require.Equal(t, test.rcpt.DepositNonce, d.DepositNonce)
			require.Equal(t, test.rcpt.DepositReceiptVersion, d.DepositReceiptVersion)
		})

		t.Run(fmt.Sprintf("%sRejectExtraData", test.name), func(t *testing.T) {
			data, err := rlp.EncodeToBytes(test.rcpt)
			require.NoError(t, err)
			data = append(data, 1, 2, 3, 4)
			d := &Receipt{}
			err = rlp.DecodeBytes(data, d)
			require.Error(t, err)
		})
	}
}

func TestRoundTripReceiptForStorage(t *testing.T) {
	tests := []struct {
		name string
		rcpt *Receipt
	}{
		{name: "Legacy", rcpt: legacyReceipt},
		{name: "AccessList", rcpt: accessListReceipt},
		{name: "EIP1559", rcpt: eip1559Receipt},
		{name: "DepositNoNonce", rcpt: depositReceiptNoNonce},
		{name: "DepositWithNonce", rcpt: depositReceiptWithNonce},
		{name: "DepositWithNonceAndVersion", rcpt: depositReceiptWithNonceAndVersion},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := rlp.EncodeToBytes((*ReceiptForStorage)(test.rcpt))
			require.NoError(t, err)

			d := &ReceiptForStorage{}
			err = rlp.DecodeBytes(data, d)
			require.NoError(t, err)
			// Only check the stored fields - the others are derived later
			require.Equal(t, test.rcpt.Status, d.Status)
			require.Equal(t, test.rcpt.CumulativeGasUsed, d.CumulativeGasUsed)
			require.Equal(t, test.rcpt.Logs, d.Logs)
			require.Equal(t, test.rcpt.DepositNonce, d.DepositNonce)
			require.Equal(t, test.rcpt.DepositReceiptVersion, d.DepositReceiptVersion)
		})
	}
}

func TestReceiptJSON(t *testing.T) {
	tests := []struct {
		name string
		rcpt *Receipt
	}{
		{name: "Legacy", rcpt: legacyReceipt},
		{name: "AccessList", rcpt: accessListReceipt},
		{name: "EIP1559", rcpt: eip1559Receipt},
		{name: "DepositNoNonce", rcpt: depositReceiptNoNonce},
		{name: "DepositWithNonce", rcpt: depositReceiptWithNonce},
		{name: "DepositWithNonceAndVersion", rcpt: depositReceiptWithNonceAndVersion},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := test.rcpt.MarshalJSON()
			if err != nil {
				t.Fatal("error marshaling receipt to json:", err)
			}
			r := Receipt{}
			err = r.UnmarshalJSON(b)
			if err != nil {
				t.Fatal("error unmarshaling receipt from json:", err)
			}

			// Make sure marshal/unmarshal doesn't affect receipt hash root computation by comparing
			// the output of EncodeIndex
			rsBefore := Receipts([]*Receipt{test.rcpt})
			rsAfter := Receipts([]*Receipt{&r})

			encBefore, encAfter := bytes.Buffer{}, bytes.Buffer{}
			rsBefore.EncodeIndex(0, &encBefore)
			rsAfter.EncodeIndex(0, &encAfter)
			if !bytes.Equal(encBefore.Bytes(), encAfter.Bytes()) {
				t.Errorf("%v: EncodeIndex differs after JSON marshal/unmarshal", test.name)
			}
		})
	}
}

// This method is based on op-geth
// https://github.com/ethereum-optimism/op-geth/commit/a290ca164a36c80a8d106d88bd482b6f82220bef
func getOptimismTxReceipts(
	t *testing.T, l1AttributesPayload []byte,
	l1GasPrice, l1GasUsed *uint256.Int, feeScalar *big.Float, l1Fee *uint256.Int) (Transactions, Receipts) {
	//to4 := common.HexToAddress("0x4")
	// Create a few transactions to have receipts for
	txs := Transactions{
		&DepositTx{
			To:    nil, // contract creation
			Value: uint256.NewInt(6),
			Gas:   50,
			Data:  l1AttributesPayload,
		},
		emptyTx,
	}

	// Create the corresponding receipts
	receipts := Receipts{
		&Receipt{
			Type:              DepositTxType,
			PostState:         libcommon.Hash{5}.Bytes(),
			CumulativeGasUsed: 50,
			Logs: []*Log{
				{
					Address: libcommon.BytesToAddress([]byte{0x33}),
					// derived fields:
					BlockNumber: blockNumber.Uint64(),
					TxHash:      txs[0].Hash(),
					TxIndex:     0,
					BlockHash:   blockHash,
					Index:       0,
				},
				{
					Address: libcommon.BytesToAddress([]byte{0x03, 0x33}),
					// derived fields:
					BlockNumber: blockNumber.Uint64(),
					TxHash:      txs[0].Hash(),
					TxIndex:     0,
					BlockHash:   blockHash,
					Index:       1,
				},
			},
			TxHash:           txs[0].Hash(),
			ContractAddress:  libcommon.HexToAddress("0x3bb898b4bbe24f68a4e9be46cfe72d1787fd74f4"),
			GasUsed:          50,
			BlockHash:        blockHash,
			BlockNumber:      blockNumber,
			TransactionIndex: 0,
			DepositNonce:     &depNonce1,
		},
		&Receipt{
			Type:              LegacyTxType,
			PostState:         libcommon.Hash{4}.Bytes(),
			CumulativeGasUsed: 50,
			Logs:              []*Log{},
			// derived fields:
			TxHash:           txs[1].Hash(),
			GasUsed:          0,
			BlockHash:        blockHash,
			BlockNumber:      blockNumber,
			TransactionIndex: 1,
			L1GasPrice:       l1GasPrice.ToBig(),
			L1GasUsed:        l1GasUsed.ToBig(),
			L1Fee:            l1Fee.ToBig(),
			FeeScalar:        feeScalar,
		},
	}
	return txs, receipts
}

// This method is based on op-geth
// https://github.com/ethereum-optimism/op-geth/commit/a290ca164a36c80a8d106d88bd482b6f82220bef
func checkBedrockReceipts(t *testing.T, receipts Receipts, txs Transactions, blockHash libcommon.Hash, blockNumber *big.Int) {
	diffDerivedFields(t, receipts, txs, blockHash, blockNumber)

	// Check that we preserved the invariant: l1Fee = l1GasPrice * l1GasUsed * l1FeeScalar
	// but with more difficult int math...
	l2Rcpt := receipts[1]
	l1GasCost := new(big.Int).Mul(l2Rcpt.L1GasPrice, l2Rcpt.L1GasUsed)
	l1Fee := new(big.Float).Mul(new(big.Float).SetInt(l1GasCost), l2Rcpt.FeeScalar)
	require.Equal(t, new(big.Float).SetInt(l2Rcpt.L1Fee), l1Fee)
}

// This test is based on op-geth
// https://github.com/ethereum-optimism/op-geth/commit/a290ca164a36c80a8d106d88bd482b6f82220bef
func TestDeriveOptimismBedrockTxReceipts(t *testing.T) {
	// Bedrock style l1 attributes with L1Scalar=7_000_000 (becomes 7 after division), L1Overhead=50, L1BaseFee=1000*1e6
	payload := libcommon.Hex2Bytes("015d8eb900000000000000000000000000000000000000000000000000000000000004d200000000000000000000000000000000000000000000000000000000000004d2000000000000000000000000000000000000000000000000000000003b9aca0000000000000000000000000000000000000000000000000000000000000004d200000000000000000000000000000000000000000000000000000000000004d200000000000000000000000000000000000000000000000000000000000004d2000000000000000000000000000000000000000000000000000000000000003200000000000000000000000000000000000000000000000000000000006acfc0015d8eb900000000000000000000000000000000000000000000000000000000000004d200000000000000000000000000000000000000000000000000000000000004d2000000000000000000000000000000000000000000000000000000003b9aca0000000000000000000000000000000000000000000000000000000000000004d200000000000000000000000000000000000000000000000000000000000004d200000000000000000000000000000000000000000000000000000000000004d2000000000000000000000000000000000000000000000000000000000000003200000000000000000000000000000000000000000000000000000000006acfc0")
	l1GasPrice := basefee
	l1GasUsed := bedrockGas
	feeScalar := big.NewFloat(float64(scalar.Uint64() / 1e6))
	l1Fee := bedrockFee
	txs, receipts := getOptimismTxReceipts(t, payload, l1GasPrice, l1GasUsed, feeScalar, l1Fee)
	senders := []libcommon.Address{libcommon.HexToAddress("0x0"), libcommon.HexToAddress("0x0")}

	// Re-derive receipts.
	clearComputedFieldsOnReceipts(t, receipts)
	err := receipts.DeriveFields(params.OptimismTestConfig, blockHash, blockNumber.Uint64(), blockTime, txs, senders)
	if err != nil {
		t.Fatalf("DeriveFields(...) = %v, want <nil>", err)
	}
	checkBedrockReceipts(t, receipts, txs, blockHash, blockNumber)

	// Should get same result with the Ecotone config because it will assume this is "first ecotone block"
	// if it sees the bedrock style L1 attributes.
	clearComputedFieldsOnReceipts(t, receipts)
	err = receipts.DeriveFields(ecotoneTestConfig, blockHash, blockNumber.Uint64(), blockTime, txs, senders)
	if err != nil {
		t.Fatalf("DeriveFields(...) = %v, want <nil>", err)
	}
	checkBedrockReceipts(t, receipts, txs, blockHash, blockNumber)
}

// This test is based on op-geth
// https://github.com/ethereum-optimism/op-geth/commit/a290ca164a36c80a8d106d88bd482b6f82220bef
func TestDeriveOptimismEcotoneTxReceipts(t *testing.T) {
	// Ecotone style l1 attributes with baseFeeScalar=2, blobBaseFeeScalar=3, baseFee=1000*1e6, blobBaseFee=10*1e6
	payload := libcommon.Hex2Bytes("440a5e20000000020000000300000000000004d200000000000004d200000000000004d2000000000000000000000000000000000000000000000000000000003b9aca00000000000000000000000000000000000000000000000000000000000098968000000000000000000000000000000000000000000000000000000000000004d200000000000000000000000000000000000000000000000000000000000004d2")
	l1GasPrice := basefee
	l1GasUsed := ecotoneGas
	l1Fee := ecotoneFee
	txs, receipts := getOptimismTxReceipts(t, payload, l1GasPrice, l1GasUsed, nil /*feeScalar*/, l1Fee)
	senders := []libcommon.Address{libcommon.HexToAddress("0x0"), libcommon.HexToAddress("0x0")}

	// Re-derive receipts.
	clearComputedFieldsOnReceipts(t, receipts)
	err := receipts.DeriveFields(params.OptimismTestConfig, blockHash, blockNumber.Uint64(), blockTime, txs, senders)
	if err == nil {
		t.Fatalf("expected error from deriving ecotone receipts with pre-ecotone config, got none")
	}

	clearComputedFieldsOnReceipts(t, receipts)
	err = receipts.DeriveFields(ecotoneTestConfig, blockHash, blockNumber.Uint64(), blockTime, txs, senders)
	if err != nil {
		t.Fatalf("DeriveFields(...) = %v, want <nil>", err)
	}
	diffDerivedFields(t, receipts, txs, blockHash, blockNumber)
}
