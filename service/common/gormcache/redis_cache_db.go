package gormcache

import (
	"context"
	"fmt"
	"sentioxyz/sentio-core/common/log"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCacheDB struct {
	*AbstractCacheDB
	client        *redis.Client
	refreshTTL    time.Duration
	cacheNotFound bool // controls whether to cache not found query results
}

func NewRedisCacheDB(client *redis.Client, ttl time.Duration, refreshAheadFactor float64, cacheNotFound bool) CacheDB {
	db := AbstractCacheDB{
		TTL: ttl,
	}
	refreshTTL := time.Duration(-1)
	if refreshAheadFactor > 0 && refreshAheadFactor < 1 {
		refreshTTL = time.Duration(float64(ttl) * refreshAheadFactor)
	}

	return &RedisCacheDB{
		&db,
		client,
		refreshTTL,
		cacheNotFound,
	}
}

func (r *RedisCacheDB) AddQuery(ctx context.Context, cacheKey string, result *CachedResult) error {
	encoded, err := r.Encode(ctx, result)
	if err != nil {
		return err
	}
	if result.IsNotFound && !r.cacheNotFound {
		return nil // skip caching not found results
	}

	if err := r.client.Set(ctx, cacheKey, encoded, r.TTL).Err(); err != nil {
		return err
	}
	// refresh ahead enabled
	if r.refreshTTL > 0 {
		refreshKey := fmt.Sprintf("%s:refresh", cacheKey)
		err = r.client.Set(ctx, refreshKey, "", r.refreshTTL).Err()
	}

	return err
}

func (r *RedisCacheDB) GetQuery(ctx context.Context, cacheKey string) (*CachedResult, error) {
	encoded, err := r.client.Get(ctx, cacheKey).Bytes()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	} else if err != nil {
		return nil, err
	}
	// refresh ahead enabled, check if refresh is needed
	if r.refreshTTL > 0 {
		refreshKey := fmt.Sprintf("%s:refresh", cacheKey)
		err = r.client.SetArgs(ctx, refreshKey, "", redis.SetArgs{
			TTL: r.refreshTTL,
			Get: true,
		}).Err()
		if err == redis.Nil {
			return nil, ErrCacheMiss
		}
	}

	var result CachedResult
	if err := r.Decode(ctx, encoded, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (r *RedisCacheDB) AddRelation(ctx context.Context, queryKey string, rel *Relation) error {
	for _, key := range rel.GetKeys() {
		// Add the query key to the set associated with the relation key
		err := r.client.SAdd(ctx, key, queryKey).Err()
		if err != nil {
			return err
		}

		// Set the expiration for the relation key
		err = r.client.Expire(ctx, key, r.TTL).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *RedisCacheDB) InvalidateQuery(ctx context.Context, rel *Relation) error {
	for _, key := range rel.GetKeys() {
		// Get all relations for this query
		data, err := r.client.SMembers(ctx, key).Result()
		if err != nil {
			return err
		}

		if len(data) > 0 {
			err = r.client.Del(ctx, data...).Err()
			if err != nil {
				return err
			}
		}
		if err = r.client.Del(ctx, key).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (r *RedisCacheDB) ResetCache() {
	log.Warn("Resetting Redis cache, this should only be used for testing")
	//r.client.FlushDB(context.Background())
}
