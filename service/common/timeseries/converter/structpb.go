package converter

import (
	"math"
	"math/big"
	"reflect"
	"time"
	"unicode/utf8"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

func convertPointer(v any) any {
	r := reflect.TypeOf(v)
	switch r.Kind() {
	case reflect.Ptr:
		if reflect.ValueOf(v).IsNil() {
			return nil
		}
		return reflect.ValueOf(v).Elem().Interface()
	default:
		return v
	}
}

func convertTime(t time.Time, timezone *time.Location) string {
	return t.In(timezone).Format(time.RFC3339Nano)
}

func convertDecimal(d decimal.Decimal) string {
	return d.String()
}

func convertBigInt(i big.Int) string {
	return i.String()
}

func convertString(s string, reserved map[string]struct{}) (string, bool) {
	if utf8.ValidString(s) {
		if _, ok := reserved[s]; !ok {
			return s, true
		}
		return "", true
	} else {
		return "", false
	}
}

func convertUInt8(i uint8, databaseType string) (*uint8, *bool) {
	if databaseType == "Bool" {
		return nil, lo.ToPtr[bool](i == 1)
	} else {
		return lo.ToPtr[uint8](i), nil
	}
}

func convertFloat64(f float64) *float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil
	}
	return lo.ToPtr(f)
}

func isArray(a any) bool {
	value := reflect.ValueOf(a)
	return value.Kind() == reflect.Slice
}

func isMap(a any) bool {
	value := reflect.ValueOf(a)
	return value.Kind() == reflect.Map
}

func isStruct(a any) bool {
	value := reflect.ValueOf(a)
	return value.Kind() == reflect.Struct
}

type ConvertOption struct {
	Timezone     *time.Location
	Reserved     map[string]struct{}
	DatabaseType string
}

func ConvertAny(a any, option *ConvertOption) any {
	if a == nil {
		return nil
	}
	a = convertPointer(a)
	switch aValue := a.(type) {
	case time.Time:
		return convertTime(aValue, option.Timezone)
	case decimal.Decimal:
		return convertDecimal(aValue)
	case big.Int:
		return convertBigInt(aValue)
	case string:
		if s, ok := convertString(aValue, option.Reserved); ok {
			return s
		} else {
			return []byte(aValue)
		}
	case uint8:
		if u, b := convertUInt8(aValue, option.DatabaseType); u != nil {
			return *u
		} else if b != nil {
			return *b
		}
	case float64:
		if f := convertFloat64(aValue); f != nil {
			return *f
		}
		return nil
	case float32:
		if f := convertFloat64(float64(aValue)); f != nil {
			return *f
		}
		return nil
	case nil:
		return nil
	case []byte:
		return aValue
	case bool:
		return a
	case int8, int16, int32, int64, uint16, uint32, uint64, int, uint, complex64, complex128:
		return a
	case clickhouse.Dynamic:
		return ConvertAny(aValue.Any(), option)
	case clickhouse.JSON:
		rValue := reflect.ValueOf(aValue)
		return convertMap(aValue, rValue, option)
	default:
		rValue := reflect.ValueOf(a)
		switch {
		case isArray(a):
			return convertArray(a, rValue, option)
		case isMap(a):
			return convertMap(a, rValue, option)
		case isStruct(a):
			return convertStruct(a, rValue, option)
		}
	}
	return "unsupported type, please contact the sentio admin"
}

func convertArray(a any, aValue reflect.Value, option *ConvertOption) []any {
	if !isArray(a) {
		return nil
	}
	result := make([]any, aValue.Len())
	for i := 0; i < aValue.Len(); i++ {
		data := aValue.Index(i).Interface()
		result[i] = ConvertAny(data, option)
	}
	return result
}

func convertMap(a any, aValue reflect.Value, option *ConvertOption) map[string]any {
	if !isMap(a) {
		return nil
	}
	result := map[string]any{}
	for _, key := range aValue.MapKeys() {
		data := aValue.MapIndex(key).Interface()
		result[key.String()] = ConvertAny(data, option)
	}
	return result
}

func convertStruct(a any, aValue reflect.Value, option *ConvertOption) map[string]any {
	if !isStruct(a) {
		return nil
	}
	result := map[string]any{}
	for i := 0; i < aValue.NumField(); i++ {
		field := aValue.Type().Field(i)
		data := aValue.Field(i).Interface()
		result[field.Name] = ConvertAny(data, option)
	}
	return result
}
