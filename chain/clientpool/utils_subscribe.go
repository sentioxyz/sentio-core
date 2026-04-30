package clientpool

import (
	"context"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/queue"
	"time"
)

func SubscribeUsingGetLatest(
	ctx context.Context,
	start uint64,
	interval time.Duration,
	checkBlockIntervalDur time.Duration,
	ch chan<- Block,
	getLatest func(ctx2 context.Context) (Block, error),
) {
	_, logger := log.FromContext(ctx)
	logger.Infof("subscribe using get latest started")
	defer func() {
		logger.Infof("subscribe using get latest finished")
	}()
	wait := interval
	var q queue.Queue[Block]
	var blockInterval time.Duration
	for {
		latest, err := getLatest(ctx)
		if err == nil {
			if latest.Number >= start {
				select {
				case ch <- latest:
				case <-ctx.Done():
					return
				}
			}
			q, blockInterval = pushLatestQueue(q, latest, checkBlockIntervalDur)
			wait = max(interval, blockInterval)
		} else {
			logger.Warnfe(err, "get latest failed")
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return
		}
	}
}
