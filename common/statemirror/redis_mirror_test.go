package statemirror

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTestMirror(t *testing.T) (*redis.Client, Mirror, func()) {
	s := miniredis.RunT(t)
	cli := redis.NewClient(&redis.Options{Addr: s.Addr()})
	m := NewRedisMirror(cli)
	cleanup := func() {
		_ = cli.Close()
		s.Close()
	}
	return cli, m, cleanup
}

func TestRedisMirror_Get_Missing(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	v, ok, err := m.Get(ctx, MappingProcessorAllocations, "0xabc")
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, "", v)
}

func TestRedisMirror_Apply_Add_Delete(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	// seed
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key string) (map[string]string, error) {
		return map[string]string{"a": "1", "b": "2"}, nil
	}))

	m.Apply(ctx, MappingProcessorAllocations, func(ctx context.Context, key string) (*StateDiff, error) {
		return &StateDiff{
			Added:   map[string]string{"b": "22", "c": "3"},
			Deleted: []string{"a"},
		}, nil
	})

	all, err := m.GetAll(ctx, MappingProcessorAllocations)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"b": "22", "c": "3"}, all)
}

func TestRedisMirror_Upsert_DeletesStale(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, MappingIndexerInfos, func(ctx context.Context, key string) (map[string]string, error) {
		return map[string]string{"a": "1", "b": "2"}, nil
	}))

	require.NoError(t, m.Upsert(ctx, MappingIndexerInfos, func(ctx context.Context, key string) (map[string]string, error) {
		return map[string]string{"b": "20", "c": "3"}, nil
	}))

	all, err := m.GetAll(ctx, MappingIndexerInfos)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"b": "20", "c": "3"}, all)
}

func TestRedisMirror_UpsertStreaming_DeletesStale(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key string) (map[string]string, error) {
		return map[string]string{"a": "1", "b": "2", "c": "3"}, nil
	}))

	require.NoError(t, m.UpsertStreaming(ctx, MappingProcessorAllocations, func(ctx context.Context, key string, emit EmitFunc) error {
		if err := emit(ctx, "b", "22"); err != nil {
			return err
		}
		if err := emit(ctx, "d", "4"); err != nil {
			return err
		}
		return nil
	}))

	all, err := m.GetAll(ctx, MappingProcessorAllocations)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"b": "22", "d": "4"}, all)
}

func TestRedisMirror_Scan_All(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key string) (map[string]string, error) {
		return map[string]string{"a": "1", "b": "2", "c": "3"}, nil
	}))

	cursor := uint64(0)
	out := map[string]string{}
	for {
		next, kv, err := m.Scan(ctx, MappingProcessorAllocations, cursor, "", 1)
		require.NoError(t, err)
		for k, v := range kv {
			out[k] = v
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	require.Equal(t, map[string]string{"a": "1", "b": "2", "c": "3"}, out)
}

func TestRedisMirror_MGet_OmitsMissing(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key string) (map[string]string, error) {
		return map[string]string{"a": "1"}, nil
	}))

	got, err := m.MGet(ctx, MappingProcessorAllocations, "a", "b")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"a": "1"}, got)
}
