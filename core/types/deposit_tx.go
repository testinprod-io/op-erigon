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
	"bytes"
	"fmt"
	"github.com/ledgerwatch/erigon/rlp"
	"io"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon/common"
)

type DepositTx struct {
	time time.Time // Time first seen locally (spam avoidance)
	// caches
	hash atomic.Value //nolint:structcheck
	size atomic.Value //nolint:structcheck
	// SourceHash uniquely identifies the source of the deposit
	SourceHash common.Hash
	// From is exposed through the types.Signer, not through TxData
	From common.Address
	// nil means contract creation
	To *common.Address `rlp:"nil"`
	// Mint is minted on L2, locked on L1, nil if no minting.
	Mint *uint256.Int
	// Value is transferred from L2 balance, executed after Mint (if any)
	Value *uint256.Int
	// gas limit
	Gas  uint64
	Data []byte
}

var _ Transaction = (*DepositTx)(nil)

func (tx DepositTx) GetChainID() *uint256.Int {
	panic("deposits are not signed and do not have a chain-ID")
}

// DepositsNonce identifies a deposit, since go-ethereum abstracts all transaction types to a core.Message.
// Deposits do not set a nonce, deposits are included by the system and cannot be repeated or included elsewhere.
const DepositsNonce uint64 = 0xffff_ffff_ffff_fffd

func (tx DepositTx) GetNonce() uint64 {
	return DepositsNonce
}

func (tx DepositTx) GetTo() *common.Address {
	return tx.To
}

func (tx DepositTx) GetGas() uint64 {
	return tx.Gas
}

func (tx DepositTx) GetValue() *uint256.Int {
	return tx.Value
}

func (tx DepositTx) GetData() []byte {
	return tx.Data
}

func (tx DepositTx) GetSender() (common.Address, bool) {
	return tx.From, false
}

func (tx DepositTx) SetSender(addr common.Address) {
	tx.From = addr
}

func (tx DepositTx) RawSignatureValues() (*uint256.Int, *uint256.Int, *uint256.Int) {
	panic("deposit tx does not have a signature")
}

func (tx DepositTx) SigningHash(chainID *big.Int) common.Hash {
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
func (tx DepositTx) EncodingSize() int {
	var buf bytes.Buffer
	if err := tx.MarshalBinary(&buf); err != nil {
		panic(err)
	}
	return len(buf.Bytes())
}

// MarshalBinary returns the canonical encoding of the transaction.
// For legacy transactions, it returns the RLP encoding. For EIP-2718 typed
// transactions, it returns the type and payload.
func (tx DepositTx) MarshalBinary(w io.Writer) error {
	if _, err := w.Write([]byte{DepositTxType}); err != nil {
		return err
	}
	if err := tx.encodePayload(w); err != nil {
		return err
	}
	return nil
}

func (tx DepositTx) encodePayload(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		tx.SourceHash,
		tx.From,
		tx.To,
		tx.Mint,
		tx.Value,
		tx.Gas,
		tx.Data,
	})
}

func (tx DepositTx) EncodeRLP(w io.Writer) error {
	return tx.MarshalBinary(w)
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
		tx.To = &common.Address{}
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
	// Data
	if tx.Data, err = s.Bytes(); err != nil {
		return err
	}
	return s.ListEnd()
}

func (tx *DepositTx) FakeSign(address common.Address) (Transaction, error) {
	cpy := tx.copy()
	cpy.SetSender(address)
	return cpy, nil
}

func (tx *DepositTx) WithSignature(signer Signer, sig []byte) (Transaction, error) {
	return tx.copy(), nil
}

func (tx DepositTx) Time() time.Time {
	return tx.time
}

func (tx DepositTx) Type() byte { return DepositTxType }

func (tx *DepositTx) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return *hash.(*common.Hash)
	}
	hash := rlpHash([]interface{}{
		tx.SourceHash,
		tx.From,
		tx.To,
		tx.Mint,
		tx.Value,
		tx.Gas,
		tx.Data,
	})
	tx.hash.Store(&hash)
	return hash
}

// not sure ab this one lol
func (tx DepositTx) Protected() bool {
	return true
}

func (tx DepositTx) IsContractDeploy() bool {
	return false
}

func (tx DepositTx) IsStarkNet() bool {
	return false
}

// All zero in the prototype
func (tx DepositTx) GetPrice() *uint256.Int  { return uint256.NewInt(0) }
func (tx DepositTx) GetTip() *uint256.Int    { return uint256.NewInt(0) }
func (tx DepositTx) GetFeeCap() *uint256.Int { return uint256.NewInt(0) }

// Is this needed at all?
func (tx DepositTx) GetEffectiveGasTip(baseFee *uint256.Int) *uint256.Int {
	if baseFee == nil {
		return tx.GetTip()
	}
	gasFeeCap := tx.GetFeeCap()
	// return 0 because effectiveFee cant be < 0
	if gasFeeCap.Lt(baseFee) {
		return uint256.NewInt(0)
	}
	effectiveFee := new(uint256.Int).Sub(gasFeeCap, baseFee)
	if tx.GetTip().Lt(effectiveFee) {
		return tx.GetTip()
	} else {
		return effectiveFee
	}
}

func (tx DepositTx) Cost() *uint256.Int {
	// No gas cost yet in prototype
	return tx.Value.Clone()
}

func (tx DepositTx) GetAccessList() AccessList {
	return nil
}

// NewDepositTransaction creates a deposit transaction
func NewDepositTransaction(to common.Address, mint *uint256.Int, amount *uint256.Int, gasLimit uint64, gasPrice *uint256.Int, data []byte) *DepositTx {
	return &DepositTx{
		// NOTE: Does the SourceHash get added some time after this function is called?
		// NOTE: from comes from TransactionMisc.from which is of type atomic.Value
		// nil means contract creation
		To: &to,
		// Mint is minted on L2, locked on L1, nil if no minting.
		Mint: mint,
		// Value is transferred from L2 balance, executed after Mint (if any)
		Value: amount,
		// gas limit
		Gas:  gasLimit,
		Data: data,
	}
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx DepositTx) copy() *DepositTx {
	cpy := &DepositTx{
		SourceHash: tx.SourceHash,
		From:       tx.From,
		To:         tx.To,
		Mint:       nil,
		Value:      new(uint256.Int),
		Gas:        tx.Gas,
		Data:       common.CopyBytes(tx.Data),
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
func (tx DepositTx) AsMessage(s Signer, _ *big.Int) (Message, error) {
	msg := Message{
		nonce:      DepositsNonce,
		gasLimit:   tx.Gas,
		gasPrice:   *uint256.NewInt(0),
		tip:        *uint256.NewInt(0),
		feeCap:     *uint256.NewInt(0),
		from:       tx.From,
		to:         tx.To,
		amount:     *tx.Value,
		data:       tx.Data,
		accessList: nil,
		checkNonce: true,
		mint:       tx.Mint,
	}

	return msg, nil
}

func (tx *DepositTx) Sender(signer Signer) (common.Address, error) {
	return tx.From, nil
}
