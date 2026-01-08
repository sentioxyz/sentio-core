package statemirror

import "context"

type SyncFunc func(ctx context.Context, key string) (map[string]string, error)
type StreamingSyncFunc func(ctx context.Context, key string, emit EmitFunc) error
type EmitFunc func(ctx context.Context, field, value string) error

type StateDiff struct {
	Added   map[string]string
	Deleted []string
}

type DiffFunc func(ctx context.Context, key string) (*StateDiff, error)

type Mirror interface {
	Upsert(ctx context.Context, key OnChainKey, syncF SyncFunc) error
	UpsertStreaming(ctx context.Context, key OnChainKey, syncF StreamingSyncFunc) error
	Apply(ctx context.Context, key OnChainKey, diffF DiffFunc)
	Get(ctx context.Context, key OnChainKey, field string) (value string, ok bool, err error)
	MGet(ctx context.Context, key OnChainKey, fields ...string) (map[string]string, error)
	GetAll(ctx context.Context, key OnChainKey) (map[string]string, error)
	Scan(ctx context.Context, key OnChainKey, cursor uint64, match string, count int) (
		nextCursor uint64, kv map[string]string, err error,
	)
}
