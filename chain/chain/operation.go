package chain

import (
	"context"
	"errors"
	"sentioxyz/sentio-core/common/utils"
	"time"

	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio/chain/slot"
	"sentioxyz/sentio/common/number"
)

// Repair if data is missing for a saved slot in dst chain, it will get data from src chain to fix it
func Repair[SLOT slot.Slot](ctx context.Context, src, dst Dimension[SLOT], interval number.Range) error {
	ctx, logger := log.FromContext(ctx, "repairRange", interval)
	logger.Info("repair begin")

	curRange, err := dst.GetRange(ctx)
	if err != nil {
		logger.Warne(err, "get range of dst chain failed")
		return err
	}

	missing := make(chan number.Range)
	var total uint64
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer close(missing)
		return dst.CheckMissing(ctx, curRange.GetIntersection(interval), missing)
	})
	g.Go(func() error {
		for repairRange := range missing {
			logger.Infof("detected missing %s", repairRange)
			if err = Copy(ctx, src, dst, repairRange, true); err != nil {
				logger.Warnfe(err, "repair %s failed", repairRange)
				return err
			}
			total += repairRange.Size()
		}
		return nil
	})
	if err = g.Wait(); err != nil {
		logger.With("totalRepaired", total).Warne(err, "repair failed")
		return err
	}
	logger.Infow("repair succeed", "totalRepaired", total)
	return nil
}

// Copy the specified range of slots from src to dst chain.
// If the slot number is not consecutive after copying, it will return ErrDiscontinuous.
// If the link is not continuous after copying, it will return ErrLink.
func Copy[SLOT slot.Slot](ctx context.Context, src, dst Dimension[SLOT], interval number.Range, overwrite bool) error {
	ctx, logger := log.FromContext(ctx, "copyRange", interval, "overwrite", overwrite)
	srcRange, err := src.GetRange(ctx)
	if err != nil {
		logger.Warne(err, "copy canceled, get range of src chain failed")
		return err
	}
	if srcRange.GetIntersection(interval).Size() == 0 {
		logger.Warnf("copy canceled, range of source %s and interval %s do not intersect, nothing can be copied",
			srcRange, interval)
		return nil
	}

	curRange, err := dst.GetRange(ctx)
	if err != nil {
		logger.Warne(err, "copy canceled, get range of dst chain failed")
		return err
	}

	if srcRange.GetIntersection(interval).GetDistance(curRange) > 0 {
		logger.Warnfe(ErrDiscontinuous, "copy canceled, intersection of source range %s and interval %s is %s, "+
			"is far away from range of destination %s", srcRange, interval, srcRange.GetIntersection(interval), curRange)
		return ErrDiscontinuous
	}

	targetRanges := number.NewSet(srcRange.GetIntersection(interval))
	if !overwrite {
		targetRanges = srcRange.GetIntersection(interval).Sub(curRange)
	}
	logger.Infof("copy begin, target is %s, source is %s, destination is %s", targetRanges, srcRange, curRange)
	if err = doCopy(ctx, src, dst, targetRanges); err != nil {
		logger.Warne(err, "copy failed")
	} else {
		logger.Infof("copy succeed, target is %s", targetRanges)
	}
	return err
}

func doCopy[SLOT slot.Slot](ctx context.Context, src, dst Dimension[SLOT], targetRanges number.Set) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, x := range targetRanges {
		targetRange := x
		slotChan := make(chan SLOT)
		g.Go(func() error {
			defer close(slotChan)
			return src.Load(ctx, targetRange, slotChan)
		})
		g.Go(func() error {
			return dst.Save(ctx, targetRange, slotChan)
		})
	}
	return g.Wait()
}

type SyncConfig struct {
	RoundInterval time.Duration

	// configs about delete outdated history.
	// DstTargetLen = 0 means do not delete history,
	// if DstTargetLen > 0, DstLeftAlign > 0 must be true
	DstTargetLen uint64
	DstLeftAlign number.Number
}

// Sync continuously synchronize the latest slot from src to dst chain
// If the src chain is rolled back, the saved slots of dst chain will also be rolled back
func Sync[SLOT slot.Slot](ctx context.Context, src, dst Dimension[SLOT], config SyncConfig) error {
	ctx, logger := log.FromContext(ctx)
	logger.Infof("sync begin, config: %v", utils.MustJSONMarshal(config))

	ticker := time.NewTicker(config.RoundInterval)
	defer ticker.Stop()
	for roundIndex := uint64(0); ; roundIndex++ {
		if roundIndex > 0 {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		roundCtx, roundLogger := log.FromContext(ctx, "round", roundIndex)

		// Determine the sync range
		curRange, err := dst.GetRange(roundCtx)
		if err != nil {
			roundLogger.Warne(err, "get range of destination chain failed")
			continue
		}
		srcRange, err := src.GetRange(roundCtx)
		if err != nil {
			roundLogger.Warne(err, "get range of source chain failed")
			continue
		}
		if srcRange.IsEmpty() {
			roundLogger.Warn("range of source chain is empty")
			continue
		}

		var syncRange number.Range
		if curRange.IsEmpty() {
			syncRange = srcRange
		} else if curRange.GetDistance(srcRange) > 0 {
			roundLogger.Warnfe(ErrDiscontinuous, "source range %s is far away from the destination range %s",
				srcRange, curRange)
			continue
		} else if srcRange.R() <= curRange.R() {
			roundLogger.Warnf("no slots need to sync, source is %s and destination is %s", srcRange, curRange)
			continue
		} else {
			syncRange = srcRange.Sub(curRange).GetLastRange()
		}

		roundLogger.Infof("got sync range %s, source is %s and destination is %s", syncRange, srcRange, curRange)
		roundLogger = roundLogger.With("syncRange", syncRange)

		// Copy slots from position
		err = doCopy(roundCtx, src, dst, number.NewSet(syncRange))

		if err == nil {
			// Copy succeed
			roundLogger.Info("sync succeed")
			curRange = number.NewRange(curRange.L(), syncRange.R())
			if config.DstTargetLen > 0 && curRange.Size() > config.DstTargetLen {
				// need to cut head
				targetRangeLeft := curRange.R() + 1 - number.Number(config.DstTargetLen)
				targetRangeLeft = targetRangeLeft / config.DstLeftAlign * config.DstLeftAlign
				targetRange := number.NewRange(targetRangeLeft, curRange.R())
				cutRange := curRange.Sub(targetRange).GetFirstRange()
				if !cutRange.IsEmpty() {
					roundLogger.Infof("destination now is %s, will delete %s and keep %s", curRange, cutRange, targetRange)
					if err = dst.Delete(roundCtx, cutRange); err != nil {
						roundLogger.Warnfe(err, "delete %s failed", cutRange)
					}
				}
			}
			continue
		}

		roundLogger.Warnfe(err, "sync failed")
		if !errors.Is(err, ErrLink) {
			continue
		}

		// Failed because link error, try to trace the fork point and clean data in dst
		var forkStart number.Number
		forkStart, err = traceForkPoint(roundCtx, src, dst, syncRange.L()-1)
		if err != nil {
			roundLogger.Warne(err, "trace fork point failed")
			continue
		}
		roundLogger.Warnf("detected fork from %d", forkStart)
		if err = dst.Delete(roundCtx, number.NewRightUnlimitedRange(forkStart)); err != nil {
			roundLogger.Warnfe(err, "delete slots from %d failed", forkStart)
		}
	}
}
