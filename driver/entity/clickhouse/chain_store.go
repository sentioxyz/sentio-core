package clickhouse

import (
	"context"
	"math"
	"sort"
	"sync"
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
// ChainStore is safe for concurrent use: persistent.Controller calls
// SetEntities/GrowthAggregation outside its own mutex during Commit, so they
// run concurrently with the read methods. All cache state is guarded by mu,
// and mu is never held across a ClickHouse round trip: reads, lists, cache
// loads, writes and reorgs release it around the store call and re-check the
// cache generations before they touch the caches again.
type ChainStore struct {
	store *Store
	chain string

	// io holds the persistent operations that run outside mu. It is a field so that tests can
	// stand in for the store while exercising the real locking.
	io chainStoreIO

	// mu guards all cache state below. No persistent query runs while it is held: the methods
	// below take it to look at or update the caches and release it around store round trips.
	mu sync.Mutex

	// loading marks entity types whose caches ensureCaches is loading from the store. While
	// marked, other callers treat the caches as unavailable and query the store directly, and
	// no second load starts.
	loading set.Set[string]

	// reorging is set while Reorg runs its persistent part; no cache loads start meanwhile.
	reorging bool

	// cacheGen counts, per entity type, the cache updates applied after persistent writes, and
	// cacheEpoch counts the cache purges. GetEntity records both before it reads the store
	// without mu and only caches the row it read when neither moved in between: otherwise the
	// row may predate a write (or a reorg) whose result is already in the caches.
	cacheGen   map[string]uint64
	cacheEpoch uint64

	// writing marks entity types whose SetEntities persistent write is in
	// flight. While marked, ensureCaches refuses to load:
	// a load during the write would capture a half-written store state, and the
	// post-write cache update would then apply the written boxes on top of it
	// (double-counting versioned-collapsing versions).
	writing set.Set[string]

	// lruCache caches individual entity lookups.  Key is "entityName/id".
	// The deleted items will not in the set.
	lruCache   *simplelru.LRU[string, *persistent.EntityBox]
	lruEvicted int

	// fullIDCache holds the complete set of known IDs for entities that are
	// too large to fully cache.  Key is entity name.
	// The deleted items will not in the set.
	fullIDCache        map[string]set.Set[string]
	fullIDCacheLoaded  map[string]bool
	fullIDCacheRefused map[string]bool

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

	// fullIDCacheMaxCount caps how many entity IDs may be loaded into the full-ID
	// cache.  Entities beyond this count fall back to per-query existence checks
	// against the persistent store; loading hundreds of millions of IDs into one
	// in-process set would otherwise exhaust the driver's memory limit.
	fullIDCacheMaxCount uint64
}

// chainStoreIO is the set of persistent operations a ChainStore performs. ChainStore never holds
// mu while calling one of them.
type chainStoreIO struct {
	getEntity    func(ctx context.Context, entityType *schema.Entity, id string) (*entityRow, error)
	listEntities func(
		ctx context.Context,
		entityType *schema.Entity,
		filters []persistent.EntityFilter,
		excludeDeleted bool,
		limit int,
	) ([]*entityRow, error)
	countEntity         func(ctx context.Context, entityType *schema.Entity, excludeDeleted bool) (uint64, error)
	getAllID            func(ctx context.Context, entityType *schema.Entity) (set.Set[string], error)
	reorg               func(ctx context.Context, blockNumber int64) error
	versionedCollapsing func(entityType *schema.Entity) bool
}

func (s *Store) chainStoreIO(chain string) chainStoreIO {
	return chainStoreIO{
		getEntity: func(ctx context.Context, entityType *schema.Entity, id string) (*entityRow, error) {
			return s.getEntity(ctx, entityType, chain, id)
		},
		listEntities: func(
			ctx context.Context,
			entityType *schema.Entity,
			filters []persistent.EntityFilter,
			excludeDeleted bool,
			limit int,
		) ([]*entityRow, error) {
			return s.listEntities(ctx, entityType, chain, filters, excludeDeleted, limit)
		},
		countEntity: func(ctx context.Context, entityType *schema.Entity, excludeDeleted bool) (uint64, error) {
			return s.countEntity(ctx, entityType, chain, excludeDeleted)
		},
		getAllID: func(ctx context.Context, entityType *schema.Entity) (set.Set[string], error) {
			return s.getAllID(ctx, entityType, chain)
		},
		reorg: func(ctx context.Context, blockNumber int64) error {
			return s.reorg(ctx, blockNumber, chain)
		},
		versionedCollapsing: func(entityType *schema.Entity) bool {
			return s.useVersionedCollapsingTable(entityType)
		},
	}
}

// NewChainStore creates a ChainStore bound to the given chain.
//   - lruCapacity: number of entity entries in the LRU cache.
//   - fullCacheDataSizeLimit: max total byte size of the full-data cache.
//   - fullIDCacheMaxCount: max number of entity IDs the full-ID cache may hold.
func NewChainStore(
	store *Store,
	chain string,
	lruCapacity int,
	fullCacheDataSizeLimit int,
	fullIDCacheMaxCount uint64,
) *ChainStore {
	cs := &ChainStore{
		store:               store,
		chain:               chain,
		writing:             set.New[string](),
		loading:             set.New[string](),
		fullCacheDataLimit:  fullCacheDataSizeLimit,
		fullIDCacheMaxCount: fullIDCacheMaxCount,
		fullIDCache:         make(map[string]set.Set[string]),
		fullIDCacheLoaded:   make(map[string]bool),
		fullIDCacheRefused:  make(map[string]bool),
		fullCache:           make(map[string]map[string]*cachedEntityBox),
		fullCacheLoaded:     make(map[string]bool),
		fullCacheRefused:    make(map[string]bool),
		cacheEntity:         make(map[string]*lru.Cache[string, *persistent.EntityBox]),
		cacheGen:            make(map[string]uint64),
	}
	if store != nil {
		cs.io = store.chainStoreIO(chain)
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

// cacheLoadPlan says which caches of an entity type still have to be loaded. Must be called under mu.
// Nothing is planned while a persistent write is in flight (the load would capture a half-written
// state), while another load or a reorg is running, or once the full-data cache is loaded.
func (c *ChainStore) cacheLoadPlan(entityType *schema.Entity) (full, ids bool) {
	name := entityType.Name
	if entityType.IsCache() || c.writing.Contains(name) || c.loading.Contains(name) || c.reorging {
		return false, false
	}
	if c.fullCacheLoaded[name] {
		return false, false
	}
	full = entityType.IsSparse() && !c.fullCacheRefused[name]
	ids = !c.fullIDCacheLoaded[name] && !c.fullIDCacheRefused[name]
	return full, ids
}

// loadedCaches is what one cache load brought back from the store.
type loadedCaches struct {
	full        map[string]*cachedEntityBox
	fullRefused bool
	ids         set.Set[string]
	idsRefused  bool
}

// ensureCaches loads the full-data cache or the full-ID cache of entityType when neither has been
// loaded or refused yet. The store queries run without mu: only one caller loads a given entity
// type at a time and, meanwhile, the others carry on with direct store queries. A load that
// overlaps a persistent write of the same entity type, whichever started first, is discarded.
// loadedNow reports that this call loaded a cache (as opposed to finding it loaded already).
func (c *ChainStore) ensureCaches(ctx context.Context, entityType *schema.Entity) (loadedNow bool, err error) {
	name := entityType.Name
	c.mu.Lock()
	full, ids := c.cacheLoadPlan(entityType)
	if !full && !ids {
		c.mu.Unlock()
		return false, nil
	}
	c.loading.Add(name)
	gen, epoch := c.cacheGen[name], c.cacheEpoch
	c.mu.Unlock()

	loaded, err := c.loadCaches(ctx, entityType, full, ids)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.loading.Remove(name)
	if err != nil {
		return false, err
	}
	// a refusal is a decision about the entity's size and stays valid whatever happened meanwhile
	if loaded.fullRefused {
		c.fullCacheRefused[name] = true
	}
	if loaded.idsRefused {
		c.fullIDCacheRefused[name] = true
	}
	if c.cacheGen[name] != gen || c.cacheEpoch != epoch || c.writing.Contains(name) {
		// a write landed, a write is still in flight (the load may have seen part of it; cacheGen
		// only moves once it has landed) or the caches were purged while loading: what was loaded
		// may be stale, the next caller loads again
		return false, nil
	}
	if loaded.full != nil {
		c.fullCache[name] = loaded.full
		c.fullCacheLoaded[name] = true
		return true, nil
	}
	if loaded.ids != nil {
		c.fullIDCache[name] = loaded.ids
		c.fullIDCacheLoaded[name] = true
		return true, nil
	}
	return false, nil
}

// loadCaches runs the store queries of a cache load, without mu. The full-data cache is loaded
// first when planned; it makes the full-ID cache unnecessary. An entity type with more rows than
// the caches may hold is refused instead.
func (c *ChainStore) loadCaches(
	ctx context.Context,
	entityType *schema.Entity,
	full, ids bool,
) (loaded loadedCaches, err error) {
	start := time.Now()
	dataSize := entityType.DataSize()
	_, logger := log.FromContext(ctx, "entity", entityType.Name, "dataSize", dataSize, "chainID", c.chain)
	knownCount := int64(-1)
	if full {
		// for the entity using versioned collapsing table, full cache should include deleted items,
		// because version is always needed
		excludeDeleted := !c.io.versionedCollapsing(entityType)
		var count uint64
		if count, err = c.io.countEntity(ctx, entityType, excludeDeleted); err != nil {
			logger.Errore(err, "load entities from persistent for full cache failed: count exists failed")
			return
		}
		// May include deleted rows for versioned-collapsing entities, which only makes the
		// full-ID cache check below stricter.
		knownCount = int64(count)
		if count > uint64(c.fullCacheDataLimit/dataSize) {
			logger.Warnw("too many entities in persistent, refuse to use full cache", "count", count)
			loaded.fullRefused = true
		} else {
			logger.Debugf("will really load all %d entities from persistent for full cache", count)
			var rows []*entityRow
			if rows, err = c.io.listEntities(ctx, entityType, nil, excludeDeleted, math.MaxInt); err != nil {
				logger.With("used", time.Since(start).String()).
					Errore(err, "load entities from persistent for full cache failed")
				return
			}
			loaded.full = make(map[string]*cachedEntityBox, len(rows))
			for _, row := range rows {
				loaded.full[row.ID] = &cachedEntityBox{EntityBox: row.EntityBox, Version: row.Version}
			}
			logger.With("used", time.Since(start).String()).
				Infow("loaded all entities from persistent into full cache", "count", len(rows))
			return
		}
	}
	if !ids {
		return
	}
	if knownCount < 0 {
		var count uint64
		if count, err = c.io.countEntity(ctx, entityType, true); err != nil {
			logger.Errore(err, "load all entity ids from persistent for full cache failed: count exists failed")
			return
		}
		knownCount = int64(count)
	}
	if uint64(knownCount) > c.fullIDCacheMaxCount {
		// holding that many IDs in memory could OOM the process; callers handle a missing ID
		// cache by querying the persistent store directly
		logger.Warnw("too many entities in persistent, refuse to use full id cache", "count", knownCount)
		loaded.idsRefused = true
		return
	}
	logger.Debugf("will load all entity ids from persistent for full id cache")
	if loaded.ids, err = c.io.getAllID(ctx, entityType); err != nil {
		logger.With("used", time.Since(start).String()).
			Errore(err, "load all entity ids from persistent for full cache failed")
		return
	}
	logger.With("used", time.Since(start).String()).
		Infow("loaded all entity ids from persistent into full id cache", "count", loaded.ids.Size())
	return
}

// GetEntity returns the entity with the given id, possibly from cache.
// fromCache is true when the result was served entirely from in-memory cache.
//
// Only the cache lookups run under mu. The store round trips (a cache load, an LRU miss) run
// without it, so that concurrent reads overlap each other instead of queueing on the cache lock,
// and other cache readers are not blocked for a ClickHouse round trip.
func (c *ChainStore) GetEntity(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
) (box *persistent.EntityBox, fromCache bool, err error) {
	loadedNow, err := c.ensureCaches(ctx, entityType)
	if err != nil {
		return nil, false, err
	}
	box, fromCache, key, gen, epoch, done := c.getEntityFromCache(entityType, id, loadedNow)
	if done {
		return box, fromCache, nil
	}

	// Not in LRU — fetch from the store, without mu.
	row, err := c.io.getEntity(ctx, entityType, id)
	if err != nil {
		return nil, false, err
	}
	if row != nil && row.Data != nil {
		box = &row.EntityBox
	}
	if box != nil {
		c.mu.Lock()
		if c.cacheGen[entityType.Name] == gen && c.cacheEpoch == epoch {
			// no write landed and no purge happened while the read was in flight
			c.lruCache.Add(key, box.Copy())
		}
		c.mu.Unlock()
	}
	return box, false, nil
}

// getEntityFromCache is the part of GetEntity that runs under mu. done reports that box and
// fromCache are the answer; otherwise the caller has to read the store for key, and gen / epoch
// are the cache generations it must compare against before caching what it read.
func (c *ChainStore) getEntityFromCache(
	entityType *schema.Entity,
	id string,
	loadedNow bool,
) (box *persistent.EntityBox, fromCache bool, key string, gen, epoch uint64, done bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entityType.IsCache() {
		cache, has := c.cacheEntity[entityType.GetName()]
		if !has {
			return nil, true, "", 0, 0, true
		}
		box, has = cache.Get(id)
		if !has {
			return nil, true, "", 0, 0, true
		}
		return box, true, "", 0, 0, true
	}
	fromCache = !loadedNow

	if c.fullCacheLoaded[entityType.Name] {
		// use fullCache.
		if cached := c.fullCache[entityType.Name][id]; cached != nil && cached.Data != nil {
			box = cached.Copy()
		}
		return box, fromCache, "", 0, 0, true
	}
	// use LRU + fullIDCache.
	// When the full-ID cache was refused (too many IDs to hold in memory),
	// skip the existence shortcut and fall through to the LRU + DB lookup.
	if c.fullIDCacheLoaded[entityType.Name] && !c.fullIDCache[entityType.Name].Contains(id) {
		return nil, fromCache, "", 0, 0, true // ID not in persistent storage
	}
	key = chainStoreCacheKey(entityType.Name, id)
	if cached, ok := c.lruCache.Get(key); ok {
		return cached.Copy(), fromCache, key, 0, 0, true
	}
	return nil, false, key, c.cacheGen[entityType.Name], c.cacheEpoch, false
}

// ListEntities returns entities matching the filters, possibly from cache.
// fromCache is true when all results came entirely from in-memory cache.
// As in GetEntity, the store query of an entity without a full-data cache runs without mu.
func (c *ChainStore) ListEntities(
	ctx context.Context,
	entityType *schema.Entity,
	filters []persistent.EntityFilter,
	limit int,
) (boxes []*persistent.EntityBox, fromCache bool, err error) {
	loadedNow, err := c.ensureCaches(ctx, entityType)
	if err != nil {
		return nil, false, err
	}
	boxes, fromCache, done, err := c.listEntitiesFromCache(entityType, filters, limit, loadedNow)
	if done || err != nil {
		return boxes, fromCache, err
	}

	// No full cache — query the store, without mu.
	rows, err := c.io.listEntities(ctx, entityType, filters, true, limit)
	if err != nil {
		return nil, false, err
	}
	for _, row := range rows {
		boxes = append(boxes, &row.EntityBox)
	}
	return boxes, false, nil
}

// listEntitiesFromCache is the part of ListEntities that runs under mu. done reports that the
// caches answered; otherwise the caller has to query the store.
func (c *ChainStore) listEntitiesFromCache(
	entityType *schema.Entity,
	filters []persistent.EntityFilter,
	limit int,
	loadedNow bool,
) (boxes []*persistent.EntityBox, fromCache bool, done bool, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entityType.IsCache() {
		cache, has := c.cacheEntity[entityType.GetName()]
		if !has {
			return nil, true, true, nil
		}
		keys := cache.Keys()
		sort.Strings(keys)
		for _, key := range keys {
			box, _ := cache.Get(key)
			var pass bool
			if pass, err = persistent.CheckFilters(filters, *box); err != nil {
				return nil, false, true, err
			} else if pass {
				boxes = append(boxes, box)
			}
			if len(boxes) >= limit {
				break
			}
		}
		return boxes, true, true, nil
	}
	if !c.fullCacheLoaded[entityType.Name] {
		return nil, false, false, nil
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
			return nil, false, true, err
		} else if pass {
			boxes = append(boxes, box.Copy())
		}
	}
	return boxes, !loadedNow, true, nil
}

// GetTimeSeriesEntityMaxID returns the maximum numeric ID for a time-series entity.
func (c *ChainStore) GetTimeSeriesEntityMaxID(ctx context.Context, entityType *schema.Entity) (int64, error) {
	return c.store.getMaxID(ctx, entityType, c.chain)
}

// SetEntities writes entities to persistent storage and updates the local cache.
//
// It runs in three phases so a slow persistent write does not block cache
// readers: under mu it loads the caches if needed and materialises the
// per-ID answers the write needs (so the write never touches the caches),
// then performs the persistent write without holding mu, and finally updates
// the caches under mu again. The `writing` mark keeps the caches from being
// loaded from a half-written store state in between.
func (c *ChainStore) SetEntities(
	ctx context.Context,
	entityType *schema.Entity,
	boxes []persistent.EntityBox,
) (int, error) {
	dataSize := entityType.DataSize()
	_, logger := log.FromContext(ctx, "entity", entityType.Name, "dataSize", dataSize, "chainID", c.chain)
	var knownExistingIDChecker func(id string) bool
	var knownPreBoxGetter func(id string) (*cachedEntityBox, bool)

	if !entityType.IsCache() && !entityType.IsTimeSeries() {
		// the caches the write consults are loaded without mu
		if _, err := c.ensureCaches(ctx, entityType); err != nil {
			return 0, err
		}
	}
	c.mu.Lock()
	if !entityType.IsCache() {
		if entityType.IsTimeSeries() {
			knownExistingIDChecker = func(id string) bool {
				return false
			}
		} else {
			ids := set.New[string]()
			for i := range boxes {
				ids.Add(boxes[i].ID)
			}
			if !c.io.versionedCollapsing(entityType) {
				// Opportunity 1: pass existing IDs to skip queryExistEntity
				if c.fullCacheLoaded[entityType.Name] {
					existing := set.New[string]()
					for _, id := range ids.DumpValues() {
						if ent, has := c.fullCache[entityType.Name][id]; has && ent.Data != nil {
							existing.Add(id)
						}
					}
					knownExistingIDChecker = existing.Contains
				} else if c.fullIDCacheLoaded[entityType.Name] {
					existing := set.New[string]()
					for _, id := range ids.DumpValues() {
						if c.fullIDCache[entityType.Name].Contains(id) {
							existing.Add(id)
						}
					}
					knownExistingIDChecker = existing.Contains
				}
			} else if c.fullCacheLoaded[entityType.Name] {
				// Opportunity 2: pass pre-values to skip listEntities for VC tables
				pre := make(map[string]*cachedEntityBox, ids.Size())
				for _, id := range ids.DumpValues() {
					if er, has := c.fullCache[entityType.Name][id]; has {
						pre[id] = er
					}
				}
				knownPreBoxGetter = func(id string) (*cachedEntityBox, bool) {
					er, has := pre[id]
					return er, has
				}
			}
		}
	}
	c.writing.Add(entityType.Name)
	c.mu.Unlock()

	created, err := c.store.setEntities(ctx, entityType, c.chain, boxes, knownExistingIDChecker, knownPreBoxGetter)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.writing.Remove(entityType.Name)
	if err != nil {
		return created, err
	}
	if entityType.IsTimeSeries() {
		return created, nil
	}
	c.applyWriteToCaches(logger, entityType, boxes)
	return created, nil
}

// applyWriteToCaches updates the caches after boxes were written to the store. Must be called
// under mu.
func (c *ChainStore) applyWriteToCaches(
	logger *log.SentioLogger,
	entityType *schema.Entity,
	boxes []persistent.EntityBox,
) {
	dataSize := entityType.DataSize()
	// a GetEntity that read the store before this write landed must not cache what it read
	c.cacheGen[entityType.Name]++
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
		// LRU-cache + fullIDCache path.  The LRU is maintained even when the
		// full-ID cache was refused, so freshly written entities can still be
		// read back without hitting the persistent store.
		idCacheLoaded := c.fullIDCacheLoaded[entityType.Name]
		for id, box := range latest {
			key := chainStoreCacheKey(entityType.Name, id)
			if box.Data == nil {
				if idCacheLoaded {
					c.fullIDCache[entityType.Name].Remove(id)
				}
				c.lruCache.Remove(key)
			} else {
				c.lruCache.Add(key, box)
				if idCacheLoaded {
					c.fullIDCache[entityType.Name].Add(id)
				}
			}
		}
	} else if c.fullCacheLoaded[entityType.Name] {
		// Full-data cache path.
		if c.io.versionedCollapsing(entityType) {
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
}

// GrowthAggregation runs growth aggregation for the chain.
func (c *ChainStore) GrowthAggregation(ctx context.Context, curBlockTime time.Time) error {
	return c.store.growthAggregation(ctx, c.chain, curBlockTime)
}

// Reorg purges caches and delegates to the underlying Store. The persistent part runs without
// mu; the caches are purged before it, so nothing stale is served from them, and again after it,
// so nothing read from the store while the reorg was in flight stays cached.
func (c *ChainStore) Reorg(ctx context.Context, blockNumber int64) error {
	c.mu.Lock()
	c.reorging = true
	c.purgeCache()
	for _, cache := range c.cacheEntity {
		for _, key := range cache.Keys() {
			box, _ := cache.Peek(key)
			if int64(box.GenBlockNumber) > blockNumber {
				cache.Remove(key)
			}
		}
	}
	c.mu.Unlock()

	err := c.io.reorg(ctx, blockNumber)

	c.mu.Lock()
	c.reorging = false
	c.purgeCache()
	c.mu.Unlock()
	return err
}

// CheckValue validates entity field values using the underlying store.
func (c *ChainStore) CheckValue(entityType *schema.Entity, data map[string]any) error {
	return c.store.CheckValue(entityType, data)
}

// Snapshot returns a map describing the current cache state (for debugging/monitoring).
func (c *ChainStore) Snapshot() any {
	c.mu.Lock()
	defer c.mu.Unlock()
	fullIDCache := make(map[string]any)
	for entity, loaded := range c.fullIDCacheLoaded {
		if loaded {
			fullIDCache[entity] = c.fullIDCache[entity].Size()
		}
	}
	for entity, refused := range c.fullIDCacheRefused {
		if refused {
			fullIDCache[entity] = map[string]any{"refused": true}
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
			"fullIDCacheMaxCount":    c.fullIDCacheMaxCount,
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
	// a GetEntity that read the store before the purge must not cache what it read
	c.cacheEpoch++
	c.lruCache.Purge()
	c.fullIDCache = make(map[string]set.Set[string])
	c.fullIDCacheLoaded = make(map[string]bool)
	c.fullIDCacheRefused = make(map[string]bool)
	c.fullCache = make(map[string]map[string]*cachedEntityBox)
	c.fullCacheLoaded = make(map[string]bool)
	c.fullCacheRefused = make(map[string]bool)
}

// chainStoreCacheKey builds an LRU key from the entity name and id.
func chainStoreCacheKey(entityName, id string) string {
	return entityName + "/" + id
}
