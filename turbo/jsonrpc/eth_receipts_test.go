package jsonrpc

import (
	"context"
	txpool "github.com/erigontech/erigon-lib/gointerfaces/txpoolproto"
	"math/big"
	"testing"

	"github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/kv/kvcache"
	"github.com/erigontech/erigon-lib/log/v3"
	"github.com/erigontech/erigon-lib/opstack"
	"github.com/erigontech/erigon/cmd/rpcdaemon/rpcdaemontest"
	"github.com/erigontech/erigon/common/u256"
	"github.com/erigontech/erigon/core/rawdb"
	"github.com/erigontech/erigon/core/types"
	"github.com/erigontech/erigon/rpc/rpccfg"
	"github.com/erigontech/erigon/turbo/rpchelper"
	"github.com/erigontech/erigon/turbo/stages/mock"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestGetReceipts(t *testing.T) {
	m, _, _ := rpcdaemontest.CreateOptimismTestSentry(t)
	stateCache := kvcache.New(kvcache.DefaultCoherentConfig)
	ctx, conn := rpcdaemontest.CreateTestGrpcConn(t, mock.Mock(t))
	mining := txpool.NewMiningClient(conn)
	ff := rpchelper.New(ctx, rpchelper.FiltersConfig{}, nil, nil, mining, func() {}, m.Log)
	api := NewEthAPI(NewBaseApi(ff, stateCache, m.BlockReader, false, rpccfg.DefaultEvmCallTimeout, m.Engine, m.Dirs, nil, nil), m.DB, nil, nil, nil, 5000000, 1e18, 100_000, false, 100_000, 128, log.New())

	db := m.DB
	defer db.Close()

	tx, err := db.BeginRw(context.Background())
	require.NoError(t, err)
	defer tx.Rollback()

	header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(100)}
	block := types.NewBlockWithHeader(header)

	require.NoError(t, rawdb.WriteBlock(tx, block))
	require.NoError(t, rawdb.WriteReceipts(tx, block.NumberU64(), nil))
	tx.Commit()

	rTx, err := db.BeginRo(context.Background())
	require.NoError(t, err)
	defer rTx.Rollback()

	receipt, err := api.getReceipts(m.Ctx, rTx, block, []common.Address{})
	require.NoError(t, err)
	require.Equal(t, 0, len(receipt))

	tx, err = db.BeginRw(context.Background())
	require.NoError(t, err)
	defer tx.Rollback()

	var (
		bedrockBlock = common.Big0.Add(m.ChainConfig.BedrockBlock, big.NewInt(1))

		l1BaseFee = uint256.NewInt(1000).Bytes32()
		overhead  = uint256.NewInt(100).Bytes32()
		scalar    = uint256.NewInt(100).Bytes32()
		fscalar   = new(big.Float).SetInt(new(uint256.Int).SetBytes(scalar[:]).ToBig())
		fdivisor  = new(big.Float).SetUint64(1_000_000)
		feeScalar = new(big.Float).Quo(fscalar, fdivisor)
	)

	systemTx := buildSystemTx(l1BaseFee, overhead, scalar)

	tx1 := types.NewTransaction(1, common.HexToAddress("0x1"), u256.Num1, 1, u256.Num1, systemTx)
	tx2 := types.NewTransaction(2, common.HexToAddress("0x2"), u256.Num2, 2, u256.Num2, nil)

	header = &types.Header{Number: bedrockBlock, Difficulty: big.NewInt(100)}
	body := &types.Body{Transactions: types.Transactions{tx1, tx2}}

	receipt1 := &types.Receipt{
		Status:            types.ReceiptStatusFailed,
		CumulativeGasUsed: 1,
		Logs: []*types.Log{
			{Address: common.BytesToAddress([]byte{0x11})},
			{Address: common.BytesToAddress([]byte{0x01, 0x11})},
		},
		TxHash:          tx1.Hash(),
		ContractAddress: common.BytesToAddress([]byte{0x01, 0x11, 0x11}),
		GasUsed:         111111,
		L1Fee:           big.NewInt(7),
	}
	receipt2 := &types.Receipt{
		PostState:         common.Hash{2}.Bytes(),
		CumulativeGasUsed: 2,
		Logs: []*types.Log{
			{Address: common.BytesToAddress([]byte{0x22})},
			{Address: common.BytesToAddress([]byte{0x02, 0x22})},
		},
		TxHash:          tx2.Hash(),
		ContractAddress: common.BytesToAddress([]byte{0x02, 0x22, 0x22}),
		GasUsed:         222222,
		L1Fee:           big.NewInt(1),
	}
	receipts := []*types.Receipt{receipt1, receipt2}

	rawdb.WriteCanonicalHash(tx, header.Hash(), header.Number.Uint64())
	rawdb.WriteHeader(tx, header)
	require.NoError(t, rawdb.WriteBody(tx, header.Hash(), header.Number.Uint64(), body))
	require.NoError(t, rawdb.WriteSenders(tx, header.Hash(), header.Number.Uint64(), body.SendersFromTxs()))

	br := m.BlockReader
	b, senders, err := br.BlockWithSenders(ctx, tx, header.Hash(), header.Number.Uint64())
	require.NoError(t, err)

	require.NoError(t, rawdb.WriteBlock(tx, b))
	require.NoError(t, rawdb.WriteReceipts(tx, b.NumberU64(), receipts))

	tx.Commit()

	rTx, err = db.BeginRo(context.Background())
	require.NoError(t, err)
	defer rTx.Rollback()

	receipts, err = api.getReceipts(m.Ctx, rTx, b, senders)
	require.NoError(t, err)
	require.Equal(t, 2, len(receipts))

	require.Equal(t, new(uint256.Int).SetBytes(l1BaseFee[:]).ToBig(), receipts[0].L1GasPrice)
	rollupDataGas1 := uint64(2492)
	require.Equal(t, new(big.Int).Add(new(big.Int).SetUint64(rollupDataGas1), new(uint256.Int).SetBytes(overhead[:]).ToBig()), receipts[0].L1GasUsed)
	require.Equal(t, opstack.L1CostPreEcotone(rollupDataGas1, new(uint256.Int).SetBytes(l1BaseFee[:]), new(uint256.Int).SetBytes(overhead[:]), new(uint256.Int).SetBytes(scalar[:])).ToBig(), receipts[0].L1Fee)
	require.Equal(t, feeScalar, receipts[0].FeeScalar)

	require.Equal(t, new(uint256.Int).SetBytes(l1BaseFee[:]).ToBig(), receipts[1].L1GasPrice)
	rollupDataGas2 := uint64(1340)
	require.Equal(t, new(big.Int).Add(new(big.Int).SetUint64(rollupDataGas2), new(uint256.Int).SetBytes(overhead[:]).ToBig()), receipts[1].L1GasUsed)
	require.Equal(t, opstack.L1CostPreEcotone(rollupDataGas2, new(uint256.Int).SetBytes(l1BaseFee[:]), new(uint256.Int).SetBytes(overhead[:]), new(uint256.Int).SetBytes(scalar[:])).ToBig(), receipts[1].L1Fee)
	require.Equal(t, feeScalar, receipts[1].FeeScalar)
}

func buildSystemTx(l1BaseFee, overhead, scalar [32]byte) []byte {
	systemInfo := []byte{0, 0, 0, 0}
	zeroBytes := uint256.NewInt(0).Bytes32()
	systemInfo = append(systemInfo, zeroBytes[:]...) // 4 - 4 + 32
	systemInfo = append(systemInfo, zeroBytes[:]...) // 4 + 1 * 32 - 4 + 2 * 32
	systemInfo = append(systemInfo, l1BaseFee[:]...) // 4 + 2 * 32 - 4 + 3 * 32 - l1Basefee
	systemInfo = append(systemInfo, zeroBytes[:]...) // 4 + 3 * 32 - 4 + 4 * 32
	systemInfo = append(systemInfo, zeroBytes[:]...) // 4 + 4 * 32 - 4 + 5 * 32
	systemInfo = append(systemInfo, zeroBytes[:]...) // 4 + 5 * 32 - 4 + 6 * 32
	systemInfo = append(systemInfo, overhead[:]...)  // 4 + 6 * 32 - 4 + 7 * 32 - overhead
	systemInfo = append(systemInfo, scalar[:]...)    // 4 + 7 * 32 - 4 + 8 * 32 - scalar
	return systemInfo
}
