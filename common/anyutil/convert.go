package anyutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	bigIntMaxUint64 = big.NewInt(0).SetUint64(math.MaxUint64)
	bigIntMinUint64 = big.NewInt(0).SetUint64(0)
	bigIntMaxInt64  = big.NewInt(0).SetInt64(math.MaxInt64)
	bigIntMinInt64  = big.NewInt(0).SetInt64(math.MinInt64)

	decimalMaxUint64 = decimal.NewFromUint64(math.MaxUint64)
	decimalMinUint64 = decimal.NewFromUint64(0)
	decimalMaxInt64  = decimal.NewFromInt(math.MaxInt64)
	decimalMinInt64  = decimal.NewFromInt(math.MinInt64)
)

func ToString(orig any) string {
	return ParseString(orig)
}

func ToStringArray(orig any) ([]string, bool) {
	value := reflect.ValueOf(orig)
	switch value.Type().Kind() {
	case reflect.Array, reflect.Slice:
		result := make([]string, value.Len())
		for i := 0; i < value.Len(); i++ {
			result[i] = ToString(value.Index(i).Interface())
		}
		return result, true
	default:
		return nil, false
	}
}

func ToStringAnyMap(origin map[any]any) map[string]any {
	result := make(map[string]any)
	for k, v := range origin {
		result[ToString(k)] = v
	}
	return result
}

func ToStringStringMap[K comparable, V any](origin map[K]V) map[string]string {
	result := make(map[string]string)
	for k, v := range origin {
		result[ToString(k)] = ToString(v)
	}
	return result
}

func MustParseInt32(origin any) int32 {
	v, _ := ParseInt32(origin)
	return v
}

func ParseInt32(origin any) (int32, error) {
	val, err := ParseInt(origin)
	if err != nil {
		return 0, err
	}
	if val < math.MinInt32 {
		return 0, errors.New("too small")
	}
	if val > math.MaxInt32 {
		return 0, errors.New("too big")
	}
	return int32(val), nil
}

func MustParseUint32(origin any) uint32 {
	v, _ := ParseUint32(origin)
	return v
}

func ParseUint32(origin any) (uint32, error) {
	val, err := ParseUint(origin)
	if err != nil {
		return 0, err
	}
	if val > math.MaxUint32 {
		return 0, errors.New("too big")
	}
	return uint32(val), nil
}

func MustParseInt(origin any) int64 {
	v, _ := ParseInt(origin)
	return v
}

func ParseInt(origin any) (int64, error) {
	// Unwrap pointers/interfaces first
	if dv, isNil := deref(origin); !isNil {
		origin = dv
	} else {
		return 0, fmt.Errorf("nil value")
	}

	switch v := origin.(type) {
	case string:
		return strconv.ParseInt(v, 10, 64)
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		if v > math.MaxInt64 {
			return 0, errors.New("too big")
		}
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		if v > math.MaxInt64 {
			return 0, errors.New("too big")
		}
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case time.Time:
		return v.UnixNano(), nil
	case json.Number:
		if iv, err := v.Int64(); err == nil {
			return iv, nil
		}
		return 0, fmt.Errorf("can't convert json.Number to int64")
	case decimal.Decimal:
		if v.GreaterThan(decimalMaxInt64) {
			return 0, errors.New("too big")
		}
		if v.LessThan(decimalMinInt64) {
			return 0, errors.New("too small")
		}
		return v.IntPart(), nil
	case big.Int:
		if v.Cmp(bigIntMaxInt64) > 0 {
			return 0, errors.New("too big")
		}
		if v.Cmp(bigIntMinInt64) < 0 {
			return 0, errors.New("too small")
		}
		return v.Int64(), nil
	default:
		return 0, fmt.Errorf("unexpected type %T", origin)
	}
}

func MustParseUint(origin any) uint64 {
	v, _ := ParseUint(origin)
	return v
}

func ParseUint(origin any) (uint64, error) {
	// Unwrap pointers/interfaces first
	if dv, isNil := deref(origin); !isNil {
		origin = dv
	} else {
		return 0, fmt.Errorf("nil value")
	}

	switch v := origin.(type) {
	case string:
		return strconv.ParseUint(v, 10, 64)
	case int:
		if v < 0 {
			return 0, errors.New("negative number")
		}
		return uint64(v), nil
	case int8:
		if v < 0 {
			return 0, errors.New("negative number")
		}
		return uint64(v), nil
	case int16:
		if v < 0 {
			return 0, errors.New("negative number")
		}
		return uint64(v), nil
	case int32:
		if v < 0 {
			return 0, errors.New("negative number")
		}
		return uint64(v), nil
	case int64:
		if v < 0 {
			return 0, errors.New("negative number")
		}
		return uint64(v), nil
	case uint:
		return uint64(v), nil
	case uint8:
		return uint64(v), nil
	case uint16:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case float32:
		if v < 0 {
			return 0, errors.New("negative number")
		}
		return uint64(v), nil
	case float64:
		if v < 0 {
			return 0, errors.New("negative number")
		}
		return uint64(v), nil
	case time.Time:
		return uint64(v.UnixNano()), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("can't convert json.Number to uint64: %w", err)
		}
		if i < 0 {
			return 0, errors.New("negative number")
		}
		return uint64(i), nil
	case decimal.Decimal:
		if v.GreaterThan(decimalMaxUint64) {
			return 0, errors.New("too big")
		}
		if v.LessThan(decimalMinUint64) {
			return 0, errors.New("negative number")
		}
		i := v.IntPart()
		if i < 0 {
			return 0, errors.New("negative number")
		}
		return uint64(i), nil
	case big.Int:
		if v.Cmp(bigIntMaxUint64) > 0 {
			return 0, errors.New("too big")
		}
		if v.Cmp(bigIntMinUint64) < 0 {
			return 0, errors.New("negative number")
		}
		return v.Uint64(), nil
	default:
		return 0, fmt.Errorf("unexpected type %T", origin)
	}
}

func MustParseFloat64(origin any) float64 {
	v, _ := ParseFloat64(origin)
	return v
}

func ParseFloat64(origin any) (float64, error) {
	// Unwrap pointers/interfaces first
	if dv, isNil := deref(origin); !isNil {
		origin = dv
	} else {
		return math.NaN(), fmt.Errorf("nil value")
	}

	switch v := origin.(type) {
	case bool:
		if v {
			return 1, nil
		} else {
			return 0, nil
		}
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, nil
		}
		if d, err := decimal.NewFromString(s); err == nil {
			f, _ := d.Float64()
			return f, nil
		}
		return math.NaN(), fmt.Errorf("can't parse string '%s' to float64", s)
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, nil
		}
		if iv, err := v.Int64(); err == nil {
			return float64(iv), nil
		}
		return math.NaN(), fmt.Errorf("can't convert json.Number to float64")
	case decimal.Decimal:
		f, _ := v.Float64()
		return f, nil
	case big.Int:
		f, _ := v.Float64()
		return f, nil
	case time.Time:
		return float64(v.UnixNano()), nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	default:
		rv := reflect.ValueOf(v)
		rv = reflect.Indirect(rv)
		if rv.IsValid() && rv.Type().ConvertibleTo(floatType) {
			fv := rv.Convert(floatType)
			return fv.Float(), nil
		} else if rv.IsValid() && rv.Type().ConvertibleTo(stringType) {
			sv := rv.Convert(stringType)
			s := sv.String()
			return strconv.ParseFloat(s, 64)
		} else {
			return math.NaN(), fmt.Errorf("can't convert %v to float64", rv.Type())
		}
	}
}

func ParseArray[F any, T any](origin []F, parser func(index int, item F) T) []T {
	r := make([]T, 0, len(origin))
	for index, item := range origin {
		r = append(r, parser(index, item))
	}
	return r
}

func ParseString(origin any) string {
	// Unwrap pointers/interfaces
	if dv, isNil := deref(origin); !isNil {
		origin = dv
	} else {
		return ""
	}

	const float64EqualityThreshold = 1e-9
	floatAlmostEqual := func(a, b float64) bool {
		return math.Abs(a-b) <= float64EqualityThreshold
	}
	switch f := origin.(type) {
	case float64:
		if floatAlmostEqual(f, math.Trunc(f)) {
			return fmt.Sprintf("%.0f", f)
		} else {
			return fmt.Sprintf("%.2f", f)
		}
	case float32:
		f64 := float64(f)
		if floatAlmostEqual(f64, math.Trunc(f64)) {
			return fmt.Sprintf("%.0f", f64)
		} else {
			return fmt.Sprintf("%.2f", f64)
		}
	case decimal.Decimal:
		return f.String()
	case big.Int:
		return f.String()
	case time.Time:
		return f.Format(time.RFC3339)
	case string:
		return f
	case []byte:
		return string(f)
	default:
		return fmt.Sprintf("%v", origin)
	}
}

func ParseTime(origin any) time.Time {
	// Unwrap pointers/interfaces
	if dv, isNil := deref(origin); !isNil {
		origin = dv
	} else {
		return time.Time{}
	}

	switch v := origin.(type) {
	case time.Time:
		return v
	case *time.Time:
		// kept for backward compatibility, though deref already handled it
		if v == nil {
			return time.Time{}
		}
		return *v
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return time.Time{}
		}
		var layouts = []string{
			time.Layout,
			time.RFC3339,
			time.RFC3339Nano,
			time.RFC822Z,
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, s); err == nil {
				return t
			}
		}
		// numeric string? interpret as epoch seconds/ms/us/ns depending on magnitude
		if iv, err := strconv.ParseInt(s, 10, 64); err == nil {
			return epochIntToTime(iv)
		}
		if fv, err := strconv.ParseFloat(s, 64); err == nil {
			sec := int64(fv)
			nsec := int64((fv - float64(sec)) * 1e9)
			return time.Unix(sec, nsec)
		}
		return time.Time{}
	case uint64, int64, int32, uint32, uint16, uint8, int16, int8, int, uint:
		iv, _ := ParseInt(v)
		return epochIntToTime(iv)
	case timestamppb.Timestamp:
		return v.AsTime()
	case *timestamppb.Timestamp:
		if v == nil {
			return time.Time{}
		}
		return v.AsTime()
	case decimal.Decimal:
		f, _ := v.Float64()
		sec := int64(f)
		nsec := int64((f - float64(sec)) * 1e9)
		return time.Unix(sec, nsec)
	case big.Int:
		f, _ := v.Float64()
		sec := int64(f)
		nsec := int64((f - float64(sec)) * 1e9)
		return time.Unix(sec, nsec)
	case float64, float32:
		f := MustParseFloat64(v)
		sec := int64(f)
		nsec := int64((f - float64(sec)) * 1e9)
		return time.Unix(sec, nsec)
	}
	return time.Time{}
}

// epochIntToTime converts an integer epoch with best-effort unit detection.
func epochIntToTime(iv int64) time.Time {
	abs := iv
	if abs < 0 {
		abs = -abs
	}
	// Detect by digit length
	digits := len(strconv.FormatInt(abs, 10))
	switch {
	case digits >= 19:
		// nanoseconds
		return time.Unix(0, iv)
	case digits >= 16:
		// microseconds
		return time.Unix(0, iv*1_000)
	case digits >= 13:
		// milliseconds
		return time.Unix(0, iv*1_000_000)
	default:
		// seconds
		return time.Unix(iv, 0)
	}
}

func ParseBool(v any) bool {
	// Unwrap pointers/interfaces
	if dv, isNil := deref(v); !isNil {
		v = dv
	} else {
		return false
	}

	switch v := v.(type) {
	case bool:
		return v
	case int8, int16, int32, uint8, uint16, uint32, int, uint, int64, uint64:
		i, _ := ParseInt(v)
		return i != 0
	case float64, float32:
		return MustParseFloat64(v) != 0
	case string:
		s := strings.TrimSpace(strings.ToLower(v))
		if s == "" {
			return false
		}
		switch s {
		case "true", "t", "yes", "y", "on", "1":
			return true
		case "false", "f", "no", "n", "off", "0":
			return false
		}
		// try numeric
		if fv, err := strconv.ParseFloat(s, 64); err == nil {
			return fv != 0
		}
		return false
	case json.Number:
		if iv, err := v.Int64(); err == nil {
			return iv != 0
		}
		if fv, err := v.Float64(); err == nil {
			return fv != 0
		}
		return false
	case decimal.Decimal:
		return v.Cmp(decimal.NewFromInt(0)) != 0
	case big.Int:
		return v.Cmp(big.NewInt(0)) != 0
	case time.Time:
		return !v.IsZero()
	default:
		return false
	}
}
