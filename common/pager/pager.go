// Package pager walks an integer range in pages whose size adapts to keep the amount of work
// (records) produced per page near a target. It is useful when the work density across the range
// is uneven: a fixed page size wastes per-page overhead on sparse stretches and risks oversized
// pages on dense ones. The page size is kept on a Step grid (so pages never become too
// fine-grained) and clamped to [Min, Max].
package pager

import "github.com/pkg/errors"

// Config controls adaptive page sizing over an integer range. Zero-valued fields are replaced with
// sensible defaults by normalize (Step=1, Min=Step, Max=Min, Initial=Min), but callers normally set
// all of them explicitly.
type Config struct {
	Target  uint64 // desired number of records per page (the value NextSize aims for)
	Min     uint64 // minimum page size (in range units)
	Max     uint64 // maximum page size (in range units)
	Step    uint64 // page size is always a multiple of Step
	Initial uint64 // initial page size guess, used before any density is observed
}

func (c Config) normalize() Config {
	if c.Step == 0 {
		c.Step = 1
	}
	if c.Min == 0 {
		c.Min = c.Step
	}
	if c.Max < c.Min {
		c.Max = c.Min
	}
	if c.Initial == 0 {
		c.Initial = c.Min
	}
	c.Min = c.snap(c.Min)
	c.Max = c.snap(c.Max)
	c.Initial = c.clamp(c.snap(c.Initial))
	return c
}

// snap rounds n to the nearest multiple of Step.
func (c Config) snap(n uint64) uint64 {
	return (n + c.Step/2) / c.Step * c.Step
}

func (c Config) clamp(n uint64) uint64 {
	return min(max(n, c.Min), c.Max)
}

// NextSize chooses the page size to use after observing that `span` range units produced `records`
// records: to land on Target it wants about span*Target/records units. The result is snapped to the
// Step grid and clamped to [Min, Max]. An empty page (records == 0) jumps straight to Max so sparse
// stretches are skipped quickly.
func (c Config) NextSize(span, records uint64) uint64 {
	c = c.normalize()
	var desired uint64
	if records == 0 || c.Target == 0 {
		desired = c.Max
	} else {
		desired = span * c.Target / records
	}
	return c.clamp(c.snap(desired))
}

// Walk traverses [from, to] inclusive in adaptive pages. process is invoked once per page with the
// inclusive [start, end] bounds and returns how many records that page produced (which drives the
// next page size) and a tooBig flag.
//
// When process reports tooBig the page is treated as not done: Walk halves the span and retries from
// the same start, deliberately ignoring the Step grid and Min floor so an unexpectedly dense region
// can always be bounded (the normal grid/Min sizing resumes once a page succeeds). process must not
// report tooBig for a single-unit span (start == end), since that cannot be split further; doing so
// returns an error rather than looping forever.
//
// Walk stops and returns the first error from process. An empty range (from > to) invokes process
// zero times and returns nil.
func Walk(from, to uint64, cfg Config, process func(start, end uint64) (records uint64, tooBig bool, err error)) error {
	cfg = cfg.normalize()
	size := cfg.Initial
	for cur := from; cur <= to; {
		end := min(cur+size-1, to)
		span := end - cur + 1
		records, tooBig, err := process(cur, end)
		if err != nil {
			return err
		}
		if tooBig {
			if span <= 1 {
				return errors.Errorf("pager: process reported tooBig for an unsplittable single-unit page at %d", cur)
			}
			size = max(1, span/2) // safety shrink: retry the same start with a smaller span
			continue
		}
		cur = end + 1
		size = cfg.NextSize(span, records)
	}
	return nil
}
