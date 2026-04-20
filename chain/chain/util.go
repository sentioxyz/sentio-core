package chain

import (
	"context"
	"errors"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio/chain/slot"
	"sentioxyz/sentio/common/number"
)

func Load[SLOT slot.Slot](loader SlotLoader[SLOT], ctx context.Context, interval number.Range) ([]SLOT, error) {
	done := make(chan struct{})
	ch := make(chan SLOT, 1024)
	var result []SLOT
	go func() {
		defer close(done)
		for st := range ch {
			result = append(result, st)
		}
	}()
	err := loader.Load(ctx, interval, ch)
	close(ch)
	<-done
	if err != nil {
		return nil, err
	}
	return result, nil
}

// traceForkPoint return the slot number on the leftmost with different hash
func traceForkPoint[SLOT slot.Slot](
	ctx context.Context,
	src, dst Dimension[SLOT],
	position number.Number,
) (number.Number, error) {
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
		logger.Debugf("tracing fork point, source is %s and destination is %s", slot.Summary(sb), slot.Summary(db))
		if sb.GetHash() == db.GetHash() {
			return p + 1, nil
		}
		if p == 0 || sb.GetParentHash() == db.GetParentHash() {
			return p, nil
		}
	}
}

func GetSlotByChecker[SLOT slot.Slot](
	ctx context.Context,
	slotCache LatestSlotCache[SLOT],
	checker func(ctx context.Context, st SLOT) (bool, error),
) (SLOT, bool, error) {
	var errFound = errors.New("found")
	var result SLOT
	_, err := slotCache.Traverse(ctx, number.NewFullRange(), func(ctx context.Context, st SLOT) error {
		if ok, err := checker(ctx, st); err != nil {
			return err
		} else if ok {
			result = st
			return errFound
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errFound) {
			return result, true, nil
		}
		return result, false, err
	}
	return result, false, nil
}

func GetSlotByHash[SLOT slot.Slot](ctx context.Context, slotCache LatestSlotCache[SLOT], hash string) (SLOT, bool, error) {
	return GetSlotByChecker(ctx, slotCache, func(ctx context.Context, st SLOT) (bool, error) {
		return st.GetHash() == hash, nil
	})
}

func QueryRangeWithCacheV2[SLOT slot.Slot, ELEM interface{}](
	ctx context.Context,
	interval number.Range,
	slotCache LatestSlotCache[SLOT],
	cachedBlockProcessor func(slot SLOT) ([]ELEM, error),
	queryResultLoader func(ctx context.Context, queryRange number.Range) (results []ELEM, err error),
) ([]ELEM, error) {
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

	queryRange := interval.Sub(cachedRange).GetFirstRange()
	// If the L of first range already exceeds current head, no need to query.
	// Examples:
	//    Cached: [100..105], Query: [106], FirstRange: [106]
	//    Cached: [100..105], Query: [103..110], FirstRange: [106..110]
	//    Cached: [100..105], Query: [99..110], FirstRange: [99..99].  [106..110] is also ignored.
	if queryRange.IsEmpty() || (!cachedRange.IsEmpty() && queryRange.L() > cachedRange.R()) {
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

func WaitSlot(ctx context.Context, rangeGetter func(context.Context) (number.Range, error), sn number.Number) error {
	for {
		r, err := rangeGetter(ctx)
		if err != nil {
			return err
		}
		if r.ContainsNumber(sn) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second * 3):
		}
	}
}
