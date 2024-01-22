package params

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum-optimism/superchain-registry/superchain"
	"github.com/ledgerwatch/erigon-lib/chain"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/params/networkname"
)

const (
	OPMainnetChainID        = 10
	OPGoerliChainID         = 420
	BaseMainnetChainID      = 8453
	BaseGoerliChainID       = 84531
	baseSepoliaChainID      = 84532
	baseGoerliDevnetChainID = 11763071
	pgnSepoliaChainID       = 58008
	devnetChainID           = 997
	chaosnetChainID         = 888
)

// OP Stack chain config
var (
	// March 17, 2023 @ 7:00:00 pm UTC
	OptimismGoerliRegolithTime = big.NewInt(1679079600)
	// May 4, 2023 @ 5:00:00 pm UTC
	BaseGoerliRegolithTime = big.NewInt(1683219600)
	// Apr 21, 2023 @ 6:30:00 pm UTC
	baseGoerliDevnetRegolithTime = big.NewInt(1682101800)
	// March 5, 2023 @ 2:48:00 am UTC
	devnetRegolithTime = big.NewInt(1677984480)
	// August 16, 2023 @ 3:34:22 am UTC
	chaosnetRegolithTime = big.NewInt(1692156862)
)

// OPStackChainConfigByName loads chain config corresponding to the chain name from superchain registry.
// This implementation is based on optimism monorepo(https://github.com/ethereum-optimism/optimism/blob/op-node/v1.4.1/op-node/chaincfg/chains.go#L59)
func OPStackChainConfigByName(name string) *superchain.ChainConfig {
	// Handle legacy name aliases
	name = networkname.HandleLegacyName(name)
	for _, chainCfg := range superchain.OPChains {
		if strings.EqualFold(chainCfg.Chain+"-"+chainCfg.Superchain, name) {
			return chainCfg
		}
	}
	return nil
}

// OPStackChainConfigByGenesisHash loads chain config corresponding to the genesis hash from superchain registry.
func OPStackChainConfigByGenesisHash(genesisHash common.Hash) *superchain.ChainConfig {
	if bytes.Equal(genesisHash.Bytes(), OPMainnetGenesisHash.Bytes()) {
		return superchain.OPChains[OPMainnetChainID]
	} else if bytes.Equal(genesisHash.Bytes(), OPGoerliGenesisHash.Bytes()) {
		return superchain.OPChains[OPGoerliChainID]
	}
	for _, chainCfg := range superchain.OPChains {
		if bytes.Equal(chainCfg.Genesis.L2.Hash[:], genesisHash.Bytes()) {
			return chainCfg
		}
	}
	return nil
}

// ChainConfigByOpStackChainName loads chain config corresponding to the chain name from superchain registry, and builds erigon chain config.
func ChainConfigByOpStackChainName(name string) *chain.Config {
	opStackChainCfg := OPStackChainConfigByName(name)
	if opStackChainCfg == nil {
		return nil
	}
	return LoadSuperChainConfig(opStackChainCfg)
}

// ChainConfigByOpStackGenesisHash loads chain config corresponding to the genesis hash from superchain registry, and builds erigon chain config.
func ChainConfigByOpStackGenesisHash(genesisHash common.Hash) *chain.Config {
	opStackChainCfg := OPStackChainConfigByGenesisHash(genesisHash)
	if opStackChainCfg == nil {
		return nil
	}
	return LoadSuperChainConfig(opStackChainCfg)
}

// LoadSuperChainConfig loads superchain config from superchain registry for given chain, and builds erigon chain config.
// This implementation is based on op-geth(https://github.com/ethereum-optimism/op-geth/blob/c7871bc4454ffc924eb128fa492975b30c9c46ad/params/superchain.go#L39)
func LoadSuperChainConfig(opStackChainCfg *superchain.ChainConfig) *chain.Config {
	superchainConfig, ok := superchain.Superchains[opStackChainCfg.Superchain]
	if !ok {
		panic("unknown superchain: " + opStackChainCfg.Superchain)
	}
	out := &chain.Config{
		ChainName:                     opStackChainCfg.Name,
		ChainID:                       new(big.Int).SetUint64(opStackChainCfg.ChainID),
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
		CanyonTime:                    new(big.Int).SetUint64(*superchainConfig.Config.CanyonTime),
		TerminalTotalDifficulty:       common.Big0,
		TerminalTotalDifficultyPassed: true,
		Ethash:                        nil,
		Clique:                        nil,
		Optimism: &chain.OptimismConfig{
			EIP1559Elasticity:        6,
			EIP1559Denominator:       50,
			EIP1559DenominatorCanyon: 250,
		},
	}

	if superchainConfig.Config.CanyonTime != nil {
		out.ShanghaiTime = new(big.Int).SetUint64(*superchainConfig.Config.CanyonTime) // Shanghai activates with Canyon
	}
	if superchainConfig.Config.EcotoneTime != nil {
		out.CancunTime = new(big.Int).SetUint64(*superchainConfig.Config.EcotoneTime) // CancunTime activates with Ecotone
	}

	// note: no actual parameters are being loaded, yet.
	// Future superchain upgrades are loaded from the superchain chConfig and applied to the geth ChainConfig here.
	_ = superchainConfig.Config

	// special overrides for OP-Stack chains with pre-Regolith upgrade history
	switch opStackChainCfg.ChainID {
	case OPGoerliChainID:
		out.LondonBlock = big.NewInt(4061224)
		out.ArrowGlacierBlock = big.NewInt(4061224)
		out.GrayGlacierBlock = big.NewInt(4061224)
		out.MergeNetsplitBlock = big.NewInt(4061224)
		out.BedrockBlock = big.NewInt(4061224)
		out.RegolithTime = OptimismGoerliRegolithTime
		out.Optimism.EIP1559Elasticity = 10
	case OPMainnetChainID:
		out.BerlinBlock = big.NewInt(3950000)
		out.LondonBlock = big.NewInt(105235063)
		out.ArrowGlacierBlock = big.NewInt(105235063)
		out.GrayGlacierBlock = big.NewInt(105235063)
		out.MergeNetsplitBlock = big.NewInt(105235063)
		out.BedrockBlock = big.NewInt(105235063)
	case BaseGoerliChainID:
		out.RegolithTime = BaseGoerliRegolithTime
		out.Optimism.EIP1559Elasticity = 10
	case baseSepoliaChainID:
		out.Optimism.EIP1559Elasticity = 10
	case baseGoerliDevnetChainID:
		out.RegolithTime = baseGoerliDevnetRegolithTime
	case pgnSepoliaChainID:
		out.Optimism.EIP1559Elasticity = 2
		out.Optimism.EIP1559Denominator = 8
	case devnetChainID:
		out.RegolithTime = devnetRegolithTime
		out.Optimism.EIP1559Elasticity = 10
	case chaosnetChainID:
		out.RegolithTime = chaosnetRegolithTime
		out.Optimism.EIP1559Elasticity = 10
	}

	return out
}
