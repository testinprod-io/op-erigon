package chainspec

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/erigontech/erigon-lib/chain/superchain"
	"github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon/execution/chain"
	"github.com/erigontech/erigon/execution/types"
)

// ChainConfigByOpStackChainName loads chain config corresponding to the chain name from superchain registry, and builds erigon chain config.
func ChainConfigByOpStackChainName(name string) *chain.Config {
	opStackChainCfg := superchain.OPStackChainConfigByName(name)
	if opStackChainCfg == nil {
		return nil
	}
	return LoadSuperChainConfig(opStackChainCfg)
}

// ChainConfigByOpStackGenesisHash loads chain config corresponding to the genesis hash from superchain registry, and builds erigon chain config.
func ChainConfigByOpStackGenesisHash(genesisHash common.Hash) *chain.Config {
	opStackChainCfg := superchain.OPStackChainConfigByGenesisHash(genesisHash)
	if opStackChainCfg == nil {
		return nil
	}
	return LoadSuperChainConfig(opStackChainCfg)
}

// LoadOPStackChainConfig loads superchain config from superchain registry for given chain, and builds erigon chain config.
// This implementation is based on op-geth(https://github.com/ethereum-optimism/op-geth/blob/c7871bc4454ffc924eb128fa492975b30c9c46ad/params/superchain.go#L39)
func LoadSuperChainConfig(chConfig *superchain.ChainConfig) *chain.Config {
	hardforks := chConfig.Hardforks
	out := &chain.Config{
		ChainName:                     chConfig.Name,
		ChainID:                       new(big.Int).SetUint64(chConfig.ChainID),
		HomesteadBlock:                common.Big0,
		DAOForkBlock:                  nil,
		TangerineWhistleBlock:         common.Big0,
		SpuriousDragonBlock:           common.Big0,
		ByzantiumBlock:                common.Big0,
		ConstantinopleBlock:           common.Big0,
		PetersburgBlock:               common.Big0,
		IstanbulBlock:                 common.Big0,
		MuirGlacierBlock:              common.Big0,
		BerlinBlock:                   common.Big0,
		LondonBlock:                   common.Big0,
		ArrowGlacierBlock:             common.Big0,
		GrayGlacierBlock:              common.Big0,
		MergeNetsplitBlock:            common.Big0,
		ShanghaiTime:                  nil,
		CancunTime:                    nil,
		PragueTime:                    nil,
		BedrockBlock:                  common.Big0,
		RegolithTime:                  big.NewInt(0),
		CanyonTime:                    nil,
		EcotoneTime:                   nil,
		FjordTime:                     nil,
		GraniteTime:                   nil,
		HoloceneTime:                  nil,
		TerminalTotalDifficulty:       common.Big0,
		TerminalTotalDifficultyPassed: true,
		Ethash:                        nil,
		Clique:                        nil,
		Optimism:                      nil,
	}

	if hardforks.CanyonTime != nil {
		out.ShanghaiTime = new(big.Int).SetUint64(*hardforks.CanyonTime) // Shanghai activates with Canyon
		out.CanyonTime = new(big.Int).SetUint64(*hardforks.CanyonTime)
	}
	if hardforks.EcotoneTime != nil {
		out.CancunTime = new(big.Int).SetUint64(*hardforks.EcotoneTime) // CancunTime activates with Ecotone
		out.EcotoneTime = new(big.Int).SetUint64(*hardforks.EcotoneTime)
	}
	if hardforks.FjordTime != nil {
		out.FjordTime = new(big.Int).SetUint64(*hardforks.FjordTime)
	}
	if hardforks.GraniteTime != nil {
		out.GraniteTime = new(big.Int).SetUint64(*hardforks.GraniteTime)
	}
	if hardforks.HoloceneTime != nil {
		out.HoloceneTime = new(big.Int).SetUint64(*hardforks.HoloceneTime)
	}
	if hardforks.IsthmusTime != nil {
		out.PragueTime = new(big.Int).SetUint64(*hardforks.IsthmusTime) // Prague activates with Isthmus
		out.IsthmusTime = new(big.Int).SetUint64(*hardforks.IsthmusTime)
	}

	if chConfig.Optimism != nil {
		out.Optimism = &chain.OptimismConfig{
			EIP1559Elasticity:  chConfig.Optimism.EIP1559Elasticity,
			EIP1559Denominator: chConfig.Optimism.EIP1559Denominator,
		}
		if chConfig.Optimism.EIP1559DenominatorCanyon != nil {
			out.Optimism.EIP1559DenominatorCanyon = *chConfig.Optimism.EIP1559DenominatorCanyon
		}
	}

	// special overrides for OP-Stack chains with pre-Regolith upgrade history
	switch chConfig.ChainID {
	case superchain.OPMainnetChainID:
		out.BerlinBlock = big.NewInt(3950000)
		out.LondonBlock = big.NewInt(105235063)
		out.ArrowGlacierBlock = big.NewInt(105235063)
		out.GrayGlacierBlock = big.NewInt(105235063)
		out.MergeNetsplitBlock = big.NewInt(105235063)
		out.BedrockBlock = big.NewInt(105235063)
	}

	return out
}

func LoadOPStackGenesisByChainName(name string) (*types.Genesis, error) {
	opStackChainCfg := superchain.OPStackChainConfigByName(name)
	if opStackChainCfg == nil {
		return nil, nil
	}

	return LoadOPStackGenesis(opStackChainCfg.ChainID)
}

// loadOPStackGenesisByChainName loads genesis block corresponding to the chain name from superchain regsitry.
// This implementation is based on op-geth(https://github.com/ethereum-optimism/op-geth/blob/acea1259d8ea2e74cf102463e4f5a7738bd5e102/core/superchain.go#L14)
func LoadOPStackGenesis(chainID uint64) (*types.Genesis, error) {
	chain, err := superchain.GetChain(chainID)
	if err != nil {
		return nil, fmt.Errorf("error getting superchain: %w", err)
	}

	chConfig, err := chain.Config()
	if err != nil {
		return nil, fmt.Errorf("error getting chain config from superchain: %w", err)
	}

	cfg := LoadSuperChainConfig(chConfig)
	gen, err := readOPStackGenesis(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to load genesis definition for chain %d: %w", chainID, err)
	}

	genesis := &types.Genesis{
		Config:        cfg,
		Nonce:         gen.Nonce,
		Timestamp:     gen.Timestamp,
		ExtraData:     gen.ExtraData,
		GasLimit:      gen.GasLimit,
		Difficulty:    gen.Difficulty,
		Mixhash:       gen.Mixhash,
		Coinbase:      gen.Coinbase,
		Alloc:         gen.Alloc,
		Number:        gen.Number,
		GasUsed:       gen.GasUsed,
		ParentHash:    gen.ParentHash,
		BaseFee:       gen.BaseFee,
		ExcessBlobGas: gen.ExcessBlobGas,
		BlobGasUsed:   gen.BlobGasUsed,
	}

	if gen.StateHash != nil {
		if len(gen.Alloc) > 0 {
			return nil, fmt.Errorf("chain definition unexpectedly contains both allocation (%d) and state-hash %s", len(gen.Alloc), *gen.StateHash)
		}
		genesis.StateHash = gen.StateHash
		genesis.Alloc = nil
	}

	if chainID == superchain.OPMainnetChainID {
		opmStateHash := common.HexToHash("0xeddb4c1786789419153a27c4c80ff44a2226b6eda04f7e22ce5bae892ea568eb")
		genesis.StateHash = &opmStateHash
	}

	return genesis, nil
}

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
