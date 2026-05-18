package registry

import (
	"context"
	"testing"

	"sentioxyz/sentio-core/common/statemirror"
	"sentioxyz/sentio-core/network/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProcessorInfoMirror implements MirrorReadOnlyState for ProcessorInfo
type mockProcessorInfoMirror struct {
	data map[string]state.ProcessorInfo
	err  error
}

func (m *mockProcessorInfoMirror) Get(ctx context.Context, field string) (state.ProcessorInfo, bool, error) {
	if m.err != nil {
		return state.ProcessorInfo{}, false, m.err
	}
	info, ok := m.data[field]
	return info, ok, nil
}

func (m *mockProcessorInfoMirror) MGet(ctx context.Context, fields ...string) (map[string]state.ProcessorInfo, error) {
	result := make(map[string]state.ProcessorInfo)
	for _, f := range fields {
		if info, ok := m.data[f]; ok {
			result[f] = info
		}
	}
	return result, nil
}

func (m *mockProcessorInfoMirror) GetAll(ctx context.Context) (map[string]state.ProcessorInfo, error) {
	return m.data, m.err
}

func (m *mockProcessorInfoMirror) Scan(ctx context.Context, cursor uint64, match string, count int) (uint64, map[string]state.ProcessorInfo, error) {
	return 0, m.data, m.err
}

// mockProcessorAllocationMirror implements MirrorReadOnlyState for []ProcessorAllocation
type mockProcessorAllocationMirror struct {
	data map[string][]state.ProcessorAllocation
	err  error
}

func (m *mockProcessorAllocationMirror) Get(ctx context.Context, field string) ([]state.ProcessorAllocation, bool, error) {
	if m.err != nil {
		return nil, false, m.err
	}
	allocs, ok := m.data[field]
	return allocs, ok, nil
}

func (m *mockProcessorAllocationMirror) MGet(ctx context.Context, fields ...string) (map[string][]state.ProcessorAllocation, error) {
	result := make(map[string][]state.ProcessorAllocation)
	for _, f := range fields {
		if allocs, ok := m.data[f]; ok {
			result[f] = allocs
		}
	}
	return result, nil
}

func (m *mockProcessorAllocationMirror) GetAll(ctx context.Context) (map[string][]state.ProcessorAllocation, error) {
	return m.data, m.err
}

func (m *mockProcessorAllocationMirror) Scan(ctx context.Context, cursor uint64, match string, count int) (uint64, map[string][]state.ProcessorAllocation, error) {
	return 0, m.data, m.err
}


// mockMirror implements statemirror.Mirror for testing
type mockMirrorForProcessor struct{}

func (m *mockMirrorForProcessor) Upsert(ctx context.Context, key statemirror.OnChainKey, syncF statemirror.SyncFunc) error {
	return nil
}

func (m *mockMirrorForProcessor) UpsertStreaming(ctx context.Context, key statemirror.OnChainKey, syncF statemirror.StreamingSyncFunc) error {
	return nil
}

func (m *mockMirrorForProcessor) Apply(ctx context.Context, key statemirror.OnChainKey, diffF statemirror.DiffFunc) error {
	return nil
}

func (m *mockMirrorForProcessor) Get(ctx context.Context, key statemirror.OnChainKey, field string) (value string, ok bool, err error) {
	return "", false, nil
}

func (m *mockMirrorForProcessor) MGet(ctx context.Context, key statemirror.OnChainKey, fields ...string) (map[string]string, error) {
	return nil, nil
}

func (m *mockMirrorForProcessor) GetAll(ctx context.Context, key statemirror.OnChainKey) (map[string]string, error) {
	return nil, nil
}

func (m *mockMirrorForProcessor) Scan(ctx context.Context, key statemirror.OnChainKey, cursor uint64, match string, count int) (nextCursor uint64, kv map[string]string, err error) {
	return 0, nil, nil
}

func TestNewProcessorRegistry_NilMirror(t *testing.T) {
	reg := NewProcessorRegistry(nil)
	assert.NotNil(t, reg)

	// Methods should return "not initialized" errors
	ctx := context.Background()
	_, err := reg.RetrieveProcessorInfo(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestProcessorRegistry_RetrieveProcessorInfo(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		processorId ProcessorId
		data        map[string]state.ProcessorInfo
		mirrorErr   error
		expected    state.ProcessorInfo
		expectErr   bool
		errContains string
	}{
		{
			name:        "Successfully retrieve processor info",
			processorId: "proc_123",
			data: map[string]state.ProcessorInfo{
				"proc_123": {
					ProcessorId:         "proc_123",
					EntitySchema:        "schema_v1",
					EntitySchemaVersion: 1,
				},
			},
			expected: state.ProcessorInfo{
				ProcessorId:         "proc_123",
				EntitySchema:        "schema_v1",
				EntitySchemaVersion: 1,
			},
		},
		{
			name:        "Processor not found",
			processorId: "proc_404",
			data: map[string]state.ProcessorInfo{
				"proc_123": {},
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name:        "Mirror error propagates",
			processorId: "proc_error",
			mirrorErr:   assert.AnError,
			expectErr:   true,
			errContains: "failed to get processor info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			infoMirror := &mockProcessorInfoMirror{
				data: tt.data,
				err:  tt.mirrorErr,
			}
			reg := &processorRegistry{
				mirror:              &mockMirrorForProcessor{},
				processorInfoMirror: infoMirror,
			}

			result, err := reg.RetrieveProcessorInfo(ctx, tt.processorId)
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

func TestProcessorRegistry_RetrieveProcessorAllocations(t *testing.T) {
	ctx := context.Background()

	sampleAllocations := []state.ProcessorAllocation{
		{ProcessorId: "proc_123", IndexerId: 1},
		{ProcessorId: "proc_123", IndexerId: 2},
		{ProcessorId: "proc_123", IndexerId: 3},
	}

	tests := []struct {
		name        string
		processorId ProcessorId
		data        map[string][]state.ProcessorAllocation
		mirrorErr   error
		expected    []state.ProcessorAllocation
		expectErr   bool
		errContains string
	}{
		{
			name:        "Successfully retrieve allocations",
			processorId: "proc_123",
			data: map[string][]state.ProcessorAllocation{
				"proc_123": sampleAllocations,
			},
			expected: sampleAllocations,
		},
		{
			name:        "Empty allocations list",
			processorId: "proc_empty",
			data: map[string][]state.ProcessorAllocation{
				"proc_empty": {},
			},
			expected: []state.ProcessorAllocation{},
		},
		{
			name:        "Allocations not found",
			processorId: "proc_404",
			data: map[string][]state.ProcessorAllocation{
				"proc_123": sampleAllocations,
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name:        "Mirror error propagates",
			processorId: "proc_error",
			mirrorErr:   assert.AnError,
			expectErr:   true,
			errContains: "failed to get processor allocation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allocMirror := &mockProcessorAllocationMirror{
				data: tt.data,
				err:  tt.mirrorErr,
			}
			reg := &processorRegistry{
				mirror: &mockMirrorForProcessor{},
				processorAllocationMirror: allocMirror,
			}

			result, err := reg.RetrieveProcessorAllocations(ctx, tt.processorId)
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
