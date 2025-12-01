package anyutil

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	protoscommon "sentioxyz/sentio-core/service/common/protos"

	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
				IntValue: MustParseInt32(v),
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
				DoubleValue: MustParseFloat64(v),
			},
		}
	case int, uint, int64, uint64:
		return &protoscommon.Any{
			AnyValue: &protoscommon.Any_LongValue{
				LongValue: MustParseInt(v),
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
