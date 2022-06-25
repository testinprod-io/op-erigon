package types

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/common/hexutil"
	"github.com/valyala/fastjson"
)

// txJSON is the JSON representation of transactions.
type txJSON struct {
	Type hexutil.Uint64 `json:"type"`

	// Common transaction fields:
	Nonce    *hexutil.Uint64 `json:"nonce"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	FeeCap   *hexutil.Big    `json:"maxFeePerGas"`
	Tip      *hexutil.Big    `json:"maxPriorityFeePerGas"`
	Gas      *hexutil.Uint64 `json:"gas"`
	Value    *hexutil.Big    `json:"value"`
	Data     *hexutil.Bytes  `json:"input"`
	V        *hexutil.Big    `json:"v"`
	R        *hexutil.Big    `json:"r"`
	S        *hexutil.Big    `json:"s"`
	To       *common.Address `json:"to"`

	// Deposit transaction fields
	SourceHash *common.Hash    `json:"sourceHash,omitempty"`
	From       *common.Address `json:"from,omitempty"`
	Mint       *hexutil.Big    `json:"mint,omitempty"`
	IsSystemTx *bool           `json:"isSystemTx,omitempty"`

	// Access list transaction fields:
	ChainID    *hexutil.Big `json:"chainId,omitempty"`
	AccessList *AccessList  `json:"accessList,omitempty"`

	// Only used for encoding:
	Hash common.Hash `json:"hash"`
}

func (tx LegacyTx) MarshalJSON() ([]byte, error) {
	var enc txJSON
	// These are set for all tx types.
	enc.Hash = tx.Hash()
	enc.Type = hexutil.Uint64(tx.Type())
	enc.Nonce = (*hexutil.Uint64)(&tx.Nonce)
	enc.Gas = (*hexutil.Uint64)(&tx.Gas)
	enc.GasPrice = (*hexutil.Big)(tx.GasPrice.ToBig())
	enc.Value = (*hexutil.Big)(tx.Value.ToBig())
	enc.Data = (*hexutil.Bytes)(&tx.Data)
	enc.To = tx.To
	enc.V = (*hexutil.Big)(tx.V.ToBig())
	enc.R = (*hexutil.Big)(tx.R.ToBig())
	enc.S = (*hexutil.Big)(tx.S.ToBig())
	return json.Marshal(&enc)
}

func (tx AccessListTx) MarshalJSON() ([]byte, error) {
	var enc txJSON
	// These are set for all tx types.
	enc.Hash = tx.Hash()
	enc.Type = hexutil.Uint64(tx.Type())
	enc.ChainID = (*hexutil.Big)(tx.ChainID.ToBig())
	enc.AccessList = &tx.AccessList
	enc.Nonce = (*hexutil.Uint64)(&tx.Nonce)
	enc.Gas = (*hexutil.Uint64)(&tx.Gas)
	enc.GasPrice = (*hexutil.Big)(tx.GasPrice.ToBig())
	enc.Value = (*hexutil.Big)(tx.Value.ToBig())
	enc.Data = (*hexutil.Bytes)(&tx.Data)
	enc.To = tx.To
	enc.V = (*hexutil.Big)(tx.V.ToBig())
	enc.R = (*hexutil.Big)(tx.R.ToBig())
	enc.S = (*hexutil.Big)(tx.S.ToBig())
	return json.Marshal(&enc)
}

func (tx DynamicFeeTransaction) MarshalJSON() ([]byte, error) {
	var enc txJSON
	// These are set for all tx types.
	enc.Hash = tx.Hash()
	enc.Type = hexutil.Uint64(tx.Type())
	enc.ChainID = (*hexutil.Big)(tx.ChainID.ToBig())
	enc.AccessList = &tx.AccessList
	enc.Nonce = (*hexutil.Uint64)(&tx.Nonce)
	enc.Gas = (*hexutil.Uint64)(&tx.Gas)
	enc.FeeCap = (*hexutil.Big)(tx.FeeCap.ToBig())
	enc.Tip = (*hexutil.Big)(tx.Tip.ToBig())
	enc.Value = (*hexutil.Big)(tx.Value.ToBig())
	enc.Data = (*hexutil.Bytes)(&tx.Data)
	enc.To = tx.To
	enc.V = (*hexutil.Big)(tx.V.ToBig())
	enc.R = (*hexutil.Big)(tx.R.ToBig())
	enc.S = (*hexutil.Big)(tx.S.ToBig())
	return json.Marshal(&enc)
}

func (tx DepositTx) MarshalJSON() ([]byte, error) {
	var enc txJSON

	enc.Hash = tx.Hash()
	enc.Type = hexutil.Uint64(tx.Type())
	enc.ChainID = (*hexutil.Big)(common.Big0)
	enc.Gas = (*hexutil.Uint64)(&tx.Gas)
	enc.Value = (*hexutil.Big)(tx.Value.ToBig())
	enc.Data = (*hexutil.Bytes)(&tx.Data)
	enc.To = tx.To
	// DepositTx fields
	enc.SourceHash = &tx.SourceHash
	enc.From = &tx.From
	if tx.Mint != nil {
		enc.Mint = (*hexutil.Big)(tx.Mint.ToBig())
	}
	enc.IsSystemTx = &tx.IsSystemTransaction
	// other fields will show up as null.
	return json.Marshal(&enc)
}

func UnmarshalTransactionFromJSON(input []byte) (Transaction, error) {
	var p fastjson.Parser
	v, err := p.ParseBytes(input)
	if err != nil {
		return nil, fmt.Errorf("parse transaction json: %w", err)
	}
	// check the type
	txTypeHex := v.GetStringBytes("type")
	var txType hexutil.Uint64 = LegacyTxType
	if txTypeHex != nil {
		if err = txType.UnmarshalText(txTypeHex); err != nil {
			return nil, err
		}
	}
	switch byte(txType) {
	case LegacyTxType:
		tx := &LegacyTx{}
		if err = tx.UnmarshalJSON(input); err != nil {
			return nil, err
		}
		return tx, nil
	case AccessListTxType:
		tx := &AccessListTx{}
		if err = tx.UnmarshalJSON(input); err != nil {
			return nil, err
		}
		return tx, nil
	case DynamicFeeTxType:
		tx := &DynamicFeeTransaction{}
		if err = tx.UnmarshalJSON(input); err != nil {
			return nil, err
		}
		return tx, nil
	case DepositTxType:
		tx := &DepositTx{}
		if err = tx.UnmarshalJSON(input); err != nil {
			return nil, err
		}
		return tx, nil
	default:
		return nil, fmt.Errorf("unknown transaction type: %v", txType)
	}
}

func (tx *LegacyTx) UnmarshalJSON(input []byte) error {
	var dec txJSON
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.To != nil {
		tx.To = dec.To
	}
	if dec.Nonce == nil {
		return errors.New("missing required field 'nonce' in transaction")
	}
	tx.Nonce = uint64(*dec.Nonce)
	if dec.GasPrice == nil {
		return errors.New("missing required field 'gasPrice' in transaction")
	}
	var overflow bool
	tx.GasPrice, overflow = uint256.FromBig(dec.GasPrice.ToInt())
	if overflow {
		return errors.New("'gasPrice' in transaction does not fit in 256 bits")
	}
	if dec.Gas == nil {
		return errors.New("missing required field 'gas' in transaction")
	}
	tx.Gas = uint64(*dec.Gas)
	if dec.Value == nil {
		return errors.New("missing required field 'value' in transaction")
	}
	tx.Value, overflow = uint256.FromBig(dec.Value.ToInt())
	if overflow {
		return errors.New("'value' in transaction does not fit in 256 bits")
	}
	if dec.Data == nil {
		return errors.New("missing required field 'input' in transaction")
	}
	tx.Data = *dec.Data
	if dec.V == nil {
		return errors.New("missing required field 'v' in transaction")
	}
	overflow = tx.V.SetFromBig(dec.V.ToInt())
	if overflow {
		return fmt.Errorf("dec.V higher than 2^256-1")
	}
	if dec.R == nil {
		return errors.New("missing required field 'r' in transaction")
	}
	overflow = tx.R.SetFromBig(dec.R.ToInt())
	if overflow {
		return fmt.Errorf("dec.R higher than 2^256-1")
	}
	if dec.S == nil {
		return errors.New("missing required field 's' in transaction")
	}
	overflow = tx.S.SetFromBig(dec.S.ToInt())
	if overflow {
		return fmt.Errorf("dec.S higher than 2^256-1")
	}
	if overflow {
		return errors.New("'s' in transaction does not fit in 256 bits")
	}
	withSignature := !tx.V.IsZero() || !tx.R.IsZero() || !tx.S.IsZero()
	if withSignature {
		if err := sanityCheckSignature(&tx.V, &tx.R, &tx.S, true); err != nil {
			return err
		}
	}
	return nil
}

func (tx *AccessListTx) UnmarshalJSON(input []byte) error {
	var dec txJSON
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	// Access list is optional for now.
	if dec.AccessList != nil {
		tx.AccessList = *dec.AccessList
	}
	if dec.ChainID == nil {
		return errors.New("missing required field 'chainId' in transaction")
	}
	var overflow bool
	tx.ChainID, overflow = uint256.FromBig(dec.ChainID.ToInt())
	if overflow {
		return errors.New("'chainId' in transaction does not fit in 256 bits")
	}
	if dec.To != nil {
		tx.To = dec.To
	}
	if dec.Nonce == nil {
		return errors.New("missing required field 'nonce' in transaction")
	}
	tx.Nonce = uint64(*dec.Nonce)
	if dec.GasPrice == nil {
		return errors.New("missing required field 'gasPrice' in transaction")
	}
	tx.GasPrice, overflow = uint256.FromBig(dec.GasPrice.ToInt())
	if overflow {
		return errors.New("'gasPrice' in transaction does not fit in 256 bits")
	}
	if dec.Gas == nil {
		return errors.New("missing required field 'gas' in transaction")
	}
	tx.Gas = uint64(*dec.Gas)
	if dec.Value == nil {
		return errors.New("missing required field 'value' in transaction")
	}
	tx.Value, overflow = uint256.FromBig(dec.Value.ToInt())
	if overflow {
		return errors.New("'value' in transaction does not fit in 256 bits")
	}
	if dec.Data == nil {
		return errors.New("missing required field 'input' in transaction")
	}
	tx.Data = *dec.Data
	if dec.V == nil {
		return errors.New("missing required field 'v' in transaction")
	}
	overflow = tx.V.SetFromBig(dec.V.ToInt())
	if overflow {
		return fmt.Errorf("dec.V higher than 2^256-1")
	}
	if dec.R == nil {
		return errors.New("missing required field 'r' in transaction")
	}
	overflow = tx.R.SetFromBig(dec.R.ToInt())
	if overflow {
		return fmt.Errorf("dec.R higher than 2^256-1")
	}
	if dec.S == nil {
		return errors.New("missing required field 's' in transaction")
	}
	overflow = tx.S.SetFromBig(dec.S.ToInt())
	if overflow {
		return fmt.Errorf("dec.S higher than 2^256-1")
	}
	withSignature := !tx.V.IsZero() || !tx.R.IsZero() || !tx.S.IsZero()
	if withSignature {
		if err := sanityCheckSignature(&tx.V, &tx.R, &tx.S, false); err != nil {
			return err
		}
	}
	return nil
}

func (tx *DynamicFeeTransaction) UnmarshalJSON(input []byte) error {
	var dec txJSON
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	// Access list is optional for now.
	if dec.AccessList != nil {
		tx.AccessList = *dec.AccessList
	}
	if dec.ChainID == nil {
		return errors.New("missing required field 'chainId' in transaction")
	}
	var overflow bool
	tx.ChainID, overflow = uint256.FromBig(dec.ChainID.ToInt())
	if overflow {
		return errors.New("'chainId' in transaction does not fit in 256 bits")
	}
	if dec.To != nil {
		tx.To = dec.To
	}
	if dec.Nonce == nil {
		return errors.New("missing required field 'nonce' in transaction")
	}
	tx.Nonce = uint64(*dec.Nonce)
	if dec.GasPrice == nil {
		return errors.New("missing required field 'gasPrice' in transaction")
	}
	tx.Tip, overflow = uint256.FromBig(dec.Tip.ToInt())
	if overflow {
		return errors.New("'tip' in transaction does not fit in 256 bits")
	}
	tx.FeeCap, overflow = uint256.FromBig(dec.FeeCap.ToInt())
	if overflow {
		return errors.New("'feeCap' in transaction does not fit in 256 bits")
	}
	if dec.Gas == nil {
		return errors.New("missing required field 'gas' in transaction")
	}
	tx.Gas = uint64(*dec.Gas)
	if dec.Value == nil {
		return errors.New("missing required field 'value' in transaction")
	}
	tx.Value, overflow = uint256.FromBig(dec.Value.ToInt())
	if overflow {
		return errors.New("'value' in transaction does not fit in 256 bits")
	}
	if dec.Data == nil {
		return errors.New("missing required field 'input' in transaction")
	}
	tx.Data = *dec.Data
	if dec.V == nil {
		return errors.New("missing required field 'v' in transaction")
	}
	overflow = tx.V.SetFromBig(dec.V.ToInt())
	if overflow {
		return fmt.Errorf("dec.V higher than 2^256-1")
	}
	if dec.R == nil {
		return errors.New("missing required field 'r' in transaction")
	}
	overflow = tx.R.SetFromBig(dec.R.ToInt())
	if overflow {
		return fmt.Errorf("dec.R higher than 2^256-1")
	}
	if dec.S == nil {
		return errors.New("missing required field 's' in transaction")
	}
	overflow = tx.S.SetFromBig(dec.S.ToInt())
	if overflow {
		return fmt.Errorf("dec.S higher than 2^256-1")
	}
	if overflow {
		return errors.New("'s' in transaction does not fit in 256 bits")
	}
	withSignature := !tx.V.IsZero() || !tx.R.IsZero() || !tx.S.IsZero()
	if withSignature {
		if err := sanityCheckSignature(&tx.V, &tx.R, &tx.S, false); err != nil {
			return err
		}
	}
	return nil
}

func (tx *DepositTx) UnmarshalJSON(input []byte) error {
	var dec txJSON
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.AccessList != nil || dec.V != nil || dec.R != nil || dec.S != nil || dec.FeeCap != nil ||
		dec.Tip != nil || dec.GasPrice != nil || (dec.Nonce != nil && *dec.Nonce != 0) {
		return errors.New("unexpected field(s) in deposit transaction")
	}

	if dec.To != nil {
		tx.To = dec.To
	}
	tx.Gas = uint64(*dec.Gas)
	if dec.Value == nil {
		return errors.New("missing required field 'value' in transaction")
	}
	var overflow bool
	tx.Value, overflow = uint256.FromBig(dec.Value.ToInt())
	if overflow {
		return errors.New("'value' in transaction does not fit in 256 bits")
	}
	// mint may be omitted or nil if there is nothing to mint.
	tx.Mint, overflow = uint256.FromBig(dec.Mint.ToInt())
	if overflow {
		return errors.New("'mint' in transaction does not fit in 256 bits")
	}
	if dec.Data == nil {
		return errors.New("missing required field 'input' in transaction")
	}
	tx.Data = *dec.Data
	if dec.From == nil {
		return errors.New("missing required field 'from' in transaction")
	}
	tx.From = *dec.From
	if dec.SourceHash == nil {
		return errors.New("missing required field 'sourceHash' in transaction")
	}
	tx.SourceHash = *dec.SourceHash
	// IsSystemTx may be omitted. Defaults to false.
	if dec.IsSystemTx != nil {
		tx.IsSystemTransaction = *dec.IsSystemTx
	}
	return nil
}
