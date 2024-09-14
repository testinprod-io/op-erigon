package tracers

import (
	"encoding/json"

	"github.com/erigontech/erigon-lib/common/hexutil"
	"github.com/erigontech/erigon/eth/tracers/logger"
	"github.com/erigontech/erigon/turbo/adapter/ethapi"
)

// TraceConfig holds extra parameters to trace functions.
type TraceConfig struct {
	*logger.LogConfig
	Tracer         *string                `json:"tracer"`
	TracerConfig   *json.RawMessage       `json:"tracerConfig,omitempty"`
	Timeout        *string                `json:"timeout,omitempty"`
	Reexec         *uint64                `json:"reexec,omitempty"`
	NoRefunds      *bool                  `json:"noRefunds,omitempty"` // Turns off gas refunds when tracing
	StateOverrides *ethapi.StateOverrides `json:"stateOverrides,omitempty"`

	BorTraceEnabled *bool
	TxIndex         *hexutil.Uint `json:"txIndex,omitempty"`
}
