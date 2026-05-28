package clickhouse

import (
	"context"
	"math"
	"sort"
	"time"

	lru "github.com/sentioxyz/golang-lru"
	"github.com/sentioxyz/golang-lru/simplelru"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// cachedEntityBox is the in-memory cache entry used by fullCache.
// It wraps a persistent entity with its ClickHouse VersionedCollapsing version counter,
// which is needed to generate correct undo rows on the next write.
// version is 0 for entities that do not use VersionedCollapsing tables.
type cachedEntityBox struct {
	persistent.EntityBox

	Version uint64
}

// ChainStore wraps Store for a single chain, providing entity read/write caching.
// It implements persistent.ChainStore (chain-bound interface).
//
// ChainStore is NOT thread-safe by itself; callers (e.g. Controller.mu) are
// expected to serialise access.
type ChainStore struct {
	store *Store
	chain string

	// lruCache caches individual entity lookups.  Key is "entityName/id".
	// The deleted items will not in the set.
	lruCache   *simplelru.LRU[string, *persistent.EntityBox]
	lruEvicted int

	// fullIDCache holds the complete set of known IDs for entities that are
	// too large to fully cache.  Key is entity name.
	// The deleted items will not in the set.
	fullIDCache       map[string]set.Set[string]
	fullIDCacheLoaded map[string]bool

	// fullCache holds all entity data for sparse (small) entities.
	// Key is entity name.
	// For the entity using versioned collapsing table, even the item was deleted, the *cachedEntityBox object
	// will also exist with a nil Data and a valid Version.
	// Or the deleted items will not in the set.
	fullCache        map[string]map[string]*cachedEntityBox
	fullCacheLoaded  map[string]bool
	fullCacheRefused map[string]bool

	// cacheEntity holds in-memory-only ("IsCache") entities.
	// Key is entity name; value is a weight-limited LRU.
	cacheEntity map[string]*lru.Cache[string, *persistent.EntityBox]

	// fullCacheDataLimit is the maximum total data bytes that can be kept in
	// fullCache before falling back to the LRU + fullIDCache path.
	fullCacheDataLimit int
}

// NewChainStore creates a ChainStore bound to the given chain.
//   - lruCapacity: number of entity entries in the LRU cache.
//   - fullCacheDataSizeLimit: max total byte size of the full-data cache.
func NewChainStore(
	store *Store,
	chain string,
	lruCapacity int,
	fullCacheDataSizeLimit int,
) *ChainStore {
	cs := &ChainStore{
		store:              store,
		chain:              chain,
		fullCacheDataLimit: fullCacheDataSizeLimit,
		fullIDCache:        make(map[string]set.Set[string]),
		fullIDCacheLoaded:  make(map[string]bool),
		fullCache:          make(map[string]map[string]*cachedEntityBox),
		fullCacheLoaded:    make(map[string]bool),
		fullCacheRefused:   make(map[string]bool),
		cacheEntity:        make(map[string]*lru.Cache[string, *persistent.EntityBox]),
	}
	var err error
	cs.lruCache, err = simplelru.NewLRU[string, *persistent.EntityBox](lruCapacity, func(_ string, _ *persistent.EntityBox) {
		cs.lruEvicted++
	})
	if err != nil {
		panic(err) // only if lruCapacity <= 0
	}
	return cs
}

// ─── persistent.ChainStore implementation ───────────────────────────────────

// GetChain returns the chain this store is bound to.
func (c *ChainStore) GetChain() string { return c.chain }

// GetEntityType returns the entity schema by name.
func (c *ChainStore) GetEntityType(entity string) *schema.Entity {
	return c.store.GetEntityType(entity)
}

// GetEntityOrInterfaceType returns the entity or interface schema by name.
func (c *ChainStore) GetEntityOrInterfaceType(name string) schema.EntityOrInterface {
	return c.store.GetEntityOrInterfaceType(name)
}

func (c *ChainStore) tryLoadCache(ctx context.Context, entityType *schema.Entity) (bool, error) {
	has, loaded, err := c.tryLoadFullCache(ctx, entityType)
	if has || err != nil {
		return loaded, err
	}
	return c.tryLoadFullIDCache(ctx, entityType)
}

// GetEntity returns the entity with the given id, possibly from cache.
// fromCache is true when the result was served entirely from in-memory cache.
func (c *ChainStore) GetEntity(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
) (box *persistent.EntityBox, fromCache bool, err error) {
	if entityType.IsCache() {
		cache, has := c.cacheEntity[entityType.GetName()]
		if !has {
			return nil, true, nil
		}
		box, has = cache.Get(id)
		if !has {
			return nil, true, nil
		}
		return box, true, nil
	}

	if fromCache, err = c.tryLoadCache(ctx, entityType); err != nil {
		return nil, false, err
	}

	if c.fullCacheLoaded[entityType.Name] {
		// use fullCache.
		if cached := c.fullCache[entityType.Name][id]; cached != nil && cached.Data != nil {
			box = cached.Copy()
		}
		return box, fromCache, nil
	} else {
		// use LRU + fullIDCache.
		if !c.fullIDCache[entityType.Name].Contains(id) {
			return nil, fromCache, nil // ID not in persistent storage
		}
		key := chainStoreCacheKey(entityType.Name, id)
		if cached, ok := c.lruCache.Get(key); ok {
			return cached.Copy(), fromCache, nil
		}
		// Not in LRU — fetch from DB.
		var row *entityRow
		row, err = c.store.getEntity(ctx, entityType, c.chain, id)
		if err != nil {
			return nil, false, err
		}
		if row != nil && row.Data != nil {
			box = &row.EntityBox
		}
		if box != nil {
			c.lruCache.Add(key, box.Copy())
		}
		return box, false, nil
	}
}

// ListEntities returns entities matching the filters, possibly from cache.
// fromCache is true when all results came entirely from in-memory cache.
func (c *ChainStore) ListEntities(
	ctx context.Context,
	entityType *schema.Entity,
	filters []persistent.EntityFilter,
	limit int,
) (boxes []*persistent.EntityBox, fromCache bool, err error) {
	if entityType.IsCache() {
		cache, has := c.cacheEntity[entityType.GetName()]
		if !has {
			return nil, true, nil
		}
		keys := cache.Keys()
		sort.Strings(keys)
		for _, key := range keys {
			box, _ := cache.Get(key)
			var pass bool
			if pass, err = persistent.CheckFilters(filters, *box); err != nil {
				return nil, false, err
			} else if pass {
				boxes = append(boxes, box)
			}
			if len(boxes) >= limit {
				break
			}
		}
		return boxes, true, nil
	}

	// Attempt to serve from the full-data cache.
	var has bool
	if has, fromCache, err = c.tryLoadFullCache(ctx, entityType); err != nil {
		return
	} else if !has {
		// No full cache — query the DB.
		rows, listErr := c.store.listEntities(ctx, entityType, c.chain, filters, true, limit)
		if listErr != nil {
			err = listErr
			return
		}
		for _, row := range rows {
			boxes = append(boxes, &row.EntityBox)
		}
		return
	}
	// Serve from full cache.
	cacheSlice := make([]string, 0, len(c.fullCache[entityType.Name]))
	for _, cached := range c.fullCache[entityType.Name] {
		if cached.Data == nil {
			continue
		}
		cacheSlice = append(cacheSlice, cached.ID)
	}
	sort.Strings(cacheSlice)
	for _, id := range cacheSlice {
		if len(boxes) >= limit {
			break
		}
		box := c.fullCache[entityType.Name][id]
		if box.Data == nil {
			continue
		}
		var pass bool
		if pass, err = persistent.CheckFilters(filters, box.EntityBox); err != nil {
			return nil, false, err
		} else if pass {
			boxes = append(boxes, box.Copy())
		}
	}
	return
}

// GetTimeSeriesEntityMaxID returns the maximum numeric ID for a time-series entity.
func (c *ChainStore) GetTimeSeriesEntityMaxID(ctx context.Context, entityType *schema.Entity) (int64, error) {
	return c.store.getMaxID(ctx, entityType, c.chain)
}

// SetEntities writes entities to persistent storage and updates the local cache.
func (c *ChainStore) SetEntities(
	ctx context.Context,
	entityType *schema.Entity,
	boxes []persistent.EntityBox,
) (int, error) {
	dataSize := entityType.DataSize()
	_, logger := log.FromContext(ctx, "entity", entityType.Name, "dataSize", dataSize, "chainID", c.chain)
	var knownExistingIDChecker func(id string) bool
	var knownPreBoxGetter func(id string) (*cachedEntityBox, bool)
	if !entityType.IsCache() {
		if entityType.IsTimeSeries() {
			knownExistingIDChecker = func(id string) bool {
				return false
			}
		} else {
			if _, err := c.tryLoadCache(ctx, entityType); err != nil {
				return 0, err
			}
			if !c.store.useVersionedCollapsingTable(entityType) {
				// Opportunity 1: pass existing IDs to skip queryExistEntity
				if c.fullCacheLoaded[entityType.Name] {
					knownExistingIDChecker = func(id string) bool {
						ent, has := c.fullCache[entityType.Name][id]
						return has && ent.Data != nil
					}
				} else if c.fullIDCacheLoaded[entityType.Name] {
					knownExistingIDChecker = func(id string) bool {
						return c.fullIDCache[entityType.Name].Contains(id)
					}
				}
			} else if c.fullCacheLoaded[entityType.Name] {
				// Opportunity 2: pass pre-values to skip listEntities for VC tables
				knownPreBoxGetter = func(id string) (*cachedEntityBox, bool) {
					er, has := c.fullCache[entityType.Name][id]
					return er, has
				}
			}
		}
	}
	created, err := c.store.setEntities(ctx, entityType, c.chain, boxes, knownExistingIDChecker, knownPreBoxGetter)
	if err != nil {
		return created, err
	}
	if entityType.IsTimeSeries() {
		return created, nil
	}
	// Build a map of the latest box per ID (later entries override earlier).
	latest := make(map[string]*persistent.EntityBox)
	for i := range boxes { // newer entries appear later
		latest[boxes[i].ID] = &boxes[i]
	}
	if entityType.IsCache() {
		cache, has := c.cacheEntity[entityType.GetName()]
		if !has {
			size := uint64(max(entityType.GetCacheSizeMB(), 10)) * 1024 * 1024
			cache, _ = lru.NewWithWeightLimitAndEvict[string, *persistent.EntityBox](
				int(size), size, (*persistent.EntityBox).MemSize, nil)
			c.cacheEntity[entityType.GetName()] = cache
		}
		for id, box := range latest {
			if box.Data != nil {
				cache.Add(id, box)
			} else {
				cache.Remove(id)
			}
		}
	} else if c.fullCacheRefused[entityType.Name] || !entityType.IsSparse() {
		// LRU-cache + fullIDCache path.
		if c.fullIDCacheLoaded[entityType.Name] {
			for id, box := range latest {
				key := chainStoreCacheKey(entityType.Name, id)
				if box.Data == nil {
					c.fullIDCache[entityType.Name].Remove(id)
					c.lruCache.Remove(key)
				} else {
					c.lruCache.Add(key, box)
					c.fullIDCache[entityType.Name].Add(id)
				}
			}
		}
	} else if c.fullCacheLoaded[entityType.Name] {
		// Full-data cache path.
		if c.store.useVersionedCollapsingTable(entityType) {
			// need deleted items and version in fullCache
			idWriteCount := make(map[string]int)
			for i := range boxes {
				idWriteCount[boxes[i].ID]++
			}
			for id, box := range latest {
				initialVersion := uint64(0)
				if existing, has := c.fullCache[entityType.Name][id]; has {
					initialVersion = existing.Version
				}
				c.fullCache[entityType.Name][id] = &cachedEntityBox{
					EntityBox: *box,
					Version:   initialVersion + uint64(idWriteCount[id]),
				}
			}
		} else {
			// full cache do not need deleted items and version is also useless
			for id, box := range latest {
				if box.Data == nil {
					delete(c.fullCache[entityType.Name], id)
				} else {
					c.fullCache[entityType.Name][id] = &cachedEntityBox{EntityBox: *box}
				}
			}
		}
		count := len(c.fullCache[entityType.Name])
		logger = logger.With("count", count)
		if count > c.fullCacheDataLimit/dataSize {
			logger.Warn("too many entities in persistent, refuse to use full cache")
			delete(c.fullCache, entityType.Name)
			delete(c.fullCacheLoaded, entityType.Name)
			c.fullCacheRefused[entityType.Name] = true
		} else {
			logger.Info("will keep to use full cache")
		}
	}
	return created, nil
}

// GrowthAggregation runs growth aggregation for the chain.
func (c *ChainStore) GrowthAggregation(ctx context.Context, curBlockTime time.Time) error {
	return c.store.growthAggregation(ctx, c.chain, curBlockTime)
}

// Reorg purges caches and delegates to the underlying Store.
func (c *ChainStore) Reorg(ctx context.Context, blockNumber int64) error {
	c.purgeCache()
	for _, cache := range c.cacheEntity {
		for _, key := range cache.Keys() {
			box, _ := cache.Peek(key)
			if int64(box.GenBlockNumber) > blockNumber {
				cache.Remove(key)
			}
		}
	}
	return c.store.reorg(ctx, blockNumber, c.chain)
}

// CheckValue validates entity field values using the underlying store.
func (c *ChainStore) CheckValue(entityType *schema.Entity, data map[string]any) error {
	return c.store.CheckValue(entityType, data)
}

// Snapshot returns a map describing the current cache state (for debugging/monitoring).
func (c *ChainStore) Snapshot() any {
	fullIDCache := make(map[string]int)
	for entity, loaded := range c.fullIDCacheLoaded {
		if loaded {
			fullIDCache[entity] = c.fullIDCache[entity].Size()
		}
	}
	fullCache := make(map[string]map[string]any)
	for entity, loaded := range c.fullCacheLoaded {
		if loaded {
			size := len(c.fullCache[entity])
			dataSize := c.GetEntityType(entity).DataSize()
			fullCache[entity] = map[string]any{
				"loaded":            true,
				"size":              size,
				"dataSize":          dataSize,
				"sizeOverLimitRate": float64(dataSize*size) / float64(c.fullCacheDataLimit),
			}
		}
	}
	for entity, refused := range c.fullCacheRefused {
		if refused {
			fullCache[entity] = map[string]any{"refused": true}
		}
	}
	cacheEntity := make(map[string]map[string]any)
	for entity, cache := range c.cacheEntity {
		cacheEntity[entity] = map[string]any{
			"total":     cache.Len(),
			"totalSize": cache.WeightTotal(),
		}
	}
	return map[string]any{
		"config": map[string]any{
			"fullCacheDataSizeLimit": c.fullCacheDataLimit,
		},
		"cacheEntity": cacheEntity,
		"lruCache": map[string]any{
			"evicted": c.lruEvicted,
			"size":    c.lruCache.Len(),
		},
		"fullIDCache": fullIDCache,
		"fullCache":   fullCache,
	}
}

// ─── internal helpers ────────────────────────────────────────────────────────

// purgeCache resets all cache state (except cacheEntity, which is trimmed by Reorg).
func (c *ChainStore) purgeCache() {
	c.lruCache.Purge()
	c.fullIDCache = make(map[string]set.Set[string])
	c.fullIDCacheLoaded = make(map[string]bool)
	c.fullCache = make(map[string]map[string]*cachedEntityBox)
	c.fullCacheLoaded = make(map[string]bool)
	c.fullCacheRefused = make(map[string]bool)
}

// chainStoreCacheKey builds an LRU key from the entity name and id.
func chainStoreCacheKey(entityName, id string) string {
	return entityName + "/" + id
}

// tryLoadFullCache attempts to load all entity data into the full-data cache.
//   - has=true if the full cache is usable (either loaded now or was already loaded).
//   - loaded=true when the data was already in cache (i.e. this was a cache hit).
func (c *ChainStore) tryLoadFullCache(
	ctx context.Context,
	entityType *schema.Entity,
) (has bool, loaded bool, err error) {
	if c.fullCacheRefused[entityType.Name] || !entityType.IsSparse() {
		return false, false, nil
	}
	if c.fullCacheLoaded[entityType.Name] {
		return true, true, nil
	}
	start := time.Now()
	dataSize := entityType.DataSize()
	_, logger := log.FromContext(ctx, "entity", entityType.Name, "dataSize", dataSize, "chainID", c.chain)
	logger.Debugf("will load all entities from persistent for full cache")
	// for the entity using versioned collapsing table, full cache should include deleted items,
	// because version is always needed
	excludeDeleted := !c.store.useVersionedCollapsingTable(entityType)
	var count uint64
	count, err = c.store.countEntity(ctx, entityType, c.chain, excludeDeleted)
	if err != nil {
		logger.Errore(err, "load entities from persistent for full cache failed: count exists failed")
		return
	}
	if count > uint64(c.fullCacheDataLimit/dataSize) {
		logger.Warnw("too many entities in persistent, refuse to use full cache", "count", count)
		c.fullCacheRefused[entityType.Name] = true
		return false, false, nil
	}
	rows, listErr := c.store.listEntities(ctx, entityType, c.chain, nil, excludeDeleted, math.MaxInt)
	logger = logger.With("used", time.Since(start).String())
	if listErr != nil {
		err = listErr
		logger.Errore(err, "load entities from persistent for full cache failed")
		return false, false, err
	}
	c.fullCache[entityType.Name] = make(map[string]*cachedEntityBox)
	for _, row := range rows {
		c.fullCache[entityType.Name][row.ID] = &cachedEntityBox{
			EntityBox: row.EntityBox,
			Version:   row.Version,
		}
	}
	c.fullCacheLoaded[entityType.Name] = true
	logger.Infow("loaded all entities from persistent into full cache", "count", len(rows))
	return true, false, nil
}

// tryLoadFullIDCache attempts to load all entity IDs into the full-ID cache.
// loaded=true when the IDs were already cached (cache hit).
func (c *ChainStore) tryLoadFullIDCache(
	ctx context.Context,
	entityType *schema.Entity,
) (loaded bool, err error) {
	loaded = c.fullIDCacheLoaded[entityType.Name]
	if loaded {
		return
	}
	start := time.Now()
	_, logger := log.FromContext(ctx, "entity", entityType.Name, "chainID", c.chain)
	logger.Debugf("will load all entity ids from persistent for full id cache")
	var ids set.Set[string]
	ids, err = c.store.getAllID(ctx, entityType, c.chain)
	logger = logger.With("used", time.Since(start).String())
	if err != nil {
		logger.Errore(err, "load all entity ids from persistent for full cache failed")
		return
	}
	c.fullIDCache[entityType.Name] = ids
	c.fullIDCacheLoaded[entityType.Name] = true
	logger.Infow("loaded all entity ids from persistent into full id cache", "count", len(ids))
	return
}
