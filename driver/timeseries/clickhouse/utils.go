package clickhouse

import (
	"crypto/sha1"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/shopspring/decimal"
	"math/big"
	"reflect"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
	"time"
)

// Only used to query latest value of each series in counter table,
// so will not face FieldTypeJSON FieldTypeArray FieldTypeToken.
func scanRow(rows driver.Rows, fields []timeseries.Field) (timeseries.Row, error) {
	placeholders := make([]any, len(fields))
	for i, field := range fields {
		switch field.Type {
		case timeseries.FieldTypeString:
			var v string
			placeholders[i] = &v
		case timeseries.FieldTypeBool:
			var v bool
			placeholders[i] = &v
		case timeseries.FieldTypeTime:
			var v time.Time
			placeholders[i] = &v
		case timeseries.FieldTypeInt:
			var v int64
			placeholders[i] = &v
		case timeseries.FieldTypeBigInt:
			var v *big.Int
			placeholders[i] = &v
		case timeseries.FieldTypeFloat:
			var v float64
			placeholders[i] = &v
		case timeseries.FieldTypeBigFloat:
			var v decimal.Decimal
			placeholders[i] = &v
		}
	}
	err := rows.Scan(placeholders...)
	if err != nil {
		return nil, err
	}
	row := make(timeseries.Row, len(fields))
	for i, field := range fields {
		row[field.Name] = reflect.ValueOf(placeholders[i]).Elem().Interface()
	}
	return row, nil
}

func buildSeriesID(row timeseries.Row, labelFields []timeseries.Field) string {
	h := sha1.New()
	for _, field := range labelFields {
		h.Write([]byte(field.Name))
		h.Write([]byte{0})
		if v, has := row[field.Name]; has {
			h.Write([]byte(fmt.Sprintf("%v", v)))
		}
		h.Write([]byte{0})
	}
	sid := fmt.Sprintf("%x", h.Sum(nil))
	return sid
}

func buildSeriesSummary(series map[string]timeseries.Row, previewNum int) string {
	preview := make(map[string]timeseries.Row, 5)
	for id, s := range series {
		preview[id] = s
		if len(preview) >= previewNum {
			break
		}
	}
	return utils.MustJSONMarshal(map[string]any{
		"total":   len(series),
		"preview": preview,
	})
}

func addValues(base, add timeseries.Row, valueFields []timeseries.Field) timeseries.Row {
	for _, field := range valueFields {
		b, hb := base[field.Name]
		a, ha := add[field.Name]
		if hb && ha {
			switch field.Type {
			case timeseries.FieldTypeInt:
				base[field.Name] = b.(int64) + a.(int64)
			case timeseries.FieldTypeBigInt:
				base[field.Name] = new(big.Int).Add(b.(*big.Int), a.(*big.Int))
			case timeseries.FieldTypeFloat:
				base[field.Name] = b.(float64) + a.(float64)
			case timeseries.FieldTypeBigFloat:
				base[field.Name] = b.(decimal.Decimal).Add(a.(decimal.Decimal))
			}
		} else if !hb && ha {
			base[field.Name] = add[field.Name]
		}
	}
	return base
}

func quote(name string) string {
	return fmt.Sprintf("`%s`", name)
}
