package data

import (
	"context"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/controller"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
)

func SubscribeUsingPolling(
	ctx context.Context,
	minWatchInterval time.Duration,
	timeout time.Duration,
	from controller.BlockHeader,
	latestGetter func(context.Context) (controller.BlockHeader, error),
	callback func(controller.BlockHeader, error),
) {
	const maxWaiting = time.Minute * 3
	_, logger := log.FromContext(ctx)
	watchInterval := minWatchInterval // estimated block interval
	waiting := watchInterval
	for {
		getCtx, cancel := context.WithTimeout(ctx, timeout)
		latest, err := latestGetter(getCtx)
		cancel()
		if err != nil {
			waiting = min(waiting*2, maxWaiting)
			logger.Warnfe(err, "get latest for subscribe failed, will retry after %s", waiting.String())
		} else if latest.GetBlockNumber() < from.GetBlockNumber() {
			waiting = min(waiting*2, maxWaiting)
			logger.Warnf("latest from %s back to %s, will be ignored, and will retry after %s",
				controller.GetBlockSummary(from), controller.GetBlockSummary(latest), waiting.String())
		} else if latest.GetBlockNumber() == from.GetBlockNumber() {
			passed := time.Since(from.GetBlockTime())
			if passed < time.Minute {
				waiting = watchInterval
				logger.Debugf("latest stay at %s, and passed %s, will retry after %s",
					controller.GetBlockSummary(from), passed, waiting.String())
			} else {
				waiting = min(waiting*2, maxWaiting)
				logger.Warnf("latest stay at %s, and passed %s, will retry after %s",
					controller.GetBlockSummary(from), passed, waiting.String())
			}
		} else {
			if latest.GetBlockTime().After(from.GetBlockTime()) {
				timeDelta := latest.GetBlockTime().Sub(from.GetBlockTime())
				blockDelta := latest.GetBlockNumber() - from.GetBlockNumber()
				watchInterval = max(timeDelta/time.Duration(blockDelta), minWatchInterval)
			} else {
				watchInterval = minWatchInterval
			}
			waiting = watchInterval
			logger.Debugf("latest growth from %s to %s, will get latest again after %s",
				controller.GetBlockSummary(from), controller.GetBlockSummary(latest), waiting.String())
			callback(latest, nil)
			from = latest
		}
		select {
		case <-time.After(waiting):
		case <-ctx.Done():
			return
		}
	}
}

func SubscribeUsingWaiting(
	ctx context.Context,
	queryInterval time.Duration,
	from controller.BlockHeader,
	waitLatest func(ctx context.Context, blockNumberGt uint64) (latest controller.BlockHeader, broken, err error),
	callback func(controller.BlockHeader, error),
) {
	_, logger := log.FromContext(ctx)
	var broken error
	for broken == nil {
		if queryInterval > 0 {
			select {
			case <-time.After(queryInterval):
			case <-ctx.Done():
				return
			}
		}
		fromText := controller.GetBlockSummary(from)
		broken = backoff.RetryNotify(
			func() error {
				callCtx, cancel := context.WithTimeout(ctx, time.Minute)
				defer cancel()
				latest, brokenErr, err := waitLatest(callCtx, from.GetBlockNumber())
				if err != nil {
					return err
				} else if brokenErr != nil {
					logger.Errorfe(broken, "wait latest from %s for subscribe failed", fromText)
					callback(latest, brokenErr)
					return backoff.Permanent(brokenErr)
				} else if latest.GetBlockNumber() < from.GetBlockNumber() {
					return errors.Errorf("latest from %s back to %s", fromText, controller.GetBlockSummary(latest))
				} else if latest.GetBlockNumber() == from.GetBlockNumber() {
					return errors.Errorf("latest stay at %s", fromText)
				}
				logger.Debugf("latest growth from %s to %s", fromText, controller.GetBlockSummary(latest))
				callback(latest, nil)
				from = latest
				return nil
			},
			backoff.WithContext(backoff.NewExponentialBackOff(backoff.WithMaxElapsedTime(0)), ctx),
			func(err error, duration time.Duration) {
				logger.Warnfe(err, "wait latest from %s for subscribe failed, will retry after %s", fromText, duration.String())
			})
	}
}
