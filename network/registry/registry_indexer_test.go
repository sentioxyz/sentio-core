package registry

import (
	"context"
	"testing"

	"sentioxyz/sentio-core/common/statemirror"
	"sentioxyz/sentio-core/network/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockIndexerMirror implements MirrorReadOnlyState for IndexerInfo
type mockIndexerMirror struct {
	data map[string]state.IndexerInfo
	err  error
}

func (m *mockIndexerMirror) Get(ctx context.Context, field string) (state.IndexerInfo, bool, error) {
	if m.err != nil {
		return state.IndexerInfo{}, false, m.err
	}
	info, ok := m.data[field]
	return info, ok, nil
}

func (m *mockIndexerMirror) MGet(ctx context.Context, fields ...string) (map[string]state.IndexerInfo, error) {
	result := make(map[string]state.IndexerInfo)
	for _, f := range fields {
		if info, ok := m.data[f]; ok {
			result[f] = info
		}
	}
	return result, nil
}

func (m *mockIndexerMirror) GetAll(ctx context.Context) (map[string]state.IndexerInfo, error) {
	return m.data, m.err
}

func (m *mockIndexerMirror) Scan(ctx context.Context, cursor uint64, match string, count int) (uint64, map[string]state.IndexerInfo, error) {
	return 0, m.data, m.err
}

// mockMirror implements statemirror.Mirror for testing
type mockMirrorForIndexer struct{}

func (m *mockMirrorForIndexer) Upsert(ctx context.Context, key statemirror.OnChainKey, syncF statemirror.SyncFunc) error {
	return nil
}

func (m *mockMirrorForIndexer) UpsertStreaming(ctx context.Context, key statemirror.OnChainKey, syncF statemirror.StreamingSyncFunc) error {
	return nil
}

func (m *mockMirrorForIndexer) Apply(ctx context.Context, key statemirror.OnChainKey, diffF statemirror.DiffFunc) error {
	return nil
}

func (m *mockMirrorForIndexer) Get(ctx context.Context, key statemirror.OnChainKey, field string) (value string, ok bool, err error) {
	return "", false, nil
}

func (m *mockMirrorForIndexer) MGet(ctx context.Context, key statemirror.OnChainKey, fields ...string) (map[string]string, error) {
	return nil, nil
}

func (m *mockMirrorForIndexer) GetAll(ctx context.Context, key statemirror.OnChainKey) (map[string]string, error) {
	return nil, nil
}

func (m *mockMirrorForIndexer) Scan(ctx context.Context, key statemirror.OnChainKey, cursor uint64, match string, count int) (nextCursor uint64, kv map[string]string, err error) {
	return 0, nil, nil
}

func TestNewIndexerRegistry_NilMirror(t *testing.T) {
	reg := NewIndexerRegistry(nil)
	assert.NotNil(t, reg)

	// Methods should return "mirror is nil" errors
	ctx := context.Background()
	_, err := reg.RetrieveIndexerInfo(ctx, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mirror is nil")
}

func TestIndexerRegistry_RetrieveIndexerInfo(t *testing.T) {
	ctx := context.Background()

	sampleIndexer := state.IndexerInfo{
		IndexerId:           42,
		IndexerUrl:          "https://indexer.example.com",
		ComputeNodeRpcPort:  8080,
		StorageNodeRpcPort:  9090,
		ClickhouseProxyPort: 8123,
		Signer:              "0xsigner",
	}

	tests := []struct {
		name        string
		indexerId   IndexerId
		data        map[string]state.IndexerInfo
		mirrorErr   error
		expected    state.IndexerInfo
		expectErr   bool
		errContains string
	}{
		{
			name:      "Successfully retrieve indexer info",
			indexerId: 42,
			data: map[string]state.IndexerInfo{
				"42": sampleIndexer,
			},
			expected: sampleIndexer,
		},
		{
			name:      "Indexer not found",
			indexerId: 404,
			data: map[string]state.IndexerInfo{
				"42": sampleIndexer,
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name:        "Mirror error propagates",
			indexerId:   1,
			mirrorErr:   assert.AnError,
			expectErr:   true,
			errContains: "failed to retrieve",
		},
		{
			name:      "Large indexer ID",
			indexerId: 18446744073709551615, // Max uint64
			data: map[string]state.IndexerInfo{
				"18446744073709551615": {
					IndexerId: 18446744073709551615,
				},
			},
			expected: state.IndexerInfo{
				IndexerId: 18446744073709551615,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexerMirror := &mockIndexerMirror{
				data: tt.data,
				err:  tt.mirrorErr,
			}
			reg := &indexerRegistry{
				mirror:        &mockMirrorForIndexer{},
				indexerMirror: indexerMirror,
			}

			result, err := reg.RetrieveIndexerInfo(ctx, tt.indexerId)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestIndexerRegistry_RetrieveAllIndexers(t *testing.T) {
	ctx := context.Background()

	indexer1 := state.IndexerInfo{
		IndexerId:  1,
		IndexerUrl: "https://indexer1.example.com",
	}
	indexer2 := state.IndexerInfo{
		IndexerId:  2,
		IndexerUrl: "https://indexer2.example.com",
	}
	indexer3 := state.IndexerInfo{
		IndexerId:  3,
		IndexerUrl: "https://indexer3.example.com",
	}

	tests := []struct {
		name        string
		data        map[string]state.IndexerInfo
		mirrorErr   error
		expected    map[IndexerId]state.IndexerInfo
		expectErr   bool
		errContains string
	}{
		{
			name: "Successfully retrieve all indexers",
			data: map[string]state.IndexerInfo{
				"1": indexer1,
				"2": indexer2,
				"3": indexer3,
			},
			expected: map[IndexerId]state.IndexerInfo{
				1: indexer1,
				2: indexer2,
				3: indexer3,
			},
		},
		{
			name:     "Empty indexers list",
			data:     map[string]state.IndexerInfo{},
			expected: map[IndexerId]state.IndexerInfo{},
		},
		{
			name:        "Mirror error propagates",
			mirrorErr:   assert.AnError,
			expectErr:   true,
			errContains: "failed to retrieve",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexerMirror := &mockIndexerMirror{
				data: tt.data,
				err:  tt.mirrorErr,
			}
			reg := &indexerRegistry{
				mirror:        &mockMirrorForIndexer{},
				indexerMirror: indexerMirror,
			}

			result, err := reg.RetrieveAllIndexers(ctx)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestIndexerRegistry_RetrieveAllIndexers_KeyConversion(t *testing.T) {
	ctx := context.Background()

	// Test that string keys are properly converted to IndexerId (uint64)
	data := map[string]state.IndexerInfo{
		"1":   {IndexerId: 1},
		"42":  {IndexerId: 42},
		"999": {IndexerId: 999},
	}

	indexerMirror := &mockIndexerMirror{data: data}
	reg := &indexerRegistry{
		mirror:        &mockMirrorForIndexer{},
		indexerMirror: indexerMirror,
	}

	result, err := reg.RetrieveAllIndexers(ctx)
	require.NoError(t, err)

	// Verify keys are properly typed as IndexerId
	assert.Len(t, result, 3)
	assert.Contains(t, result, IndexerId(1))
	assert.Contains(t, result, IndexerId(42))
	assert.Contains(t, result, IndexerId(999))

	// Verify the IndexerId field matches the key
	for id, info := range result {
		assert.Equal(t, uint64(id), info.IndexerId)
	}
}
