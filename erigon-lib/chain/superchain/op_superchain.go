package superchain

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/erigontech/erigon-lib/common"
)

var (
	OPMainnetGenesisHash = common.HexToHash("0x7ca38a1916c42007829c55e69d3e9a73265554b586a499015373241b8a3fa48b")
)

const (
	OPMainnetChainID = 10
)

var OPStackSupport = ProtocolVersionV0{Build: [8]byte{}, Major: 9, Minor: 0, Patch: 0, PreRelease: 0}.Encode()

// OPStackChainConfigByName loads chain config corresponding to the chain name from superchain registry.
// This implementation is based on optimism monorepo(https://github.com/ethereum-optimism/optimism/blob/op-node/v1.4.1/op-node/chaincfg/chains.go#L59)
func OPStackChainConfigByName(name string) *ChainConfig {
	// Handle legacy name aliases
	for _, chain := range Chains {
		if strings.EqualFold(chain.Name+"-"+chain.Network, name) {
			chainCfg, err := chain.Config()
			if err == nil {
				return chainCfg
			}
		}
	}
	return nil
}

// OPStackChainConfigByGenesisHash loads chain config corresponding to the genesis hash from superchain registry.
func OPStackChainConfigByGenesisHash(genesisHash common.Hash) *ChainConfig {
	if bytes.Equal(genesisHash.Bytes(), OPMainnetGenesisHash.Bytes()) {
		chainCfg, err := Chains[OPMainnetChainID].Config()
		if err == nil {
			return chainCfg
		}
	}
	for _, chain := range Chains {
		chainCfg, err := chain.Config()
		if err == nil {
			if bytes.Equal(chainCfg.Genesis.L2.Hash[:], genesisHash.Bytes()) {
				return chainCfg
			}
		}
	}
	return nil
}

// ProtocolVersion encodes the OP-Stack protocol version. See OP-Stack superchain-upgrade specification.
type ProtocolVersion [32]byte

func (p ProtocolVersion) MarshalText() ([]byte, error) {
	return common.Hash(p).MarshalText()
}

func (p *ProtocolVersion) UnmarshalText(input []byte) error {
	return (*common.Hash)(p).UnmarshalText(input)
}

func (p ProtocolVersion) Parse() (versionType uint8, build [8]byte, major, minor, patch, preRelease uint32) {
	versionType = p[0]
	if versionType != 0 {
		return
	}
	// bytes 1:8 reserved for future use
	copy(build[:], p[8:16])                        // differentiates forks and custom-builds of standard protocol
	major = binary.BigEndian.Uint32(p[16:20])      // incompatible API changes
	minor = binary.BigEndian.Uint32(p[20:24])      // identifies additional functionality in backwards compatible manner
	patch = binary.BigEndian.Uint32(p[24:28])      // identifies backward-compatible bug-fixes
	preRelease = binary.BigEndian.Uint32(p[28:32]) // identifies unstable versions that may not satisfy the above
	return
}

func (p ProtocolVersion) String() string {
	versionType, build, major, minor, patch, preRelease := p.Parse()
	if versionType != 0 {
		return "v0.0.0-unknown." + common.Hash(p).String()
	}
	ver := fmt.Sprintf("v%d.%d.%d", major, minor, patch)
	if preRelease != 0 {
		ver += fmt.Sprintf("-%d", preRelease)
	}
	if build != ([8]byte{}) {
		if humanBuildTag(build) {
			ver += "+" + strings.TrimRight(string(build[:]), "\x00")
		} else {
			ver += fmt.Sprintf("+0x%x", build)
		}
	}
	return ver
}

// humanBuildTag identifies which build tag we can stringify for human-readable versions
func humanBuildTag(v [8]byte) bool {
	for i, c := range v { // following semver.org advertised regex, alphanumeric with '-' and '.', except leading '.'.
		if c == 0 { // trailing zeroed are allowed
			for _, d := range v[i:] {
				if d != 0 {
					return false
				}
			}
			return true
		}
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || (c == '.' && i > 0)) {
			return false
		}
	}
	return true
}

// ProtocolVersionComparison is used to identify how far ahead/outdated a protocol version is relative to another.
// This value is used in metrics and switch comparisons, to easily identify each type of version difference.
// Negative values mean the version is outdated.
// Positive values mean the version is up-to-date.
// Matching versions have a 0.
type ProtocolVersionComparison int

const (
	AheadMajor         ProtocolVersionComparison = 4
	OutdatedMajor      ProtocolVersionComparison = -4
	AheadMinor         ProtocolVersionComparison = 3
	OutdatedMinor      ProtocolVersionComparison = -3
	AheadPatch         ProtocolVersionComparison = 2
	OutdatedPatch      ProtocolVersionComparison = -2
	AheadPrerelease    ProtocolVersionComparison = 1
	OutdatedPrerelease ProtocolVersionComparison = -1
	Matching           ProtocolVersionComparison = 0
	DiffVersionType    ProtocolVersionComparison = 100
	DiffBuild          ProtocolVersionComparison = 101
	EmptyVersion       ProtocolVersionComparison = 102
)

func (p ProtocolVersion) Compare(other ProtocolVersion) (cmp ProtocolVersionComparison) {
	if p == (ProtocolVersion{}) || (other == (ProtocolVersion{})) {
		return EmptyVersion
	}
	aVersionType, aBuild, aMajor, aMinor, aPatch, aPreRelease := p.Parse()
	bVersionType, bBuild, bMajor, bMinor, bPatch, bPreRelease := other.Parse()
	if aVersionType != bVersionType {
		return DiffVersionType
	}
	if aBuild != bBuild {
		return DiffBuild
	}
	fn := func(a, b uint32, ahead, outdated ProtocolVersionComparison) ProtocolVersionComparison {
		if a == b {
			return Matching
		}
		if a > b {
			return ahead
		}
		return outdated
	}
	if c := fn(aMajor, bMajor, AheadMajor, OutdatedMajor); c != Matching {
		return c
	}
	if c := fn(aMinor, bMinor, AheadMinor, OutdatedMinor); c != Matching {
		return c
	}
	if c := fn(aPatch, bPatch, AheadPatch, OutdatedPatch); c != Matching {
		return c
	}
	return fn(aPreRelease, bPreRelease, AheadPrerelease, OutdatedPrerelease)
}

type ProtocolVersionV0 struct {
	Build                           [8]byte
	Major, Minor, Patch, PreRelease uint32
}

func (v ProtocolVersionV0) Encode() (out ProtocolVersion) {
	copy(out[8:16], v.Build[:])
	binary.BigEndian.PutUint32(out[16:20], v.Major)
	binary.BigEndian.PutUint32(out[20:24], v.Minor)
	binary.BigEndian.PutUint32(out[24:28], v.Patch)
	binary.BigEndian.PutUint32(out[28:32], v.PreRelease)
	return
}
