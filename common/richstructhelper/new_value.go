package richstructhelper

import (
	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math/big"
	"sentioxyz/sentio-core/service/common/protos"
	"time"
)

func NewNullValue() *protos.RichValue {
	return &protos.RichValue{Value: &protos.RichValue_NullValue_{}}
}

func NewIntValue(x int32) *protos.RichValue {
	return &protos.RichValue{Value: &protos.RichValue_IntValue{IntValue: x}}
}

func NewInt64Value(x int64) *protos.RichValue {
	return &protos.RichValue{Value: &protos.RichValue_Int64Value{Int64Value: x}}
}

func NewFloatValue(x float64) *protos.RichValue {
	return &protos.RichValue{Value: &protos.RichValue_FloatValue{FloatValue: x}}
}

func NewStringValue(str string) *protos.RichValue {
	return &protos.RichValue{Value: &protos.RichValue_StringValue{StringValue: str}}
}

func NewBytesValue(bytes []byte) *protos.RichValue {
	return &protos.RichValue{Value: &protos.RichValue_BytesValue{BytesValue: bytes}}
}

func NewBoolValue(b bool) *protos.RichValue {
	return &protos.RichValue{Value: &protos.RichValue_BoolValue{BoolValue: b}}
}

func NewTimestampValue(t time.Time) *protos.RichValue {
	return &protos.RichValue{Value: &protos.RichValue_TimestampValue{TimestampValue: timestamppb.New(t)}}
}

func NewBigIntValue(n *big.Int) *protos.RichValue {
	return &protos.RichValue{Value: &protos.RichValue_BigintValue{BigintValue: buildBigInteger(n)}}
}

func NewBigDecimalValue(n decimal.Decimal) *protos.RichValue {
	return &protos.RichValue{Value: &protos.RichValue_BigdecimalValue{BigdecimalValue: buildBigDecimal(n)}}
}

func NewListValue(items ...*protos.RichValue) *protos.RichValue {
	return &protos.RichValue{Value: &protos.RichValue_ListValue{ListValue: &protos.RichValueList{Values: items}}}
}
