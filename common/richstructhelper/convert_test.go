package richstructhelper

import (
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"math/big"
	"sentioxyz/sentio-core/service/common/protos"
	"testing"
)

func Test_GetBoolean(t *testing.T) {
	testcases := []struct {
		ok bool
		e  bool
		v  *protos.RichValue
	}{
		{true, true, NewIntValue(1)},
		{true, true, NewIntValue(-1)},
		{true, false, NewIntValue(0)},
		{true, true, NewInt64Value(1)},
		{true, true, NewInt64Value(-1)},
		{true, false, NewInt64Value(0)},
		{true, true, NewBigIntValue(new(big.Int).SetInt64(1))},
		{true, true, NewBigIntValue(new(big.Int).SetInt64(-1))},
		{true, false, NewBigIntValue(new(big.Int).SetInt64(0))},
		{true, true, NewFloatValue(1)},
		{true, true, NewFloatValue(0.1)},
		{true, true, NewFloatValue(0.01)},
		{true, true, NewFloatValue(10)},
		{true, true, NewFloatValue(100)},
		{true, true, NewFloatValue(-1)},
		{true, true, NewFloatValue(-0.1)},
		{true, true, NewFloatValue(-0.01)},
		{true, true, NewFloatValue(-10)},
		{true, true, NewFloatValue(-100)},
		{true, false, NewFloatValue(0)},
		{true, true, NewBigDecimalValue(decimal.NewFromFloat(1))},
		{true, true, NewBigDecimalValue(decimal.NewFromFloat(0.1))},
		{true, true, NewBigDecimalValue(decimal.NewFromFloat(0.01))},
		{true, true, NewBigDecimalValue(decimal.NewFromFloat(10))},
		{true, true, NewBigDecimalValue(decimal.NewFromFloat(100))},
		{true, true, NewBigDecimalValue(decimal.NewFromFloat(-1))},
		{true, true, NewBigDecimalValue(decimal.NewFromFloat(-0.1))},
		{true, true, NewBigDecimalValue(decimal.NewFromFloat(-0.01))},
		{true, true, NewBigDecimalValue(decimal.NewFromFloat(-10))},
		{true, true, NewBigDecimalValue(decimal.NewFromFloat(-100))},
		{true, false, NewBigDecimalValue(decimal.NewFromFloat(0))},
		{true, true, NewStringValue("true")},
		{true, true, NewStringValue("True")},
		{true, true, NewStringValue("TRUE")},
		{true, true, NewStringValue("t")},
		{true, true, NewStringValue("T")},
		{true, true, NewStringValue("1")},
		{true, false, NewStringValue("false")},
		{true, false, NewStringValue("False")},
		{true, false, NewStringValue("FALSE")},
		{true, false, NewStringValue("f")},
		{true, false, NewStringValue("F")},
		{true, false, NewStringValue("0")},
		{false, false, NewStringValue("")},
		{false, false, NewStringValue("2")},
		{false, false, NewStringValue("tt")},
	}
	for i, testcase := range testcases {
		v, is := GetBoolean(testcase.v)
		assert.Equal(t, testcase.ok, is, "case #%d: %v", i, testcase)
		assert.Equal(t, testcase.e, v, "case #%d: %v", i, testcase)
	}
}

func Test_GetBigInt(t *testing.T) {
	testcases := []struct {
		ok bool
		e  *big.Int
		v  *protos.RichValue
	}{
		{true, big.NewInt(123), NewIntValue(123)},
		{true, big.NewInt(123), NewInt64Value(123)},
		{true, big.NewInt(123), NewBigIntValue(new(big.Int).SetInt64(123))},
		{true, big.NewInt(123), NewFloatValue(123)},
		{true, big.NewInt(123), NewFloatValue(123.1)},
		{true, big.NewInt(123), NewFloatValue(122.9)},
		{true, big.NewInt(123), NewBigDecimalValue(decimal.NewFromFloat(123))},
		{true, big.NewInt(123), NewBigDecimalValue(decimal.NewFromFloat(123.1))},
		{true, big.NewInt(123), NewBigDecimalValue(decimal.NewFromFloat(122.9))},
		{true, big.NewInt(123), NewStringValue("123")},
		{true, big.NewInt(291), NewStringValue("0x123")},
		{true, big.NewInt(83), NewStringValue("0123")},
		{false, nil, NewStringValue("a123")},
	}
	for i, testcase := range testcases {
		v, is := GetBigInt(testcase.v)
		assert.Equal(t, testcase.ok, is, "case #%d: %v", i, testcase)
		assert.Equal(t, testcase.e, v, "case #%d: %v", i, testcase)
	}
}

func Test_GetInt(t *testing.T) {
	testcases := []struct {
		ok bool
		e  int32
		v  *protos.RichValue
	}{
		{true, 123, NewIntValue(123)},
		{true, 123, NewInt64Value(123)},
		{true, 1569325055, NewInt64Value(99999999999999999)}, //1569325055=0x5D89FFFF 99999999999999999=0x16345785D89FFFF
		{true, 123, NewBigIntValue(new(big.Int).SetInt64(123))},
		{true, 123, NewFloatValue(123)},
		{true, 123, NewFloatValue(123.1)},
		{true, 123, NewFloatValue(122.9)},
		{true, 123, NewBigDecimalValue(decimal.NewFromFloat(123))},
		{true, 123, NewBigDecimalValue(decimal.NewFromFloat(123.1))},
		{true, 123, NewBigDecimalValue(decimal.NewFromFloat(122.9))},
		{true, 123, NewStringValue("123")},
		{true, 291, NewStringValue("0x123")},
		{true, 83, NewStringValue("0123")},
		{false, 0, NewStringValue("a123")},
	}
	for i, testcase := range testcases {
		v, is := GetInt(testcase.v)
		assert.Equal(t, testcase.ok, is, "case #%d: %v", i, testcase)
		assert.Equal(t, testcase.e, v, "case #%d: %v", i, testcase)
	}
}

func Test_GetInt64(t *testing.T) {
	testcases := []struct {
		ok bool
		e  int64
		v  *protos.RichValue
	}{
		{true, 123, NewIntValue(123)},
		{true, 123, NewInt64Value(123)},
		{true, 99999999999999999, NewInt64Value(99999999999999999)},
		{true, 123, NewBigIntValue(new(big.Int).SetInt64(123))},
		{true, 123, NewFloatValue(123)},
		{true, 123, NewFloatValue(123.1)},
		{true, 123, NewFloatValue(122.9)},
		{true, 123, NewBigDecimalValue(decimal.NewFromFloat(123))},
		{true, 123, NewBigDecimalValue(decimal.NewFromFloat(123.1))},
		{true, 123, NewBigDecimalValue(decimal.NewFromFloat(122.9))},
		{true, 123, NewStringValue("123")},
		{true, 291, NewStringValue("0x123")},
		{true, 83, NewStringValue("0123")},
		{false, 0, NewStringValue("a123")},
	}
	for i, testcase := range testcases {
		v, is := GetInt64(testcase.v)
		assert.Equal(t, testcase.ok, is, "case #%d: %v", i, testcase)
		assert.Equal(t, testcase.e, v, "case #%d: %v", i, testcase)
	}
}

func Test_GetBigDecimal(t *testing.T) {
	testcases := []struct {
		ok bool
		e  decimal.Decimal
		v  *protos.RichValue
	}{
		{true, decimal.NewFromFloat(123), NewIntValue(123)},
		{true, decimal.NewFromFloat(123), NewInt64Value(123)},
		{true, decimal.NewFromFloat(123), NewBigIntValue(new(big.Int).SetInt64(123))},
		{true, decimal.NewFromFloat(123), NewFloatValue(123)},
		{true, decimal.NewFromFloat(123.1), NewFloatValue(123.1)},
		{true, decimal.NewFromFloat(122.9), NewFloatValue(122.9)},
		{true, decimal.NewFromFloat(123), NewBigDecimalValue(decimal.NewFromFloat(123))},
		{true, decimal.NewFromFloat(123.1), NewBigDecimalValue(decimal.NewFromFloat(123.1))},
		{true, decimal.NewFromFloat(122.9), NewBigDecimalValue(decimal.NewFromFloat(122.9))},
		{true, decimal.NewFromFloat(123), NewStringValue("123")},
		{true, decimal.NewFromFloat(123.1), NewStringValue("123.1")},
		{true, decimal.NewFromFloat(122.9), NewStringValue("122.9")},
		{true, decimal.NewFromFloat(122.9), NewStringValue("+122.9")},
		{true, decimal.NewFromFloat(-122.9), NewStringValue("-122.9")},
		{false, decimal.Decimal{}, NewStringValue("122.9.123")},
		{false, decimal.Decimal{}, NewStringValue("a122.9")},
	}
	for i, testcase := range testcases {
		v, is := GetBigDecimal(testcase.v)
		assert.Equal(t, testcase.ok, is, "case #%d: %v", i, testcase)
		assert.Equal(t, testcase.e, v, "case #%d: %v", i, testcase)
	}
}

func Test_GetFloat(t *testing.T) {
	testcases := []struct {
		ok bool
		e  float64
		v  *protos.RichValue
	}{
		{true, 123, NewIntValue(123)},
		{true, 123, NewInt64Value(123)},
		{true, 123, NewBigIntValue(new(big.Int).SetInt64(123))},
		{true, 123, NewFloatValue(123)},
		{true, 123.1, NewFloatValue(123.1)},
		{true, 122.9, NewFloatValue(122.9)},
		{true, 123, NewBigDecimalValue(decimal.NewFromFloat(123))},
		{true, 123.1, NewBigDecimalValue(decimal.NewFromFloat(123.1))},
		{true, 122.9, NewBigDecimalValue(decimal.NewFromFloat(122.9))},
		{true, 123, NewStringValue("123")},
		{true, 123.1, NewStringValue("123.1")},
		{true, 122.9, NewStringValue("122.9")},
		{true, 122.9, NewStringValue("+122.9")},
		{true, -122.9, NewStringValue("-122.9")},
		{false, 0, NewStringValue("122.9.123")},
		{false, 0, NewStringValue("a122.9")},
	}
	for i, testcase := range testcases {
		v, is := GetFloat(testcase.v)
		assert.Equal(t, testcase.ok, is, "case #%d: %v", i, testcase)
		assert.Equal(t, testcase.e, v, "case #%d: %v", i, testcase)
	}
}
