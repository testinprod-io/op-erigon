// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package params

import (
	"fmt"

	"github.com/ledgerwatch/erigon-lib/kv"
)

var (
	// Following vars are injected through the build flags (see Makefile)
	GitCommit string
	GitBranch string
	GitTag    string
)

// Version is the version of upstream erigon
// see https://calver.org
const (
	VersionMajor       = 2        // Major version component of the current release
	VersionMinor       = 48       // Minor version component of the current release
	VersionMicro       = 1        // Patch version component of the current release
	VersionModifier    = "stable" // Modifier component of the current release
	VersionKeyCreated  = "ErigonVersionCreated"
	VersionKeyFinished = "ErigonVersionFinished"
)

// OPVersion is the version of op-erigon
const (
	OPVersionMajor    = 0          // Major version component of the current release
	OPVersionMinor    = 2          // Minor version component of the current release
	OPVersionMicro    = 0          // Patch version component of the current release
	OPVersionModifier = "unstable" // Version metadata to append to the version string
)

func withModifier(vsn string) string {
	if !isStable() {
		vsn += "-" + OPVersionModifier
	}
	return vsn
}

func isStable() bool {
	return OPVersionModifier == "stable"
}

func isRelease() bool {
	return isStable() || OPVersionModifier == "alpha" || OPVersionModifier == "beta"
}

// Version holds the textual version string.
var Version = func() string {
	return fmt.Sprintf("%d.%02d.%d", OPVersionMajor, OPVersionMinor, OPVersionMicro)
}()

// VersionWithMeta holds the textual version string including the metadata.
var VersionWithMeta = func() string {
	v := Version
	if OPVersionModifier != "" {
		v += "-" + OPVersionModifier
	}
	return v
}()

// ErigonVersion holds the textual erigon version string.
var ErigonVersion = func() string {
	return fmt.Sprintf("%d.%d.%d", VersionMajor, VersionMinor, VersionMicro)
}()

// ErigonVersionWithMeta holds the textual erigon version string including the metadata.
var ErigonVersionWithMeta = func() string {
	v := ErigonVersion
	if VersionModifier != "" {
		v += "-" + VersionModifier
	}
	return v
}()

// ArchiveVersion holds the textual version string used for op-erigon archives.
// e.g. "1.8.11-dea1ce05" for stable releases, or
//
//	"1.8.13-unstable-21c059b6" for unstable releases
func ArchiveVersion(gitCommit string) string {
	vsn := withModifier(Version)
	if len(gitCommit) >= 8 {
		vsn += "-" + gitCommit[:8]
	}
	return vsn
}

func VersionWithCommit(gitCommit string) string {
	vsn := VersionWithMeta
	if len(gitCommit) >= 8 {
		vsn += "-" + gitCommit[:8]
	}
	return vsn
}

func SetErigonVersion(tx kv.RwTx, versionKey string) error {
	versionKeyByte := []byte(versionKey)
	hasVersion, err := tx.Has(kv.DatabaseInfo, versionKeyByte)
	if err != nil {
		return err
	}
	if hasVersion {
		return nil
	}
	// Save version if it does not exist
	if err := tx.Put(kv.DatabaseInfo, versionKeyByte, []byte(Version)); err != nil {
		return err
	}
	return nil
}
