package data

import (
	"context"
	"errors"
	"strconv"

	"sentioxyz/sentio-core/common/utils"

	lru "github.com/sentioxyz/golang-lru"
	"golang.org/x/sync/singleflight"
)

// maxTakeoverAttempts bounds the GetOrFetch takeover loop. Each retry means the previous flight
// died with its starter's cancellation while we are still alive, so a couple of attempts is
// plenty; the bound only guards against a fetch that keeps returning context.Canceled for some
// other reason, which would otherwise loop forever since our own context never expires it.
const maxTakeoverAttempts = 3

// BlockCache is an LRU keyed by block number combined with singleflight. Chain clients prefetch
// headers/blocks concurrently from several fetchers, so the same block is frequently requested by
// multiple goroutines at nearly the same time. BlockCache caches the value and collapses concurrent
// misses for the same block into a single fetch, which keeps per-block RPCs (e.g. a header lookup)
// off the hot path and from being duplicated.
type BlockCache[V any] struct {
	cache *lru.Cache[uint64, V]
	sf    singleflight.Group
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

// Remove drops blockNumber from the cache.
func (c *BlockCache[V]) Remove(blockNumber uint64) {
	c.cache.Remove(blockNumber)
}

// Keys returns the currently cached block numbers (used to evict a reorged range).
func (c *BlockCache[V]) Keys() []uint64 {
	return c.cache.Keys()
}

// GetOrFetch returns the cached value for blockNumber, or fetches it via fetch and caches the result.
// Concurrent misses for the same block are coalesced into a single fetch. fetch is not invoked even
// when a caller arrives just after an in-flight fetch finished: the cache is re-checked inside the
// flight, so the just-fetched value is reused rather than fetched again.
//
// The flight runs the fn of the caller that starts it, so it lives and dies with that caller's
// context (fetch should capture the caller's own ctx). A caller sharing a flight whose starter got
// canceled mid-way therefore sees a context.Canceled that says nothing about its own liveness:
// when our ctx is still alive we simply retry — the dead flight is gone, so the retry starts (or
// joins) a fresh one under a live context. Dying with the starter also means no flight can outlive
// its run: a reorg's ResetCache never races a detached fetch. Each caller is released by its own
// ctx while waiting, without aborting the shared flight.
func (c *BlockCache[V]) GetOrFetch(
	ctx context.Context,
	blockNumber uint64,
	fetch func() (V, error),
) (V, error) {
	var zero V
	for attempt := 1; ; attempt++ {
		// Also serves as the fast path: on the first pass it avoids the strconv + singleflight
		// bookkeeping when the block is already cached.
		if v, ok := c.cache.Get(blockNumber); ok {
			return v, nil
		}
		ch := c.sf.DoChan(strconv.FormatUint(blockNumber, 10), func() (any, error) {
			// Re-check inside the flight (double-checked): a preceding flight for this block may
			// have finished and populated the cache between the miss above and entering DoChan.
			if v, ok := c.cache.Get(blockNumber); ok {
				return v, nil
			}
			v, err := fetch()
			if err != nil {
				return nil, err
			}
			c.cache.Add(blockNumber, v)
			return v, nil
		})
		select {
		case res := <-ch:
			if res.Err == nil {
				return res.Val.(V), nil
			}
			if errors.Is(res.Err, context.Canceled) && ctx.Err() == nil && attempt < maxTakeoverAttempts {
				continue
			}
			return zero, res.Err
		case <-ctx.Done():
			return zero, ctx.Err()
		}
	}
}

// Snapshot renders up to maxCount entries for the debug tracker, using valuePreview to stringify each
// value. It mirrors utils.CacheSnapshot so callers don't need to reach for the underlying cache.
func (c *BlockCache[V]) Snapshot(maxCount int, valuePreview func(V) string) any {
	return utils.CacheSnapshot(c.cache, maxCount, valuePreview)
}
