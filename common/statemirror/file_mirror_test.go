package statemirror

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestFileMirror(t *testing.T) (Mirror, func()) {
	tmpDir, err := os.MkdirTemp("", "file_mirror_test_*")
	require.NoError(t, err)

	m, err := NewFileMirror(tmpDir)
	require.NoError(t, err)

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	return m, cleanup
}

func TestFileMirror_Get_Missing(t *testing.T) {
	m, cleanup := newTestFileMirror(t)
	defer cleanup()

	ctx := context.Background()
	v, ok, err := m.Get(ctx, MappingProcessorAllocations, "0xabc")
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, "", v)
}

func TestFileMirror_Apply_Add_Delete(t *testing.T) {
	m, cleanup := newTestFileMirror(t)
	defer cleanup()

	ctx := context.Background()
	// seed
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"a": "1", "b": "2"}, nil
	}))

	err := m.Apply(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (*StateDiff, error) {
		return &StateDiff{
			Added:   map[string]string{"b": "22", "c": "3"},
			Deleted: []string{"a"},
		}, nil
	})
	require.NoError(t, err)

	all, err := m.GetAll(ctx, MappingProcessorAllocations)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"b": "22", "c": "3"}, all)
}

func TestFileMirror_Upsert_RemovesOldFields(t *testing.T) {
	m, cleanup := newTestFileMirror(t)
	defer cleanup()

	ctx := context.Background()

	// Initial data
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"a": "1", "b": "2", "c": "3"}, nil
	}))

	all, err := m.GetAll(ctx, MappingProcessorAllocations)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"a": "1", "b": "2", "c": "3"}, all)

	// Update with fewer fields - should remove "c"
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"a": "1", "b": "22"}, nil
	}))

	all, err = m.GetAll(ctx, MappingProcessorAllocations)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"a": "1", "b": "22"}, all)
}

func TestFileMirror_UpsertStreaming(t *testing.T) {
	m, cleanup := newTestFileMirror(t)
	defer cleanup()

	ctx := context.Background()

	err := m.UpsertStreaming(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey, emit EmitFunc) error {
		for i := 0; i < 100; i++ {
			if err := emit(ctx, string(rune('a'+i%26)), string(rune('0'+i%10))); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)

	all, err := m.GetAll(ctx, MappingProcessorAllocations)
	require.NoError(t, err)
	// Should have 26 unique keys (a-z)
	require.Equal(t, 26, len(all))
}

func TestFileMirror_MGet(t *testing.T) {
	m, cleanup := newTestFileMirror(t)
	defer cleanup()

	ctx := context.Background()

	// Seed data
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"a": "1", "b": "2", "c": "3"}, nil
	}))

	// Test MGet with existing and non-existing fields
	result, err := m.MGet(ctx, MappingProcessorAllocations, "a", "c", "d")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"a": "1", "c": "3"}, result)

	// Test MGet with empty fields
	result, err = m.MGet(ctx, MappingProcessorAllocations)
	require.NoError(t, err)
	require.Equal(t, map[string]string{}, result)
}

func TestFileMirror_Scan(t *testing.T) {
	m, cleanup := newTestFileMirror(t)
	defer cleanup()

	ctx := context.Background()

	// Seed data
	data := map[string]string{
		"user:1":  "alice",
		"user:2":  "bob",
		"user:3":  "charlie",
		"admin:1": "dave",
		"admin:2": "eve",
	}
	require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return data, nil
	}))

	// Test scan all
	cursor := uint64(0)
	allScanned := make(map[string]string)
	iterations := 0
	for {
		next, kv, err := m.Scan(ctx, MappingProcessorAllocations, cursor, "", 10)
		require.NoError(t, err)
		iterations++

		for k, v := range kv {
			allScanned[k] = v
		}

		if next == 0 {
			break
		}
		cursor = next
		require.Less(t, iterations, 10, "too many iterations")
	}
	require.Equal(t, data, allScanned)

	// Test scan with pattern
	cursor = 0
	userScanned := make(map[string]string)
	for {
		next, kv, err := m.Scan(ctx, MappingProcessorAllocations, cursor, "user:*", 10)
		require.NoError(t, err)

		for k, v := range kv {
			userScanned[k] = v
		}

		if next == 0 {
			break
		}
		cursor = next
	}
	require.Equal(t, 3, len(userScanned))
	require.Equal(t, "alice", userScanned["user:1"])
	require.Equal(t, "bob", userScanned["user:2"])
	require.Equal(t, "charlie", userScanned["user:3"])
}

func TestFileMirror_MultipleKeys(t *testing.T) {
	m, cleanup := newTestFileMirror(t)
	defer cleanup()

	ctx := context.Background()

	// Store data for different keys
	require.NoError(t, m.Upsert(ctx, "Key1", func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"a": "1", "b": "2"}, nil
	}))

	require.NoError(t, m.Upsert(ctx, "Key2", func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"x": "10", "y": "20"}, nil
	}))

	// Verify Key1
	all1, err := m.GetAll(ctx, "Key1")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"a": "1", "b": "2"}, all1)

	// Verify Key2
	all2, err := m.GetAll(ctx, "Key2")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"x": "10", "y": "20"}, all2)
}

func TestFileMirror_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file_mirror_test_*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// Create first mirror and write data
	{
		m, err := NewFileMirror(tmpDir)
		require.NoError(t, err)

		require.NoError(t, m.Upsert(ctx, MappingProcessorAllocations, func(ctx context.Context, key OnChainKey) (map[string]string, error) {
			return map[string]string{"a": "1", "b": "2"}, nil
		}))
	}

	// Create second mirror and verify data persists
	{
		m, err := NewFileMirror(tmpDir)
		require.NoError(t, err)

		all, err := m.GetAll(ctx, MappingProcessorAllocations)
		require.NoError(t, err)
		require.Equal(t, map[string]string{"a": "1", "b": "2"}, all)
	}
}

func TestFileMirror_WithOptions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "file_mirror_test_*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	customDir := filepath.Join(tmpDir, "custom")
	m, err := NewFileMirror(tmpDir,
		WithBaseDir(customDir),
		WithFileExtension(".data"),
	)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, m.Upsert(ctx, "TestKey", func(ctx context.Context, key OnChainKey) (map[string]string, error) {
		return map[string]string{"a": "1"}, nil
	}))

	// Verify file exists with custom extension in custom directory
	expectedFile := filepath.Join(customDir, "TestKey.data")
	_, err = os.Stat(expectedFile)
	require.NoError(t, err)
}
