package richstructhelper

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/types/known/timestamppb"

	"sentioxyz/sentio-core/service/common/protos"
)

var (
	mustGetBigInt = func(v *protos.BigInteger) big.Int {
		var bigint big.Int
		bigint.SetBytes(v.GetData())
		if v.GetNegative() {
			bigint.Neg(&bigint)
		}
		return bigint
	}

	mustGetDecimal = func(v *protos.BigDecimal) decimal.Decimal {
		bigint := mustGetBigInt(v.GetValue())
		return decimal.NewFromBigInt(&bigint, v.GetExp())
	}
)

func IsNull(val *protos.RichValue) bool {
	_, is := val.GetValue().(*protos.RichValue_NullValue_)
	return is
}

func GetString(val *protos.RichValue) (string, bool) {
	switch v := val.GetValue().(type) {
	case *protos.RichValue_NullValue_:
		return "<null>", true
	case *protos.RichValue_StringValue:
		return v.StringValue, true
	case *protos.RichValue_BytesValue:
		return hexutil.Encode(v.BytesValue), true
	case *protos.RichValue_BoolValue:
		return strconv.FormatBool(v.BoolValue), true
	case *protos.RichValue_IntValue:
		return strconv.FormatInt(int64(v.IntValue), 10), true
	case *protos.RichValue_Int64Value:
		return strconv.FormatInt(v.Int64Value, 10), true
	case *protos.RichValue_TimestampValue:
		return v.TimestampValue.String(), true
	case *protos.RichValue_FloatValue:
		return fmt.Sprintf("%f", v.FloatValue), true
	case *protos.RichValue_BigintValue:
		return fromBigInt(v.BigintValue).String(), true
	case *protos.RichValue_BigdecimalValue:
		return fromBigDecimal(v.BigdecimalValue).String(), true
	}
	return "", false
}

func GetBoolean(val *protos.RichValue) (bool, bool) {
	switch v := val.GetValue().(type) {
	case *protos.RichValue_BoolValue:
		return v.BoolValue, true
	case *protos.RichValue_IntValue:
		return v.IntValue != 0, true
	case *protos.RichValue_Int64Value:
		return v.Int64Value != 0, true
	case *protos.RichValue_BigintValue:
		return fromBigInt(v.BigintValue).Sign() != 0, true
	case *protos.RichValue_FloatValue:
		return v.FloatValue != 0, true
	case *protos.RichValue_BigdecimalValue:
		return !fromBigDecimal(v.BigdecimalValue).IsZero(), true
	case *protos.RichValue_StringValue:
		if b, err := strconv.ParseBool(v.StringValue); err == nil {
			return b, true
		}
	}
	return false, false
}

func GetBigInt(val *protos.RichValue) (*big.Int, bool) {
	switch v := val.GetValue().(type) {
	case *protos.RichValue_IntValue:
		return big.NewInt(int64(v.IntValue)), true
	case *protos.RichValue_Int64Value:
		return big.NewInt(v.Int64Value), true
	case *protos.RichValue_BigintValue:
		return fromBigInt(v.BigintValue), true
	case *protos.RichValue_FloatValue:
		return decimal.NewFromFloat(v.FloatValue).Round(0).BigInt(), true
	case *protos.RichValue_BigdecimalValue:
		return fromBigDecimal(v.BigdecimalValue).Round(0).BigInt(), true
	case *protos.RichValue_StringValue:
		return new(big.Int).SetString(v.StringValue, 0)
	}
	return nil, false
}

func GetInt(val *protos.RichValue) (int32, bool) {
	switch v := val.GetValue().(type) {
	case *protos.RichValue_IntValue:
		return v.IntValue, true
	case *protos.RichValue_Int64Value:
		return int32(v.Int64Value), true
	case *protos.RichValue_BigintValue:
		return int32(fromBigInt(v.BigintValue).Int64()), true
	case *protos.RichValue_FloatValue:
		return int32(math.Round(v.FloatValue)), true
	case *protos.RichValue_BigdecimalValue:
		return int32(fromBigDecimal(v.BigdecimalValue).Round(0).IntPart()), true
	case *protos.RichValue_StringValue:
		if d, err := strconv.ParseInt(v.StringValue, 0, 32); err == nil {
			return int32(d), true
		}
	}
	return 0, false
}

func GetInt64(val *protos.RichValue) (int64, bool) {
	switch v := val.GetValue().(type) {
	case *protos.RichValue_IntValue:
		return int64(v.IntValue), true
	case *protos.RichValue_Int64Value:
		return v.Int64Value, true
	case *protos.RichValue_BigintValue:
		return fromBigInt(v.BigintValue).Int64(), true
	case *protos.RichValue_FloatValue:
		return int64(math.Round(v.FloatValue)), true
	case *protos.RichValue_BigdecimalValue:
		return fromBigDecimal(v.BigdecimalValue).Round(0).IntPart(), true
	case *protos.RichValue_StringValue:
		if d, err := strconv.ParseInt(v.StringValue, 0, 64); err == nil {
			return d, true
		}
	}
	return 0, false
}

func GetBigDecimal(val *protos.RichValue) (decimal.Decimal, bool) {
	switch v := val.GetValue().(type) {
	case *protos.RichValue_IntValue:
		return decimal.NewFromInt(int64(v.IntValue)), true
	case *protos.RichValue_Int64Value:
		return decimal.NewFromInt(v.Int64Value), true
	case *protos.RichValue_BigintValue:
		return decimal.NewFromBigInt(fromBigInt(v.BigintValue), 0), true
	case *protos.RichValue_FloatValue:
		return decimal.NewFromFloat(v.FloatValue), true
	case *protos.RichValue_BigdecimalValue:
		return fromBigDecimal(v.BigdecimalValue), true
	case *protos.RichValue_StringValue:
		if d, err := decimal.NewFromString(v.StringValue); err == nil {
			return d, true
		}
	}
	return decimal.Decimal{}, false
}

func GetFloat(val *protos.RichValue) (float64, bool) {
	switch v := val.GetValue().(type) {
	case *protos.RichValue_IntValue:
		return float64(v.IntValue), true
	case *protos.RichValue_Int64Value:
		return float64(v.Int64Value), true
	case *protos.RichValue_BigintValue:
		if f, acc := fromBigInt(v.BigintValue).Float64(); acc == big.Exact {
			return f, true
		}
	case *protos.RichValue_FloatValue:
		return v.FloatValue, true
	case *protos.RichValue_BigdecimalValue:
		f, _ := fromBigDecimal(v.BigdecimalValue).Float64()
		return f, true
	case *protos.RichValue_StringValue:
		if f, err := strconv.ParseFloat(v.StringValue, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func GetValue(val *protos.RichValue) (interface{}, bool) {
	switch v := val.GetValue().(type) {
	case *protos.RichValue_NullValue_:
		return nil, true
	case *protos.RichValue_StringValue:
		return v.StringValue, true
	case *protos.RichValue_BytesValue:
		return v.BytesValue, true
	case *protos.RichValue_BoolValue:
		return v.BoolValue, true
	case *protos.RichValue_IntValue:
		return v.IntValue, true
	case *protos.RichValue_Int64Value:
		return v.Int64Value, true
	case *protos.RichValue_TimestampValue:
		return v.TimestampValue.AsTime(), true
	case *protos.RichValue_FloatValue:
		return v.FloatValue, true
	case *protos.RichValue_BigintValue:
		return fromBigInt(v.BigintValue), true
	case *protos.RichValue_BigdecimalValue:
		return fromBigDecimal(v.BigdecimalValue), true
	}
	return nil, false
}

func GetTokenPrice(val *protos.RichValue, defaultTimestamp time.Time) (map[string]any, bool) {
	var tokenAmount = make(map[string]any)
	switch v := val.GetValue().(type) {
	case *protos.RichValue_TokenValue:
		var (
			amount    decimal.Decimal
			timestamp *timestamppb.Timestamp
		)
		amount = mustGetDecimal(v.TokenValue.GetAmount())
		if v.TokenValue.GetSpecifiedAt() != nil {
			timestamp = v.TokenValue.GetSpecifiedAt()
		} else {
			timestamp = timestamppb.New(defaultTimestamp)
		}
		switch v.TokenValue.GetToken().Id.(type) {
		case *protos.CoinID_Symbol:
			tokenAmount["address"] = ""
			tokenAmount["chain"] = ""
			tokenAmount["symbol"] = v.TokenValue.GetToken().GetSymbol()
			tokenAmount["amount"] = amount
			tokenAmount["timestamp"] = timestamp.AsTime()
		case *protos.CoinID_Address:
			tokenAmount["address"] = v.TokenValue.GetToken().GetAddress().GetAddress()
			tokenAmount["chain"] = v.TokenValue.GetToken().GetAddress().GetChain()
			tokenAmount["symbol"] = ""
			tokenAmount["amount"] = amount
			tokenAmount["timestamp"] = timestamp.AsTime()
		}
	default:
		return nil, false
	}
	return tokenAmount, true
}
