package jsonrpc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/cmd/rpcdaemon/rpcdaemontest"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"
)

type MockServer struct {
	Server  *httptest.Server
	Payload string
}

func (m *MockServer) Start() {
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(m.Payload))
	}))
}

func (m *MockServer) Stop() {
	m.Server.Close()
}

func (m *MockServer) UpdatePayload(payload string) {
	m.Payload = payload
}

func (m *MockServer) GetRPC() (*rpc.Client, error) {
	if m.Server == nil {
		return nil, fmt.Errorf("server is not started")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	client, err := rpc.DialContext(ctx, m.Server.URL, log.New())
	cancel()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func TestGetBalanceHistoricalRPC(t *testing.T) {
	m, _, _ := rpcdaemontest.CreateOptimismTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 100_000, false, 100_000, 128, log.New())
	addr := libcommon.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")

	table := []struct {
		caseName  string
		payload   string
		appendAPI bool
		isError   bool
		expected  string
	}{
		{
			caseName:  "missing api",
			payload:   "",
			appendAPI: false,
			isError:   true,
			expected:  "no historical RPC is available for this historical (pre-bedrock) execution request",
		},
		{
			caseName:  "success",
			payload:   "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"0x1\"}",
			appendAPI: true,
			isError:   false,
			expected:  "0x1",
		},
		{
			caseName:  "failure",
			payload:   "{\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32000,\"message\":\"error\"}}",
			appendAPI: true,
			isError:   true,
			expected:  "historical backend error: error",
		},
	}

	for _, tt := range table {
		t.Run(tt.caseName, func(t *testing.T) {
			if tt.appendAPI {
				s := MockServer{}
				s.Start()
				defer s.Stop()
				historicalRPCService, err := s.GetRPC()
				if err != nil {
					t.Errorf("failed to start mock server: %v", err)
				}
				api.historicalRPCService = historicalRPCService
				s.UpdatePayload(tt.payload)
			}

			bal, err := api.GetBalance(m.Ctx, addr, rpc.BlockNumberOrHashWithNumber(0))
			if tt.isError {
				require.Error(t, err, tt.caseName)
				require.Equal(t, tt.expected, fmt.Sprintf("%v", err), tt.caseName)
			} else {
				require.NoError(t, err, tt.caseName)
				require.Equal(t, tt.expected, fmt.Sprintf("%v", bal), tt.caseName)
			}
		})
	}
}

func TestGetTransactionCountHistoricalRPC(t *testing.T) {
	m, _, _ := rpcdaemontest.CreateOptimismTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 100_000, false, 100_000, 128, log.New())
	addr := libcommon.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")

	table := []struct {
		caseName  string
		payload   string
		appendAPI bool
		isError   bool
		expected  string
	}{
		{
			caseName:  "missing api",
			payload:   "",
			appendAPI: false,
			isError:   true,
			expected:  "no historical RPC is available for this historical (pre-bedrock) execution request",
		},
		{
			caseName:  "success",
			payload:   "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"0x1\"}",
			appendAPI: true,
			isError:   false,
			expected:  "0x1",
		},
		{
			caseName:  "failure",
			payload:   "{\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32000,\"message\":\"error\"}}",
			appendAPI: true,
			isError:   true,
			expected:  "historical backend error: error",
		},
	}

	for _, tt := range table {
		t.Run(tt.caseName, func(t *testing.T) {
			if tt.appendAPI {
				s := MockServer{}
				s.Start()
				defer s.Stop()
				historicalRPCService, err := s.GetRPC()
				if err != nil {
					t.Errorf("failed to start mock server: %v", err)
				}
				api.historicalRPCService = historicalRPCService
				s.UpdatePayload(tt.payload)
			}

			val, err := api.GetTransactionCount(m.Ctx, addr, rpc.BlockNumberOrHashWithNumber(0))
			if tt.isError {
				require.Error(t, err, tt.caseName)
				require.Equal(t, tt.expected, fmt.Sprintf("%v", err), tt.caseName)
			} else {
				require.NoError(t, err, tt.caseName)
				require.Equal(t, tt.expected, fmt.Sprintf("%v", val), tt.caseName)
			}
		})
	}
}

func TestGetCodeHistoricalRPC(t *testing.T) {
	m, _, _ := rpcdaemontest.CreateOptimismTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 100_000, false, 100_000, 128, log.New())
	addr := libcommon.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")

	table := []struct {
		caseName  string
		payload   string
		appendAPI bool
		isError   bool
		expected  string
	}{
		{
			caseName:  "missing api",
			payload:   "",
			appendAPI: false,
			isError:   true,
			expected:  "no historical RPC is available for this historical (pre-bedrock) execution request",
		},
		{
			caseName:  "success",
			payload:   "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"0x4200000000000000000000000000000000000010\"}",
			appendAPI: true,
			isError:   false,
			expected:  "0x4200000000000000000000000000000000000010",
		},
		{
			caseName:  "failure",
			payload:   "{\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32000,\"message\":\"error\"}}",
			appendAPI: true,
			isError:   true,
			expected:  "historical backend error: error",
		},
	}

	for _, tt := range table {
		t.Run(tt.caseName, func(t *testing.T) {
			if tt.appendAPI {
				s := MockServer{}
				s.Start()
				defer s.Stop()
				historicalRPCService, err := s.GetRPC()
				if err != nil {
					t.Errorf("failed to start mock server: %v", err)
				}
				api.historicalRPCService = historicalRPCService
				s.UpdatePayload(tt.payload)
			}

			val, err := api.GetCode(m.Ctx, addr, rpc.BlockNumberOrHashWithNumber(0))
			if tt.isError {
				require.Error(t, err, tt.caseName)
				require.Equal(t, tt.expected, fmt.Sprintf("%v", err), tt.caseName)
			} else {
				require.NoError(t, err, tt.caseName)
				require.Equal(t, tt.expected, fmt.Sprintf("%v", val), tt.caseName)
			}
		})
	}
}

func TestGetStorageAtHistoricalRPC(t *testing.T) {
	m, _, _ := rpcdaemontest.CreateOptimismTestSentry(t)
	api := NewEthAPI(newBaseApiForTest(m), m.DB, nil, nil, nil, 5000000, 100_000, false, 100_000, 128, log.New())
	addr := libcommon.HexToAddress("0x71562b71999873db5b286df957af199ec94617f7")

	table := []struct {
		caseName  string
		payload   string
		appendAPI bool
		isError   bool
		expected  string
	}{
		{
			caseName:  "missing api",
			payload:   "",
			appendAPI: false,
			isError:   true,
			expected:  "no historical RPC is available for this historical (pre-bedrock) execution request",
		},
		{
			caseName:  "success",
			payload:   "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"0x4200000000000000000000000000000000000010\"}",
			appendAPI: true,
			isError:   false,
			expected:  "0x0000000000000000000000004200000000000000000000000000000000000010",
		},
		{
			caseName:  "failure",
			payload:   "{\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32000,\"message\":\"error\"}}",
			appendAPI: true,
			isError:   true,
			expected:  "historical backend error: error",
		},
	}

	for _, tt := range table {
		t.Run(tt.caseName, func(t *testing.T) {
			if tt.appendAPI {
				s := MockServer{}
				s.Start()
				defer s.Stop()
				historicalRPCService, err := s.GetRPC()
				if err != nil {
					t.Errorf("failed to start mock server: %v", err)
				}
				api.historicalRPCService = historicalRPCService
				s.UpdatePayload(tt.payload)
			}

			val, err := api.GetStorageAt(m.Ctx, addr, "1", rpc.BlockNumberOrHashWithNumber(0))
			if tt.isError {
				require.Error(t, err, tt.caseName)
				require.Equal(t, tt.expected, fmt.Sprintf("%v", err), tt.caseName)
			} else {
				require.NoError(t, err, tt.caseName)
				require.Equal(t, tt.expected, fmt.Sprintf("%v", val), tt.caseName)
			}
		})
	}
}
