package supernode

import (
	"context"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"math"
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

func splitRange[ELEM any](
	ctx context.Context,
	slotCache chain.LatestSlotCache[*aptos.Slot],
	interval rg.Range,
	cachedProcessor func(slot *aptos.Slot, tx *api.CommittedTransaction) ([]ELEM, error),
	uncachedLoader func(ctx context.Context, queryRange rg.Range) (results []ELEM, err error),
) ([]ELEM, error) {
	if interval.IsEmpty() {
		return nil, nil
	}
	var cached []ELEM
	_, logger := log.FromContext(ctx)
	start := time.Now()
	var cachedVersionLeft, cachedVersionRight uint64 = math.MaxUint64, 0
	_, err := slotCache.Traverse(ctx, rg.Range{}, func(ctx context.Context, st *aptos.Slot) error {
		cachedVersionLeft = min(cachedVersionLeft, st.FirstVersion)
		cachedVersionRight = max(cachedVersionRight, st.LastVersion)
		if interval.Intersection(rg.NewRange(st.FirstVersion, st.LastVersion)).IsEmpty() {
			return nil
		}
		for _, tx := range st.Transactions {
			if !interval.Contains(tx.Version()) {
				continue
			}
			elems, err := cachedProcessor(st, tx)
			if err != nil {
				return err
			}
			cached = append(cached, elems...)
		}
		return nil
	})
	logger.Debugf("traverse cache used %s", time.Since(start).String())
	if err != nil {
		return nil, err
	}
	// slotCache always non-empty, so here cachedRange always non-empty
	cachedRange := rg.NewRange(cachedVersionLeft, cachedVersionRight)

	queryRange := interval.Remove(cachedRange).First()
	// If the Start of first range already exceeds current head, no need to query.
	// Examples:
	//    Cached: [100..105], Query: [106], FirstRange: [106]
	//    Cached: [100..105], Query: [103..110], FirstRange: [106..110]
	//    Cached: [100..105], Query: [99..110], FirstRange: [99..99].  [106..110] is also ignored.
	if queryRange.IsEmpty() || (!cachedRange.IsEmpty() && queryRange.Start > *cachedRange.End) {
		return cached, nil
	}

	// load uncached data
	start = time.Now()
	queried, err := uncachedLoader(ctx, queryRange)
	logger.Debugf("queryResultLoader used %s", time.Since(start).String())
	if err != nil {
		return nil, err
	}
	return utils.MergeArr(queried, cached), nil
}
