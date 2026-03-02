package matrix

import (
	"math/big"
	"reflect"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/anyutil"

	clickhouselib "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/shopspring/decimal"
)

type CohortResultType string

const (
	CohortUser      CohortResultType = "user"
	CohortChain     CohortResultType = "chain"
	CohortUpdatedAt CohortResultType = "updated_at"
	CohortAgg       CohortResultType = "total"
)

const (
	TimeFieldName = "timestamp"
	AggFieldName  = "agg"
)

type Matrix interface {
	ColumnTypes() []clickhouselib.ColumnType
	ColumnType(idx int) clickhouselib.ColumnType
	ColumnNames() []string
	Len() int

	Data() [][]any
	DataByRow(idx int) []any
	DataByCol(idx int) []any
	DataByColName(name string) []any
	DataValue(rowIdx, colIdx int) any

	TimeSeriesTimeValue(idx int) time.Time
	TimeSeriesAggValue(idx int) any
	TimeSeriesLabelsValue(idx int) map[string]any

	CohortValue(idx int, cohortType CohortResultType) any
}

type matrix struct {
	data                    [][]any
	columnTypes             []clickhouselib.ColumnType
	columnNames             []string
	columnNameInvertedIndex map[string]int
}

func dynamicScanType(v any) any {
	switch v := v.(type) {
	case nil:
		return nil
	case *string:
		return *v
	case string:
		return v
	case *time.Time:
		return *v
	case time.Time:
		return v
	case *decimal.Decimal:
		return *v
	case decimal.Decimal:
		return v
	case *big.Int:
		return *v
	case big.Int:
		return v
	case *int, *int8, *int16, *int32, *int64:
		return anyutil.MustParseInt(reflect.ValueOf(v).Elem().Interface())
	case int, int8, int16, int32, int64:
		return anyutil.MustParseInt(v)
	case *uint, *uint8, *uint16, *uint32, *uint64:
		return anyutil.MustParseUint(reflect.ValueOf(v).Elem().Interface())
	case uint, uint8, uint16, uint32, uint64:
		return anyutil.MustParseUint(v)
	case *float32, *float64:
		return anyutil.MustParseFloat64(reflect.ValueOf(v).Elem().Interface())
	case float32, float64:
		return anyutil.MustParseFloat64(v)
	case *bool:
		return *v
	case bool:
		return v
	}
	return v
}

func NewMatrix(rows clickhouselib.Rows) (Matrix, error) {
	m := &matrix{
		columnTypes:             rows.ColumnTypes(),
		columnNames:             rows.Columns(),
		columnNameInvertedIndex: make(map[string]int),
		data:                    [][]any{},
	}
	for idx, name := range m.columnNames {
		m.columnNameInvertedIndex[name] = idx
	}
	for rows.Next() {
		var (
			vars = make([]any, len(m.columnTypes))
			row  []any
		)
		for i := range m.columnTypes {
			switch strings.ToLower(m.columnTypes[i].DatabaseTypeName()) {
			case "json":
				vars[i] = new(string)
			default:
				vars[i] = reflect.New(m.columnTypes[i].ScanType()).Interface()
			}
		}
		if err := rows.Scan(vars...); err != nil {
			return nil, err
		}
		for _, v := range vars {
			row = append(row, dynamicScanType(v))
		}
		m.data = append(m.data, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *matrix) ColumnTypes() []clickhouselib.ColumnType {
	return m.columnTypes
}

func (m *matrix) ColumnType(idx int) clickhouselib.ColumnType {
	return m.columnTypes[idx]
}

func (m *matrix) Len() int {
	return len(m.data)
}

func (m *matrix) Data() [][]any {
	return m.data
}

func (m *matrix) DataByRow(idx int) []any {
	if idx >= len(m.data) {
		return nil
	}
	return m.data[idx]
}

func (m *matrix) DataByCol(idx int) []any {
	var result []any
	for _, row := range m.data {
		if idx >= len(row) {
			return nil
		}
		result = append(result, row[idx])
	}
	return result
}

func (m *matrix) DataByColName(name string) []any {
	for idx, colName := range m.columnNames {
		if colName == name {
			return m.DataByCol(idx)
		}
	}
	return nil
}

func (m *matrix) DataValue(rowIdx, colIdx int) any {
	if rowIdx >= len(m.data) {
		return nil
	}
	if colIdx >= len(m.data[rowIdx]) {
		return nil
	}
	return m.data[rowIdx][colIdx]
}

func (m *matrix) TimeSeriesTimeValue(idx int) time.Time {
	if idx >= len(m.data) {
		return time.Time{}
	}
	colIdx, ok := m.columnNameInvertedIndex[TimeFieldName]
	if ok {
		return anyutil.ParseTime(m.DataValue(idx, colIdx))
	}
	return time.Time{}
}

func (m *matrix) TimeSeriesAggValue(idx int) any {
	if idx >= len(m.data) {
		return nil
	}
	colIdx, ok := m.columnNameInvertedIndex[AggFieldName]
	if ok {
		return m.DataValue(idx, colIdx)
	}
	return nil
}

func (m *matrix) TimeSeriesLabelsValue(idx int) map[string]any {
	if idx >= len(m.data) {
		return nil
	}
	var result = make(map[string]any)
	for colIdx, colName := range m.columnNames {
		if colName != TimeFieldName && colName != AggFieldName {
			result[colName] = m.DataValue(idx, colIdx)
		}
	}
	return result
}

func (m *matrix) CohortValue(idx int, cohortType CohortResultType) any {
	if idx >= len(m.data) {
		return nil
	}
	colIdx, ok := m.columnNameInvertedIndex[string(cohortType)]
	if ok {
		return m.DataValue(idx, colIdx)
	}
	return nil
}

func (m *matrix) ColumnNames() []string {
	return m.columnNames
}
