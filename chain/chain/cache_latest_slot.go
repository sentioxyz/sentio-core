package chain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/timehist"
	"sentioxyz/sentio-core/common/timer"
	"sentioxyz/sentio-core/common/utils"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type StdLatestSlotCache[SLOT Slot] struct {
	name    string
	network string

	cacheBlockTimeLenMax time.Duration
	cacheBlockTimeLenMin time.Duration
	nodeClient           clientpool.Shell
	persistent           Dimension[SLOT]
	l2Cache              Dimension[SLOT]
	l2CacheDumpInterval  time.Duration

	lock     sync.RWMutex
	memCache map[uint64]SLOT
	curRange rg.Range
	ready    bool

	blockWaiter *concurrency.StatusWaiter[uint64]

	statLock           sync.Mutex
	loadExtUsed        timehist.Histogram
	loadExtFailed      uint64
	loadExtReorg       uint64
	repairUsed         timehist.Histogram
	repairOK           uint64
	repairFailed       uint64
	lastRepairErr      error
	l2CacheDumpUsed    timehist.Histogram
	l2CacheDumpFailed  uint64
	l2CacheLastDumpAt  time.Time
	l2CacheLastDumpErr error

	growthUsed   metric.Int64Histogram
	growthMargin metric.Int64Gauge
}

func NewStdLatestSlotCache[SLOT Slot](
	name string,
	network string,
	cacheBlockTimeLenMax time.Duration,
	cacheBlockTimeLenMin time.Duration,
	nodeClient clientpool.Shell,
	persistent Dimension[SLOT],
	l2Cache Dimension[SLOT],
	l2CacheDumpInterval time.Duration,
	growthUsed metric.Int64Histogram,
	growthMargin metric.Int64Gauge,
) *StdLatestSlotCache[SLOT] {
	if cacheBlockTimeLenMin > cacheBlockTimeLenMax {
		panic(errors.Errorf("cacheBlockTimeLenMin should not greater than cacheBlockTimeLenMax"))
	}
	return &StdLatestSlotCache[SLOT]{
		name:                 name,
		network:              network,
		cacheBlockTimeLenMax: cacheBlockTimeLenMax,
		cacheBlockTimeLenMin: cacheBlockTimeLenMin,
		nodeClient:           nodeClient,
		persistent:           persistent,
		l2Cache:              l2Cache,
		l2CacheDumpInterval:  l2CacheDumpInterval,
		memCache:             make(map[uint64]SLOT),
		curRange:             rg.EmptyRange,
		blockWaiter:          concurrency.NewStatusWaiter[uint64](0),
		growthUsed:           growthUsed,
		growthMargin:         growthMargin,
	}
}

var ErrNotReady = errors.New("cache not ready")

func (c *StdLatestSlotCache[SLOT]) GetRange(ctx context.Context) (rg.Range, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if !c.ready {
		return rg.Range{}, ErrNotReady
	}
	return c.curRange, nil
}

func (c *StdLatestSlotCache[SLOT]) Wait(ctx context.Context, latestGt uint64) (latest uint64, err error) {
	var got bool
	c.lock.RLock()
	if c.ready && *c.curRange.End > latestGt {
		got, latest = true, *c.curRange.End
	}
	c.lock.RUnlock()
	if got {
		return latest, nil
	}
	return c.blockWaiter.Wait(ctx, func(bn uint64) bool {
		return bn > latestGt
	})
}

func (c *StdLatestSlotCache[SLOT]) Traverse(
	ctx context.Context,
	interval rg.Range,
	fn func(ctx context.Context, st SLOT) error,
) (rg.Range, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if !c.ready {
		return rg.Range{}, ErrNotReady
	}
	interval = interval.Intersection(c.curRange)
	for sn := interval.Start; sn <= *interval.End; sn++ {
		if err := fn(ctx, c.memCache[sn]); err != nil {
			return c.curRange, err
		}
	}
	return c.curRange, nil
}

func (c *StdLatestSlotCache[SLOT]) GetByNumber(ctx context.Context, sn uint64) (SLOT, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	var st SLOT
	if !c.ready {
		return st, ErrNotReady
	}
	if !c.curRange.Contains(sn) {
		return st, ErrSlotNotFound
	}
	return c.memCache[sn], nil
}

func (c *StdLatestSlotCache[SLOT]) GetByChecker(ctx context.Context, checker func(SLOT) bool) (SLOT, error) {
	var errFound = errors.New("found")
	var result SLOT
	_, err := c.Traverse(ctx, rg.Range{}, func(ctx context.Context, st SLOT) error {
		if checker(st) {
			result = st
			return errFound
		}
		return nil
	})
	if err == nil {
		return result, ErrSlotNotFound
	}
	if errors.Is(err, errFound) {
		return result, nil
	}
	return result, err
}

func (c *StdLatestSlotCache[SLOT]) GetByHash(ctx context.Context, hash string) (SLOT, error) {
	return c.GetByChecker(ctx, func(st SLOT) bool {
		return st.GetHash() == hash
	})
}

func (c *StdLatestSlotCache[SLOT]) tryLoadL2Cache(
	ctx context.Context,
	extRange rg.Range,
	memCacheSize uint64,
) rg.Range {
	c.lock.RLock()
	curRange := c.curRange
	c.lock.RUnlock()
	if c.l2Cache == nil || !curRange.IsEmpty() || extRange.IsEmpty() {
		return curRange
	}

	startAt := time.Now()
	expRange := rg.NewRangeByEndAndSize(*extRange.End, memCacheSize)
	_, logger := log.FromContext(ctx, "extRange", extRange.String(), "expRange", expRange.String())

	// load range
	hasRange, err := c.l2Cache.GetRange(ctx)
	if err != nil {
		logger.Errorfe(err, "get range from l2cache failed, cached slots will be empty")
		return curRange
	}
	// load slots in curRange
	loadRange := hasRange.Intersection(expRange)
	logger = logger.With("hasRange", hasRange.String(), "loadRange", loadRange.String())
	var slots []SLOT
	if slots, err = Load(c.l2Cache, ctx, loadRange); err != nil {
		logger.Errorfe(err, "load slots from l2cache failed, cached slots will be empty")
		return curRange
	}
	logger.Infow("loaded slots to mem", "used", time.Since(startAt).String())

	c.lock.Lock()
	defer c.lock.Unlock()
	c.curRange = loadRange
	c.memCache = make(map[uint64]SLOT)
	for _, st := range slots {
		c.memCache[st.GetNumber()] = st
	}
	return c.curRange
}

func (c *StdLatestSlotCache[SLOT]) loadFromPersistent(
	ctx context.Context,
	curRange rg.Range,
	extRange rg.Range,
	maxMemCacheSize uint64,
	minMemCacheSize uint64,
) (newRange rg.Range, loaded []SLOT, reorg bool, err error) {
	_, logger := log.FromContext(ctx)
	newRange = curRange
	if !curRange.IsEmpty() && *curRange.End == *extRange.End {
		logger.Debugf("will not growth because latest still %d", *curRange.End)
		return
	}
	if !curRange.IsEmpty() && *curRange.End > *extRange.End {
		logger.Warnf("ignored reverse growth %d => %d", *curRange.End, *extRange.End)
		return
	}

	// will growth, first calculate the new range
	if curRange.IsEmpty() {
		newRange = rg.NewRangeByEndAndSize(*extRange.End, minMemCacheSize).Intersection(extRange)
	} else {
		newRange = rg.NewRangeByEndAndSize(*extRange.End, maxMemCacheSize).Intersection(extRange)
		newRange.Start = max(newRange.Start, curRange.Start)
	}
	logger = logger.With("extRange", extRange.String(), "curRange", curRange.String(), "newRange", newRange.String())
	logger.Debug("will load data for growth")

	// load data needed
	loadRange := newRange.Remove(curRange).Last()
	for round := 0; ; round++ {
		roundLogger := logger.With("loadRange", loadRange.String())
		roundLogger.Debug("will load")
		loadStartTime := time.Now()
		loaded, err = Load[SLOT](c.persistent, ctx, loadRange)
		used := time.Since(loadStartTime)
		c.statLock.Lock()
		c.loadExtUsed = c.loadExtUsed.Incr(used)
		if err != nil {
			c.loadExtFailed += 1
			c.statLock.Unlock()
			roundLogger.With("used", used.String()).
				Errore(err, "growth failed because load from persistent failed")
			err = errors.Wrapf(err, "load %s failed", loadRange)
			return
		}
		c.statLock.Unlock()
		roundLogger.Infow("loaded from persistent", "used", used.String())
		var tpl SLOT
		if !tpl.Linked() {
			return
		}
		checking := loaded
		if loadRange.Start > 0 && curRange.Contains(loadRange.Start-1) && newRange.Contains(loadRange.Start-1) {
			// data not in newRange will be abandon, data not in curRange is not exists,
			// so must both in newRange and curRange.
			// The read must hold the lock: the repair loop replaces memCache entries
			// concurrently (with an equal-hash slot, so either version links the same).
			c.lock.RLock()
			boundary := c.memCache[loadRange.Start-1]
			c.lock.RUnlock()
			checking = utils.Prepend(loaded, boundary)
		}
		err = CheckLinksMismatch(checking)
		if err == nil {
			return
		}
		c.statLock.Lock()
		c.loadExtReorg += 1
		c.statLock.Unlock()
		loadRange = loadRange.MoveLeftBorder(1 << round).Intersection(newRange)
		roundLogger.Warnfe(err, "detected link mismatch, will reload")
		reorg = true
	}
}

func (c *StdLatestSlotCache[SLOT]) growth(ctx context.Context, bi time.Duration) error {
	_, logger := log.FromContext(ctx)
	t := timer.NewTimer()
	start := t.Start("A")
	logger.Debug("start to growth")

	maxMemCacheSize := uint64(c.cacheBlockTimeLenMax/bi) + 1
	minMemCacheSize := uint64(c.cacheBlockTimeLenMin/bi) + 1

	readRangeStart := t.Start("LR")
	extRange, err := c.persistent.GetRange(ctx)
	if err != nil {
		logger.Errorfe(err, "growth failed because get range from persistent failed")
		return err
	}
	readRangeStart.End()

	loadStart := t.Start("LC")
	curRange := c.tryLoadL2Cache(ctx, extRange, maxMemCacheSize)
	loadStart.End()

	// load new data
	readStart := t.Start("LE")
	newRange, loaded, reorg, err := c.loadFromPersistent(ctx, curRange, extRange, maxMemCacheSize, minMemCacheSize)
	readStart.End()
	if err != nil {
		return err
	}

	// update cache
	c.lock.Lock()
	defer c.lock.Unlock()
	updateStart := t.Start("W")
	// set new slots
	for _, st := range loaded {
		c.memCache[st.GetNumber()] = st
	}
	// delete useless slots
	toDelRange := curRange.Remove(newRange).First()
	for sn := toDelRange.Start; sn <= *toDelRange.End; sn++ {
		delete(c.memCache, sn)
	}
	// update range
	c.curRange = newRange
	updateStart.End()
	used := start.End()
	// growth succeed, now is ready
	c.ready = true

	options := metric.WithAttributeSet(attribute.NewSet(
		attribute.String("name", c.name),
		attribute.String("network", c.network),
		attribute.Bool("reorg", reorg),
	))
	if c.growthUsed != nil {
		c.growthUsed.Record(ctx, used.Milliseconds(), options)
	}
	if c.growthMargin != nil && !curRange.IsEmpty() {
		c.growthMargin.Record(ctx, int64(*newRange.End-*curRange.End), options)
	}

	// notice the waiters
	c.blockWaiter.NewStatus(*newRange.End)

	logger = logger.With(
		"used", t.ReportDistribution("A", "LS,LR,LC,LE,W"),
		"extRange", extRange.String(),
		"curRange", curRange.String(),
		"newRange", newRange.String(),
		"maxMemCacheSize", maxMemCacheSize,
		"minMemCacheSize", minMemCacheSize)
	if curRange.IsEmpty() {
		logger.Infof("growth succeed")
	} else {
		logger.Infof("growth succeed and moved %d", *newRange.End-*curRange.End)
	}
	return nil
}

func (c *StdLatestSlotCache[SLOT]) dump(ctx context.Context) {
	startAt := time.Now()

	// get data to dump
	c.lock.RLock()
	if !c.ready {
		c.lock.RUnlock()
		return
	}
	slots := utils.GetMapValuesOrderByKey(c.memCache)
	curRange := c.curRange
	c.lock.RUnlock()
	if curRange.IsEmpty() {
		return
	}

	// dump data slots
	_, logger := log.FromContext(ctx, "curRange", curRange.String())
	logger.Debug("start to dump to l2cache")
	g, gctx := errgroup.WithContext(ctx)
	ch := make(chan SLOT)
	g.Go(func() error {
		defer close(ch)
		for _, st := range slots {
			select {
			case ch <- st:
			case <-gctx.Done():
				return gctx.Err()
			}
		}
		return nil
	})
	g.Go(func() error {
		return c.l2Cache.Save(gctx, curRange, ch)
	})
	err := g.Wait()
	used := time.Since(startAt)
	c.statLock.Lock()
	defer c.statLock.Unlock()
	c.l2CacheLastDumpAt = time.Now()
	c.l2CacheLastDumpErr = err
	c.l2CacheDumpUsed = c.l2CacheDumpUsed.Incr(used)
	if err != nil {
		c.l2CacheDumpFailed += 1
		logger.Errorfe(err, "dump to l2cache failed")
		return
	}
	logger.Infow("dump to l2cache succeed", "used", used.String())
}

// KeepGrowth is the only entrypoint that will update curRange; memCache entries may also be
// replaced (never inserted or deleted) by KeepRepair, always under the write lock
func (c *StdLatestSlotCache[SLOT]) KeepGrowth(ctx context.Context) error {
	if _, err := c.nodeClient.WaitBlockInterval(ctx); err != nil {
		return err // only because ctx canceled
	}
	for round := 0; ; round++ {
		latest, bi, _, _ := c.nodeClient.GetState()
		roundCtx, logger := log.FromContext(ctx, "round", round)
		if err := c.growth(roundCtx, bi); err != nil {
			logger.Errorfe(err, "growth failed")
		}
		if _, err := c.nodeClient.WaitBlock(ctx, latest.Number+1); err != nil {
			return err // only because ctx canceled
		}
	}
}

// repairRound scans the cached slots for degraded ones (non-empty Features()) and asks the
// repairer to rebuild them, patching repaired slots back into the cache. It stops at the
// first failed attempt and leaves the rest for the next round, so a still-broken upstream
// is probed by at most one request per round.
func (c *StdLatestSlotCache[SLOT]) repairRound(ctx context.Context, repairer SlotRepairer[SLOT]) {
	c.lock.RLock()
	if !c.ready || c.curRange.IsEmpty() {
		c.lock.RUnlock()
		return
	}
	var degraded []SLOT
	for sn := c.curRange.Start; sn <= *c.curRange.End; sn++ {
		if st, has := c.memCache[sn]; has && len(st.Features()) > 0 {
			degraded = append(degraded, st)
		}
	}
	c.lock.RUnlock()
	if len(degraded) == 0 {
		return
	}

	_, logger := log.FromContext(ctx, "degraded", len(degraded))
	var repairedCount int
	for _, st := range degraded {
		startAt := time.Now()
		fixed, ok, err := repairer.RepairSlot(ctx, st)
		used := time.Since(startAt)
		c.statLock.Lock()
		c.lastRepairErr = err
		if err != nil {
			c.repairFailed += 1
		} else if ok {
			c.repairUsed = c.repairUsed.Incr(used)
			c.repairOK += 1
		}
		c.statLock.Unlock()
		if err != nil {
			logger.Warnfe(err, "repair slot %d failed, leave the rest %d degraded slots to the next round",
				st.GetNumber(), len(degraded)-repairedCount)
			break
		}
		if !ok {
			continue
		}
		// patch back under the write lock, guarding against eviction or reorg replacement
		// that may have happened while the repair was running
		c.lock.Lock()
		if cur, has := c.memCache[fixed.GetNumber()]; has && cur.GetHash() == fixed.GetHash() {
			c.memCache[fixed.GetNumber()] = fixed
			repairedCount++
		}
		c.lock.Unlock()
	}
	if repairedCount > 0 {
		logger.Infow("repaired degraded slots in cache", "repaired", repairedCount)
	}
}

// KeepRepair periodically retries rebuilding degraded slots in the cache (e.g. evm slots that
// entered without trace data), so consumers needing the degraded part do not have to wait for
// the cache window to slide past them. No-op if the persistent dimension does not implement
// SlotRepairer.
func (c *StdLatestSlotCache[SLOT]) KeepRepair(ctx context.Context, interval time.Duration) error {
	repairer, ok := c.persistent.(SlotRepairer[SLOT])
	if !ok {
		return nil
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for round := 0; ; round++ {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
		roundCtx, _ := log.FromContext(ctx, "round", round)
		c.repairRound(roundCtx, repairer)
	}
}

func (c *StdLatestSlotCache[SLOT]) KeepDump(ctx context.Context) error {
	if c.l2Cache == nil {
		return nil
	}
	ticker := time.NewTicker(c.l2CacheDumpInterval)
	defer ticker.Stop()
	for round := 0; ; round++ {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
		roundCtx, _ := log.FromContext(ctx, "round", round)
		c.dump(roundCtx)
	}
}

func (c *StdLatestSlotCache[SLOT]) Snapshot() any {
	c.lock.RLock()
	defer c.lock.RUnlock()
	c.statLock.Lock()
	defer c.statLock.Unlock()

	featureRanges := make(map[string]rg.RangeSet)
	if c.curRange.End != nil {
		for n := c.curRange.Start; n <= *c.curRange.End; n++ {
			if slot, ok := c.memCache[n]; ok {
				for _, fea := range slot.Features() {
					rs, has := featureRanges[fea]
					if !has {
						rs = rg.EmptyRangeSet
					}
					featureRanges[fea] = rs.Union(rg.NewSingleRange(n))
				}
			}
		}
	}

	m := map[string]any{
		"name":                 c.name,
		"network":              c.network,
		"cacheBlockTimeLenMax": c.cacheBlockTimeLenMax.String(),
		"cacheBlockTimeLenMin": c.cacheBlockTimeLenMin.String(),
		"memCache": map[string]any{
			"ready":    c.ready,
			"len":      len(c.memCache),
			"range":    c.curRange.String(),
			"features": utils.MapMapNoError(featureRanges, rg.RangeSet.String),
		},
		"loadExternal": map[string]any{
			"used":       c.loadExtUsed.String(),
			"count":      c.loadExtUsed.Sum(),
			"failed":     c.loadExtFailed,
			"reorgCount": c.loadExtReorg,
		},
	}
	if _, isRepairer := c.persistent.(SlotRepairer[SLOT]); isRepairer {
		m["repair"] = map[string]any{
			"used":    c.repairUsed.String(),
			"ok":      c.repairOK,
			"failed":  c.repairFailed,
			"lastErr": fmt.Sprintf("%+v", c.lastRepairErr),
		}
	}
	if c.l2Cache != nil {
		m["l2cache"] = map[string]any{
			"dumpInterval": c.l2CacheDumpInterval.String(),
			"dumpUsed":     c.l2CacheDumpUsed.String(),
			"dumpCount":    c.l2CacheDumpUsed.Sum(),
			"dumpFailed":   c.l2CacheDumpFailed,
			"lastDumpAt":   c.l2CacheLastDumpAt.String(),
			"lastDumpErr":  fmt.Sprintf("%+v", c.l2CacheLastDumpErr),
		}
	}
	return m
}
