package statemirror

import "context"

type SyncFunc func(ctx context.Context, key OnChainKey) (map[string]string, error)
type StreamingSyncFunc func(ctx context.Context, key OnChainKey, emit EmitFunc) error
type EmitFunc func(ctx context.Context, field, value string) error

type StateDiff struct {
	Added   map[string]string
	Deleted []string
}

type DiffFunc func(ctx context.Context, key OnChainKey) (*StateDiff, error)

type Mirror interface {
	Upsert(ctx context.Context, key OnChainKey, syncF SyncFunc) error
	UpsertStreaming(ctx context.Context, key OnChainKey, syncF StreamingSyncFunc) error
	Apply(ctx context.Context, key OnChainKey, diffF DiffFunc) error
	Get(ctx context.Context, key OnChainKey, field string) (value string, ok bool, err error)
	MGet(ctx context.Context, key OnChainKey, fields ...string) (map[string]string, error)
	GetAll(ctx context.Context, key OnChainKey) (map[string]string, error)
	Scan(ctx context.Context, key OnChainKey, cursor uint64, match string, count int) (
		nextCursor uint64, kv map[string]string, err error,
	)
}

type StateCodec[K comparable, V any] interface {
	Field(k K) (string, error) // key -> field string
	ParseField(field string) (K, error)
	Encode(v V) (string, error) // value -> string
	Decode(s string) (V, error)
}
