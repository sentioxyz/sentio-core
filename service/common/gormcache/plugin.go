package gormcache

import (
	"context"
	"flag"
	"sentioxyz/sentio-core/common/log"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Plugin struct {
	cache CacheDB
}

func NewGormCachePlugin(cache CacheDB) *Plugin {
	return &Plugin{cache}
}

func (p *Plugin) Name() string {
	return "gorm_cache_plugin"
}

type ModelWithCacheHints interface {
	CacheHints() []Relation
}

var ErrCacheHit = errors.New("cache hit")
var ErrCacheMiss = errors.New("cache miss")

func (p *Plugin) Initialize(db *gorm.DB) error {
	err := db.Callback().Query().Before("gorm:query").Register("gorm:cache:before_query", p.BeforeQuery())
	if err != nil {
		return err
	}
	err = db.Callback().Query().After("gorm:query").Register("gorm:cache:after_query", p.AfterQuery())
	if err != nil {
		return err
	}

	err = db.Callback().Update().After("*").Register("gorm:cache:after_update", p.AfterUpdate())
	if err != nil {
		return err
	}

	err = db.Callback().Create().After("gorm:before_create").Register("gorm:cache:after_create", p.AfterCreate())
	if err != nil {
		return err
	}

	err = db.Callback().Delete().After("gorm:before_create").Register("gorm:cache:after_create", p.AfterDelete())
	if err != nil {
		return err
	}

	return nil
}

var CacheTTL = flag.Duration("redis-cache-ttl", 5*time.Minute, "The redis cache TTL")
var RefreshAheadFactor = flag.Float64("redis-cache-refresh-ahead", 0.2, "The redis cache refresh ahead factor")
var CacheNotFound = flag.Bool("redis-cache-not-found", false, " caching not found results")

func SetupDBWithRedisCache(db *gorm.DB, redisClient *redis.Client) error {
	err := redisClient.Ping(context.Background()).Err()
	if err != nil {
		log.Errore(err, "failed to connect to redis, the database initialized without cache")
		return err
	}
	cacheDB := NewRedisCacheDB(redisClient, *CacheTTL, *RefreshAheadFactor, *CacheNotFound)
	err = db.Use(NewGormCachePlugin(cacheDB))

	return err
}
