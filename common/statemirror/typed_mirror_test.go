package statemirror

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type processorProperties struct {
	ProcessorId string `json:"processor_id" db:"processor_id"`

	Version     int32     `json:"version" db:"version"`
	ShardingIdx int32     `json:"sharding_idx" db:"sharding_idx"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

var exampleJSONCodec = JSONCodec[string, processorProperties]{
	FieldFunc: func(k string) (string, error) {
		return fmt.Sprintf("k:%s", k), nil
	},
	ParseFunc: func(s string) (string, error) {
		return strings.TrimPrefix(s, "k:"), nil
	},
}

var exampleJSONCodecWithFieldErr = JSONCodec[string, processorProperties]{
	FieldFunc: func(k string) (string, error) {
		return "", fmt.Errorf("field error for key %q", k)
	},
	ParseFunc: func(s string) (string, error) {
		return strings.TrimPrefix(s, "k:"), nil
	},
}

type testCodec struct {
	fieldErrKey  int
	decodeErrRaw string
}

func (c testCodec) Field(k int) (string, error) {
	if k == c.fieldErrKey {
		return "", fmt.Errorf("field error for key %d", k)
	}
	return fmt.Sprintf("k:%d", k), nil
}

func (c testCodec) ParseField(field string) (int, error) {
	if !strings.HasPrefix(field, "k:") {
		return 0, fmt.Errorf("bad field: %q", field)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(field, "k:"))
	if err != nil {
		return 0, fmt.Errorf("bad field: %q", field)
	}
	return n, nil
}

func (c testCodec) Encode(v int) (string, error) {
	return fmt.Sprintf("v:%d", v), nil
}

func (c testCodec) Decode(s string) (int, error) {
	if c.decodeErrRaw != "" && s == c.decodeErrRaw {
		return 0, fmt.Errorf("decode error for %q", s)
	}
	if !strings.HasPrefix(s, "v:") {
		return 0, fmt.Errorf("bad value: %q", s)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(s, "v:"))
	if err != nil {
		return 0, fmt.Errorf("bad value: %q", s)
	}
	return n, nil
}

func TestTypedMirror_Get_Missing(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	tm := TypedMirror[string, processorProperties]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: exampleJSONCodec,
	}

	ctx := context.Background()
	v, ok, err := tm.Get(ctx, "processor-id1")
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, processorProperties{}, v)
}

func TestTypedMirror_Get_OK(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	fetch := func(ctx context.Context, key OnChainKey) (map[string]processorProperties, error) {
		return map[string]processorProperties{
			"processor-id1": {
				ProcessorId: "processor-id1",
				Version:     1,
				ShardingIdx: 1,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			"processor-id2": {
				ProcessorId: "processor-id2",
				Version:     2,
				ShardingIdx: 2,
				CreatedAt:   time.Now(),
			},
		}, nil
	}

	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, BuildSyncFunc[string, processorProperties](exampleJSONCodec, fetch)))

	tm := TypedMirror[string, processorProperties]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: exampleJSONCodec,
	}

	v, ok, err := tm.Get(ctx, "processor-id1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "processor-id1", v.ProcessorId)
}

func TestTypedMirror_Get_CodecFieldError(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	tm := TypedMirror[string, processorProperties]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: exampleJSONCodecWithFieldErr,
	}

	ctx := context.Background()
	v, ok, err := tm.Get(ctx, "processor-id1")
	require.Error(t, err)
	require.False(t, ok)
	require.Equal(t, processorProperties{}, v)
}

func TestTypedMirror_Get_DecodeError(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"k:1": "v:bad"}, nil
	}))

	tm := TypedMirror[int, int]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: testCodec{},
	}

	v, ok, err := tm.Get(ctx, 1)
	require.Error(t, err)
	require.True(t, ok)
	require.Equal(t, 0, v)
}

func TestTypedMirror_MGet_OmitsMissing(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"k:1": "v:10"}, nil
	}))

	tm := TypedMirror[int, int]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: testCodec{},
	}

	got, err := tm.MGet(ctx, 1, 2)
	require.NoError(t, err)
	require.Equal(t, map[int]int{1: 10}, got)
}

func TestTypedMirror_MGet_EmptyInput(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	tm := TypedMirror[int, int]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: testCodec{},
	}

	ctx := context.Background()
	got, err := tm.MGet(ctx)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestTypedMirror_MGet_CodecFieldError(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	tm := TypedMirror[int, int]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: testCodec{fieldErrKey: 7},
	}

	ctx := context.Background()
	_, err := tm.MGet(ctx, 1, 7)
	require.Error(t, err)
}

func TestTypedMirror_MGet_DecodeError(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"k:1": "v:10", "k:2": "v:bad"}, nil
	}))

	tm := TypedMirror[int, int]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: testCodec{},
	}

	_, err := tm.MGet(ctx, 1, 2)
	require.Error(t, err)
}

func TestTypedMirror_GetAll_OK(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"k:1": "v:10", "k:2": "v:20"}, nil
	}))

	tm := TypedMirror[int, int]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: testCodec{},
	}

	got, err := tm.GetAll(ctx, MappingProcessorAllocations)
	require.NoError(t, err)
	require.Equal(t, map[int]int{1: 10, 2: 20}, got)
}

func TestTypedMirror_GetAll_ParseFieldError(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"bad": "v:10"}, nil
	}))

	tm := TypedMirror[int, int]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: testCodec{},
	}

	_, err := tm.GetAll(ctx, MappingProcessorAllocations)
	require.Error(t, err)
}

func TestTypedMirror_Scan_All(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"k:1": "v:10", "k:2": "v:20", "k:3": "v:30"}, nil
	}))

	tm := TypedMirror[int, int]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: testCodec{},
	}

	cursor := uint64(0)
	out := map[int]int{}
	for {
		next, kv, err := tm.Scan(ctx, cursor, "", 1)
		require.NoError(t, err)
		for k, v := range kv {
			out[k] = v
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	require.Equal(t, map[int]int{1: 10, 2: 20, 3: 30}, out)
}

func TestTypedMirror_Scan_Match(t *testing.T) {
	_, m, cleanup := newTestMirror(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"k:1": "v:10", "k:2": "v:20", "k:30": "v:300"}, nil
	}))

	tm := TypedMirror[int, int]{
		m:     m,
		key:   MappingProcessorAllocations,
		codec: testCodec{},
	}

	cursor := uint64(0)
	out := map[int]int{}
	for {
		next, kv, err := tm.Scan(ctx, cursor, "k:3*", 10)
		require.NoError(t, err)
		for k, v := range kv {
			out[k] = v
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	require.Equal(t, map[int]int{30: 300}, out)
}
