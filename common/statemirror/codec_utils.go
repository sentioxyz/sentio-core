package statemirror

import "context"

func BuildSyncFunc[K comparable, V any](
	codec StateCodec[K, V],
	fetch func(ctx context.Context, key OnChainKey) (map[K]V, error),
) SyncFunc {
	return func(ctx context.Context, onChainKey OnChainKey) (map[string]string, error) {
		m, err := fetch(ctx, onChainKey)
		if err != nil {
			return nil, err
		}

		out := make(map[string]string, len(m))
		for k, v := range m {
			field, err := codec.Field(k)
			if err != nil {
				return nil, err
			}
			val, err := codec.Encode(v)
			if err != nil {
				return nil, err
			}
			out[field] = val
		}
		return out, nil
	}
}

func BuildStreamingSyncFunc[K comparable, V any](
	codec StateCodec[K, V],
	stream func(ctx context.Context, key OnChainKey, emit func(K, V) error) error,
) StreamingSyncFunc {
	return func(ctx context.Context, onChainKey OnChainKey, emit EmitFunc) error {
		return stream(ctx, onChainKey, func(k K, v V) error {
			field, err := codec.Field(k)
			if err != nil {
				return err
			}
			val, err := codec.Encode(v)
			if err != nil {
				return err
			}
			return emit(ctx, field, val)
		})
	}
}

type TypedDiff[K comparable, V any] struct {
	Added   map[K]V
	Deleted []K
}

type TypedDiffFunc[K comparable, V any] func(ctx context.Context, key OnChainKey) (*TypedDiff[K, V], error)

func BuildDiffFunc[K comparable, V any](
	codec StateCodec[K, V],
	typed TypedDiffFunc[K, V],
) DiffFunc {
	return func(ctx context.Context, key OnChainKey) (*StateDiff, error) {
		d, err := typed(ctx, key)
		if err != nil {
			return nil, err
		}
		if d == nil {
			return &StateDiff{Added: map[string]string{}}, nil
		}

		out := &StateDiff{
			Added:   make(map[string]string, len(d.Added)),
			Deleted: make([]string, 0, len(d.Deleted)),
		}

		for k, v := range d.Added {
			field, err := codec.Field(k)
			if err != nil {
				return nil, err
			}
			val, err := codec.Encode(v)
			if err != nil {
				return nil, err
			}
			out.Added[field] = val
		}
		for _, k := range d.Deleted {
			field, err := codec.Field(k)
			if err != nil {
				return nil, err
			}
			out.Deleted = append(out.Deleted, field)
		}
		return out, nil
	}
}
