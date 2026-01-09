package statemirror

import "context"

type TypedMirror[K comparable, V any] struct {
	m     Mirror
	key   OnChainKey
	codec StateCodec[K, V]
}

func NewTypedMirror[K comparable, V any](m Mirror, key OnChainKey, codec StateCodec[K, V]) MirrorReadOnlyState[K, V] {
	return TypedMirror[K, V]{m: m, key: key, codec: codec}
}

func (t TypedMirror[K, V]) Get(ctx context.Context, k K) (V, bool, error) {
	var zero V
	field, err := t.codec.Field(k)
	if err != nil {
		return zero, false, err
	}

	s, ok, err := t.m.Get(ctx, t.key, field)
	if err != nil || !ok {
		return zero, ok, err
	}

	v, err := t.codec.Decode(s)
	if err != nil {
		return zero, true, err
	}
	return v, true, nil
}

func (t TypedMirror[K, V]) MGet(ctx context.Context, ks ...K) (map[K]V, error) {
	fields := make([]string, 0, len(ks))
	idx := make(map[string]K, len(ks))
	for _, k := range ks {
		f, err := t.codec.Field(k)
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
		idx[f] = k
	}

	raw, err := t.m.MGet(ctx, t.key, fields...)
	if err != nil {
		return nil, err
	}

	out := make(map[K]V, len(raw))
	for f, s := range raw {
		k := idx[f]
		v, err := t.codec.Decode(s)
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

func (t TypedMirror[K, V]) GetAll(ctx context.Context) (map[K]V, error) {
	all, err := t.m.GetAll(ctx, t.key)
	if err != nil {
		return nil, err
	}

	out := make(map[K]V, len(all))
	for f, s := range all {
		k, err := t.codec.ParseField(f)
		if err != nil {
			return nil, err
		}
		v, err := t.codec.Decode(s)
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

func (t TypedMirror[K, V]) Scan(ctx context.Context, cursor uint64, match string, count int) (uint64, map[K]V, error) {
	next, raw, err := t.m.Scan(ctx, t.key, cursor, match, count)
	if err != nil {
		return 0, nil, err
	}

	out := make(map[K]V, len(raw))
	for f, s := range raw {
		k, err := t.codec.ParseField(f)
		if err != nil {
			return 0, nil, err
		}
		v, err := t.codec.Decode(s)
		if err != nil {
			return 0, nil, err
		}
		out[k] = v
	}
	return next, out, nil
}
