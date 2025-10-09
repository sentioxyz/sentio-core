package gormcache

import (
	"context"
	"crypto"
	"fmt"
	"time"

	"github.com/patrickmn/go-cache"
)

type LocalCache struct {
	*AbstractCacheDB
	cache *cache.Cache
}

func NewLocalCacheDB(ttl time.Duration) CacheDB {
	return &LocalCache{
		AbstractCacheDB: &AbstractCacheDB{
			TTL: ttl,
		},
		cache: cache.New(ttl, ttl),
	}
}

func hashKey(s string) string {
	//use md5 to generate a hash string
	h := crypto.MD5.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (l *LocalCache) Size() int {
	return l.cache.ItemCount()
}

func (l *LocalCache) AddQuery(ctx context.Context, cacheKey string, result *CachedResult) error {
	l.cache.Set(cacheKey, result, l.TTL)
	return nil
}

func (l *LocalCache) GetQuery(ctx context.Context, cacheKey string) (*CachedResult, error) {
	if val, found := l.cache.Get(cacheKey); found {
		if result, ok := val.(*CachedResult); ok {
			return result, nil
		}
	}
	return nil, ErrCacheMiss
}

func (l *LocalCache) AddRelation(ctx context.Context, queryKey string, rel *Relation) error {
	var relkeys []string
	if rel.Column == "*" {
		key := fmt.Sprintf("rel:%s:*", rel.TableName)
		relkeys = append(relkeys, key)
	} else {
		for _, k := range rel.Values {
			key := fmt.Sprintf("rel:%s:%s:%v", rel.TableName, rel.Column, k)
			relkeys = append(relkeys, key)
		}
	}
	for _, key := range relkeys {
		var queryKeys map[string]bool
		if keys, ok := l.cache.Get(key); ok {
			queryKeys, _ = keys.(map[string]bool)
		}
		if queryKeys == nil {
			queryKeys = make(map[string]bool)
		}
		queryKeys[queryKey] = true
		l.cache.Set(key, queryKeys, l.TTL)
	}
	return nil
}

func (l *LocalCache) InvalidateQuery(ctx context.Context, rel *Relation) error {
	var relkeys []string
	if rel.Column == "*" {
		key := fmt.Sprintf("rel:%s:*", rel.TableName)
		relkeys = append(relkeys, key)
	} else {
		for _, k := range rel.Values {
			key := fmt.Sprintf("rel:%s:%s:%v", rel.TableName, rel.Column, k)
			relkeys = append(relkeys, key)
		}
	}
	for _, key := range relkeys {
		if val, found := l.cache.Get(key); found {
			if queryKeys, ok := val.(map[string]bool); ok {
				for queryKey := range queryKeys {
					l.cache.Delete(queryKey)
				}
			}
			l.cache.Delete(key)
		}

	}
	return nil
}

func (l *LocalCache) ResetCache() {
	l.cache.Flush()
	l.ResetCacheCount()
}
