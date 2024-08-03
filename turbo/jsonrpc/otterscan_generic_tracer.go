// Copyright 2024 The Erigon Authors
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

package jsonrpc

import (
	"context"
	"github.com/ledgerwatch/erigon-lib/opstack"

	"github.com/erigontech/erigon-lib/chain"
	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/log/v3"
	"github.com/erigontech/erigon/cmd/state/exec3"

	"github.com/erigontech/erigon/core/types"
	"github.com/erigontech/erigon/core/vm"
)

type GenericTracer interface {
	vm.EVMLogger
	SetTransaction(tx types.Transaction)
	Found() bool
}

func (api *OtterscanAPIImpl) genericTracer(dbtx kv.Tx, ctx context.Context, blockNum, txnID uint64, txIndex int, chainConfig *chain.Config, tracer GenericTracer) error {
	ttx := dbtx.(kv.TemporalTx)
	executor := exec3.NewTraceWorker(ttx, chainConfig, api.engine(), api._blockReader, tracer)

	// if block number changed, calculate all related field
	header, err := api._blockReader.HeaderByNumber(ctx, ttx, blockNum)
	if err != nil {
		return err
	}
	if header == nil {
		log.Warn("[rpc] header is nil", "blockNum", blockNum)
		return nil
	}
	executor.ChangeBlock(header)

	txn, err := api._txnReader.TxnByIdxInBlock(ctx, ttx, blockNum, txIndex)
	if err != nil {
		return err
	}
	if txn == nil {
		log.Warn("[rpc genericTracer] txn is nil", "blockNum", blockNum, "txIndex", txIndex)
		return nil
	}
<<<<<<< HEAD

	header := block.Header()
	rules := chainConfig.Rules(block.NumberU64(), header.Time)
	signer := types.MakeSigner(chainConfig, blockNum, header.Time)
	for idx, tx := range block.Transactions() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ibs.SetTxContext(tx.Hash(), block.Hash(), idx)

		msg, _ := tx.AsMessage(*signer, header.BaseFee, rules)

		BlockContext := core.NewEVMBlockContext(header, core.GetHashFn(header, getHeader), engine, nil)
		BlockContext.L1CostFunc = opstack.NewL1CostFunc(chainConfig, ibs)
		TxContext := core.NewEVMTxContext(msg)

		vmenv := vm.NewEVM(BlockContext, TxContext, ibs, chainConfig, vm.Config{Debug: true, Tracer: tracer})
		if _, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(tx.GetGas()).AddBlobGas(tx.GetBlobGas()), true /* refunds */, false /* gasBailout */); err != nil {
			return err
		}
		_ = ibs.FinalizeTx(rules, cachedWriter)

		if tracer.Found() {
			tracer.SetTransaction(tx)
			return nil
		}
=======
	_, err = executor.ExecTxn(txnID, txIndex, txn)
	if err != nil {
		return err
>>>>>>> v3.0.0-alpha1
	}
	return nil
}
