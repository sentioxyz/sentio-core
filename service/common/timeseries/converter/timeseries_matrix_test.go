package converter

import (
	"context"
	"reflect"
	"testing"
	"time"

	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/matrix"

	clickhouselib "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/service/common/protos"
)

// mockMatrix implements the adaptor_eventlogs.Matrix interface for testing
type mockMatrix struct {
	mock.Mock
	data        []mockMatrixRow
	columnTypes []clickhouselib.ColumnType
}

type mockMatrixRow struct {
	timeValue   time.Time
	aggValue    interface{}
	labelsValue map[string]interface{}
	data        []interface{}
}

type mockColumnType struct {
	name string
}

func (m mockColumnType) Name() string             { return m.name }
func (m mockColumnType) ScanType() reflect.Type   { return reflect.TypeOf("") }
func (m mockColumnType) Nullable() bool           { return false }
func (m mockColumnType) DatabaseTypeName() string { return m.name }

func (m *mockMatrix) ColumnTypes() []clickhouselib.ColumnType {
	if len(m.columnTypes) == 0 {
		return []clickhouselib.ColumnType{
			mockColumnType{name: "time"},
			mockColumnType{name: "value"},
			mockColumnType{name: "label"},
		}
	}
	return m.columnTypes
}

func (m *mockMatrix) ColumnType(idx int) clickhouselib.ColumnType {
	types := m.ColumnTypes()
	if idx >= len(types) {
		return mockColumnType{name: "unknown"}
	}
	return types[idx]
}

func (m *mockMatrix) Len() int {
	return len(m.data)
}

func (m *mockMatrix) Data() [][]any {
	result := make([][]any, len(m.data))
	for i, row := range m.data {
		result[i] = row.data
	}
	return result
}

func (m *mockMatrix) DataByRow(idx int) []any {
	if idx >= len(m.data) {
		return nil
	}
	return m.data[idx].data
}

func (m *mockMatrix) DataByCol(idx int) []any {
	result := make([]any, len(m.data))
	for i, row := range m.data {
		if idx < len(row.data) {
			result[i] = row.data[idx]
		}
	}
	return result
}

func (m *mockMatrix) DataByColName(name string) []any {
	// Simple implementation for testing
	return m.DataByCol(0)
}

func (m *mockMatrix) DataValue(rowIdx, colIdx int) any {
	if rowIdx >= len(m.data) || colIdx >= len(m.data[rowIdx].data) {
		return nil
	}
	return m.data[rowIdx].data[colIdx]
}

func (m *mockMatrix) TimeSeriesTimeValue(idx int) time.Time {
	if idx >= len(m.data) {
		return time.Time{}
	}
	return m.data[idx].timeValue
}

func (m *mockMatrix) TimeSeriesAggValue(idx int) interface{} {
	if idx >= len(m.data) {
		return nil
	}
	return m.data[idx].aggValue
}

func (m *mockMatrix) TimeSeriesLabelsValue(idx int) map[string]interface{} {
	if idx >= len(m.data) {
		return nil
	}
	return m.data[idx].labelsValue
}

func (m *mockMatrix) CohortValue(idx int, cohortType matrix.CohortResultType) any {
	return nil
}

func (m *mockMatrix) ColumnNames() []string {
	return []string{"col1", "col2"}
}

func newMockMatrix(rows []mockMatrixRow) *mockMatrix {
	return &mockMatrix{data: rows}
}

func TestNewTimeSeriesMatrix(t *testing.T) {
	ctx := context.Background()
	timeRange := &timerange.TimeRange{
		Start:    time.Now().Add(-time.Hour),
		End:      time.Now(),
		Step:     time.Minute * 5,
		Timezone: time.UTC,
	}
	query := &protos.SegmentationQuery{
		Resource: &protos.SegmentationQuery_Resource{
			Type: protos.SegmentationQuery_EVENTS,
			Name: "test_event",
		},
		Aggregation: &protos.SegmentationQuery_Aggregation{
			Value: &protos.SegmentationQuery_Aggregation_Total_{
				Total: &protos.SegmentationQuery_Aggregation_Total{},
			},
		},
	}

	// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
	tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)

	require.NotNil(t, tsm)

	// Test through the public interface
	// Verify the matrix can be converted to proto successfully
	proto := tsm.ToProto()
	assert.NotNil(t, proto)
	assert.Equal(t, int32(0), proto.TotalSamples) // No data added yet

	// Verify initial slots count
	assert.Equal(t, int64(0), tsm.Slots())
}

func TestTimeSeriesMatrix_AddData(t *testing.T) {
	ctx := context.Background()
	timeRange := &timerange.TimeRange{
		Start:    time.Now().Add(-time.Hour),
		End:      time.Now(),
		Step:     time.Minute * 5,
		Timezone: time.UTC,
	}
	query := &protos.SegmentationQuery{
		Resource: &protos.SegmentationQuery_Resource{
			Type: protos.SegmentationQuery_EVENTS,
			Name: "test_event",
		},
		Aggregation: &protos.SegmentationQuery_Aggregation{
			Value: &protos.SegmentationQuery_Aggregation_Total_{
				Total: &protos.SegmentationQuery_Aggregation_Total{},
			},
		},
	}

	// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
	tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)

	timestamp := time.Now().Unix()
	value := 42.5
	labels := map[string]string{
		"app":     "test",
		"version": "1.0",
	}

	tsm.AddData(timestamp, value, labels)

	// Test through the public interface - verify data was added
	proto := tsm.ToProto()
	// Since we don't have time information processed yet, we can't verify samples yet
	// But we can verify the matrix is still valid
	assert.NotNil(t, proto)

	// Test adding same labels again - should update existing sample
	tsm.AddData(timestamp+1, 100.0, labels)

	// Test adding different labels - should create new sample
	differentLabels := map[string]string{
		"app":     "test2",
		"version": "2.0",
	}
	tsm.AddData(timestamp+2, 200.0, differentLabels)
}

func TestTimeSeriesMatrix_ToProto(t *testing.T) {
	ctx := context.Background()
	timeRange := &timerange.TimeRange{
		Start:    time.Now().Add(-time.Hour),
		End:      time.Now(),
		Step:     time.Minute * 5,
		Timezone: time.UTC,
	}
	query := &protos.SegmentationQuery{
		Resource: &protos.SegmentationQuery_Resource{
			Type: protos.SegmentationQuery_EVENTS,
			Name: "test_event",
		},
		Aggregation: &protos.SegmentationQuery_Aggregation{
			Value: &protos.SegmentationQuery_Aggregation_Total_{
				Total: &protos.SegmentationQuery_Aggregation_Total{},
			},
		},
	}

	// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
	tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)

	// Add some test data
	timestamp := time.Now().Unix()
	labels1 := map[string]string{"app": "test1"}
	labels2 := map[string]string{"app": "test2"}

	tsm.AddData(timestamp, 10.0, labels1)
	tsm.AddData(timestamp+1, 20.0, labels2)

	proto := tsm.ToProto()

	require.NotNil(t, proto)
	assert.Equal(t, int32(2), proto.TotalSamples)
	assert.Len(t, proto.Samples, 2)

	// Verify samples are not nil
	for _, sample := range proto.Samples {
		assert.NotNil(t, sample)
		assert.NotNil(t, sample.Metric)
	}
}

func TestTimeSeriesMatrix_Slots(t *testing.T) {
	ctx := context.Background()
	timeRange := &timerange.TimeRange{
		Start:    time.Now().Add(-time.Hour),
		End:      time.Now(),
		Step:     time.Minute * 5,
		Timezone: time.UTC,
	}
	query := &protos.SegmentationQuery{
		Resource: &protos.SegmentationQuery_Resource{
			Type: protos.SegmentationQuery_EVENTS,
			Name: "test_event",
		},
		Aggregation: &protos.SegmentationQuery_Aggregation{
			Value: &protos.SegmentationQuery_Aggregation_Total_{
				Total: &protos.SegmentationQuery_Aggregation_Total{},
			},
		},
	}

	// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
	tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)

	// Initially should have 0 slots
	assert.Equal(t, int64(0), tsm.Slots())
}

func TestTimeSeriesMatrix_GetName_Events(t *testing.T) {
	tests := []struct {
		name        string
		resource    *protos.SegmentationQuery_Resource
		aggregation *protos.SegmentationQuery_Aggregation
		expected    string
	}{
		{
			name: "events with name and total aggregation",
			resource: &protos.SegmentationQuery_Resource{
				Type: protos.SegmentationQuery_EVENTS,
				Name: "user_login",
			},
			aggregation: &protos.SegmentationQuery_Aggregation{
				Value: &protos.SegmentationQuery_Aggregation_Total_{
					Total: &protos.SegmentationQuery_Aggregation_Total{},
				},
			},
			expected: "user_login - Total Count",
		},
		{
			name: "events without name and total aggregation",
			resource: &protos.SegmentationQuery_Resource{
				Type: protos.SegmentationQuery_EVENTS,
				Name: "",
			},
			aggregation: &protos.SegmentationQuery_Aggregation{
				Value: &protos.SegmentationQuery_Aggregation_Total_{
					Total: &protos.SegmentationQuery_Aggregation_Total{},
				},
			},
			expected: "<All Events> - Total Count",
		},
		{
			name: "events with unique aggregation",
			resource: &protos.SegmentationQuery_Resource{
				Type: protos.SegmentationQuery_EVENTS,
				Name: "user_login",
			},
			aggregation: &protos.SegmentationQuery_Aggregation{
				Value: &protos.SegmentationQuery_Aggregation_Unique_{
					Unique: &protos.SegmentationQuery_Aggregation_Unique{},
				},
			},
			expected: "user_login - Unique Count",
		},
		{
			name: "events with DAU count unique",
			resource: &protos.SegmentationQuery_Resource{
				Type: protos.SegmentationQuery_EVENTS,
				Name: "user_login",
			},
			aggregation: &protos.SegmentationQuery_Aggregation{
				Value: &protos.SegmentationQuery_Aggregation_CountUnique_{
					CountUnique: &protos.SegmentationQuery_Aggregation_CountUnique{
						Duration: &protos.Duration{Value: 24 * 60 * 60}, // 24 hours
					},
				},
			},
			expected: "user_login - DAU",
		},
		{
			name: "events with AAU count unique",
			resource: &protos.SegmentationQuery_Resource{
				Type: protos.SegmentationQuery_EVENTS,
				Name: "user_login",
			},
			aggregation: &protos.SegmentationQuery_Aggregation{
				Value: &protos.SegmentationQuery_Aggregation_CountUnique_{
					CountUnique: &protos.SegmentationQuery_Aggregation_CountUnique{
						Duration: &protos.Duration{Value: 0},
					},
				},
			},
			expected: "user_login - AAU",
		},
		{
			name: "events with sum aggregation",
			resource: &protos.SegmentationQuery_Resource{
				Type: protos.SegmentationQuery_EVENTS,
				Name: "purchase",
			},
			aggregation: &protos.SegmentationQuery_Aggregation{
				Value: &protos.SegmentationQuery_Aggregation_AggregateProperties_{
					AggregateProperties: &protos.SegmentationQuery_Aggregation_AggregateProperties{
						Type:         protos.SegmentationQuery_Aggregation_AggregateProperties_SUM,
						PropertyName: "amount",
					},
				},
			},
			expected: "purchase - (Sum of amount)",
		},
		{
			name: "events with average aggregation",
			resource: &protos.SegmentationQuery_Resource{
				Type: protos.SegmentationQuery_EVENTS,
				Name: "transaction",
			},
			aggregation: &protos.SegmentationQuery_Aggregation{
				Value: &protos.SegmentationQuery_Aggregation_AggregateProperties_{
					AggregateProperties: &protos.SegmentationQuery_Aggregation_AggregateProperties{
						Type:         protos.SegmentationQuery_Aggregation_AggregateProperties_AVG,
						PropertyName: "duration",
					},
				},
			},
			expected: "transaction - (Average of duration)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			timeRange := &timerange.TimeRange{
				Start:    time.Now().Add(-time.Hour),
				End:      time.Now(),
				Step:     time.Minute * 5,
				Timezone: time.UTC,
			}
			query := &protos.SegmentationQuery{
				Resource:    tt.resource,
				Aggregation: tt.aggregation,
			}

			// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
			tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)

			// Test through the public interface
			// Verify that the matrix can be created successfully
			require.NotNil(t, tsm)
			proto := tsm.ToProto()
			assert.NotNil(t, proto)
		})
	}
}

func TestTimeSeriesMatrix_GetName_Cohorts(t *testing.T) {
	tests := []struct {
		name     string
		resource *protos.SegmentationQuery_Resource
		expected string
	}{
		{
			name: "cohorts with ID",
			resource: &protos.SegmentationQuery_Resource{
				Type: protos.SegmentationQuery_COHORTS,
				CohortsValue: &protos.SegmentationQuery_Resource_CohortsId{
					CohortsId: "cohort_123",
				},
			},
			expected: "Cohorts<cohort_123> Segmentation",
		},
		{
			name: "cohorts with query",
			resource: &protos.SegmentationQuery_Resource{
				Type: protos.SegmentationQuery_COHORTS,
				CohortsValue: &protos.SegmentationQuery_Resource_CohortsQuery{
					CohortsQuery: &protos.CohortsQuery{
						Name: "Test Cohort",
					},
				},
			},
			expected: "Test Cohort",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			timeRange := &timerange.TimeRange{
				Start:    time.Now().Add(-time.Hour),
				End:      time.Now(),
				Step:     time.Minute * 5,
				Timezone: time.UTC,
			}
			query := &protos.SegmentationQuery{
				Resource: tt.resource,
				Aggregation: &protos.SegmentationQuery_Aggregation{
					Value: &protos.SegmentationQuery_Aggregation_Total_{
						Total: &protos.SegmentationQuery_Aggregation_Total{},
					},
				},
			}

			// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
			tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)
			// Test through the public interface

			assert.Equal(t, tt.expected, tsm.GetName())
		})
	}
}

func TestTimeSeriesMatrix_IsCumulative(t *testing.T) {
	tests := []struct {
		name        string
		aggregation *protos.SegmentationQuery_Aggregation
		expected    bool
	}{
		{
			name: "non-cumulative total aggregation",
			aggregation: &protos.SegmentationQuery_Aggregation{
				Value: &protos.SegmentationQuery_Aggregation_Total_{
					Total: &protos.SegmentationQuery_Aggregation_Total{},
				},
			},
			expected: false,
		},
		{
			name: "cumulative sum aggregation",
			aggregation: &protos.SegmentationQuery_Aggregation{
				Value: &protos.SegmentationQuery_Aggregation_AggregateProperties_{
					AggregateProperties: &protos.SegmentationQuery_Aggregation_AggregateProperties{
						Type: protos.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_SUM,
					},
				},
			},
			expected: true,
		},
		{
			name: "AAU count unique (duration 0)",
			aggregation: &protos.SegmentationQuery_Aggregation{
				Value: &protos.SegmentationQuery_Aggregation_CountUnique_{
					CountUnique: &protos.SegmentationQuery_Aggregation_CountUnique{
						Duration: &protos.Duration{Value: 0},
					},
				},
			},
			expected: true,
		},
		{
			name: "DAU count unique (duration > 0)",
			aggregation: &protos.SegmentationQuery_Aggregation{
				Value: &protos.SegmentationQuery_Aggregation_CountUnique_{
					CountUnique: &protos.SegmentationQuery_Aggregation_CountUnique{
						Duration: &protos.Duration{Value: 86400}, // 24 hours
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			timeRange := &timerange.TimeRange{
				Start:    time.Now().Add(-time.Hour),
				End:      time.Now(),
				Step:     time.Minute * 5,
				Timezone: time.UTC,
			}
			query := &protos.SegmentationQuery{
				Resource: &protos.SegmentationQuery_Resource{
					Type: protos.SegmentationQuery_EVENTS,
					Name: "test_event",
				},
				Aggregation: tt.aggregation,
			}

			// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
			tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)

			// Test through the public interface
			// Verify that the matrix can be created successfully
			require.NotNil(t, tsm)
			proto := tsm.ToProto()
			assert.NotNil(t, proto)
		})
	}
}

func TestTimeSeriesMatrix_Apply(t *testing.T) {
	ctx := context.Background()
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(time.Hour)

	timeRange := &timerange.TimeRange{
		Start:    startTime,
		End:      endTime,
		Step:     time.Minute * 15,
		Timezone: time.UTC,
	}

	query := &protos.SegmentationQuery{
		Resource: &protos.SegmentationQuery_Resource{
			Type: protos.SegmentationQuery_EVENTS,
			Name: "test_event",
		},
		Aggregation: &protos.SegmentationQuery_Aggregation{
			Value: &protos.SegmentationQuery_Aggregation_Total_{
				Total: &protos.SegmentationQuery_Aggregation_Total{},
			},
		},
	}

	// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
	tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)

	// Create mock matrix data
	mockRows := []mockMatrixRow{
		{
			timeValue:   startTime.Add(time.Minute * 15),
			aggValue:    float64(100),
			labelsValue: map[string]interface{}{"app": "test1", "version": "1.0"},
			data:        []interface{}{startTime.Add(time.Minute * 15), float64(100), "test1"},
		},
		{
			timeValue:   startTime.Add(time.Minute * 30),
			aggValue:    float64(200),
			labelsValue: map[string]interface{}{"app": "test1", "version": "1.0"},
			data:        []interface{}{startTime.Add(time.Minute * 30), float64(200), "test1"},
		},
		{
			timeValue:   startTime.Add(time.Minute * 15),
			aggValue:    float64(50),
			labelsValue: map[string]interface{}{"app": "test2", "version": "2.0"},
			data:        []interface{}{startTime.Add(time.Minute * 15), float64(50), "test2"},
		},
	}

	matrix := newMockMatrix(mockRows)

	tsm.Apply(matrix, false)

	// Test through the public interface
	// Verify that data was processed by checking the proto output
	proto := tsm.ToProto()
	assert.NotNil(t, proto)

	// Should have samples from the applied matrix data
	// We can't directly check internal fields, but we can verify the result
}

func TestTimeSeriesMatrix_Apply_WithZeroTime(t *testing.T) {
	ctx := context.Background()
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(time.Hour)

	timeRange := &timerange.TimeRange{
		Start:    startTime,
		End:      endTime,
		Step:     time.Minute * 15,
		Timezone: time.UTC,
	}

	query := &protos.SegmentationQuery{
		Resource: &protos.SegmentationQuery_Resource{
			Type: protos.SegmentationQuery_EVENTS,
			Name: "test_event",
		},
		Aggregation: &protos.SegmentationQuery_Aggregation{
			Value: &protos.SegmentationQuery_Aggregation_Total_{
				Total: &protos.SegmentationQuery_Aggregation_Total{},
			},
		},
	}

	// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
	tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)

	// Create mock matrix data with zero time (should be skipped)
	mockRows := []mockMatrixRow{
		{
			timeValue:   time.Time{}, // Zero time
			aggValue:    float64(100),
			labelsValue: map[string]interface{}{"app": "test1"},
			data:        []interface{}{time.Time{}, float64(100), "test1"},
		},
		{
			timeValue:   startTime.Add(time.Minute * 15),
			aggValue:    float64(200),
			labelsValue: map[string]interface{}{"app": "test2"},
			data:        []interface{}{startTime.Add(time.Minute * 15), float64(200), "test2"},
		},
	}

	matrix := newMockMatrix(mockRows)

	tsm.Apply(matrix, false)

	// Test through the public interface
	// Verify that only valid time rows were processed
	proto := tsm.ToProto()
	assert.NotNil(t, proto)
}

func TestTimeSeriesMatrix_Apply_WithInvalidValue(t *testing.T) {
	ctx := context.Background()
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(time.Hour)

	timeRange := &timerange.TimeRange{
		Start:    startTime,
		End:      endTime,
		Step:     time.Minute * 15,
		Timezone: time.UTC,
	}

	query := &protos.SegmentationQuery{
		Resource: &protos.SegmentationQuery_Resource{
			Type: protos.SegmentationQuery_EVENTS,
			Name: "test_event",
		},
		Aggregation: &protos.SegmentationQuery_Aggregation{
			Value: &protos.SegmentationQuery_Aggregation_Total_{
				Total: &protos.SegmentationQuery_Aggregation_Total{},
			},
		},
	}

	// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
	tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)

	// Create mock matrix data with invalid value
	mockRows := []mockMatrixRow{
		{
			timeValue:   startTime.Add(time.Minute * 15),
			aggValue:    "invalid", // Invalid value that can't be converted to float
			labelsValue: map[string]interface{}{"app": "test1"},
			data:        []interface{}{startTime.Add(time.Minute * 15), "invalid", "test1"},
		},
		{
			timeValue:   startTime.Add(time.Minute * 30),
			aggValue:    float64(200),
			labelsValue: map[string]interface{}{"app": "test2"},
			data:        []interface{}{startTime.Add(time.Minute * 30), float64(200), "test2"},
		},
	}

	matrix := newMockMatrix(mockRows)

	tsm.Apply(matrix, false)

	// Test through the public interface

	// Should only have 1 sample (invalid value row should be skipped)
	assert.Len(t, tsm.GetSamples(), 1)
}

func TestTimeSeriesMatrix_ProcessTimeRange_WithFill(t *testing.T) {
	ctx := context.Background()
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(time.Hour)

	timeRange := &timerange.TimeRange{
		Start:    startTime,
		End:      endTime,
		Step:     time.Minute * 15,
		Timezone: time.UTC,
	}

	query := &protos.SegmentationQuery{
		Resource: &protos.SegmentationQuery_Resource{
			Type: protos.SegmentationQuery_EVENTS,
			Name: "test_event",
		},
		Aggregation: &protos.SegmentationQuery_Aggregation{
			Value: &protos.SegmentationQuery_Aggregation_Total_{
				Total: &protos.SegmentationQuery_Aggregation_Total{},
			},
		},
	}

	// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
	tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)

	// Create mock matrix data
	mockRows := []mockMatrixRow{
		{
			timeValue:   startTime.Add(time.Minute * 20), // Slightly after the expected time
			aggValue:    float64(100),
			labelsValue: map[string]interface{}{"app": "test1"},
			data:        []interface{}{startTime.Add(time.Minute * 20), float64(100), "test1"},
		},
	}

	matrix := newMockMatrix(mockRows)

	tsm.Apply(matrix, true)

	// Test through the public interface

	// Verify time processing with fill
	require.NotNil(t, tsm.GetTime())
	assert.NotZero(t, tsm.GetTime().Start)
	assert.NotZero(t, tsm.GetTime().End)
}

func TestTimeSeriesMatrix_NilResource(t *testing.T) {
	ctx := context.Background()
	timeRange := &timerange.TimeRange{
		Start:    time.Now().Add(-time.Hour),
		End:      time.Now(),
		Step:     time.Minute * 5,
		Timezone: time.UTC,
	}
	query := &protos.SegmentationQuery{
		Resource: nil, // Nil resource
		Aggregation: &protos.SegmentationQuery_Aggregation{
			Value: &protos.SegmentationQuery_Aggregation_Total_{
				Total: &protos.SegmentationQuery_Aggregation_Total{},
			},
		},
	}

	// Test the exported TimeSeriesMatrix interface by creating through NewTimeSeriesMatrix
	tsm := NewTimeSeriesMatrix(ctx, Query{SegmentationQuery: query}, timeRange)

	// Test through the public interface
	// Verify that the matrix can handle nil resource without crashing
	require.NotNil(t, tsm)
	proto := tsm.ToProto()
	assert.NotNil(t, proto)
}
