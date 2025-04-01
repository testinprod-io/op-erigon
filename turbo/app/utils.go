package app

import (
	"time"

	libcommon "github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/crypto"
	"github.com/erigontech/erigon-lib/log/v3"
)

var emptyCodeHash = crypto.Keccak256Hash(nil)
var emptyHash = libcommon.Hash{}

func StatusReporter(msg string, idx *int) chan struct{} {
	startTime := time.Now()
	ticker := time.NewTicker(8 * time.Second)
	quit := make(chan struct{})

	go func(index *int) {
		for {
			select {
			case <-ticker.C:
				log.Info(msg, "index", *index, "elapsed", time.Since(startTime))
			case <-quit:
				log.Info(msg, "index", *index, "elapsed", time.Since(startTime))
				ticker.Stop()
				return
			}
		}
	}(idx)

	return quit
}
