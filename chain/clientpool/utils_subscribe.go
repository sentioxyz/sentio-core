package clientpool

import (
	"context"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/queue"
	"time"
)

func Subscribe(
	ctx context.Context,
	checkBlockIntervalDur time.Duration,
	latestChan <-chan Block, // other channel to receive latest block
	interval time.Duration,
	getLatest func(context.Context) (Block, error), // used to get latest immediately
	stop func(latest Block) bool,
	out chan<- Block,
) {
	_, logger := log.FromContext(ctx)
	logger.Infof("subscribe latest started")
	defer func() {
		logger.Infof("subscribe latest finished")
	}()
	wait := interval
	var q queue.Queue[Block]
	var blockInterval time.Duration
	for {
		var latest Block
		var has bool
		var fromLatestChan bool
		waiter := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			waiter.Stop()
			return
		case <-waiter.C:
			waiter.Stop()
			var err error
			latest, err = getLatest(ctx)
			if err != nil {
				logger.Warnfe(err, "get latest failed")
				continue
			}
		case latest, has = <-latestChan:
			waiter.Stop()
			if !has {
				latestChan = make(chan Block) // reset to a never closed chan
				continue
			}
			fromLatestChan = true
		}
		if stop != nil && stop(latest) {
			return
		}
		q, blockInterval = pushLatestQueue(q, latest, checkBlockIntervalDur)
		wait = max(interval, blockInterval)
		if fromLatestChan {
			wait *= 2 // use latestChan first
		}
		select {
		case out <- latest:
		case <-ctx.Done():
			return
		}
	}
}
