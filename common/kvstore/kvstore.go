package kvstore

import "context"

type Store[T any] interface {
	List(ctx context.Context, ch chan<- string) error
	Get(ctx context.Context, keys ...string) (map[string]T, error)
	Set(ctx context.Context, kvs map[string]T) error
	Del(ctx context.Context, keys ...string) error
}
