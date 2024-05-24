package misc

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ledgerwatch/erigon-lib/chain"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDeployerCodeHash(t *testing.T) {
	// superchain-registry verified contract bytecode is correct.
	// Before integraing superchain-registry, we manually verify here.
	codeHash := crypto.Keccak256Hash(create2DeployerCode)
	require.Equal(t, create2DeployerCodeHash, codeHash)
}

func TestEnsureCreate2Deployer(t *testing.T) {
	canyonTime := uint64(1000)
	var tests = []struct {
		name      string
		override  func(cfg *chain.Config)
		timestamp uint64
		applied   bool
	}{
		{
			name:      "at hardfork",
			timestamp: canyonTime,
			applied:   true,
		},
		{
			name: "another chain ID",
			override: func(cfg *chain.Config) {
				cfg.ChainID = big.NewInt(params.OPMainnetChainID)
			},
			timestamp: canyonTime,
			applied:   true,
		},
		{
			name:      "pre canyon",
			timestamp: canyonTime - 1,
			applied:   false,
		},
		{
			name:      "post hardfork",
			timestamp: canyonTime + 1,
			applied:   false,
		},
		{
			name: "canyon not configured",
			override: func(cfg *chain.Config) {
				cfg.CanyonTime = nil
			},
			timestamp: canyonTime,
			applied:   false,
		},
		{
			name: "not optimism",
			override: func(cfg *chain.Config) {
				cfg.Optimism = nil
			},
			timestamp: canyonTime,
			applied:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := chain.Config{
				ChainID:    big.NewInt(params.OPMainnetChainID),
				Optimism:   &chain.OptimismConfig{},
				CanyonTime: big.NewInt(int64(canyonTime)),
			}
			if tt.override != nil {
				tt.override(&cfg)
			}

			_, tx := memdb.NewTestTx(t)
			state := state.New(state.NewPlainStateReader(tx))
			// make sure state is empty
			assert.NotEqual(t, state.GetCode(create2DeployerAddress), create2DeployerCode)

			EnsureCreate2Deployer(&cfg, tt.timestamp, state)

			applied := bytes.Equal(state.GetCode(create2DeployerAddress), create2DeployerCode)
			assert.Equal(t, tt.applied, applied)
		})
	}
}
