package chain

import (
	"context"
	"errors"
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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type NodeClient interface {
	GetState() (latest clientpool.Block, blockInterval time.Duration, ready bool, psi uint64)
	WaitBlock(ctx context.Context, numberGE uint64) (clientpool.Block, error)
	WaitBlockInterval(ctx context.Context) (time.Duration, error)
}

type StdLatestSlotCache[SLOT Slot] struct {
	name    string
	network string

	cacheBlockTimeLen   time.Duration
	nodeClient          NodeClient
	persistent          Dimension[SLOT]
	l2Cache             Dimension[SLOT]
	l2CacheDumpInterval time.Duration

	lock     sync.RWMutex
	memCache map[uint64]SLOT
	curRange rg.Range
	ready    bool

	blockWaiter *concurrency.StatusWaiter[uint64]

	statLock           sync.Mutex
	loadExtUsed        timehist.Histogram
	loadExtFailed      uint64
	loadExtReorg       uint64
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
	cacheBlockTimeLen time.Duration,
	nodeClient NodeClient,
	persistent Dimension[SLOT],
	l2Cache Dimension[SLOT],
	l2CacheDumpInterval time.Duration,
	growthUsed metric.Int64Histogram,
	growthMargin metric.Int64Gauge,
) *StdLatestSlotCache[SLOT] {
	return &StdLatestSlotCache[SLOT]{
		name:                name,
		network:             network,
		cacheBlockTimeLen:   cacheBlockTimeLen,
		nodeClient:          nodeClient,
		persistent:          persistent,
		l2Cache:             l2Cache,
		l2CacheDumpInterval: l2CacheDumpInterval,
		memCache:            make(map[uint64]SLOT),
		curRange:            rg.EmptyRange,
		blockWaiter:         concurrency.NewStatusWaiter[uint64](0),
		growthUsed:          growthUsed,
		growthMargin:        growthMargin,
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
	if c.l2Cache == nil || !c.curRange.IsEmpty() || extRange.IsEmpty() {
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
	memCacheSize uint64,
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
	newRange = rg.NewRangeByEndAndSize(*extRange.End, memCacheSize).Intersection(extRange)
	if !curRange.IsEmpty() {
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
			err = fmt.Errorf("load %s failed: %w", loadRange, err)
			return
		}
		c.statLock.Unlock()
		roundLogger.Infow("loaded from persistent", "used", used.String())
		var tpl SLOT
		if !tpl.Linked() {
			return
		}
		checking := loaded
		if curRange.Contains(loadRange.Start-1) && newRange.Contains(loadRange.Start-1) {
			// data not in newRange will be abandon, data not in curRange is not exists,
			// so must both in newRange and curRange.
			checking = utils.Prepend(loaded, c.memCache[loadRange.Start-1])
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

func (c *StdLatestSlotCache[SLOT]) growth(ctx context.Context) error {
	_, logger := log.FromContext(ctx)
	t := timer.NewTimer()
	start := t.Start("A")
	logger.Debug("start to growth")

	// get growth speed and update c.size
	loadSpeedStart := t.Start("LS")
	bi, err := c.nodeClient.WaitBlockInterval(ctx)
	if err != nil {
		logger.Errorfe(err, "growth failed because get block interval failed")
		return err
	}
	memCacheSize := uint64(c.cacheBlockTimeLen/bi) + 1
	loadSpeedStart.End()

	readRangeStart := t.Start("LR")
	extRange, err := c.persistent.GetRange(ctx)
	if err != nil {
		logger.Errorfe(err, "growth failed because get range from persistent failed")
		return err
	}
	readRangeStart.End()

	loadStart := t.Start("LC")
	curRange := c.tryLoadL2Cache(ctx, extRange, memCacheSize)
	loadStart.End()

	// load new data
	readStart := t.Start("LE")
	newRange, loaded, reorg, err := c.loadFromPersistent(ctx, curRange, extRange, memCacheSize)
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
	moved := *newRange.End - *curRange.End

	options := metric.WithAttributeSet(attribute.NewSet(
		attribute.String("name", c.name),
		attribute.String("network", c.network),
		attribute.Bool("reorg", reorg),
	))
	if c.growthUsed != nil {
		c.growthUsed.Record(ctx, used.Milliseconds(), options)
	}
	if c.growthMargin != nil {
		c.growthMargin.Record(ctx, int64(moved), options)
	}

	// notice the waiters
	c.blockWaiter.NewStatus(*newRange.End)

	logger.With(
		"used", t.ReportDistribution("A", "LS,LR,LC,LE,W"),
		"extRange", extRange.String(),
		"curRange", curRange.String(),
		"newRange", newRange.String(),
		"memCacheSize", memCacheSize).
		Infof("growth succeed and moved %d", moved)
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

// KeepGrowth is the only entrypoint that will update memCache and curRange
func (c *StdLatestSlotCache[SLOT]) KeepGrowth(ctx context.Context) error {
	if _, err := c.nodeClient.WaitBlock(ctx, 0); err != nil {
		return err // only because ctx canceled
	}
	for round := 0; ; round++ {
		latest, _, _, _ := c.nodeClient.GetState()
		roundCtx, logger := log.FromContext(ctx, "round", round)
		if err := c.growth(roundCtx); err != nil {
			logger.Errorfe(err, "growth failed")
		}
		if _, err := c.nodeClient.WaitBlock(ctx, latest.Number+1); err != nil {
			return err // only because ctx canceled
		}
	}
}

func (c *StdLatestSlotCache[SLOT]) KeepDump(ctx context.Context) error {
	if c.l2Cache == nil {
		return nil
	}
	ticker := time.NewTicker(c.l2CacheDumpInterval)
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
	m := map[string]any{
		"name":              c.name,
		"network":           c.network,
		"cacheBlockTimeLen": c.cacheBlockTimeLen.String(),
		"memCache": map[string]any{
			"ready": c.ready,
			"len":   len(c.memCache),
			"range": c.curRange.String(),
		},
		"loadExternal": map[string]any{
			"used":       c.loadExtUsed.String(),
			"count":      c.loadExtUsed.Sum(),
			"failed":     c.loadExtFailed,
			"reorgCount": c.loadExtReorg,
		},
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
