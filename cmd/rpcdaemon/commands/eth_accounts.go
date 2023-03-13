package commands

import (
	"context"
	"fmt"
	"math/big"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutility"
	"github.com/ledgerwatch/erigon-lib/gointerfaces"
	"google.golang.org/grpc"

	"github.com/ledgerwatch/erigon/turbo/rpchelper"

	txpool_proto "github.com/ledgerwatch/erigon-lib/gointerfaces/txpool"

	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/common/hexutil"
	"github.com/ledgerwatch/erigon/rpc"
)

// GetBalance implements eth_getBalance. Returns the balance of an account for a given address.
func (api *APIImpl) GetBalance(ctx context.Context, address libcommon.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Big, error) {
	tx, err1 := api.db.BeginRo(ctx)
	if err1 != nil {
		return nil, fmt.Errorf("getBalance cannot open tx: %w", err1)
	}
	defer tx.Rollback()

	// Handle pre-bedrock blocks
	var blockNum uint64
	if number, ok := blockNrOrHash.Number(); ok {
		blockNum = uint64(number)
	} else if hash, ok := blockNrOrHash.Hash(); ok {
		block, err := api.blockByHashWithSenders(tx, hash)
		if block == nil {
			return nil, fmt.Errorf("header not found")
		}
		if err != nil {
			return nil, err
		}
		blockNum = block.NumberU64()
	} else {
		return nil, fmt.Errorf("invalid block number of hash")
	}

	chainConfig, err := api.chainConfig(tx)
	if err != nil {
		return nil, err
	}
	if chainConfig.IsOptimismPreBedrock(blockNum) {
		if api.historicalRPCService != nil {
			var res hexutil.Big
			err := api.historicalRPCService.CallContext(ctx, &res, "eth_getBalance", address, fmt.Sprintf("0x%x", blockNum))
			if err != nil {
				return nil, fmt.Errorf("historical backend error: %w", err)
			}
			return &res, nil
		} else {
			return nil, rpc.ErrNoHistoricalFallback
		}
	}

	reader, err := rpchelper.CreateStateReader(ctx, tx, blockNrOrHash, 0, api.filters, api.stateCache, api.historyV3(tx), "")
	if err != nil {
		return nil, err
	}

	acc, err := reader.ReadAccountData(address)
	if err != nil {
		return nil, fmt.Errorf("cant get a balance for account %x: %w", address.String(), err)
	}
	if acc == nil {
		// Special case - non-existent account is assumed to have zero balance
		return (*hexutil.Big)(big.NewInt(0)), nil
	}

	return (*hexutil.Big)(acc.Balance.ToBig()), nil
}

// GetTransactionCount implements eth_getTransactionCount. Returns the number of transactions sent from an address (the nonce).
func (api *APIImpl) GetTransactionCount(ctx context.Context, address libcommon.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Uint64, error) {
	if blockNrOrHash.BlockNumber != nil && *blockNrOrHash.BlockNumber == rpc.PendingBlockNumber {
		reply, err := api.txPool.Nonce(ctx, &txpool_proto.NonceRequest{
			Address: gointerfaces.ConvertAddressToH160(address),
		}, &grpc.EmptyCallOption{})
		if err != nil {
			return nil, err
		}
		if reply.Found {
			reply.Nonce++
			return (*hexutil.Uint64)(&reply.Nonce), nil
		}
	}
	tx, err1 := api.db.BeginRo(ctx)
	if err1 != nil {
		return nil, fmt.Errorf("getTransactionCount cannot open tx: %w", err1)
	}
	defer tx.Rollback()

	// Handle pre-bedrock blocks
	var blockNum uint64
	if number, ok := blockNrOrHash.Number(); ok {
		blockNum = uint64(number)
	} else if hash, ok := blockNrOrHash.Hash(); ok {
		block, err := api.blockByHashWithSenders(tx, hash)
		if block == nil {
			return nil, fmt.Errorf("header not found")
		}
		if err != nil {
			return nil, err
		}
		blockNum = block.NumberU64()
	} else {
		return nil, fmt.Errorf("invalid block number of hash")
	}

	chainConfig, err := api.chainConfig(tx)
	if err != nil {
		return nil, err
	}
	if chainConfig.IsOptimismPreBedrock(blockNum) {
		if api.historicalRPCService != nil {
			var res hexutil.Uint64
			err := api.historicalRPCService.CallContext(ctx, &res, "eth_getTransactionCount", address, fmt.Sprintf("0x%x", blockNum))
			if err != nil {
				return nil, fmt.Errorf("historical backend error: %w", err)
			}
			return &res, nil
		} else {
			return nil, rpc.ErrNoHistoricalFallback
		}
	}

	reader, err := rpchelper.CreateStateReader(ctx, tx, blockNrOrHash, 0, api.filters, api.stateCache, api.historyV3(tx), "")
	if err != nil {
		return nil, err
	}
	nonce := hexutil.Uint64(0)
	acc, err := reader.ReadAccountData(address)
	if acc == nil || err != nil {
		return &nonce, err
	}
	return (*hexutil.Uint64)(&acc.Nonce), err
}

// GetCode implements eth_getCode. Returns the byte code at a given address (if it's a smart contract).
func (api *APIImpl) GetCode(ctx context.Context, address libcommon.Address, blockNrOrHash rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	tx, err1 := api.db.BeginRo(ctx)
	if err1 != nil {
		return nil, fmt.Errorf("getCode cannot open tx: %w", err1)
	}

	// Handle pre-bedrock blocks
	var blockNum uint64
	if number, ok := blockNrOrHash.Number(); ok {
		blockNum = uint64(number)
	} else if hash, ok := blockNrOrHash.Hash(); ok {
		block, err := api.blockByHashWithSenders(tx, hash)
		if block == nil {
			return nil, fmt.Errorf("header not found")
		}
		if err != nil {
			return nil, err
		}
		blockNum = block.NumberU64()
	} else {
		return nil, fmt.Errorf("invalid block number of hash")
	}

	chainConfig, err := api.chainConfig(tx)
	if err != nil {
		return nil, err
	}
	if chainConfig.IsOptimismPreBedrock(blockNum) {
		if api.historicalRPCService != nil {
			var res hexutil.Bytes
			err := api.historicalRPCService.CallContext(ctx, &res, "eth_getCode", address, fmt.Sprintf("0x%x", blockNum))
			if err != nil {
				return nil, fmt.Errorf("historical backend error: %w", err)
			}
			return res, nil
		} else {
			return nil, rpc.ErrNoHistoricalFallback
		}
	}

	defer tx.Rollback()
	reader, err := rpchelper.CreateStateReader(ctx, tx, blockNrOrHash, 0, api.filters, api.stateCache, api.historyV3(tx), chainConfig.ChainName)
	if err != nil {
		return nil, err
	}

	acc, err := reader.ReadAccountData(address)
	if acc == nil || err != nil {
		return hexutil.Bytes(""), nil
	}
	res, _ := reader.ReadAccountCode(address, acc.Incarnation, acc.CodeHash)
	if res == nil {
		return hexutil.Bytes(""), nil
	}
	return res, nil
}

// GetStorageAt implements eth_getStorageAt. Returns the value from a storage position at a given address.
func (api *APIImpl) GetStorageAt(ctx context.Context, address libcommon.Address, index string, blockNrOrHash rpc.BlockNumberOrHash) (string, error) {
	var empty []byte

	tx, err1 := api.db.BeginRo(ctx)
	if err1 != nil {
		return hexutility.Encode(common.LeftPadBytes(empty, 32)), err1
	}
	defer tx.Rollback()

	// Handle pre-bedrock blocks
	var blockNum uint64
	if number, ok := blockNrOrHash.Number(); ok {
		blockNum = uint64(number)
	} else if hash, ok := blockNrOrHash.Hash(); ok {
		block, err := api.blockByHashWithSenders(tx, hash)
		if block == nil {
			return hexutility.Encode(common.LeftPadBytes(empty, 32)), fmt.Errorf("block %x not found", hash)
		}
		if err != nil {
			return hexutility.Encode(common.LeftPadBytes(empty, 32)), err
		}
		blockNum = block.NumberU64()
	} else {
		return hexutility.Encode(common.LeftPadBytes(empty, 32)), fmt.Errorf("invalid block number of hash")
	}

	chainConfig, err := api.chainConfig(tx)
	if err != nil {
		return hexutility.Encode(common.LeftPadBytes(empty, 32)), err
	}
	if chainConfig.IsOptimismPreBedrock(blockNum) {
		if api.historicalRPCService != nil {
			var res hexutil.Bytes
			err := api.historicalRPCService.CallContext(ctx, &res, "eth_getStorageAt", address, fmt.Sprintf("0x%x", blockNum))
			if err != nil {
				return hexutility.Encode(common.LeftPadBytes(empty, 32)), fmt.Errorf("historical backend error: %w", err)
			}
			return hexutility.Encode(common.LeftPadBytes(res, 32)), nil
		} else {
			return hexutility.Encode(common.LeftPadBytes(empty, 32)), rpc.ErrNoHistoricalFallback
		}
	}

	reader, err := rpchelper.CreateStateReader(ctx, tx, blockNrOrHash, 0, api.filters, api.stateCache, api.historyV3(tx), "")
	if err != nil {
		return hexutility.Encode(common.LeftPadBytes(empty, 32)), err
	}
	acc, err := reader.ReadAccountData(address)
	if acc == nil || err != nil {
		return hexutility.Encode(common.LeftPadBytes(empty, 32)), err
	}

	location := libcommon.HexToHash(index)
	res, err := reader.ReadAccountStorage(address, acc.Incarnation, &location)
	if err != nil {
		res = empty
	}
	return hexutility.Encode(common.LeftPadBytes(res, 32)), err
}

// Exist returns whether an account for a given address exists in the database.
func (api *APIImpl) Exist(ctx context.Context, address libcommon.Address, blockNrOrHash rpc.BlockNumberOrHash) (bool, error) {
	tx, err1 := api.db.BeginRo(ctx)
	if err1 != nil {
		return false, err1
	}
	defer tx.Rollback()

	reader, err := rpchelper.CreateStateReader(ctx, tx, blockNrOrHash, 0, api.filters, api.stateCache, api.historyV3(tx), "")
	if err != nil {
		return false, err
	}
	acc, err := reader.ReadAccountData(address)
	if err != nil {
		return false, err
	}
	if acc == nil {
		return false, nil
	}

	return true, nil
}
