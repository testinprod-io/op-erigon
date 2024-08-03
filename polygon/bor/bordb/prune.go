<<<<<<< HEAD
=======
// Copyright 2024 The Erigon Authors
// This file is part of Erigon.
//
// Erigon is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Erigon is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Erigon. If not, see <http://www.gnu.org/licenses/>.

>>>>>>> v3.0.0-alpha1
package bordb

import (
	"encoding/binary"
	"errors"
	"math"

<<<<<<< HEAD
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/polygon/bor/snaptype"
	"github.com/ledgerwatch/erigon/polygon/heimdall"
=======
	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon/polygon/bor/snaptype"
	"github.com/erigontech/erigon/polygon/heimdall"
>>>>>>> v3.0.0-alpha1
)

// PruneBorBlocks - delete [1, to) old blocks after moving it to snapshots.
// keeps genesis in db: [1, to)
<<<<<<< HEAD
// doesn't change sequences of kv.EthTx and kv.NonCanonicalTxs
// doesn't delete Receipts, Senders, Canonical markers, TotalDifficulty
func PruneBorBlocks(tx kv.RwTx, blockTo uint64, blocksDeleteLimit int, SpanIdAt func(number uint64) uint64) error {
	c, err := tx.Cursor(kv.BorEventNums)
	if err != nil {
		return err
=======
// doesn't change sequences of kv.EthTx
// doesn't delete Receipts, Senders, Canonical markers, TotalDifficulty
func PruneBorBlocks(tx kv.RwTx, blockTo uint64, blocksDeleteLimit int, SpanIdAt func(number uint64) uint64) (deleted int, err error) {
	c, err := tx.Cursor(kv.BorEventNums)
	if err != nil {
		return deleted, err
>>>>>>> v3.0.0-alpha1
	}
	defer c.Close()
	var blockNumBytes [8]byte
	binary.BigEndian.PutUint64(blockNumBytes[:], blockTo)
	k, v, err := c.Seek(blockNumBytes[:])
	if err != nil {
<<<<<<< HEAD
		return err
=======
		return deleted, err
>>>>>>> v3.0.0-alpha1
	}
	var eventIdTo uint64 = math.MaxUint64
	if k != nil {
		eventIdTo = binary.BigEndian.Uint64(v)
	}
<<<<<<< HEAD
	c1, err := tx.RwCursor(kv.BorEvents)
	if err != nil {
		return err
=======

	c1, err := tx.RwCursor(kv.BorEvents)
	if err != nil {
		return deleted, err
>>>>>>> v3.0.0-alpha1
	}
	defer c1.Close()
	counter := blocksDeleteLimit
	for k, _, err = c1.First(); err == nil && k != nil && counter > 0; k, _, err = c1.Next() {
		eventId := binary.BigEndian.Uint64(k)
		if eventId >= eventIdTo {
			break
		}
		if err = c1.DeleteCurrent(); err != nil {
<<<<<<< HEAD
			return err
		}
		counter--
	}
	if err != nil {
		return err
=======
			return deleted, err
		}
		deleted++
		counter--
	}
	if err != nil {
		return deleted, err
>>>>>>> v3.0.0-alpha1
	}
	firstSpanToKeep := SpanIdAt(blockTo)
	c2, err := tx.RwCursor(kv.BorSpans)
	if err != nil {
<<<<<<< HEAD
		return err
=======
		return deleted, err
>>>>>>> v3.0.0-alpha1
	}
	defer c2.Close()
	counter = blocksDeleteLimit
	for k, _, err := c2.First(); err == nil && k != nil && counter > 0; k, _, err = c2.Next() {
		spanId := binary.BigEndian.Uint64(k)
		if spanId >= firstSpanToKeep {
			break
		}
		if err = c2.DeleteCurrent(); err != nil {
<<<<<<< HEAD
			return err
		}
=======
			return deleted, err
		}
		deleted++
>>>>>>> v3.0.0-alpha1
		counter--
	}

	if snaptype.CheckpointsEnabled() {
		checkpointCursor, err := tx.RwCursor(kv.BorCheckpoints)
		if err != nil {
<<<<<<< HEAD
			return err
=======
			return deleted, err
>>>>>>> v3.0.0-alpha1
		}

		defer checkpointCursor.Close()
		lastCheckpointToRemove, err := heimdall.CheckpointIdAt(tx, blockTo)

		if err != nil {
<<<<<<< HEAD
			return err
=======
			return deleted, err
>>>>>>> v3.0.0-alpha1
		}

		var checkpointIdBytes [8]byte
		binary.BigEndian.PutUint64(checkpointIdBytes[:], uint64(lastCheckpointToRemove))
		for k, _, err := checkpointCursor.Seek(checkpointIdBytes[:]); err == nil && k != nil; k, _, err = checkpointCursor.Prev() {
			if err = checkpointCursor.DeleteCurrent(); err != nil {
<<<<<<< HEAD
				return err
			}
=======
				return deleted, err
			}
			deleted++
>>>>>>> v3.0.0-alpha1
		}
	}

	if snaptype.MilestonesEnabled() {
		milestoneCursor, err := tx.RwCursor(kv.BorMilestones)

		if err != nil {
<<<<<<< HEAD
			return err
=======
			return deleted, err
>>>>>>> v3.0.0-alpha1
		}

		defer milestoneCursor.Close()

		var lastMilestoneToRemove heimdall.MilestoneId

		for blockCount := 1; err != nil && blockCount < blocksDeleteLimit; blockCount++ {
			lastMilestoneToRemove, err = heimdall.MilestoneIdAt(tx, blockTo-uint64(blockCount))

			if !errors.Is(err, heimdall.ErrMilestoneNotFound) {
<<<<<<< HEAD
				return err
			} else {
				if blockCount == blocksDeleteLimit-1 {
					return nil
=======
				return deleted, err
			} else {
				if blockCount == blocksDeleteLimit-1 {
					return deleted, nil
>>>>>>> v3.0.0-alpha1
				}
			}
		}

		var milestoneIdBytes [8]byte
		binary.BigEndian.PutUint64(milestoneIdBytes[:], uint64(lastMilestoneToRemove))
		for k, _, err := milestoneCursor.Seek(milestoneIdBytes[:]); err == nil && k != nil; k, _, err = milestoneCursor.Prev() {
			if err = milestoneCursor.DeleteCurrent(); err != nil {
<<<<<<< HEAD
				return err
			}
		}
	}

	return nil
=======
				return deleted, err
			}
			deleted++
		}
	}

	return deleted, nil
>>>>>>> v3.0.0-alpha1
}
