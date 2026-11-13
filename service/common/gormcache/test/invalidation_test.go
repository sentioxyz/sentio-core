package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
	"sentioxyz/sentio-core/service/common/gormcache"
)

func TestExplicitInvalidation(t *testing.T) {
	ctx := context.Background()
	cache := gormcache.NewLocalCacheDB(time.Hour)
	db := &gorm.DB{Config: &gorm.Config{Plugins: map[string]gorm.Plugin{
		"gorm_cache_plugin": gormcache.NewGormCachePlugin(cache),
	}}}
	for _, id := range []string{"changed", "untouched"} {
		if err := cache.AddQuery(ctx, id, &gormcache.CachedResult{}); err != nil {
			t.Fatal(err)
		}
		if err := cache.AddRelation(ctx, id, &gormcache.Relation{TableName: "items", Column: "id", Values: []any{id}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := gormcache.InvalidateQueries(ctx, db, gormcache.Relation{TableName: "items", Column: "id", Values: []any{"changed"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.GetQuery(ctx, "changed"); !errors.Is(err, gormcache.ErrCacheMiss) {
		t.Fatalf("changed query: %v", err)
	}
	if _, err := cache.GetQuery(ctx, "untouched"); err != nil {
		t.Fatalf("unrelated query was evicted: %v", err)
	}
	if err := gormcache.InvalidateQueries(ctx, &gorm.DB{Config: &gorm.Config{}}, gormcache.Relation{}); err != nil {
		t.Fatalf("disabled cache: %v", err)
	}
}

type failingInvalidationCache struct {
	gormcache.CacheDB
	err error
}

func (c failingInvalidationCache) InvalidateQuery(context.Context, *gormcache.Relation) error {
	return c.err
}

func TestExplicitInvalidationReturnsCacheFailure(t *testing.T) {
	want := errors.New("cache unavailable")
	db := &gorm.DB{Config: &gorm.Config{Plugins: map[string]gorm.Plugin{
		"gorm_cache_plugin": gormcache.NewGormCachePlugin(failingInvalidationCache{err: want}),
	}}}
	if err := gormcache.InvalidateQueries(context.Background(), db, gormcache.Relation{TableName: "items", Column: "id", Values: []any{"changed"}}); !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}
