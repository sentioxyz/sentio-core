package persistent

import (
	"context"
	lru "github.com/sentioxyz/golang-lru"
	"go.opentelemetry.io/otel/metric"
	"math"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	"sort"
	"time"

	"github.com/sentioxyz/golang-lru/simplelru"
)

type CachedStore struct {
	store Store

	chain                  string
	fullCacheDataSizeLimit int

	usedMetric metric.Float64Histogram

	// cached entities from persistent storage, if this entity type refused full cache
	// key is <entityName>/<id>, only used in GetEntity
	cache        *simplelru.LRU[string, *EntityBox]
	cacheEvicted int

	fullIDCache       map[string]map[string]bool
	fullIDCacheLoaded map[string]bool

	// cached entities from persistent storage
	// key is <entityName>, used in ListEntities and GetEntity
	fullCache        map[string]map[string]*EntityBox
	fullCacheLoaded  map[string]bool
	fullCacheRefused map[string]bool

	// cache entity do not have data in persistent storage, will not use fullCache and fullIDCache, just save data here
	// map key is <entityName>, lru cache key is <id>
	cacheEntity map[string]*lru.Cache[string, *EntityBox]
}

func NewCachedStore(
	persistent Store,
	chain string,
	capacity int,
	fullCacheDataSizeLimit int,
	usedMetric metric.Float64Histogram,
) *CachedStore {
	c := &CachedStore{
		store:                  persistent,
		chain:                  chain,
		usedMetric:             usedMetric,
		fullIDCache:            make(map[string]map[string]bool),
		fullIDCacheLoaded:      make(map[string]bool),
		fullCacheDataSizeLimit: fullCacheDataSizeLimit,
		fullCache:              make(map[string]map[string]*EntityBox),
		fullCacheLoaded:        make(map[string]bool),
		fullCacheRefused:       make(map[string]bool),
		cacheEntity:            make(map[string]*lru.Cache[string, *EntityBox]),
	}
	var err error
	c.cache, err = simplelru.NewLRU[string, *EntityBox](capacity, func(key string, value *EntityBox) {
		c.cacheEvicted++
	})
	if err != nil {
		panic(err) // only if capacity <= 0
	}
	return c
}

func (c *CachedStore) purgeCache() {
	c.cache.Purge()
	c.fullIDCache = make(map[string]map[string]bool)
	c.fullIDCacheLoaded = make(map[string]bool)
	c.fullCache = make(map[string]map[string]*EntityBox)
	c.fullCacheRefused = make(map[string]bool)
	c.fullCacheLoaded = make(map[string]bool)
}

func (c *CachedStore) GetChain() string {
	return c.chain
}

func (c *CachedStore) InitEntitySchema(ctx context.Context) error {
	c.purgeCache()
	return c.store.InitEntitySchema(ctx)
}

func (c *CachedStore) GetEntityType(entity string) *schema.Entity {
	return c.store.GetEntityType(entity)
}

func (c *CachedStore) GetEntityOrInterfaceType(name string) schema.EntityOrInterface {
	return c.store.GetEntityOrInterfaceType(name)
}

func (c *CachedStore) Reorg(ctx context.Context, blockNumber int64) error {
	c.purgeCache()
	for _, cache := range c.cacheEntity {
		for _, key := range cache.Keys() {
			box, _ := cache.Peek(key)
			if int64(box.GenBlockNumber) > blockNumber {
				cache.Remove(key)
			}
		}
	}
	return c.store.Reorg(ctx, blockNumber, c.chain)
}

func (c *CachedStore) SetEntities(ctx context.Context, entityType *schema.Entity, boxes []EntityBox) (int, error) {
	dataSize := entityType.DataSize()
	_, logger := log.FromContext(ctx, "entity", entityType.Name, "dataSize", dataSize, "chainID", c.chain)
	created, err := c.store.SetEntities(ctx, entityType, boxes)
	if err != nil {
		return created, err
	}
	if entityType.IsTimeSeries() {
		return created, nil
	}
	// update cache
	latest := make(map[string]*EntityBox)
	for i := range boxes { // newer always in behind
		latest[boxes[i].ID] = &boxes[i]
	}
	if entityType.IsCache() {
		// this is cache entity
		cache, has := c.cacheEntity[entityType.GetName()]
		if !has {
			size := max(entityType.GetCacheSizeMB(), 10) * 1024 * 1024
			cache, _ = lru.NewWithWeightLimitAndEvict[string, *EntityBox](int(size), size, (*EntityBox).MemSize, nil)
			c.cacheEntity[entityType.GetName()] = cache
		}
		for id, box := range latest {
			cache.Add(id, box)
		}
	} else if c.fullCacheRefused[entityType.Name] || !entityType.IsSparse() {
		// using lru cache and full id cache
		if c.fullIDCacheLoaded[entityType.Name] {
			// full id cache loaded
			for id, box := range latest {
				c.cache.Add(cacheKey(entityType.Name, id), box)
				if box.Data == nil {
					delete(c.fullIDCache[entityType.Name], id)
				} else {
					c.fullIDCache[entityType.Name][id] = true
				}
			}
		}
	} else if c.fullCacheLoaded[entityType.Name] {
		// using full cache and loaded full cache
		for id, box := range latest {
			if box.Data == nil {
				delete(c.fullCache[entityType.Name], id)
			} else {
				c.fullCache[entityType.Name][id] = box
			}
		}
		count := len(c.fullCache[entityType.Name])
		logger = logger.With("count", count)
		if count > c.fullCacheDataSizeLimit/dataSize {
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

func (c *CachedStore) GrowthAggregation(ctx context.Context, curBlockTime time.Time) error {
	return c.store.GrowthAggregation(ctx, c.chain, curBlockTime)
}

func (c *CachedStore) tryLoadFullCache(
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
	// first check count
	var count uint64
	count, err = c.store.CountEntity(ctx, entityType, c.chain)
	if err != nil {
		logger.Errore(err, "load entities from persistent for full cache failed: count exists failed")
		return
	}
	if count > uint64(c.fullCacheDataSizeLimit/dataSize) {
		logger.Warnw("too many entities in persistent, refuse to use full cache", "count", count)
		c.fullCacheRefused[entityType.Name] = true
		return false, false, nil
	}
	// then get all and put into full cache
	var data []*EntityBox
	data, err = c.store.ListEntities(ctx, entityType, c.chain, nil, math.MaxInt)
	logger = logger.With("used", time.Since(start).String())
	if err != nil {
		logger.Errore(err, "load entities from persistent for full cache failed")
		return false, false, err
	}
	c.fullCache[entityType.Name] = make(map[string]*EntityBox)
	for _, box := range data {
		c.fullCache[entityType.Name][box.ID] = box
	}
	c.fullCacheLoaded[entityType.Name] = true
	logger.Infow("loaded all entities from persistent into full cache", "count", len(data))
	return true, false, nil
}

func (c *CachedStore) tryLoadFullIDCache(ctx context.Context, entityType *schema.Entity) (loaded bool, err error) {
	loaded = c.fullIDCacheLoaded[entityType.Name]
	if loaded {
		return
	}
	start := time.Now()
	_, logger := log.FromContext(ctx, "entity", entityType.Name, "chainID", c.chain)
	logger.Debugf("will load all entity ids from persistent for full id cache")
	// then get all and put into full cache
	var ids []string
	ids, err = c.store.GetAllID(ctx, entityType, c.chain)
	logger = logger.With("used", time.Since(start).String())
	if err != nil {
		logger.Errore(err, "load all entity ids from persistent for full cache failed")
		return
	}
	c.fullIDCache[entityType.Name] = utils.BuildSet(ids)
	c.fullIDCacheLoaded[entityType.Name] = true
	logger.Infow("loaded all entity ids from persistent into full id cache", "count", len(ids))
	return
}

func (c *CachedStore) GetTimeSeriesMaxID(ctx context.Context, entityType *schema.Entity) (int64, error) {
	return c.store.GetMaxID(ctx, entityType, c.chain)
}

func (c *CachedStore) GetEntity(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
) (box *EntityBox, fromCache bool, err error) {
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
	var has bool
	if has, fromCache, err = c.tryLoadFullCache(ctx, entityType); err != nil {
		return
	} else if has {
		// load from full cache
		box = c.fullCache[entityType.Name][id].Copy()
		return
	}
	// full data is too large, use lru cache and full id cache
	// first check full id cache
	if fromCache, err = c.tryLoadFullIDCache(ctx, entityType); err != nil {
		return
	}
	if !c.fullIDCache[entityType.Name][id] {
		// not exists, just return nil
		return
	}
	// exist, try get from lru cache
	key := cacheKey(entityType.Name, id)
	var cache *EntityBox
	if cache, has = c.cache.Get(key); has {
		// make sure the changes over the returned box will now modify c.cache
		return cache.Copy(), fromCache, nil
	}
	// missing in lru cache, try load from persistent
	box, err = c.store.GetEntity(ctx, entityType, c.chain, id)
	if err == nil {
		c.cache.Add(key, box)
	}
	return box, false, err
}

func (c *CachedStore) ListEntities(
	ctx context.Context,
	entityType *schema.Entity,
	filters []EntityFilter,
	limit int,
) (boxes []*EntityBox, fromCache bool, err error) {
	if entityType.IsCache() {
		cache, has := c.cacheEntity[entityType.GetName()]
		if !has {
			return nil, true, nil
		}
		keys := cache.Keys()
		sort.Strings(keys)
		for _, key := range keys {
			box, _ := cache.Get(key)
			if pass, cke := checkFilters(filters, *box); cke != nil {
				return nil, false, cke
			} else if pass {
				boxes = append(boxes, box)
			}
			if len(boxes) >= limit {
				break
			}
		}
		return boxes, true, nil
	}

	var has bool
	if has, fromCache, err = c.tryLoadFullCache(ctx, entityType); err != nil {
		return
	} else if !has {
		// missing in full cache, try load from persistent
		boxes, err = c.store.ListEntities(ctx, entityType, c.chain, filters, limit)
		return
	}
	// has full cache, prepare data from it
	cache := make([]*EntityBox, 0, len(c.fullCache[entityType.Name]))
	for _, box := range c.fullCache[entityType.Name] {
		cache = append(cache, box)
	}
	sort.Slice(cache, func(i, j int) bool {
		return cache[i].ID < cache[j].ID
	})
	for _, box := range cache {
		if len(boxes) >= limit {
			break
		}
		if pass, cke := checkFilters(filters, *box); cke != nil {
			return nil, false, cke
		} else if pass {
			boxes = append(boxes, box)
		}
	}
	return
}

func (c *CachedStore) NewTxn() *Txn {
	txn := &Txn{
		start: time.Now(),
		report: TxnReport{
			TotalCommit:       make(map[string]int),
			TotalCommitCreate: make(map[string]int),
			TotalGetFrom:      make(map[string]map[string]int),
			TotalGetFromUsed:  make(map[string]map[string]time.Duration),
			TotalListFrom:     make(map[string]map[string]int),
			TotalListFromUsed: make(map[string]map[string]time.Duration),
		},
		storeCacheEvicted: c.cacheEvicted,
		recordMetric:      SimpleNoticeController{UsedMetric: c.usedMetric},
	}
	txn.Controller = NewController(c, txn)
	return txn
}

func (c *CachedStore) Snapshot() any {
	fullIDCache := make(map[string]int)
	for entity, loaded := range c.fullIDCacheLoaded {
		if loaded {
			fullIDCache[entity] = len(c.fullIDCache[entity])
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
				"sizeOverLimitRate": float64(dataSize*size) / float64(c.fullCacheDataSizeLimit),
			}
		}
	}
	for entity, refused := range c.fullCacheRefused {
		if refused {
			fullCache[entity] = map[string]any{
				"refused": true,
			}
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
			"fullCacheDataSizeLimit": c.fullCacheDataSizeLimit,
		},
		"cacheEntity": cacheEntity,
		"lruCache": map[string]any{
			"evicted": c.cacheEvicted,
			"size":    c.cache.Len(),
		},
		"fullIDCache": fullIDCache,
		"fullCache":   fullCache,
	}
}

func cacheKey(entity, id string) string {
	return entity + "/" + id
}
