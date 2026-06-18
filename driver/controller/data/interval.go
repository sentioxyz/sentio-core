package data

import (
	"context"
	"fmt"
	"sort"
	"time"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/common/window"
	"sentioxyz/sentio-core/driver/controller"
)

type BlockInterval struct {
	Backfill uint64
	Watching uint64
}

type TimeInterval struct {
	Backfill time.Duration
	Watching time.Duration
}

type IntervalConfig struct {
	BlockInterval *BlockInterval
	TimeInterval  *TimeInterval
}

func (c IntervalConfig) String() string {
	if c.BlockInterval != nil {
		if c.BlockInterval.Backfill == c.BlockInterval.Watching {
			return fmt.Sprintf("Per:%d", c.BlockInterval.Watching)
		} else {
			return fmt.Sprintf("Backfill:%d,Watching:%d", c.BlockInterval.Backfill, c.BlockInterval.Watching)
		}
	}
	if c.TimeInterval != nil {
		if c.TimeInterval.Backfill == c.TimeInterval.Watching {
			return fmt.Sprintf("Per:%s", c.TimeInterval.Watching)
		} else {
			return fmt.Sprintf("Backfill:%s,Watching:%s", c.TimeInterval.Backfill, c.TimeInterval.Watching)
		}
	}
	return "empty"
}

// watchingDiffers reports whether the finer watching window differs from the backfill window, i.e.
// whether the watching pass can produce points the backfill pass does not.
func (c IntervalConfig) watchingDiffers() bool {
	if c.BlockInterval != nil {
		return c.BlockInterval.Watching != c.BlockInterval.Backfill
	}
	if c.TimeInterval != nil {
		return c.TimeInterval.Watching != c.TimeInterval.Backfill
	}
	return false
}

func (c IntervalConfig) Equal(a IntervalConfig) bool {
	if c.BlockInterval != nil {
		return a.BlockInterval != nil &&
			a.BlockInterval.Backfill == c.BlockInterval.Backfill &&
			a.BlockInterval.Watching == c.BlockInterval.Watching
	}
	if c.TimeInterval != nil {
		return a.TimeInterval != nil &&
			a.TimeInterval.Backfill == c.TimeInterval.Backfill &&
			a.TimeInterval.Watching == c.TimeInterval.Watching
	}
	return false
}

func ContainsInterval(list []IntervalConfig, target IntervalConfig) bool {
	for _, item := range list {
		if item.Equal(target) {
			return true
		}
	}
	return false
}

type IntervalRequirement struct {
	controller.BlockRange
	IntervalConfig
}

func (r IntervalRequirement) String() string {
	return fmt.Sprintf("IntervalRequirement[%s]%s", r.IntervalConfig.String(), r.BlockRange.String())
}

func (r IntervalRequirement) Snapshot() any {
	return map[string]any{
		"config": r.IntervalConfig.String(),
		"range":  r.BlockRange.String(),
	}
}

// MergeIntervalRequirements Will remove the requirement of being fully included
func MergeIntervalRequirements(reqs []IntervalRequirement) (result []IntervalRequirement) {
	index := func(r IntervalRequirement) int {
		if r.BlockInterval != nil {
			return 0
		}
		return 1
	}
	// It will ensure that all block intervals are in front of the time interval
	sort.Slice(reqs, func(i, j int) bool {
		if ix, jx := index(reqs[i]), index(reqs[j]); ix != jx {
			return ix < jx
		}
		if reqs[i].StartBlock != reqs[j].StartBlock {
			return reqs[i].StartBlock < reqs[j].StartBlock
		}
		return reqs[i].BlockRange.Include(reqs[j].BlockRange)
	})
	for i, req := range reqs {
		var included bool
		for j := i - 1; j >= 0 && index(req) == index(reqs[j]); j-- {
			pre := reqs[j]
			if pre.BlockRange.Include(req.BlockRange) && pre.IntervalConfig.Equal(req.IntervalConfig) {
				included = true
				break
			}
		}
		if !included {
			result = append(result, req)
		}
	}
	return
}

func QueryInterval(
	ctx context.Context,
	start, end uint64,
	first uint64,
	latest controller.BlockHeader,
	req IntervalRequirement,
	timeGetter func(ctx context.Context, blockNumber uint64) (time.Time, error),
) ([]uint64, error) {
	itv := req.IntervalConfig
	// inWatching is only used when the watching window differs from the backfill window; when they
	// are equal the watching scan/points just duplicate the backfill ones, so there is no need to
	// even fetch end's block time to decide it.
	var inWatching bool
	if itv.watchingDiffers() {
		endTime, err := timeGetter(ctx, end)
		if err != nil {
			return nil, err
		}
		inWatching = latest.GetBlockTime().Sub(endTime) < controller.WatchingDelay
	}
	var blockNumbers []uint64
	if itv.BlockInterval != nil {
		for n := start; n <= end; n++ {
			if n%itv.BlockInterval.Backfill == 0 || (inWatching && n%itv.BlockInterval.Watching == 0) {
				blockNumbers = append(blockNumbers, n)
			}
		}
	} else if itv.TimeInterval != nil {
		bns, err := window.FindStartPoints[int64](
			ctx, int64(start), int64(end), 0,
			func(ctx context.Context, n int64) (time.Time, error) {
				// start may equal to first and window.FindStartPoints will query the time of start-1,
				// so here need to check whether n is less than first
				if n < int64(first) {
					return time.Time{}, nil
				}
				t, getErr := timeGetter(ctx, uint64(n))
				if getErr != nil {
					return time.Time{}, getErr
				}
				return t.Truncate(itv.TimeInterval.Backfill), nil
			})
		if err != nil {
			return nil, err
		}
		ns := utils.BuildSet(bns)
		// inWatching is already false when the watching window equals the backfill window, so this
		// extra scan only runs when it can add new points.
		if inWatching {
			bns, err = window.FindStartPoints[int64](
				ctx, int64(start), int64(end), 0,
				func(ctx context.Context, n int64) (time.Time, error) {
					t, getErr := timeGetter(ctx, uint64(n))
					if getErr != nil {
						return time.Time{}, getErr
					}
					return t.Truncate(itv.TimeInterval.Watching), nil
				})
			if err != nil {
				return nil, err
			}
			utils.MergeInto(ns, bns)
		}
		for n := range ns {
			blockNumbers = append(blockNumbers, uint64(n))
		}
	}
	return blockNumbers, nil
}
