package misc

import (
	"math/big"
	"testing"

	"github.com/ledgerwatch/erigon-lib/chain"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
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
		name       string
		override   func(cfg *chain.Config)
		timestamp  uint64
		applied    bool
		codeExists bool
	}{
		{
			name: "another chain ID",
			override: func(cfg *chain.Config) {
				cfg.ChainID = params.OptimismMainnetChainConfig.ChainID
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
			name:      "at hardfork exactly",
			timestamp: canyonTime,
			applied:   true,
		},
		{
			name:      "post hardfork",
			timestamp: canyonTime + 1,
			applied:   true,
		},
		{
			name:       "post hardfork but already deployed",
			timestamp:  canyonTime,
			applied:    false,
			codeExists: true,
		},
		{
			name:       "pre Canyon but already deployed",
			timestamp:  canyonTime - 1,
			applied:    false,
			codeExists: true,
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
				ChainID:    params.OptimismGoerliChainConfig.ChainID,
				Optimism:   &chain.OptimismConfig{},
				CanyonTime: big.NewInt(int64(canyonTime)),
			}
			if tt.override != nil {
				tt.override(&cfg)
			}

			_, tx := memdb.NewTestTx(t)
			state := state.New(state.NewPlainStateReader(tx))

			if !tt.codeExists {
				// make sure state is empty
				assert.Equal(t, libcommon.Hash{}, state.GetCodeHash(create2DeployerAddress))
				assert.NotEqual(t, create2DeployerCode, state.GetCode(create2DeployerAddress))
			} else {
				state.SetCode(create2DeployerAddress, create2DeployerCode)
			}

			applied := EnsureCreate2Deployer(&cfg, tt.timestamp, state)
			assert.Equal(t, tt.applied, applied)

			if applied || tt.codeExists {
				assert.Equal(t, create2DeployerCodeHash, state.GetCodeHash(create2DeployerAddress))
				assert.Equal(t, create2DeployerCode, state.GetCode(create2DeployerAddress))
			} else {
				assert.Equal(t, libcommon.Hash{}, state.GetCodeHash(create2DeployerAddress))
				assert.NotEqual(t, create2DeployerCode, state.GetCode(create2DeployerAddress))
			}
		})
	}
}
