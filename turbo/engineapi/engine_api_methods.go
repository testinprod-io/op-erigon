package engineapi

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"

	libcommon "github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/common/hexutil"
	"github.com/erigontech/erigon-lib/common/hexutility"
	"github.com/erigontech/erigon-lib/log/v3"
	"github.com/erigontech/erigon/cl/clparams"
	"github.com/erigontech/erigon/params"
	"github.com/erigontech/erigon/rpc"
	"github.com/erigontech/erigon/turbo/engineapi/engine_types"
)

var ourCapabilities = []string{
	"engine_forkchoiceUpdatedV1",
	"engine_forkchoiceUpdatedV2",
	"engine_forkchoiceUpdatedV3",
	"engine_newPayloadV1",
	"engine_newPayloadV2",
	"engine_newPayloadV3",
	"engine_newPayloadV4",
	"engine_getPayloadV1",
	"engine_getPayloadV2",
	"engine_getPayloadV3",
	"engine_getPayloadV4",
	"engine_exchangeTransitionConfigurationV1",
	"engine_getPayloadBodiesByHashV1",
	"engine_getPayloadBodiesByRangeV1",
	"engine_getClientVersionV1",
}

// Returns the most recent version of the payload(for the payloadID) at the time of receiving the call
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/paris.md#engine_getpayloadv1
func (e *EngineServer) GetPayloadV1(ctx context.Context, payloadId hexutility.Bytes) (*engine_types.ExecutionPayload, error) {

	decodedPayloadId := binary.BigEndian.Uint64(payloadId)
	e.logger.Info("Received GetPayloadV1", "payloadId", decodedPayloadId)

	response, err := e.getPayload(ctx, decodedPayloadId, clparams.BellatrixVersion)
	if err != nil {
		return nil, err
	}

	return response.ExecutionPayload, nil
}

// Same as [GetPayloadV1] with addition of blockValue
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/shanghai.md#engine_getpayloadv2
func (e *EngineServer) GetPayloadV2(ctx context.Context, payloadID hexutility.Bytes) (*engine_types.GetPayloadResponse, error) {
	decodedPayloadId := binary.BigEndian.Uint64(payloadID)
	e.logger.Info("Received GetPayloadV2", "payloadId", decodedPayloadId)
	return e.getPayload(ctx, decodedPayloadId, clparams.CapellaVersion)
}

// Same as [GetPayloadV2], with addition of blobsBundle containing valid blobs, commitments, proofs
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/cancun.md#engine_getpayloadv3
func (e *EngineServer) GetPayloadV3(ctx context.Context, payloadID hexutility.Bytes) (*engine_types.GetPayloadResponse, error) {
	decodedPayloadId := binary.BigEndian.Uint64(payloadID)
	e.logger.Info("Received GetPayloadV3", "payloadId", decodedPayloadId)
	return e.getPayload(ctx, decodedPayloadId, clparams.DenebVersion)
}

// Same as [GetPayloadV3], but returning ExecutionPayloadV4 (= ExecutionPayloadV3 + requests)
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/prague.md#engine_getpayloadv4
func (e *EngineServer) GetPayloadV4(ctx context.Context, payloadID hexutility.Bytes) (*engine_types.GetPayloadResponse, error) {
	decodedPayloadId := binary.BigEndian.Uint64(payloadID)
	e.logger.Info("Received GetPayloadV4", "payloadId", decodedPayloadId)
	return e.getPayload(ctx, decodedPayloadId, clparams.ElectraVersion)
}

// Updates the forkchoice state after validating the headBlockHash
// Additionally, builds and returns a unique identifier for an initial version of a payload
// (asynchronously updated with transactions), if payloadAttributes is not nil and passes validation
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/paris.md#engine_forkchoiceupdatedv1
func (e *EngineServer) ForkchoiceUpdatedV1(ctx context.Context, forkChoiceState *engine_types.ForkChoiceState, payloadAttributes *engine_types.PayloadAttributes) (*engine_types.ForkChoiceUpdatedResponse, error) {
	return e.forkchoiceUpdated(ctx, forkChoiceState, payloadAttributes, clparams.BellatrixVersion)
}

// Same as, and a replacement for, [ForkchoiceUpdatedV1], post Shanghai
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/shanghai.md#engine_forkchoiceupdatedv2
func (e *EngineServer) ForkchoiceUpdatedV2(ctx context.Context, forkChoiceState *engine_types.ForkChoiceState, payloadAttributes *engine_types.PayloadAttributes) (*engine_types.ForkChoiceUpdatedResponse, error) {
	return e.forkchoiceUpdated(ctx, forkChoiceState, payloadAttributes, clparams.CapellaVersion)
}

// Successor of [ForkchoiceUpdatedV2] post Cancun, with stricter check on params
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/cancun.md#engine_forkchoiceupdatedv3
func (e *EngineServer) ForkchoiceUpdatedV3(ctx context.Context, forkChoiceState *engine_types.ForkChoiceState, payloadAttributes *engine_types.PayloadAttributes) (*engine_types.ForkChoiceUpdatedResponse, error) {
	return e.forkchoiceUpdated(ctx, forkChoiceState, payloadAttributes, clparams.DenebVersion)
}

// NewPayloadV1 processes new payloads (blocks) from the beacon chain without withdrawals.
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/paris.md#engine_newpayloadv1
func (e *EngineServer) NewPayloadV1(ctx context.Context, payload *engine_types.ExecutionPayload) (*engine_types.PayloadStatus, error) {
	return e.newPayload(ctx, payload, nil, nil, nil, clparams.BellatrixVersion)
}

// NewPayloadV2 processes new payloads (blocks) from the beacon chain with withdrawals.
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/shanghai.md#engine_newpayloadv2
func (e *EngineServer) NewPayloadV2(ctx context.Context, payload *engine_types.ExecutionPayload) (*engine_types.PayloadStatus, error) {
	return e.newPayload(ctx, payload, nil, nil, nil, clparams.CapellaVersion)
}

// NewPayloadV3 processes new payloads (blocks) from the beacon chain with withdrawals & blob gas.
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/cancun.md#engine_newpayloadv3
func (e *EngineServer) NewPayloadV3(ctx context.Context, payload *engine_types.ExecutionPayload,
	expectedBlobHashes []libcommon.Hash, parentBeaconBlockRoot *libcommon.Hash) (*engine_types.PayloadStatus, error) {
	return e.newPayload(ctx, payload, expectedBlobHashes, parentBeaconBlockRoot, nil, clparams.DenebVersion)
}

// NewPayloadV4 processes new payloads (blocks) from the beacon chain with withdrawals, blob gas and requests.
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/prague.md#engine_newpayloadv4
func (e *EngineServer) NewPayloadV4(ctx context.Context, payload *engine_types.ExecutionPayload,
	expectedBlobHashes []libcommon.Hash, parentBeaconBlockRoot *libcommon.Hash, executionRequests []hexutility.Bytes) (*engine_types.PayloadStatus, error) {
	// TODO(racytech): add proper version or refactor this part
	// add all version ralated checks here so the newpayload doesn't have to deal with checks
	if e.config.IsOptimismIsthmus(payload.Timestamp.Uint64()) && payload.WithdrawalsRoot == nil {
		return nil, &rpc.InvalidParamsError{Message: "nil withdrawalsRoot post-isthmus"}
	}
	return e.newPayload(ctx, payload, expectedBlobHashes, parentBeaconBlockRoot, executionRequests, clparams.ElectraVersion)
}

// Returns an array of execution payload bodies referenced by their block hashes
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/shanghai.md#engine_getpayloadbodiesbyhashv1
func (e *EngineServer) GetPayloadBodiesByHashV1(ctx context.Context, hashes []libcommon.Hash) ([]*engine_types.ExecutionPayloadBody, error) {
	return e.getPayloadBodiesByHash(ctx, hashes)
}

// Returns an ordered (as per canonical chain) array of execution payload bodies, with corresponding execution block numbers from "start", up to "count"
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/shanghai.md#engine_getpayloadbodiesbyrangev1
func (e *EngineServer) GetPayloadBodiesByRangeV1(ctx context.Context, start, count hexutil.Uint64) ([]*engine_types.ExecutionPayloadBody, error) {
	return e.getPayloadBodiesByRange(ctx, uint64(start), uint64(count))
}

// Receives consensus layer's transition configuration and checks if the execution layer has the correct configuration.
// Can also be used to ping the execution layer (heartbeats).
// See https://github.com/ethereum/execution-apis/blob/v1.0.0-beta.1/src/engine/specification.md#engine_exchangetransitionconfigurationv1
func (e *EngineServer) ExchangeTransitionConfigurationV1(ctx context.Context, beaconConfig *engine_types.TransitionConfiguration) (*engine_types.TransitionConfiguration, error) {
	terminalTotalDifficulty := e.config.TerminalTotalDifficulty

	if terminalTotalDifficulty == nil {
		return nil, fmt.Errorf("the execution layer doesn't have a terminal total difficulty. expected: %v", beaconConfig.TerminalTotalDifficulty)
	}

	if terminalTotalDifficulty.Cmp((*big.Int)(beaconConfig.TerminalTotalDifficulty)) != 0 {
		return nil, fmt.Errorf("the execution layer has a wrong terminal total difficulty. expected %v, but instead got: %d", beaconConfig.TerminalTotalDifficulty, terminalTotalDifficulty)
	}

	return &engine_types.TransitionConfiguration{
		TerminalTotalDifficulty: (*hexutil.Big)(terminalTotalDifficulty),
		TerminalBlockHash:       libcommon.Hash{},
		TerminalBlockNumber:     (*hexutil.Big)(libcommon.Big0),
	}, nil
}

// Returns the node's code and commit details in a slice
// See https://github.com/ethereum/execution-apis/blob/main/src/engine/identification.md#engine_getclientversionv1
func (e *EngineServer) GetClientVersionV1(ctx context.Context, callerVersion *engine_types.ClientVersionV1) ([]engine_types.ClientVersionV1, error) {
	if callerVersion != nil {
		e.logger.Info("[GetClientVersionV1] Received request from" + callerVersion.String())
	}
	commitString := params.GitCommit
	if len(commitString) >= 8 {
		commitString = commitString[:8]
	} else {
		commitString = "00000000" // shouldn't be triggered
	}
	result := make([]engine_types.ClientVersionV1, 1)
	result[0] = engine_types.ClientVersionV1{
		Code:    params.ClientCode,
		Name:    params.ClientName,
		Version: params.VersionWithCommit(params.GitCommit),
		Commit:  "0x" + commitString,
	}
	return result, nil
}

func (e *EngineServer) ExchangeCapabilities(fromCl []string) []string {
	missingOurs := compareCapabilities(fromCl, ourCapabilities)
	missingCl := compareCapabilities(ourCapabilities, fromCl)

	if len(missingCl) > 0 || len(missingOurs) > 0 {
		e.logger.Debug("ExchangeCapabilities mismatches", "cl_unsupported", missingCl, "erigon_unsupported", missingOurs)
	}

	return ourCapabilities
}

func (e *EngineServer) SignalSuperchainV1(ctx context.Context, signal *engine_types.SuperchainSignal) (params.ProtocolVersion, error) {
	if signal == nil {
		log.Info("Received empty superchain version signal", "local", params.OPStackSupport)
		return params.OPStackSupport, nil
	}

	// log any warnings/info
	logger := log.New("local", params.OPStackSupport, "required", signal.Required, "recommended", signal.Recommended)
	LogProtocolVersionSupport(logger, params.OPStackSupport, signal.Recommended, "recommended")
	LogProtocolVersionSupport(logger, params.OPStackSupport, signal.Required, "required")

	if err := e.HandleRequiredProtocolVersion(signal.Required); err != nil {
		log.Error("Failed to handle required protocol version", "err", err, "required", signal.Required)
		return params.OPStackSupport, err
	}

	return params.OPStackSupport, nil
}

// HandleRequiredProtocolVersion handles the protocol version signal. This implements opt-in halting,
// the protocol version data is already logged and metered when signaled through the Engine API.
func (e *EngineServer) HandleRequiredProtocolVersion(required params.ProtocolVersion) error {
	var needLevel int
	switch e.ethConfig.RollupHaltOnIncompatibleProtocolVersion {
	case "major":
		needLevel = 3
	case "minor":
		needLevel = 2
	case "patch":
		needLevel = 1
	default:
		return nil // do not consider halting if not configured to
	}
	haveLevel := 0
	switch params.OPStackSupport.Compare(required) {
	case params.OutdatedMajor:
		haveLevel = 3
	case params.OutdatedMinor:
		haveLevel = 2
	case params.OutdatedPatch:
		haveLevel = 1
	}
	if haveLevel >= needLevel { // halt if we opted in to do so at this granularity
		log.Error("Opted to halt, unprepared for protocol change", "required", required, "local", params.OPStackSupport)
		return e.nodeCloser()
	}
	return nil
}

func LogProtocolVersionSupport(logger log.Logger, local, other params.ProtocolVersion, name string) {
	switch local.Compare(other) {
	case params.AheadMajor:
		logger.Info(fmt.Sprintf("Ahead with major %s protocol version change", name))
	case params.AheadMinor, params.AheadPatch, params.AheadPrerelease:
		logger.Debug(fmt.Sprintf("Ahead with compatible %s protocol version change", name))
	case params.Matching:
		logger.Debug(fmt.Sprintf("Latest %s protocol version is supported", name))
	case params.OutdatedMajor:
		logger.Error(fmt.Sprintf("Outdated with major %s protocol change", name))
	case params.OutdatedMinor:
		logger.Warn(fmt.Sprintf("Outdated with minor backward-compatible %s protocol change", name))
	case params.OutdatedPatch:
		logger.Info(fmt.Sprintf("Outdated with support backward-compatible %s protocol change", name))
	case params.OutdatedPrerelease:
		logger.Debug(fmt.Sprintf("New %s protocol pre-release is available", name))
	case params.DiffBuild:
		logger.Debug(fmt.Sprintf("Ignoring %s protocolversion signal, local build is different", name))
	case params.DiffVersionType:
		logger.Warn(fmt.Sprintf("Failed to recognize %s protocol version signal version-type", name))
	case params.EmptyVersion:
		logger.Debug(fmt.Sprintf("No %s protocol version available to check", name))
	}
}
