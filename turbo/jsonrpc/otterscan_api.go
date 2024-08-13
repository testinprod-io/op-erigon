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
	"errors"
	"fmt"
	"github.com/erigontech/erigon/eth/tracers"
	"math/big"

	"github.com/holiman/uint256"
	"golang.org/x/sync/errgroup"

	"github.com/erigontech/erigon-lib/chain"
	"github.com/erigontech/erigon-lib/common"
	hexutil2 "github.com/erigontech/erigon-lib/common/hexutil"
	"github.com/erigontech/erigon-lib/common/hexutility"
	"github.com/erigontech/erigon-lib/kv"

	"github.com/erigontech/erigon/consensus"
	"github.com/erigontech/erigon/core"
	"github.com/erigontech/erigon/core/rawdb"
	"github.com/erigontech/erigon/core/types"
	"github.com/erigontech/erigon/core/vm"
	"github.com/erigontech/erigon/core/vm/evmtypes"
	"github.com/erigontech/erigon/eth/ethutils"
	"github.com/erigontech/erigon/rpc"
	"github.com/erigontech/erigon/turbo/adapter/ethapi"
	"github.com/erigontech/erigon/turbo/rpchelper"
	"github.com/erigontech/erigon/turbo/transactions"
)

// API_LEVEL Must be incremented every time new additions are made
const API_LEVEL = 8

type TransactionsWithReceipts struct {
	Txs       []*RPCTransaction        `json:"txs"`
	Receipts  []map[string]interface{} `json:"receipts"`
	FirstPage bool                     `json:"firstPage"`
	LastPage  bool                     `json:"lastPage"`
}

type OtterscanAPI interface {
	GetApiLevel() uint8
	GetInternalOperations(ctx context.Context, hash common.Hash) ([]*InternalOperation, error)
	SearchTransactionsBefore(ctx context.Context, addr common.Address, blockNum uint64, pageSize uint16) (*TransactionsWithReceipts, error)
	SearchTransactionsAfter(ctx context.Context, addr common.Address, blockNum uint64, pageSize uint16) (*TransactionsWithReceipts, error)
	GetBlockDetails(ctx context.Context, number rpc.BlockNumber) (map[string]interface{}, error)
	GetBlockDetailsByHash(ctx context.Context, hash common.Hash) (map[string]interface{}, error)
	GetBlockTransactions(ctx context.Context, number rpc.BlockNumber, pageNumber uint8, pageSize uint8) (map[string]interface{}, error)
	HasCode(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (bool, error)
	TraceTransaction(ctx context.Context, hash common.Hash) ([]*TraceEntry, error)
	GetTransactionError(ctx context.Context, hash common.Hash) (hexutility.Bytes, error)
	GetTransactionBySenderAndNonce(ctx context.Context, addr common.Address, nonce uint64) (*common.Hash, error)
	GetContractCreator(ctx context.Context, addr common.Address) (*ContractCreatorData, error)
}

type OtterscanAPIImpl struct {
	*BaseAPI
	db          kv.RoDB
	maxPageSize uint64
}

func NewOtterscanAPI(base *BaseAPI, db kv.RoDB, maxPageSize uint64) *OtterscanAPIImpl {
	return &OtterscanAPIImpl{
		BaseAPI:     base,
		db:          db,
		maxPageSize: maxPageSize,
	}
}

func (api *OtterscanAPIImpl) GetApiLevel() uint8 {
	return API_LEVEL
}

// TODO: dedup from eth_txs.go#GetTransactionByHash
func (api *OtterscanAPIImpl) getTransactionByHash(ctx context.Context, tx kv.Tx, hash common.Hash) (types.Transaction, *types.Block, common.Hash, uint64, uint64, error) {
	// https://infura.io/docs/ethereum/json-rpc/eth-getTransactionByHash
	blockNum, ok, err := api.txnLookup(ctx, tx, hash)
	if err != nil {
		return nil, nil, common.Hash{}, 0, 0, err
	}
	if !ok {
		return nil, nil, common.Hash{}, 0, 0, nil
	}

	block, err := api.blockByNumberWithSenders(ctx, tx, blockNum)
	if err != nil {
		return nil, nil, common.Hash{}, 0, 0, err
	}
	if block == nil {
		return nil, nil, common.Hash{}, 0, 0, nil
	}
	blockHash := block.Hash()
	var txnIndex uint64
	var txn types.Transaction
	for i, transaction := range block.Transactions() {
		if transaction.Hash() == hash {
			txn = transaction
			txnIndex = uint64(i)
			break
		}
	}

	// Add GasPrice for the DynamicFeeTransaction
	// var baseFee *big.Int
	// if chainConfig.IsLondon(blockNum) && blockHash != (common.Hash{}) {
	// 	baseFee = block.BaseFee()
	// }

	// if no transaction was found then we return nil
	if txn == nil {
		return nil, nil, common.Hash{}, 0, 0, nil
	}
	return txn, block, blockHash, blockNum, txnIndex, nil
}

func (api *OtterscanAPIImpl) relayToHistoricalBackend(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	return api.historicalRPCService.CallContext(ctx, result, method, args...)
}

func (api *OtterscanAPIImpl) translateCaptureStart(gethTrace *GethTrace, tracer vm.EVMLogger, vmenv *vm.EVM) error {
	from := common.HexToAddress(gethTrace.From)
	to := common.HexToAddress(gethTrace.To)
	input, err := hexutil2.Decode(gethTrace.Input)
	if err != nil {
		if err != hexutil2.ErrEmptyString {
			return err
		}
		input = []byte{}
	}
	valueBig, err := hexutil2.DecodeBig(gethTrace.Value)
	if err != nil {
		if err != hexutil2.ErrEmptyString {
			return err
		}
		valueBig = big.NewInt(0)
	}
	value, _ := uint256.FromBig(valueBig)
	gas, err := hexutil2.DecodeUint64(gethTrace.Gas)
	if err != nil {
		return err
	}
	_, isPrecompile := vmenv.Precompile(to)
	// dummy code
	code := []byte{}
	tracer.CaptureStart(vmenv, from, to, isPrecompile, false, input, gas, value, code)
	return nil
}

func (api *OtterscanAPIImpl) translateOpcode(typStr string) (vm.OpCode, error) {
	switch typStr {
	default:
	case "CALL":
		return vm.CALL, nil
	case "STATICCALL":
		return vm.STATICCALL, nil
	case "DELEGATECALL":
		return vm.DELEGATECALL, nil
	case "CALLCODE":
		return vm.CALLCODE, nil
	case "CREATE":
		return vm.CREATE, nil
	case "CREATE2":
		return vm.CREATE2, nil
	case "SELFDESTRUCT":
		return vm.SELFDESTRUCT, nil
	}
	return vm.INVALID, fmt.Errorf("unable to translate %s", typStr)
}

func (api *OtterscanAPIImpl) translateCaptureEnter(gethTrace *GethTrace, tracer vm.EVMLogger, vmenv *vm.EVM) error {
	from := common.HexToAddress(gethTrace.From)
	to := common.HexToAddress(gethTrace.To)
	input, err := hexutil2.Decode(gethTrace.Input)
	if err != nil {
		if err != hexutil2.ErrEmptyString {
			return err
		}
		input = []byte{}
	}
	valueBig, err := hexutil2.DecodeBig(gethTrace.Value)
	if err != nil {
		if err != hexutil2.ErrEmptyString {
			return err
		}
		valueBig = big.NewInt(0)
	}
	value, _ := uint256.FromBig(valueBig)
	gas, err := hexutil2.DecodeUint64(gethTrace.Gas)
	if err != nil {
		return err
	}
	typStr := gethTrace.Type
	typ, err := api.translateOpcode(typStr)
	if err != nil {
		return err
	}
	_, isPrecompile := vmenv.Precompile(to)
	tracer.CaptureEnter(typ, from, to, isPrecompile, false, input, gas, value, nil)
	return nil
}

func (api *OtterscanAPIImpl) translateCaptureExit(gethTrace *GethTrace, tracer vm.EVMLogger) error {
	usedGas, err := hexutil2.DecodeUint64(gethTrace.GasUsed)
	if err != nil {
		return err
	}
	output, err := hexutil2.Decode(gethTrace.Output)
	if err != nil {
		if err != hexutil2.ErrEmptyString {
			return err
		}
		output = []byte{}
	}
	err = errors.New(gethTrace.Error)
	tracer.CaptureExit(output, usedGas, err)
	return nil
}

func (api *OtterscanAPIImpl) translateRelayTraceResult(gethTrace *GethTrace, tracer vm.EVMLogger, chainConfig *chain.Config) error {
	vmenv := vm.NewEVM(evmtypes.BlockContext{}, evmtypes.TxContext{}, nil, chainConfig, vm.Config{})
	type traceWithIndex struct {
		gethTrace *GethTrace
		idx       int // children index
	}
	callStacks := make([]*traceWithIndex, 0)
	started := false
	// Each call stack can call and trigger sub call stack.
	// rootIndex indicates the index of child for current inspected parent node trace.
	rootIndex := 0
	var trace *GethTrace = gethTrace
	// iterative postorder traversal
	for trace != nil || len(callStacks) > 0 {
		if trace != nil {
			// push back
			callStacks = append(callStacks, &traceWithIndex{trace, rootIndex})
			if !started {
				started = true
				if err := api.translateCaptureStart(trace, tracer, vmenv); err != nil {
					return err
				}
			} else {
				if err := api.translateCaptureEnter(trace, tracer, vmenv); err != nil {
					return err
				}
			}
			rootIndex = 0
			if len(trace.Calls) > 0 {
				trace = trace.Calls[0]
			} else {
				trace = nil
			}
			continue
		}
		// pop back
		top := callStacks[len(callStacks)-1]
		callStacks = callStacks[:len(callStacks)-1]
		if err := api.translateCaptureExit(top.gethTrace, tracer); err != nil {
			return err
		}
		// pop back callstack repeatly until popped element is last children of top of the callstack
		for len(callStacks) > 0 && top.idx == len(callStacks[len(callStacks)-1].gethTrace.Calls)-1 {
			// pop back
			top = callStacks[len(callStacks)-1]
			callStacks = callStacks[:len(callStacks)-1]
			if err := api.translateCaptureExit(top.gethTrace, tracer); err != nil {
				return err
			}
		}
		if len(callStacks) > 0 {
			trace = callStacks[len(callStacks)-1].gethTrace.Calls[top.idx+1]
			rootIndex = top.idx + 1
		}
	}
	return nil
}

func (api *OtterscanAPIImpl) runTracer(ctx context.Context, tx kv.Tx, hash common.Hash, tracer vm.EVMLogger) (*evmtypes.ExecutionResult, error) {
	txn, block, _, _, txIndex, err := api.getTransactionByHash(ctx, tx, hash)
	if err != nil {
		return nil, err
	}
	if txn == nil {
		return nil, fmt.Errorf("transaction %#x not found", hash)
	}

	chainConfig, err := api.chainConfig(ctx, tx)
	if err != nil {
		return nil, err
	}

	blockNum := block.NumberU64()
	if chainConfig.IsOptimismPreBedrock(blockNum) {
		if api.historicalRPCService == nil {
			return nil, rpc.ErrNoHistoricalFallback
		}
		// geth returns nested json so we have to flatten
		treeResult := &GethTrace{}
		callTracer := "callTracer"
		if err := api.relayToHistoricalBackend(ctx, treeResult, "debug_traceTransaction", hash, &tracers.TraceConfig{Tracer: &callTracer}); err != nil {
			return nil, fmt.Errorf("historical backend error: %w", err)
		}
		if tracer != nil {
			err := api.translateRelayTraceResult(treeResult, tracer, chainConfig)
			if err != nil {
				return nil, err
			}
		}
		usedGas, err := hexutil2.DecodeUint64(treeResult.GasUsed)
		if err != nil {
			return nil, err
		}
		returnData, err := hexutil2.Decode(treeResult.Output)
		if err != nil {
			if err != hexutil2.ErrEmptyString {
				return nil, err
			}
			returnData = []byte{}
		}
		result := &evmtypes.ExecutionResult{
			UsedGas:    usedGas,
			Err:        errors.New(treeResult.Error),
			ReturnData: returnData,
		}
		return result, nil
	}

	engine := api.engine()

	msg, blockCtx, txCtx, ibs, _, err := transactions.ComputeTxEnv(ctx, engine, block, chainConfig, api._blockReader, tx, int(txIndex))
	if err != nil {
		return nil, err
	}

	var vmConfig vm.Config
	if tracer == nil {
		vmConfig = vm.Config{}
	} else {
		vmConfig = vm.Config{Debug: true, Tracer: tracer}
	}
	vmenv := vm.NewEVM(blockCtx, txCtx, ibs, chainConfig, vmConfig)

	result, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(msg.Gas()).AddBlobGas(msg.BlobGas()), true, false /* gasBailout */)
	if err != nil {
		return nil, fmt.Errorf("tracing failed: %v", err)
	}

	return result, nil
}

func (api *OtterscanAPIImpl) GetInternalOperations(ctx context.Context, hash common.Hash) ([]*InternalOperation, error) {
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	tracer := NewOperationsTracer(ctx)
	if _, err := api.runTracer(ctx, tx, hash, tracer); err != nil {
		return nil, err
	}

	return tracer.Results, nil
}

// Search transactions that touch a certain address.
//
// It searches back a certain block (excluding); the results are sorted descending.
//
// The pageSize indicates how many txs may be returned. If there are less txs than pageSize,
// they are just returned. But it may return a little more than pageSize if there are more txs
// than the necessary to fill pageSize in the last found block, i.e., let's say you want pageSize == 25,
// you already found 24 txs, the next block contains 4 matches, then this function will return 28 txs.
func (api *OtterscanAPIImpl) SearchTransactionsBefore(ctx context.Context, addr common.Address, blockNum uint64, pageSize uint16) (*TransactionsWithReceipts, error) {
	if uint64(pageSize) > api.maxPageSize {
		return nil, fmt.Errorf("max allowed page size: %v", api.maxPageSize)
	}

	dbtx, err := api.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer dbtx.Rollback()

	return api.searchTransactionsBeforeV3(dbtx.(kv.TemporalTx), ctx, addr, blockNum, pageSize)
}

// Search transactions that touch a certain address.
//
// It searches forward a certain block (excluding); the results are sorted descending.
//
// The pageSize indicates how many txs may be returned. If there are less txs than pageSize,
// they are just returned. But it may return a little more than pageSize if there are more txs
// than the necessary to fill pageSize in the last found block, i.e., let's say you want pageSize == 25,
// you already found 24 txs, the next block contains 4 matches, then this function will return 28 txs.
func (api *OtterscanAPIImpl) SearchTransactionsAfter(ctx context.Context, addr common.Address, blockNum uint64, pageSize uint16) (*TransactionsWithReceipts, error) {
	if uint64(pageSize) > api.maxPageSize {
		return nil, fmt.Errorf("max allowed page size: %v", api.maxPageSize)
	}

	dbtx, err := api.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer dbtx.Rollback()

	return api.searchTransactionsAfterV3(dbtx.(kv.TemporalTx), ctx, addr, blockNum, pageSize)
}

func (api *OtterscanAPIImpl) traceBlocks(ctx context.Context, addr common.Address, chainConfig *chain.Config, pageSize, resultCount uint16, callFromToProvider BlockProvider) ([]*TransactionsWithReceipts, bool, error) {
	// Estimate the common case of user address having at most 1 interaction/block and
	// trace N := remaining page matches as number of blocks to trace concurrently.
	// TODO: this is not optimimal for big contract addresses; implement some better heuristics.
	estBlocksToTrace := pageSize - resultCount
	results := make([]*TransactionsWithReceipts, estBlocksToTrace)
	totalBlocksTraced := 0
	hasMore := true

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(1024) // we don't want limit much here, but protecting from infinity attack
	for i := 0; i < int(estBlocksToTrace); i++ {
		i := i // we will pass it to goroutine

		var nextBlock uint64
		var err error
		nextBlock, hasMore, err = callFromToProvider()
		if err != nil {
			return nil, false, err
		}
		// TODO: nextBlock == 0 seems redundant with hasMore == false
		if !hasMore && nextBlock == 0 {
			break
		}

		totalBlocksTraced++

		eg.Go(func() error {
			// don't return error from searchTraceBlock if canceled - to avoid 1 block fail impact to other blocks
			// if return error - `errgroup` will interrupt all other goroutines
			// but passing `ctx` - then user still can cancel request
			select {
			case <-ctx.Done():
				// do not return error because error already returned at problematic goroutine
				return nil
			default:
				// return err if not canceled. it means db inconsistency detected
				return api.searchTraceBlock(ctx, addr, chainConfig, i, nextBlock, results)
			}
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, false, err
	}

	return results[:totalBlocksTraced], hasMore, nil
}

func delegateGetBlockByNumber(tx kv.Tx, b *types.Block, number rpc.BlockNumber, inclTx bool) (map[string]interface{}, error) {
	td, err := rawdb.ReadTd(tx, b.Hash(), b.NumberU64())
	if err != nil {
		return nil, err
	}
	additionalFields := make(map[string]interface{})
	receipts := rawdb.ReadRawReceipts(tx, uint64(number.Int64()))
	response, err := ethapi.RPCMarshalBlock(b, inclTx, inclTx, additionalFields, receipts)
	if !inclTx {
		delete(response, "transactions") // workaround for https://github.com/erigontech/erigon/issues/4989#issuecomment-1218415666
	}
	response["totalDifficulty"] = (*hexutil2.Big)(td)
	response["transactionCount"] = b.Transactions().Len()

	if err == nil && number == rpc.PendingBlockNumber {
		// Pending blocks need to nil out a few fields
		for _, field := range []string{"hash", "nonce", "miner"} {
			response[field] = nil
		}
	}

	// Explicitly drop unwanted fields
	response["logsBloom"] = nil
	return response, err
}

// TODO: temporary workaround due to API breakage from watch_the_burn
type internalIssuance struct {
	BlockReward string `json:"blockReward,omitempty"`
	UncleReward string `json:"uncleReward,omitempty"`
	Issuance    string `json:"issuance,omitempty"`
}

func delegateIssuance(tx kv.Tx, block *types.Block, chainConfig *chain.Config, engine consensus.EngineReader) (internalIssuance, error) {
	// TODO: aura seems to be already broken in the original version of this RPC method
	rewards, err := engine.CalculateRewards(chainConfig, block.HeaderNoCopy(), block.Uncles(), func(contract common.Address, data []byte) ([]byte, error) {
		return nil, nil
	})
	if err != nil {
		return internalIssuance{}, err
	}

	blockReward := uint256.NewInt(0)
	uncleReward := uint256.NewInt(0)
	for _, r := range rewards {
		if r.Kind == consensus.RewardAuthor {
			blockReward.Add(blockReward, &r.Amount)
		}
		if r.Kind == consensus.RewardUncle {
			uncleReward.Add(uncleReward, &r.Amount)
		}
	}

	var ret internalIssuance
	ret.BlockReward = hexutil2.EncodeBig(blockReward.ToBig())
	ret.UncleReward = hexutil2.EncodeBig(uncleReward.ToBig())

	blockReward.Add(blockReward, uncleReward)
	ret.Issuance = hexutil2.EncodeBig(blockReward.ToBig())
	return ret, nil
}

func delegateBlockFees(ctx context.Context, tx kv.Tx, block *types.Block, senders []common.Address, chainConfig *chain.Config, receipts types.Receipts) (*big.Int, uint64, error) {
	gasUsedDepositTx := uint64(0)
	fee := big.NewInt(0)
	gasUsed := big.NewInt(0)

	totalFees := big.NewInt(0)
	for _, receipt := range receipts {
		txn := block.Transactions()[receipt.TransactionIndex]
		effectiveGasPrice := uint64(0)
		if !chainConfig.IsLondon(block.NumberU64()) {
			effectiveGasPrice = txn.GetPrice().Uint64()
		} else {
			baseFee, _ := uint256.FromBig(block.BaseFee())
			if chainConfig.IsOptimism() && receipt.IsDepositTxReceipt() {
				// if depositTx, no fee consumption
				gasUsedDepositTx += receipt.GasUsed
				continue
			}
			gasPrice := new(big.Int).Add(block.BaseFee(), txn.GetEffectiveGasTip(baseFee).ToBig())
			effectiveGasPrice = gasPrice.Uint64()
		}

		fee.SetUint64(effectiveGasPrice)
		gasUsed.SetUint64(receipt.GasUsed)
		fee.Mul(fee, gasUsed)

		totalFees.Add(totalFees, fee)
	}

	return totalFees, gasUsedDepositTx, nil
}

func (api *OtterscanAPIImpl) getBlockWithSenders(ctx context.Context, number rpc.BlockNumber, tx kv.Tx) (*types.Block, []common.Address, error) {
	if number == rpc.PendingBlockNumber {
		return api.pendingBlock(), nil, nil
	}

	n, hash, _, err := rpchelper.GetBlockNumber(rpc.BlockNumberOrHashWithNumber(number), tx, api.filters)
	if err != nil {
		return nil, nil, err
	}

	block, err := api.blockWithSenders(ctx, tx, hash, n)
	if err != nil {
		return nil, nil, err
	}
	if block == nil {
		return nil, nil, nil
	}
	return block, block.Body().SendersFromTxs(), nil
}

func (api *OtterscanAPIImpl) GetBlockTransactions(ctx context.Context, number rpc.BlockNumber, pageNumber uint8, pageSize uint8) (map[string]interface{}, error) {
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	b, senders, err := api.getBlockWithSenders(ctx, number, tx)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}

	chainConfig, err := api.chainConfig(ctx, tx)
	if err != nil {
		return nil, err
	}

	getBlockRes, err := delegateGetBlockByNumber(tx, b, number, true)
	if err != nil {
		return nil, err
	}

	if len(senders) != b.Transactions().Len() {
		// fallback; set senders from inspecting tx
		senders = b.Body().SendersFromTxs()
	}
	// Receipts
	receipts, err := api.getReceipts(ctx, tx, b, senders)
	if err != nil {
		return nil, fmt.Errorf("getReceipts error: %v", err)
	}

	result := make([]map[string]interface{}, 0, len(receipts))
	for _, receipt := range receipts {
		txn := b.Transactions()[receipt.TransactionIndex]
		marshalledRcpt := ethutils.MarshalReceipt(receipt, txn, chainConfig, b.HeaderNoCopy(), txn.Hash(), true)
		marshalledRcpt["logs"] = nil
		marshalledRcpt["logsBloom"] = nil
		result = append(result, marshalledRcpt)
	}

	// Pruned block attrs
	prunedBlock := map[string]interface{}{}
	for _, k := range []string{"timestamp", "miner", "baseFeePerGas"} {
		prunedBlock[k] = getBlockRes[k]
	}

	// Crop txn input to 4bytes
	var txs = getBlockRes["transactions"].([]interface{})
	for _, rawTx := range txs {
		rpcTx := rawTx.(*ethapi.RPCTransaction)
		if len(rpcTx.Input) >= 4 {
			rpcTx.Input = rpcTx.Input[:4]
		}
	}

	// Crop page
	pageEnd := b.Transactions().Len() - int(pageNumber)*int(pageSize)
	pageStart := pageEnd - int(pageSize)
	if pageEnd < 0 {
		pageEnd = 0
	}
	if pageStart < 0 {
		pageStart = 0
	}

	response := map[string]interface{}{}
	getBlockRes["transactions"] = getBlockRes["transactions"].([]interface{})[pageStart:pageEnd]
	response["fullblock"] = getBlockRes
	response["receipts"] = result[pageStart:pageEnd]
	return response, nil
}
