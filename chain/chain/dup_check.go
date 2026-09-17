package chain

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
)

// duplicateCheck drives the periodic DuplicateChecker run of Sync. Every DupCheckInterval it
// scans the slots synced since the previous run, so consecutive windows tile the destination
// range without overlap: a window always ends at a committed watermark and a save interval always
// starts right after one, so a single flush never straddles two windows. The check runs in its own
// goroutine and never blocks the sync loop; a run still in flight makes the next tick skip.
type duplicateCheck struct {
	checker  DuplicateChecker
	interval time.Duration
	lookback uint64

	mu        sync.Mutex
	running   bool
	disabled  bool
	lastStart time.Time
	next      uint64 // first slot of the next window, valid when hasNext; a failed run keeps it
	hasNext   bool
}

func newDuplicateCheck[SLOT Slot](dst Dimension[SLOT], config SyncConfig) *duplicateCheck {
	if config.DupCheckInterval <= 0 {
		return nil
	}
	checker, ok := dst.(DuplicateChecker)
	if !ok {
		return nil
	}
	return &duplicateCheck{
		checker:  checker,
		interval: config.DupCheckInterval,
		lookback: config.DupCheckLookback,
	}
}

// maybeStart is called after every successful sync round with the destination range cur. The
// first run only looks back DupCheckLookback slots from the watermark (0 = nothing: everything
// synced before startup is trusted), later runs cover (previous watermark, cur.End].
func (c *duplicateCheck) maybeStart(ctx context.Context, cur rg.Range) {
	if c == nil || cur.IsEmpty() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.disabled || c.running {
		return
	}
	now := time.Now()
	if !c.lastStart.IsZero() && now.Sub(c.lastStart) < c.interval {
		return
	}
	c.lastStart = now
	end := *cur.End
	if !c.hasNext {
		c.next = end + 1 - min(c.lookback, end-cur.Start+1) // lookback 0 gives an empty first window
		c.hasNext = true
	}
	if c.next > end {
		return
	}
	c.running = true
	go c.run(ctx, rg.NewRange(c.next, end))
}

// rewind moves the start of the next window back to from, so slots the sync is about to write
// again are checked again however far the check had already got. Without it the window start
// stays above a rolled back watermark and the rewritten slots are never scanned.
func (c *duplicateCheck) rewind(from uint64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.hasNext || from < c.next {
		c.next, c.hasNext = from, true
	}
}

func (c *duplicateCheck) run(ctx context.Context, interval rg.Range) {
	_, logger := log.FromContext(ctx, "dupCheckRange", interval.String())
	startAt := time.Now()
	reports, err := c.checker.CheckDuplicates(ctx, interval)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	switch {
	case errors.Is(err, ErrDuplicateCheckUnsupported):
		c.disabled = true
		logger.Info("duplicate check disabled, destination does not support it")
	case err != nil:
		// `next` stays at the window start so the next run covers it again
		if ctx.Err() == nil {
			logger.Warnfe(err, "duplicate check failed, the range will be checked again next time")
		}
	default:
		c.next = *interval.End + 1
		for _, r := range reports {
			logger.Errorw("detected duplicate rows",
				"table", r.Table,
				"dupGroups", r.Groups,
				"extraRows", r.ExtraRows,
				"firstSlot", r.First,
				"lastSlot", r.Last)
		}
		logger.Infow("duplicate check finished", "tablesWithDuplicates", len(reports), "used", time.Since(startAt).String())
	}
}
