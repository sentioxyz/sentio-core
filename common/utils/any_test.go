package utils

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAny2Float(t *testing.T) {
	var float64Value = 1234.00
	var float32Value float32 = 1234.00
	var int64Value int64 = 1234
	var int32Value int32 = 1234
	var intValue = 1234
	var uint64Value uint64 = 1234
	var uint32Value uint32 = 1234
	var uintValue uint = 1234
	var stringValue = "1234.00"

	anySlice := []any{
		float64Value,
		float32Value,
		int64Value,
		int32Value,
		intValue,
		uint64Value,
		uint32Value,
		uintValue,
		stringValue,
	}
	for _, anyValue := range anySlice {
		value, err := Any2Float(anyValue)
		require.NoError(t, err)
		require.Equal(t, float64Value, value)
	}
}

func TestAny2String(t *testing.T) {
	var float64Value = 1234.00
	var float32Value float32 = 1234.00
	var int64Value int64 = 1234
	var int32Value int32 = 1234
	var intValue = 1234
	var uint64Value uint64 = 1234
	var uint32Value uint32 = 1234
	var uintValue uint = 1234
	var stringValue = "1234"
	var float64ValueWithPrecision = 1234.123456789
	var float32ValueWithPrecision = 1234.1234

	anySlice := []any{
		float64Value,
		float32Value,
		int64Value,
		int32Value,
		intValue,
		uint64Value,
		uint32Value,
		uintValue,
		stringValue,
	}
	for _, anyValue := range anySlice {
		require.Equal(t, stringValue, Any2String(anyValue))
	}
	require.Equal(t, "1234.12", Any2String(float64ValueWithPrecision))
	require.Equal(t, "1234.12", Any2String(float32ValueWithPrecision))
}

func TestAny2String2(t *testing.T) {
	var a json.RawMessage
	fmt.Println("!!!", string(a), "!!!")
}
