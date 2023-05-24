package app

import (
	"time"

	"github.com/ledgerwatch/log/v3"
)

func StatusReporter(msg string, idx *int) chan struct{} {
	startTime := time.Now()
	ticker := time.NewTicker(8 * time.Second)
	quit := make(chan struct{})

	go func(index *int) {
		for {
			select {
			case <-ticker.C:
				log.Info(msg, "index", *index, "elapsed", time.Duration(time.Since(startTime)))
			case <-quit:
				log.Info(msg, "index", *index, "elapsed", time.Duration(time.Since(startTime)))
				ticker.Stop()
				return
			}
		}
	}(idx)

	return quit
}
