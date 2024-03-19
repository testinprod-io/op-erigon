package gaspricecfg

import (
	"math/big"

	"github.com/ledgerwatch/erigon/params"
)

var DefaultIgnorePrice = big.NewInt(2 * params.Wei)

var (
	DefaultMaxPrice = big.NewInt(500 * params.GWei)

	DefaultMinSuggestedPriorityFee = big.NewInt(1e6 * params.Wei) // 0.001 gwei, for Optimism fee suggestion
)

type Config struct {
	Blocks           int
	Percentile       int
	MaxHeaderHistory int
	MaxBlockHistory  int
	Default          *big.Int `toml:",omitempty"`
	MaxPrice         *big.Int `toml:",omitempty"`
	IgnorePrice      *big.Int `toml:",omitempty"`

	MinSuggestedPriorityFee *big.Int `toml:",omitempty"` // for Optimism fee suggestion
}
