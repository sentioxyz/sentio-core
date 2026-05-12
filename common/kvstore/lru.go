package kvstore

import (
	"context"
	lru "github.com/sentioxyz/golang-lru"
)

type LRUKVStore[T any] struct {
	cache *lru.Cache[string, T]
}

func NewLRUKVStore[T any](size int) (*LRUKVStore[T], error) {
	cache, err := lru.New[string, T](size)
	if err != nil {
		return nil, err
	}
	return &LRUKVStore[T]{cache: cache}, nil
}

func (s *LRUKVStore[T]) List(ctx context.Context, ch chan<- string) error {
	for _, key := range s.cache.Keys() {
		select {
		case ch <- key:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *LRUKVStore[T]) Get(ctx context.Context, keys ...string) (map[string]T, error) {
	r := make(map[string]T)
	for _, key := range keys {
		if value, has := s.cache.Get(key); has {
			r[key] = value
		}
	}
	return r, nil
}

func (s *LRUKVStore[T]) Set(ctx context.Context, kvs map[string]T) error {
	for key, val := range kvs {
		s.cache.Add(key, val)
	}
	return nil
}

func (s *LRUKVStore[T]) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		s.cache.Remove(key)
	}
	return nil
}
