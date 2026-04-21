package chain

import (
	"context"
	"fmt"

	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
)

type SimpleDimension[SLOT Slot] struct {
	RangeStore
	SimpleSlotStore[SLOT]
}

func NewSimpleDimension[SLOT Slot](
	rangeStore RangeStore,
	slotStore SimpleSlotStore[SLOT],
) *SimpleDimension[SLOT] {
	return &SimpleDimension[SLOT]{
		RangeStore:      rangeStore,
		SimpleSlotStore: slotStore,
	}
}

func (d *SimpleDimension[SLOT]) Init(ctx context.Context) error {
	cur, err := d.RangeStore.Get(ctx)
	if err != nil {
		return fmt.Errorf("get current range failed: %w", err)
	}
	var start uint64
	if !cur.IsEmpty() {
		start = *cur.End + 1
	}
	if err = d.SimpleSlotStore.Delete(ctx, rg.Range{Start: start}); err != nil {
		return fmt.Errorf("clean datas in the right of current range %s failed: %w", cur, err)
	}
	return nil
}

func (d *SimpleDimension[SLOT]) GetRange(ctx context.Context) (rg.Range, error) {
	return d.RangeStore.Get(ctx)
}

func (d *SimpleDimension[SLOT]) Wait(ctx context.Context, sn uint64) error {
	return WaitSlot(ctx, d.RangeStore.Get, sn)
}

func (d *SimpleDimension[SLOT]) Save(ctx context.Context, interval rg.Range, slotChan <-chan SLOT) error {
	_, logger := log.FromContext(ctx, "saveRange", interval.String())
	curRange, err := d.RangeStore.Get(ctx)
	if err != nil {
		return fmt.Errorf("get current range failed: %w", err)
	}

	if !curRange.IsEmpty() && !interval.IsEmpty() && curRange.GetDistance(interval) > 0 {
		logger.Warnfe(ErrDiscontinuous, "save failed: curRange %s is far away from target %s", curRange, interval)
		return ErrDiscontinuous
	}

	savedChan := make(chan rg.Range, 1000)
	doneGroup, doneCtx := errgroup.WithContext(ctx)

	// slot input checkers
	var inCheckers []func(int, SLOT) error

	// check all slot from slotChan are continuous
	inCheckers = append(inCheckers, func(index int, st SLOT) error {
		if !interval.Contains(st.GetNumber()) {
			err := fmt.Errorf("%w: slot number %d out of range", ErrDiscontinuous, st.GetNumber())
			logger.Warnfe(err, "save %s failed", curRange.String())
			return err
		}
		if expNumber := interval.Start + uint64(index); st.GetNumber() != expNumber {
			err := fmt.Errorf("%w: next slot should be %d, not %d", ErrDiscontinuous, expNumber, st.GetNumber())
			logger.Warnfe(err, "save %s failed", curRange.String())
			return err
		}
		return nil
	})

	var slotTpl SLOT
	if slotTpl.Linked() {
		// check slot link
		var leftPoint Slot
		if 0 < interval.Start && curRange.Contains(interval.Start-1) {
			if leftPoint, err = d.LoadHeader(ctx, interval.Start-1); err != nil {
				logger.Warnfe(err, "save failed: get right link point slot at %d failed", interval.Start-1)
				return err
			}
			logger.Debugf("get right link point slot: %s", SlotSummary(leftPoint))
		}
		inCheckers = append(inCheckers, func(index int, st SLOT) error {
			if leftPoint != nil && CheckLinkMismatch(leftPoint, st) {
				logger.Warnfe(ErrLink, "detected link mismatch, exist %s but new is %s", SlotSummary(leftPoint), SlotSummary(st))
				return ErrLink
			}
			leftPoint = st
			return nil
		})
	}

	slotChan = concurrency.InCheck(doneGroup, doneCtx, slotChan, inCheckers...)

	doneGroup.Go(func() error {
		defer close(savedChan)
		return d.SimpleSlotStore.Save(doneCtx, interval, slotChan, savedChan)
	})
	doneGroup.Go(func() error {
		defer logger.Debugf("range updater ended")
		var completed rg.RangeSet
		seed := curRange
		if curRange.IsEmpty() {
			seed = rg.NewSingleRange(interval.Start)
		}
		for doneRange := range savedChan {
			completed = completed.Union(doneRange)
			afterRange, _ := completed.Union(curRange).FindContains(seed)
			logger.With(
				"doneRange", doneRange.String(),
				"count", completed.Size(),
				"completed", completed.String(),
			).Infof("saved slots increased")
			if !curRange.Equal(afterRange) {
				var newRange rg.Range
				newRange, err = d.RangeStore.Update(doneCtx, afterRange.Cover)
				if err != nil {
					logger.Warne(err, "update range failed")
					return err
				} else {
					logger.Infof("range updated, %s=>%s", curRange, newRange)
					curRange = newRange
				}
			}
		}
		return nil
	})
	if err = doneGroup.Wait(); err != nil {
		logger.Warne(err, "save failed")
		return err
	}
	logger.Info("save succeed")
	return nil
}

func (d *SimpleDimension[SLOT]) Delete(ctx context.Context, targetRange rg.Range) error {
	ctx, logger := log.FromContext(ctx)
	after, err := d.RangeStore.Update(ctx, func(curRange rg.Range) rg.Range {
		return curRange.Remove(targetRange).First()
	})
	if err != nil {
		logger.Warne(err, "delete %s failed: update range failed", targetRange)
		return err
	}
	if err = d.SimpleSlotStore.Delete(ctx, targetRange); err != nil {
		logger.Warne(err, "delete %s failed: clean in simple slot store failed", targetRange)
		return err
	}
	logger.Infof("delete %s succeed, curRange is %s", targetRange, after)
	return nil
}
