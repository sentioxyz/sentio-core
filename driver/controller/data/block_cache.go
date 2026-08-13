package data

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"

	lru "github.com/sentioxyz/golang-lru"
	"golang.org/x/sync/singleflight"
)

// flightTimeout bounds a detached in-flight fetch. The flight runs on a context decoupled from
// every caller (see GetOrFetch), so without its own deadline an unresponsive RPC would leak the
// flight goroutine forever.
const flightTimeout = time.Minute

// ErrFlightInvalidated reports that InvalidateRange ran while the fetch was in flight, so the
// fetched value may describe the invalidated (e.g. reorged-away) state and was discarded instead
// of being cached or served. It is a transient condition: retrying starts a fresh fetch.
var ErrFlightInvalidated = errors.New("in-flight fetch invalidated by a cache reset, retry")

// BlockCache is an LRU keyed by block number combined with singleflight. Chain clients prefetch
// headers/blocks concurrently from several fetchers, so the same block is frequently requested by
// multiple goroutines at nearly the same time. BlockCache caches the value and collapses concurrent
// misses for the same block into a single fetch, which keeps per-block RPCs (e.g. a header lookup)
// off the hot path and from being duplicated.
type BlockCache[V any] struct {
	cache *lru.Cache[uint64, V]
	sf    singleflight.Group

	// mu orders flight completion against InvalidateRange: a flight may commit its value only
	// after re-checking (under mu) that no invalidation happened since it started. gen counts
	// invalidations; it is part of the singleflight key, so callers arriving after an
	// invalidation never join a flight started before it.
	mu  sync.Mutex
	gen uint64
}

// NewBlockCache creates a BlockCache holding up to size entries. It errors only when size <= 0.
func NewBlockCache[V any](size int) (*BlockCache[V], error) {
	cache, err := lru.New[uint64, V](size)
	if err != nil {
		return nil, err
	}
	return &BlockCache[V]{cache: cache}, nil
}

// Get returns the cached value for blockNumber, if present.
func (c *BlockCache[V]) Get(blockNumber uint64) (V, bool) {
	return c.cache.Get(blockNumber)
}

// Add stores v for blockNumber, overwriting any existing entry.
func (c *BlockCache[V]) Add(blockNumber uint64, v V) {
	c.cache.Add(blockNumber, v)
}

// InvalidateRange drops every cached entry in r AND obsoletes every fetch currently in flight
// (whatever block it is for): an in-flight value may describe the invalidated state, is not yet
// in the LRU (so removing keys cannot catch it), and its flight keeps running detached from the
// canceled callers — so it must not be cached or served once it completes. Obsoleted flights fail
// with ErrFlightInvalidated and their waiters simply retry. Invalidation is global rather than
// per-key because it happens only on rare events (reorgs), where one wasted retry for an
// unrelated in-flight block is cheaper than tracking per-key generations.
func (c *BlockCache[V]) InvalidateRange(r controller.BlockRange) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	for _, bn := range c.cache.Keys() {
		if r.Contains(bn) {
			c.cache.Remove(bn)
		}
	}
}

// GetOrFetch returns the cached value for blockNumber, or fetches it via fetch and caches the result.
// Concurrent misses for the same block are coalesced into a single fetch. fetch is not invoked even
// when a caller arrives just after an in-flight fetch finished: the cache is re-checked inside the
// flight, so the just-fetched value is reused rather than fetched again.
//
// The flight is shared by callers that know nothing about each other, so fetch runs on a context
// detached from the caller that happened to start it (bounded by flightTimeout instead): a starter
// whose request dies must not fail every waiter with its context.Canceled. fetch must therefore use
// the context it is handed, not one captured from a caller. Each waiter is still released by its
// own ctx: cancellation returns that ctx's error immediately while the flight finishes (and caches)
// in the background — unless InvalidateRange runs meanwhile, which discards the flight's value
// (see ErrFlightInvalidated).
func (c *BlockCache[V]) GetOrFetch(
	ctx context.Context,
	blockNumber uint64,
	fetch func(ctx context.Context) (V, error),
) (V, error) {
	// Fast path: avoids the strconv + singleflight bookkeeping when the block is already cached.
	if v, ok := c.cache.Get(blockNumber); ok {
		return v, nil
	}
	c.mu.Lock()
	gen := c.gen
	c.mu.Unlock()
	key := strconv.FormatUint(gen, 10) + ":" + strconv.FormatUint(blockNumber, 10)
	ch := c.sf.DoChan(key, func() (any, error) {
		// Re-check inside the flight (double-checked): a preceding flight for this block may have
		// finished and populated the cache between the miss above and entering DoChan.
		if v, ok := c.cache.Get(blockNumber); ok {
			return v, nil
		}
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), flightTimeout)
		defer cancel()
		v, err := fetch(fetchCtx)
		if err != nil {
			return nil, err
		}
		// Commit under mu so the staleness re-check and the Add are atomic with respect to
		// InvalidateRange: without it an invalidation could slip between them and the stale
		// value would land in the cache anyway.
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.gen != gen {
			return nil, ErrFlightInvalidated
		}
		c.cache.Add(blockNumber, v)
		return v, nil
	})
	select {
	case res := <-ch:
		if res.Err != nil {
			var zero V
			return zero, res.Err
		}
		return res.Val.(V), nil
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	}
}

// Snapshot renders up to maxCount entries for the debug tracker, using valuePreview to stringify each
// value. It mirrors utils.CacheSnapshot so callers don't need to reach for the underlying cache.
func (c *BlockCache[V]) Snapshot(maxCount int, valuePreview func(V) string) any {
	return utils.CacheSnapshot(c.cache, maxCount, valuePreview)
}
