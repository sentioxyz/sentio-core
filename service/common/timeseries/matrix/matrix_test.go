package matrix

import (
	"math/big"
	"reflect"
	"testing"
	"time"

	anyutil "sentioxyz/sentio-core/common/utils"

	clickhouselib "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockColumnType struct {
	name     string
	scanType reflect.Type
}

func (m mockColumnType) Name() string             { return m.name }
func (m mockColumnType) ScanType() reflect.Type   { return m.scanType }
func (m mockColumnType) Nullable() bool           { return false }
func (m mockColumnType) DatabaseTypeName() string { return m.name }

type mockRows struct {
	mock.Mock
	columns     []string
	columnTypes []clickhouselib.ColumnType
	data        [][]any
	currentRow  int
}

func (m *mockRows) Next() bool {
	if m.currentRow >= len(m.data) {
		return false
	}
	m.currentRow++
	return true
}

func (m *mockRows) Scan(dest ...any) error {
	if m.currentRow == 0 || m.currentRow > len(m.data) {
		return nil
	}
	row := m.data[m.currentRow-1]
	for i, d := range dest {
		if i < len(row) {
			reflect.ValueOf(d).Elem().Set(reflect.ValueOf(row[i]))
		}
	}
	return nil
}

func (m *mockRows) Err() error {
	return nil
}

func (m *mockRows) Columns() []string {
	return m.columns
}

func (m *mockRows) ColumnTypes() []clickhouselib.ColumnType {
	return m.columnTypes
}

func (m *mockRows) Close() error {
	return nil
}

func (m *mockRows) ScanStruct(dest any) error {
	return nil
}

func (m *mockRows) Totals(dest ...any) error {
	return nil
}

func (m *mockRows) ColumnNames() []string {
	return m.columns
}

func TestDynamicScanType(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: nil,
		},
		{
			name:     "string pointer",
			input:    stringPtr("test"),
			expected: "test",
		},
		{
			name:     "string value",
			input:    "test",
			expected: "test",
		},
		{
			name:     "time pointer",
			input:    timePtr(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
			expected: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "time value",
			input:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "decimal pointer",
			input:    decimalPtr(decimal.NewFromFloat(123.45)),
			expected: decimal.NewFromFloat(123.45),
		},
		{
			name:     "decimal value",
			input:    decimal.NewFromFloat(123.45),
			expected: decimal.NewFromFloat(123.45),
		},
		{
			name:     "big.Int pointer",
			input:    bigIntPtr(big.NewInt(12345)),
			expected: *big.NewInt(12345),
		},
		{
			name:     "big.Int value",
			input:    *big.NewInt(12345),
			expected: *big.NewInt(12345),
		},
		{
			name:     "int pointer",
			input:    intPtr(42),
			expected: int64(42),
		},
		{
			name:     "int value",
			input:    42,
			expected: int64(42),
		},
		{
			name:     "int64 value",
			input:    int64(42),
			expected: int64(42),
		},
		{
			name:     "uint pointer",
			input:    uintPtr(42),
			expected: uint64(42),
		},
		{
			name:     "uint value",
			input:    uint(42),
			expected: uint64(42),
		},
		{
			name:     "float32 pointer",
			input:    float32Ptr(3.14),
			expected: float64(float32(3.14)),
		},
		{
			name:     "float64 value",
			input:    3.14,
			expected: float64(3.14),
		},
		{
			name:     "bool pointer",
			input:    boolPtr(true),
			expected: true,
		},
		{
			name:     "bool value",
			input:    true,
			expected: true,
		},
		{
			name:     "unsupported type",
			input:    []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dynamicScanType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewMatrix(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		columns := []string{"id", "name", "value"}
		columnTypes := []clickhouselib.ColumnType{
			mockColumnType{name: "id", scanType: reflect.TypeOf(int64(0))},
			mockColumnType{name: "name", scanType: reflect.TypeOf("")},
			mockColumnType{name: "value", scanType: reflect.TypeOf(float64(0))},
		}

		data := [][]any{
			{int64(1), "test1", 10.5},
			{int64(2), "test2", 20.7},
		}

		rows := &mockRows{
			columns:     columns,
			columnTypes: columnTypes,
			data:        data,
		}

		matrix, err := NewMatrix(rows)
		assert.NoError(t, err)
		assert.NotNil(t, matrix)
		assert.Equal(t, 2, matrix.Len())
		assert.Equal(t, columnTypes, matrix.ColumnTypes())
	})

	t.Run("empty rows", func(t *testing.T) {
		columns := []string{"id"}
		columnTypes := []clickhouselib.ColumnType{
			mockColumnType{name: "id", scanType: reflect.TypeOf(int64(0))},
		}

		rows := &mockRows{
			columns:     columns,
			columnTypes: columnTypes,
			data:        [][]any{},
		}

		matrix, err := NewMatrix(rows)
		assert.NoError(t, err)
		assert.NotNil(t, matrix)
		assert.Equal(t, 0, matrix.Len())
	})
}

func TestMatrix_ColumnType(t *testing.T) {
	columnTypes := []clickhouselib.ColumnType{
		mockColumnType{name: "id", scanType: reflect.TypeOf(int64(0))},
		mockColumnType{name: "name", scanType: reflect.TypeOf("")},
	}

	m := &matrix{
		columnTypes: columnTypes,
		data:        [][]any{},
	}

	assert.Equal(t, columnTypes[0], m.ColumnType(0))
	assert.Equal(t, columnTypes[1], m.ColumnType(1))
}

func TestMatrix_Data(t *testing.T) {
	data := [][]any{
		{1, "test1"},
		{2, "test2"},
	}

	m := &matrix{
		data: data,
	}

	assert.Equal(t, data, m.Data())
}

func TestMatrix_DataByRow(t *testing.T) {
	data := [][]any{
		{1, "test1"},
		{2, "test2"},
	}

	m := &matrix{
		data: data,
	}

	t.Run("valid index", func(t *testing.T) {
		row := m.DataByRow(0)
		assert.Equal(t, []any{1, "test1"}, row)
	})

	t.Run("invalid index", func(t *testing.T) {
		row := m.DataByRow(10)
		assert.Nil(t, row)
	})
}

func TestMatrix_DataByCol(t *testing.T) {
	data := [][]any{
		{1, "test1"},
		{2, "test2"},
	}

	m := &matrix{
		data: data,
	}

	t.Run("valid index", func(t *testing.T) {
		col := m.DataByCol(0)
		assert.Equal(t, []any{1, 2}, col)
	})

	t.Run("invalid index", func(t *testing.T) {
		col := m.DataByCol(10)
		assert.Nil(t, col)
	})
}

func TestMatrix_DataByColName(t *testing.T) {
	data := [][]any{
		{1, "test1"},
		{2, "test2"},
	}
	columnNames := []string{"id", "name"}

	m := &matrix{
		data:        data,
		columnNames: columnNames,
	}

	t.Run("existing column", func(t *testing.T) {
		col := m.DataByColName("id")
		assert.Equal(t, []any{1, 2}, col)
	})

	t.Run("non-existing column", func(t *testing.T) {
		col := m.DataByColName("unknown")
		assert.Nil(t, col)
	})
}

func TestMatrix_DataValue(t *testing.T) {
	data := [][]any{
		{1, "test1"},
		{2, "test2"},
	}

	m := &matrix{
		data: data,
	}

	t.Run("valid indices", func(t *testing.T) {
		value := m.DataValue(0, 1)
		assert.Equal(t, "test1", value)
	})

	t.Run("invalid row index", func(t *testing.T) {
		value := m.DataValue(10, 1)
		assert.Nil(t, value)
	})

	t.Run("invalid col index", func(t *testing.T) {
		value := m.DataValue(0, 10)
		assert.Nil(t, value)
	})
}

func TestMatrix_TimeSeriesTimeValue(t *testing.T) {
	testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	data := [][]any{
		{testTime, 42, "label1"},
		{testTime.Add(time.Hour), 43, "label2"},
	}
	columnNames := []string{TimeFieldName, AggFieldName, "label"}

	m := &matrix{
		data:        data,
		columnNames: columnNames,
		columnNameInvertedIndex: map[string]int{
			TimeFieldName: 0,
			AggFieldName:  1,
			"label":       2,
		},
	}

	t.Run("valid index", func(t *testing.T) {
		result := m.TimeSeriesTimeValue(0)
		expected := anyutil.Any2Time(testTime)
		assert.Equal(t, expected, result)
	})

	t.Run("invalid index", func(t *testing.T) {
		result := m.TimeSeriesTimeValue(10)
		assert.Equal(t, time.Time{}, result)
	})

	t.Run("no time field", func(t *testing.T) {
		m.columnNames = []string{"other", AggFieldName, "label"}
		m.columnNameInvertedIndex = map[string]int{}
		result := m.TimeSeriesTimeValue(0)
		assert.Equal(t, time.Time{}, result)
	})
}

func TestMatrix_TimeSeriesAggValue(t *testing.T) {
	data := [][]any{
		{time.Now(), 42, "label1"},
		{time.Now(), 43, "label2"},
	}
	columnNames := []string{TimeFieldName, AggFieldName, "label"}

	m := &matrix{
		data:        data,
		columnNames: columnNames,
		columnNameInvertedIndex: map[string]int{
			TimeFieldName: 0,
			AggFieldName:  1,
			"label":       2,
		},
	}

	t.Run("valid index", func(t *testing.T) {
		result := m.TimeSeriesAggValue(0)
		assert.Equal(t, 42, result)
	})

	t.Run("invalid index", func(t *testing.T) {
		result := m.TimeSeriesAggValue(10)
		assert.Nil(t, result)
	})

	t.Run("no agg field", func(t *testing.T) {
		m.columnNames = []string{TimeFieldName, "other", "label"}
		m.columnNameInvertedIndex = map[string]int{}
		result := m.TimeSeriesAggValue(0)
		assert.Nil(t, result)
	})
}

func TestMatrix_TimeSeriesLabelsValue(t *testing.T) {
	data := [][]any{
		{time.Now(), 42, "label1", "extra"},
		{time.Now(), 43, "label2", "extra2"},
	}
	columnNames := []string{TimeFieldName, AggFieldName, "label", "extra"}

	m := &matrix{
		data:        data,
		columnNames: columnNames,
		columnNameInvertedIndex: map[string]int{
			TimeFieldName: 0,
			AggFieldName:  1,
			"label":       2,
			"extra":       3,
		},
	}

	t.Run("valid index", func(t *testing.T) {
		result := m.TimeSeriesLabelsValue(0)
		assert.Equal(t, map[string]any{
			"label": "label1",
			"extra": "extra",
		}, result)
	})

	t.Run("invalid index", func(t *testing.T) {
		result := m.TimeSeriesLabelsValue(10)
		assert.Nil(t, result)
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func decimalPtr(d decimal.Decimal) *decimal.Decimal {
	return &d
}

func bigIntPtr(b *big.Int) *big.Int {
	return b
}

func intPtr(i int) *int {
	return &i
}

func uintPtr(u uint) *uint {
	return &u
}

func float32Ptr(f float32) *float32 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}
