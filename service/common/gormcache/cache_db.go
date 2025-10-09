package gormcache

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync/atomic"
	"time"
)

type CachedResult struct {
	AffectRows int64
	Data       []byte
	SQL        string
	ErrString  string
	IsNotFound bool // indicates if the result is not found
}
type CacheDB interface {
	AddQuery(ctx context.Context, cacheKey string, result *CachedResult) error
	GetQuery(ctx context.Context, cacheKey string) (*CachedResult, error)
	Decode(ctx context.Context, data []byte, dest any) error
	IncrCacheCount(hit bool)
	GetCacheCount() (int, int)
	ResetCacheCount() (int, int)
	Encode(ctx context.Context, dest any) ([]byte, error)
	GetCacheKey(table string, sql string, args []any, preloads map[string][]any) string
	AddRelation(ctx context.Context, cacheKey string, rel *Relation) error
	InvalidateQuery(ctx context.Context, rel *Relation) error
	SetExpiration(duration time.Duration)
	// ResetCache test only
	ResetCache()
}

type Cache interface {
	AddQuery(ctx context.Context, cacheKey string, result *CachedResult) error
	GetQuery(ctx context.Context, cacheKey string) (*CachedResult, error)
	AddRelation(ctx context.Context, cacheKey string, rel *Relation) error
	InvalidateQuery(ctx context.Context, rel *Relation) error
	ResetCache()
}

type Relation struct {
	TableName string
	Column    string
	Values    []any
}

func (rel *Relation) GetKeys() []string {
	var keys []string
	if len(rel.Values) == 0 {
		key := fmt.Sprintf("rel:%s:%s", rel.TableName, rel.Column)
		keys = append(keys, key)
	} else {
		for _, v := range rel.Values {
			key := fmt.Sprintf("rel:%s:%s:%v", rel.TableName, rel.Column, v)
			keys = append(keys, key)
		}
	}
	return keys
}

type AbstractCacheDB struct {
	TTL       time.Duration
	hitCount  atomic.Int64
	missCount atomic.Int64
}

func (l *AbstractCacheDB) GetCacheKey(table string, sql string, args []any, preloads map[string][]any) string {
	queryID := hashKey(sql)
	preloadStr := ""
	for preload := range preloads {
		preloadStr += preload + ","
	}
	argstr := ""
	for _, arg := range args {
		argstr += fmt.Sprintf("%v,", arg)
	}
	return fmt.Sprintf("query:%s:%s:[%s]:(%s)", table, queryID, preloadStr, argstr)
}

func (l *AbstractCacheDB) Decode(ctx context.Context, data []byte, dest any) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(dest)
}

func (l *AbstractCacheDB) Encode(ctx context.Context, dest any) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(dest); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (l *AbstractCacheDB) IncrCacheCount(hit bool) {
	if hit {
		l.hitCount.Add(1)
	} else {
		l.missCount.Add(1)
	}
}

func (l *AbstractCacheDB) SetExpiration(duration time.Duration) {
	l.TTL = duration
}

func (l *AbstractCacheDB) GetCacheCount() (int, int) {
	return int(l.hitCount.Load()), int(l.missCount.Load())
}

func (l *AbstractCacheDB) ResetCacheCount() (int, int) {
	hit := l.hitCount.Load()
	miss := l.missCount.Load()
	l.hitCount.Store(0)
	l.missCount.Store(0)
	return int(hit), int(miss)
}
