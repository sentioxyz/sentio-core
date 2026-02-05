package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	protoscommon "sentioxyz/sentio-core/service/common/protos"

	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var floatType = reflect.TypeOf(float64(0))
var stringType = reflect.TypeOf("")

func Proto2Any(a *protoscommon.Any) any {
	switch a.AnyValue.(type) {
	case *protoscommon.Any_StringValue:
		return a.GetStringValue()
	case *protoscommon.Any_IntValue:
		return a.GetIntValue()
	case *protoscommon.Any_BoolValue:
		return a.GetBoolValue()
	case *protoscommon.Any_DoubleValue:
		return a.GetDoubleValue()
	case *protoscommon.Any_DateValue:
		return a.GetDateValue().AsTime()
	case *protoscommon.Any_LongValue:
		return a.GetLongValue()
	case *protoscommon.Any_ListValue:
		return a.GetListValue().GetValues()
	}
	return nil
}

func Proto2String(a *protoscommon.Any) string {
	switch a.AnyValue.(type) {
	case *protoscommon.Any_StringValue:
		return a.GetStringValue()
	case *protoscommon.Any_IntValue:
		return fmt.Sprintf("%d", a.GetIntValue())
	case *protoscommon.Any_BoolValue:
		return fmt.Sprintf("%t", a.GetBoolValue())
	case *protoscommon.Any_DoubleValue:
		return fmt.Sprintf("%f", a.GetDoubleValue())
	case *protoscommon.Any_LongValue:
		return fmt.Sprintf("%d", a.GetLongValue())
	case *protoscommon.Any_DateValue:
		return a.GetDateValue().AsTime().Format("2006-01-02T15:04:05")
	case *protoscommon.Any_ListValue:
		var values []string
		for _, v := range a.GetListValue().GetValues() {
			values = append(values, "'"+v+"'")
		}
		return strings.Join(values, ", ")
	}
	return ""
}

func ProtoToClickhouseValue(a *protoscommon.Any) any {
	switch a.AnyValue.(type) {
	case *protoscommon.Any_StringValue:
		return "'" + a.GetStringValue() + "'"
	case *protoscommon.Any_IntValue:
		return fmt.Sprintf("%d::Int64", a.GetIntValue())
	case *protoscommon.Any_BoolValue:
		return fmt.Sprintf("%t::Bool", a.GetBoolValue())
	case *protoscommon.Any_DoubleValue:
		return fmt.Sprintf("toDecimal256OrZero('%s', 30)", fmt.Sprintf("%f", a.GetDoubleValue()))
	case *protoscommon.Any_LongValue:
		return fmt.Sprintf("%d::Int256", a.GetLongValue())
	case *protoscommon.Any_DateValue:
		goTime := a.GetDateValue().AsTime().UTC()
		return fmt.Sprintf("toDateTime64('%s', 6, 'UTC')",
			goTime.Format("2006-01-02 15:04:05"))
	case *protoscommon.Any_ListValue:
		var values []string
		for _, v := range a.GetListValue().GetValues() {
			values = append(values, "'"+v+"'")
		}
		return strings.Join(values, ", ")
	}
	return ""
}

func ToInt32(val any) int32 {
	switch v := val.(type) {
	case int8:
		return int32(v)
	case int16:
		return int32(v)
	case int32:
		return v
	case uint8:
		return int32(v)
	case uint16:
		return int32(v)
	case uint32:
		return int32(v)
	default:
		return 0
	}
}

func ToInt64(val any) int64 {
	switch v := val.(type) {
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		return 0
	}
}

func ToUInt64(val any) uint64 {
	switch v := val.(type) {
	case int:
		return uint64(v)
	case int8:
		return uint64(v)
	case int16:
		return uint64(v)
	case int32:
		return uint64(v)
	case int64:
		return uint64(v)
	case uint:
		return uint64(v)
	case uint8:
		return uint64(v)
	case uint16:
		return uint64(v)
	case uint32:
		return uint64(v)
	case uint64:
		return v
	}
	return 0
}

func ToFloat64(val any) float64 {
	switch v := val.(type) {
	case float32:
		return float64(v)
	case float64:
		return v
	default:
		return 0
	}
}

func Any2Proto(a any) *protoscommon.Any {
	// Unwrap pointers/interfaces
	if dv, isNil := deref(a); !isNil {
		a = dv
	} else {
		return nil
	}

	switch v := a.(type) {
	case string:
		return &protoscommon.Any{
			AnyValue: &protoscommon.Any_StringValue{
				StringValue: v,
			},
		}
	case []string:
		return &protoscommon.Any{
			AnyValue: &protoscommon.Any_ListValue{
				ListValue: &protoscommon.StringList{Values: v},
			},
		}
	case int8, int16, int32, uint8, uint16, uint32:
		return &protoscommon.Any{
			AnyValue: &protoscommon.Any_IntValue{
				IntValue: ToInt32(v),
			},
		}
	case bool:
		return &protoscommon.Any{
			AnyValue: &protoscommon.Any_BoolValue{
				BoolValue: v,
			},
		}
	case float64, float32:
		return &protoscommon.Any{
			AnyValue: &protoscommon.Any_DoubleValue{
				DoubleValue: ToFloat64(v),
			},
		}
	case int, uint, int64, uint64:
		return &protoscommon.Any{
			AnyValue: &protoscommon.Any_LongValue{
				LongValue: ToInt64(v),
			},
		}
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return &protoscommon.Any{AnyValue: &protoscommon.Any_LongValue{LongValue: i}}
		}
		if f, err := v.Float64(); err == nil {
			return &protoscommon.Any{AnyValue: &protoscommon.Any_DoubleValue{DoubleValue: f}}
		}
		return nil
	case decimal.Decimal:
		f, _ := v.Float64()
		return &protoscommon.Any{
			AnyValue: &protoscommon.Any_DoubleValue{
				DoubleValue: f,
			},
		}
	case big.Int:
		f, _ := v.Float64()
		return &protoscommon.Any{
			AnyValue: &protoscommon.Any_DoubleValue{
				DoubleValue: f,
			},
		}
	case time.Time:
		return &protoscommon.Any{
			AnyValue: &protoscommon.Any_DateValue{
				DateValue: timestamppb.New(v),
			},
		}
	}
	return nil
}

func Any2Float(v any) (float64, error) {
	// Unwrap pointers/interfaces first
	if dv, isNil := deref(v); !isNil {
		v = dv
	} else {
		return math.NaN(), fmt.Errorf("nil value")
	}

	switch i := v.(type) {
	case float64:
		return i, nil
	case float32:
		return float64(i), nil
	case int64:
		return float64(i), nil
	case int32:
		return float64(i), nil
	case int:
		return float64(i), nil
	case uint64:
		return float64(i), nil
	case uint32:
		return float64(i), nil
	case uint:
		return float64(i), nil
	case bool:
		if i {
			return 1, nil
		}
		return 0, nil
	case string:
		s := strings.TrimSpace(i)
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
		if f, err := i.Float64(); err == nil {
			return f, nil
		}
		if iv, err := i.Int64(); err == nil {
			return float64(iv), nil
		}
		return math.NaN(), fmt.Errorf("can't convert json.Number to float64")
	case decimal.Decimal:
		f, _ := i.Float64()
		return f, nil
	case big.Int:
		f, _ := i.Float64()
		return f, nil
	case time.Time:
		return float64(i.Unix()), nil
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

func Any2Int64(v any) int64 {
	f, _ := Any2Float(v)
	return int64(f)
}

func Any2Uint64(v any) uint64 {
	f, _ := Any2Float(v)
	return uint64(f)
}

func Any2Int(v any) int {
	f, _ := Any2Float(v)
	return int(f)
}

func Any2Uint(v any) uint {
	f, _ := Any2Float(v)
	return uint(f)
}

func Any2Int32(v any) int32 {
	f, _ := Any2Float(v)
	return int32(f)
}

func Any2Uint32(v any) uint32 {
	f, _ := Any2Float(v)
	return uint32(f)
}

// Any2String converts any to string
// if v is float64 or float32, and its precision part is 0, it will be converted to int
// otherwise it will be converted to string by %.2f
func Any2String(v any) string {
	// Unwrap pointers/interfaces
	if dv, isNil := deref(v); !isNil {
		v = dv
	} else {
		return ""
	}

	const float64EqualityThreshold = 1e-9
	floatAlmostEqual := func(a, b float64) bool {
		return math.Abs(a-b) <= float64EqualityThreshold
	}
	switch f := v.(type) {
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
	default:
		return fmt.Sprintf("%v", v)
	}
}

func Any2Time(v any) time.Time {
	// Unwrap pointers/interfaces
	if dv, isNil := deref(v); !isNil {
		v = dv
	} else {
		return time.Time{}
	}

	switch v := v.(type) {
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
		iv := ToInt64(v)
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
		f := ToFloat64(v)
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

func Any2Bool(v any) bool {
	// Unwrap pointers/interfaces
	if dv, isNil := deref(v); !isNil {
		v = dv
	} else {
		return false
	}

	switch v := v.(type) {
	case bool:
		return v
	case int8, int16, int32, uint8, uint16, uint32:
		return ToInt32(v) != 0
	case int, uint, int64, uint64:
		return ToInt64(v) != 0
	case float64, float32:
		return ToFloat64(v) != 0
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

func StringUnmarshaler[V any](raw string) (V, error) {
	var val V
	err := json.Unmarshal([]byte(raw), &val)
	return val, err
}

func MustJSONMarshal(v any) string {
	bs, _ := json.Marshal(v)
	return string(bs)
}

func MustJSONMarshalAndCompress(v any) string {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func ConvertByJSONMarshal(from, to any) error {
	if to != nil && reflect.TypeOf(to).Kind() != reflect.Ptr {
		panic(fmt.Errorf("type of parameter 'to' is %T, it must be pointer", to))
	}
	b, err := json.Marshal(from)
	if err != nil {
		return fmt.Errorf("marshal 'from' failed: %w", err)
	}
	if err = json.Unmarshal(b, to); err != nil {
		return fmt.Errorf("unmarshal 'to' failed: %w", err)
	}
	return nil
}

func IsNil(v any) (is bool) {
	vv := reflect.ValueOf(v)
	if vv.Kind() == reflect.Invalid {
		return true
	}
	defer func() {
		if recover() != nil {
			is = false
		}
	}()
	return vv.IsNil()
}

func EqualWithNil[V comparable](a *V, b *V) bool {
	if a == nil && b == nil {
		return true
	}
	if a != nil && b != nil {
		return *a == *b
	}
	return false
}

// deref unwraps pointers and interfaces until it reaches a concrete value.
// If the input (or any level) is nil, returns (nil, true).
func deref(v any) (any, bool) {
	if v == nil {
		return nil, true
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil, true
		}
		rv = rv.Elem()
	}
	return rv.Interface(), false
}
