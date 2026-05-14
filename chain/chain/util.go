package chain

import (
	"context"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

func Load[SLOT Slot](loader SlotLoader[SLOT], ctx context.Context, interval rg.Range) (result []SLOT, err error) {
	_, logger := log.FromContext(ctx)
	ch := make(chan SLOT)
	go func() {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				logger.Errorf("panic when load slots: %v", panicErr)
				err = errors.Errorf("panic when load slots: %v", panicErr)
			}
			close(ch)
		}()
		err = loader.Load(ctx, interval, ch)
	}()
	for st := range ch {
		result = append(result, st)
	}
	return
}

// traceForkPoint return the slot number on the leftmost with different hash
func traceForkPoint[SLOT Slot](
	ctx context.Context,
	src, dst Dimension[SLOT],
	position uint64,
) (uint64, error) {
	ctx, logger := log.FromContext(ctx, "position", position)
	logger.Debug("trace fork point begin")
	for p := position; ; p-- {
		db, err := dst.LoadHeader(ctx, p)
		if err != nil && !errors.Is(err, ErrSlotNotFound) {
			return 0, err
		}
		sb, err := src.LoadHeader(ctx, p)
		if err != nil && !errors.Is(err, ErrSlotNotFound) {
			return 0, err
		}
		if sb == nil || db == nil {
			return p, nil
		}
		logger.Debugf("tracing fork point, source is %s and destination is %s", SlotSummary(sb), SlotSummary(db))
		if sb.GetHash() == db.GetHash() {
			return p + 1, nil
		}
		if p == 0 || sb.GetParentHash() == db.GetParentHash() {
			return p, nil
		}
	}
}

func QueryRangeWithCache[SLOT Slot, ELEM any](
	ctx context.Context,
	interval rg.Range,
	slotCache LatestSlotCache[SLOT],
	cachedBlockProcessor func(slot SLOT) ([]ELEM, error),
	queryResultLoader func(ctx context.Context, queryRange rg.Range) (results []ELEM, err error),
) ([]ELEM, error) {
	if interval.IsEmpty() {
		return nil, nil
	}
	var cached []ELEM
	_, logger := log.FromContext(ctx)
	start := time.Now()
	cachedRange, err := slotCache.Traverse(ctx, interval, func(ctx context.Context, st SLOT) error {
		elems, err := cachedBlockProcessor(st)
		if err != nil {
			return err
		}
		cached = append(cached, elems...)
		return nil
	})
	logger.Debugf("traverse cache used %s", time.Since(start).String())
	if err != nil {
		return nil, err
	}

	queryRange := interval.Remove(cachedRange).First()
	// If the Start of first range already exceeds current head, no need to query.
	// Examples:
	//    Cached: [100..105], Query: [106], FirstRange: [106]
	//    Cached: [100..105], Query: [103..110], FirstRange: [106..110]
	//    Cached: [100..105], Query: [99..110], FirstRange: [99..99].  [106..110] is also ignored.
	if queryRange.IsEmpty() || (!cachedRange.IsEmpty() && queryRange.Start > *cachedRange.End) {
		return cached, nil
	}

	start = time.Now()
	queried, err := queryResultLoader(ctx, queryRange)
	logger.Debugf("queryResultLoader used %s", time.Since(start).String())
	if err != nil {
		return nil, err
	}
	return utils.MergeArr(queried, cached), nil
}

func CheckRange[ELEM any](
	rangeStore RangeStore,
	do func(context.Context, rg.Range) ([]ELEM, error),
) func(context.Context, rg.Range) ([]ELEM, error) {
	return func(ctx context.Context, queryRange rg.Range) ([]ELEM, error) {
		r, err := rangeStore.Get(ctx)
		if err != nil {
			return nil, err
		}
		if !r.Include(queryRange) {
			return nil, errors.Errorf("request range %s not in scope of range store %s", queryRange, r)
		}
		return do(ctx, queryRange)
	}
}
