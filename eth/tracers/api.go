package tracers

import (
	"github.com/ledgerwatch/erigon/eth/tracers/logger"
	"github.com/ledgerwatch/erigon/turbo/adapter/ethapi"
)

// TraceConfig holds extra parameters to trace functions.
type TraceConfig struct {
	*logger.LogConfig
	Tracer         *string                `json:"tracer"`
	Timeout        *string                `json:"timeout,omitempty"`
	Reexec         *uint64                `json:"reexec,omitempty"`
	NoRefunds      *bool                  `json:"-"` // Turns off gas refunds when tracing
	StateOverrides *ethapi.StateOverrides `json:"-"`
}
