// Copyright 2021 The go-ethereum Authors
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
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"sync/atomic"
	"time"

	"github.com/erigontech/erigon-lib/chain"
	libcommon "github.com/erigontech/erigon-lib/common"
	types2 "github.com/erigontech/erigon-lib/types"

	"github.com/erigontech/erigon/rlp"

	"github.com/erigontech/erigon/common"
	"github.com/holiman/uint256"
)

type DepositTx struct {
	time time.Time // Time first seen locally (spam avoidance)
	// caches
	hash atomic.Value //nolint:structcheck
	size atomic.Value //nolint:structcheck
	// SourceHash uniquely identifies the source of the deposit
	SourceHash libcommon.Hash
	// From is exposed through the types.Signer, not through TxData
	From libcommon.Address
	// nil means contract creation
	To *libcommon.Address `rlp:"nil"`
	// Mint is minted on L2, locked on L1, nil if no minting.
	Mint *uint256.Int `rlp:"nil"`
	// Value is transferred from L2 balance, executed after Mint (if any)
	Value *uint256.Int
	// gas limit
	Gas uint64
	// Field indicating if this transaction is exempt from the L2 gas limit.
	IsSystemTransaction bool
	// Normal Tx data
	Data []byte
}

func (tx *DepositTx) GetChainID() *uint256.Int {
	panic("deposits are not signed and do not have a chain-ID")
}

func (tx *DepositTx) GetNonce() uint64 {
	return 0
}

func (tx *DepositTx) GetTo() *libcommon.Address {
	return tx.To
}

func (tx *DepositTx) GetBlobGas() uint64 {
	return 0
}

func (tx *DepositTx) GetBlobHashes() []libcommon.Hash {
	return []libcommon.Hash{}
}

func (tx *DepositTx) GetGas() uint64 {
	return tx.Gas
}

func (tx *DepositTx) GetValue() *uint256.Int {
	return tx.Value
}

func (tx *DepositTx) GetData() []byte {
	return tx.Data
}

func (tx *DepositTx) GetSender() (libcommon.Address, bool) {
	return tx.From, true
}

func (tx *DepositTx) SetSender(addr libcommon.Address) {
	tx.From = addr
}

func (tx *DepositTx) RawSignatureValues() (*uint256.Int, *uint256.Int, *uint256.Int) {
	panic("deposit tx does not have a signature")
}

func (tx *DepositTx) SigningHash(chainID *big.Int) libcommon.Hash {
	panic("deposit tx does not have a signing hash")
}

// NOTE: Need to check this
func (tx *DepositTx) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := tx.EncodingSize()
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// NOTE: Need to check this
func (tx *DepositTx) EncodingSize() int {
	payloadSize, _, _, _ := tx.payloadSize()
	envelopeSize := payloadSize
	// Add envelope size and type size
	if payloadSize >= 56 {
		envelopeSize += (bits.Len(uint(payloadSize)) + 7) / 8
	}
	envelopeSize += 2
	return envelopeSize
}

// MarshalBinary returns the canonical encoding of the transaction.
// For legacy transactions, it returns the RLP encoding. For EIP-2718 typed
// transactions, it returns the type and payload.
func (tx *DepositTx) MarshalBinary(w io.Writer) error {
	payloadSize, nonceLen, gasLen, accessListLen := tx.payloadSize()
	var b [33]byte
	// encode TxType
	b[0] = DepositTxType
	if _, err := w.Write(b[:1]); err != nil {
		return err
	}
	if err := tx.encodePayload(w, b[:], payloadSize, nonceLen, gasLen, accessListLen); err != nil {
		return err
	}
	return nil
}

func (tx *DepositTx) payloadSize() (payloadSize int, nonceLen, gasLen, accessListLen int) {
	// size of SourceHash
	payloadSize += 33
	// size of From
	payloadSize += 21
	// size of To
	payloadSize++
	if tx.To != nil {
		payloadSize += 20
	}
	// size of Mint
	payloadSize++
	payloadSize += rlp.Uint256LenExcludingHead(tx.Mint)
	// size of Value
	payloadSize++
	payloadSize += rlp.Uint256LenExcludingHead(tx.Value)
	// size of Gas
	payloadSize++
	gasLen = rlp.IntLenExcludingHead(tx.Gas)
	payloadSize += gasLen
	// size of IsSystemTransaction
	payloadSize++
	// size of Data
	payloadSize++
	switch len(tx.Data) {
	case 0:
	case 1:
		if tx.Data[0] >= 128 {
			payloadSize++
		}
	default:
		if len(tx.Data) >= 56 {
			payloadSize += (bits.Len(uint(len(tx.Data))) + 7) / 8
		}
		payloadSize += len(tx.Data)
	}
	return payloadSize, 0, gasLen, 0
}

func (tx *DepositTx) encodePayload(w io.Writer, b []byte, payloadSize, nonceLen, gasLen, accessListLen int) error {
	// prefix
	if err := EncodeStructSizePrefix(payloadSize, w, b); err != nil {
		return err
	}
	// encode SourceHash
	b[0] = 128 + 32
	if _, err := w.Write(b[:1]); err != nil {
		return nil
	}
	if _, err := w.Write(tx.SourceHash.Bytes()); err != nil {
		return nil
	}
	// encode From
	b[0] = 128 + 20
	if _, err := w.Write(b[:1]); err != nil {
		return nil
	}
	if _, err := w.Write(tx.From.Bytes()); err != nil {
		return nil
	}
	// encode To
	if tx.To == nil {
		b[0] = 128
	} else {
		b[0] = 128 + 20
	}
	if _, err := w.Write(b[:1]); err != nil {
		return err
	}
	if tx.To != nil {
		if _, err := w.Write(tx.To.Bytes()); err != nil {
			return err
		}
	}
	// encode Mint
	if err := tx.Mint.EncodeRLP(w); err != nil {
		return err
	}
	// encode Value
	if err := tx.Value.EncodeRLP(w); err != nil {
		return err
	}
	// encode Gas
	if err := rlp.EncodeInt(tx.Gas, w, b); err != nil {
		return err
	}
	// encode IsSystemTransaction
	if tx.IsSystemTransaction {
		b[0] = 0x01
	} else {
		b[0] = 0x80
	}
	if _, err := w.Write(b[:1]); err != nil {
		return nil
	}
	// encode Data
	if err := rlp.EncodeString(tx.Data, w, b); err != nil {
		return err
	}
	return nil
}

func (tx *DepositTx) EncodeRLP(w io.Writer) error {
	payloadSize, nonceLen, gasLen, accessListLen := tx.payloadSize()
	envelopeSize := payloadSize
	if payloadSize >= 56 {
		envelopeSize += (bits.Len(uint(payloadSize)) + 7) / 8
	}
	// size of struct prefix and TxType
	envelopeSize += 2
	var b [33]byte
	// envelope
	if err := rlp.EncodeStringSizePrefix(envelopeSize, w, b[:]); err != nil {
		return err
	}
	// encode TxType
	b[0] = DepositTxType
	if _, err := w.Write(b[:1]); err != nil {
		return err
	}
	if err := tx.encodePayload(w, b[:], payloadSize, nonceLen, gasLen, accessListLen); err != nil {
		return err
	}
	return nil
}

func (tx *DepositTx) DecodeRLP(s *rlp.Stream) error {
	_, err := s.List()
	if err != nil {
		return err
	}
	var b []byte
	// SourceHash
	if b, err = s.Bytes(); err != nil {
		return err
	}
	if len(b) != 32 {
		return fmt.Errorf("wrong size for Source hash: %d", len(b))
	}
	copy(tx.SourceHash[:], b)
	// From
	if b, err = s.Bytes(); err != nil {
		return err
	}
	if len(b) != 20 {
		return fmt.Errorf("wrong size for From hash: %d", len(b))
	}
	copy(tx.From[:], b)
	// To (optional)
	if b, err = s.Bytes(); err != nil {
		return err
	}
	if len(b) > 0 && len(b) != 20 {
		return fmt.Errorf("wrong size for To: %d", len(b))
	}
	if len(b) > 0 {
		tx.To = &libcommon.Address{}
		copy((*tx.To)[:], b)
	}
	// Mint
	if b, err = s.Uint256Bytes(); err != nil {
		return err
	}
	tx.Mint = new(uint256.Int).SetBytes(b)
	// Value
	if b, err = s.Uint256Bytes(); err != nil {
		return err
	}
	tx.Value = new(uint256.Int).SetBytes(b)
	// Gas
	if tx.Gas, err = s.Uint(); err != nil {
		return err
	}
	if tx.IsSystemTransaction, err = s.Bool(); err != nil {
		return err
	}
	// Data
	if tx.Data, err = s.Bytes(); err != nil {
		return err
	}
	return s.ListEnd()
}

func (tx *DepositTx) FakeSign(address libcommon.Address) Transaction {
	cpy := tx.copy()
	cpy.SetSender(address)
	return cpy
}

func (tx *DepositTx) WithSignature(signer Signer, sig []byte) (Transaction, error) {
	return tx.copy(), nil
}

func (tx *DepositTx) Time() time.Time {
	return tx.time
}

func (tx *DepositTx) Type() byte { return DepositTxType }

func (tx *DepositTx) Hash() libcommon.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return *hash.(*libcommon.Hash)
	}
	hash := prefixedRlpHash(DepositTxType, []interface{}{
		tx.SourceHash,
		tx.From,
		tx.To,
		tx.Mint,
		tx.Value,
		tx.Gas,
		tx.IsSystemTransaction,
		tx.Data,
	})
	tx.hash.Store(&hash)
	return hash
}

// not sure ab this one lol
func (tx *DepositTx) Protected() bool {
	return true
}

func (tx *DepositTx) IsContractDeploy() bool {
	return false
}

func (tx *DepositTx) IsStarkNet() bool {
	return false
}

// All zero in the prototype
func (tx *DepositTx) GetPrice() *uint256.Int  { return uint256.NewInt(0) }
func (tx *DepositTx) GetTip() *uint256.Int    { return uint256.NewInt(0) }
func (tx *DepositTx) GetFeeCap() *uint256.Int { return uint256.NewInt(0) }
func (tx *DepositTx) GetEffectiveGasTip(baseFee *uint256.Int) *uint256.Int {
	return uint256.NewInt(0)
}

func (tx *DepositTx) Cost() *uint256.Int {
	return tx.Value.Clone()
}

func (tx *DepositTx) GetAccessList() types2.AccessList {
	return nil
}

// NewDepositTransaction creates a deposit transaction
func NewDepositTransaction(
	sourceHash libcommon.Hash,
	from libcommon.Address,
	to libcommon.Address,
	mint *uint256.Int,
	value *uint256.Int,
	gasLimit uint64,
	isSystemTx bool,
	data []byte) *DepositTx {
	return &DepositTx{
		SourceHash:          sourceHash,
		From:                from,
		To:                  &to,
		Mint:                mint,
		Value:               value,
		Gas:                 gasLimit,
		IsSystemTransaction: isSystemTx,
		Data:                data,
	}
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx *DepositTx) copy() *DepositTx {
	cpy := &DepositTx{
		SourceHash:          tx.SourceHash,
		From:                tx.From,
		To:                  tx.To,
		Mint:                nil,
		Value:               new(uint256.Int),
		Gas:                 tx.Gas,
		IsSystemTransaction: tx.IsSystemTransaction,
		Data:                libcommon.CopyBytes(tx.Data),
	}
	if tx.Mint != nil {
		cpy.Mint = new(uint256.Int).Set(tx.Mint)
	}
	if tx.Value != nil {
		cpy.Value.Set(tx.Value)
	}
	return cpy
}

// AsMessage returns the transaction as a core.Message.
func (tx *DepositTx) AsMessage(s Signer, baseFee *big.Int, rules *chain.Rules) (Message, error) {
	msg := Message{
		nonce:       0,
		gasLimit:    tx.Gas,
		gasPrice:    *uint256.NewInt(0),
		tip:         *uint256.NewInt(0),
		feeCap:      *uint256.NewInt(0),
		from:        tx.From,
		to:          tx.To,
		amount:      *tx.Value,
		data:        tx.Data,
		accessList:  nil,
		checkNonce:  true,
		isSystemTx:  tx.IsSystemTransaction,
		isDepositTx: true,
		mint:        tx.Mint,
	}
	return msg, nil
}

func (tx *DepositTx) Sender(signer Signer) (libcommon.Address, error) {
	return tx.From, nil
}

func (tx *DepositTx) cachedSender() (sender libcommon.Address, ok bool) {
	s := tx.From
	return s, true
	//if s == nil {
	//	return sender, false
	//}
	//return s.(libcommon.Address), true
}
func (tx *DepositTx) RollupCostData() types2.RollupCostData {
	return types2.RollupCostData{}
}

func (tx *DepositTx) GetDataHashes() []libcommon.Hash {
	// Only blob txs have data hashes
	return []libcommon.Hash{}
}

func (tx *DepositTx) Unwrap() Transaction {
	return tx
}
