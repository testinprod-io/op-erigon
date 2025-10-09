package core

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/erigontech/erigon-lib/chain/superchain"
	"github.com/erigontech/erigon/execution/types"
)

// loadOPStackGenesisByChainName loads genesis block corresponding to the chain name from superchain regsitry.
// This implementation is based on op-geth(https://github.com/ethereum-optimism/op-geth/blob/c7871bc4454ffc924eb128fa492975b30c9c46ad/core/superchain.go#L13)
//func LoadOPStackGenesisByChainName(name string) (*types.Genesis, error) {
//	opStackChainCfg := superchain.OPStackChainConfigByName(name)
//	if opStackChainCfg == nil {
//		return nil, nil
//	}
//
//	return LoadOPStackGenesis(opStackChainCfg.ChainID)
//}

// loadOPStackGenesisByChainName loads genesis block corresponding to the chain name from superchain regsitry.
// This implementation is based on op-geth(https://github.com/ethereum-optimism/op-geth/blob/acea1259d8ea2e74cf102463e4f5a7738bd5e102/core/superchain.go#L14)
//func LoadOPStackGenesis(chainID uint64) (*types.Genesis, error) {
//	chain, err := superchain.GetChain(chainID)
//	if err != nil {
//		return nil, fmt.Errorf("error getting superchain: %w", err)
//	}
//
//	chConfig, err := chain.Config()
//	if err != nil {
//		return nil, fmt.Errorf("error getting chain config from superchain: %w", err)
//	}
//
//	cfg := superchain.LoadSuperChainConfig(chConfig)
//	gen, err := readOPStackGenesis(chain)
//	if err != nil {
//		return nil, fmt.Errorf("failed to load genesis definition for chain %d: %w", chainID, err)
//	}
//
//	genesis := &types.Genesis{
//		Config:        cfg,
//		Nonce:         gen.Nonce,
//		Timestamp:     gen.Timestamp,
//		ExtraData:     gen.ExtraData,
//		GasLimit:      gen.GasLimit,
//		Difficulty:    gen.Difficulty,
//		Mixhash:       gen.Mixhash,
//		Coinbase:      gen.Coinbase,
//		Alloc:         gen.Alloc,
//		Number:        gen.Number,
//		GasUsed:       gen.GasUsed,
//		ParentHash:    gen.ParentHash,
//		BaseFee:       gen.BaseFee,
//		ExcessBlobGas: gen.ExcessBlobGas,
//		BlobGasUsed:   gen.BlobGasUsed,
//	}
//
//	if gen.StateHash != nil {
//		if len(gen.Alloc) > 0 {
//			return nil, fmt.Errorf("chain definition unexpectedly contains both allocation (%d) and state-hash %s", len(gen.Alloc), *gen.StateHash)
//		}
//		genesis.StateHash = gen.StateHash
//		genesis.Alloc = nil
//	}
//
//	if chainID == superchain.OPMainnetChainID {
//		opmStateHash := common.HexToHash("0xeddb4c1786789419153a27c4c80ff44a2226b6eda04f7e22ce5bae892ea568eb")
//		genesis.StateHash = &opmStateHash
//	}
//
//	tmpdir := os.TempDir()
//	dirs := datadir.New(tmpdir)
//	genesisBlock, _, err := genesiswrite.GenesisToBlock(genesis, dirs, log.Root())
//	if err != nil {
//		return nil, fmt.Errorf("failed to build genesis block: %w", err)
//	}
//	genesisBlockHash := genesisBlock.Hash()
//	expectedHash := chConfig.Genesis.L2.Hash
//
//	// Verify we correctly produced the genesis config by recomputing the genesis-block-hash,
//	// and check the genesis matches the chain genesis definition.
//	if chConfig.Genesis.L2.Number != genesisBlock.NumberU64() {
//		switch chainID {
//		case superchain.OPMainnetChainID:
//			expectedHash = common.HexToHash("0x7ca38a1916c42007829c55e69d3e9a73265554b586a499015373241b8a3fa48b")
//		default:
//			return nil, fmt.Errorf("unknown stateless genesis definition for chain %d", chainID)
//		}
//	}
//	if expectedHash != genesisBlockHash {
//		return nil, fmt.Errorf("chainID=%d: produced genesis with hash %s but expected %s", chainID, genesisBlockHash, expectedHash)
//	}
//	return genesis, nil
//}

func readOPStackGenesis(chain *superchain.Chain) (*types.Genesis, error) {
	genData, err := chain.GenesisData()
	if err != nil {
		return nil, fmt.Errorf("error getting genesis data from superchain: %w", err)
	}
	gen := new(types.Genesis)
	if err := json.Unmarshal(genData, gen); err != nil {
		return nil, fmt.Errorf("failed to unmarshal genesis data: %w", err)
	}
	return gen, nil
}
