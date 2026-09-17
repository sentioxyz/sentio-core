package chain

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/log"
)

// duplicateCheck drives the periodic self-check of Sync: every DupCheckInterval it asks the
// destination to look itself over for rows that share a unique key. The destination picks the
// window, which is the stretch of slots its range store still retains, so consecutive checks
// overlap and there is no progress to remember between them or across a restart. The scan runs in
// its own goroutine and never blocks the sync loop; one still in flight makes the next tick skip.
type duplicateCheck struct {
	checker  DuplicateChecker
	interval time.Duration

	mu        sync.Mutex
	running   bool
	disabled  bool
	lastStart time.Time
}

func newDuplicateCheck[SLOT Slot](dst Dimension[SLOT], config SyncConfig) *duplicateCheck {
	if config.DupCheckInterval <= 0 {
		return nil
	}
	checker, ok := dst.(DuplicateChecker)
	if !ok {
		return nil
	}
	return &duplicateCheck{checker: checker, interval: config.DupCheckInterval}
}

// maybeStart runs a check if one is due and none is in flight.
func (c *duplicateCheck) maybeStart(ctx context.Context) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.disabled || c.running {
		return
	}
	if now := time.Now(); c.lastStart.IsZero() || now.Sub(c.lastStart) >= c.interval {
		c.lastStart = now
		c.running = true
		go c.run(ctx)
	}
}

func (c *duplicateCheck) run(ctx context.Context) {
	ctx, logger := log.FromContext(ctx)
	startAt := time.Now()
	window, reports, err := c.checker.CheckDuplicates(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	switch {
	case errors.Is(err, ErrDuplicateCheckUnsupported):
		c.disabled = true
		logger.Info("duplicate check disabled, destination does not support it")
	case err != nil:
		if ctx.Err() == nil {
			logger.Warnfe(err, "duplicate check failed, the window will be checked again next time")
		}
	default:
		logger = logger.With("window", window.String())
		for _, r := range reports {
			logger.Errorw("detected duplicate rows",
				"table", r.Table,
				"dupGroups", r.Groups,
				"extraRows", r.ExtraRows,
				"firstSlot", r.First,
				"lastSlot", r.Last)
		}
		logger.Infow("duplicate check finished",
			"tablesWithDuplicates", len(reports), "used", time.Since(startAt).String())
	}
}
