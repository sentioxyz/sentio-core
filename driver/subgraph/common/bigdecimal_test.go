package common

import (
	"fmt"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func Test_BuildBigDecimalFromString(t *testing.T) {
	testcases := [][]any{
		{"", &BigDecimal{Digits: &BigInt{}, Exp: &BigInt{}}},
		{".", &BigDecimal{Digits: &BigInt{}, Exp: &BigInt{}}},
		{"1", &BigDecimal{Digits: MustBuildBigInt(1), Exp: &BigInt{}}},
		{"10", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(1)}},
		{"100", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(2)}},
		{".1", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(-1)}},
		{".11", &BigDecimal{Digits: MustBuildBigInt(11), Exp: MustBuildBigInt(-2)}},
		{"1.1", &BigDecimal{Digits: MustBuildBigInt(11), Exp: MustBuildBigInt(-1)}},
		{"9.9", &BigDecimal{Digits: MustBuildBigInt(99), Exp: MustBuildBigInt(-1)}},
		{
			"999999999.999999999",
			&BigDecimal{Digits: MustBuildBigInt(999999999999999999), Exp: MustBuildBigInt(-9)},
		},
		{"100000000.000000000", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(8)}},
		{
			"100000000.000000001",
			&BigDecimal{Digits: MustBuildBigInt(100000000000000001), Exp: MustBuildBigInt(-9)},
		},
		{"000000000.000000001", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(-9)}},
		{"000000000.000000009", &BigDecimal{Digits: MustBuildBigInt(9), Exp: MustBuildBigInt(-9)}},
		{"0.000000001e+10", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(1)}},
		{"0.1e-10", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(-11)}},
		{"1.1e-10", &BigDecimal{Digits: MustBuildBigInt(11), Exp: MustBuildBigInt(-11)}},
		{"1.1e+10", &BigDecimal{Digits: MustBuildBigInt(11), Exp: MustBuildBigInt(9)}},
		{"1.10000e+10", &BigDecimal{Digits: MustBuildBigInt(11), Exp: MustBuildBigInt(9)}},
	}
	for i, testcase := range testcases {
		str := testcase[0].(string)
		v, err := BuildBigDecimalFromString(str)
		assert.NoError(t, err, fmt.Sprintf("case #%d: %v", i, str))
		assert.Equal(t, testcase[1], v, fmt.Sprintf("case #%d: %v", i, str))
	}
}

func Test_BuildBigDecimalAndToDecimal(t *testing.T) {
	testcases := [][]any{
		{decimal.New(0, 0), &BigDecimal{Digits: &BigInt{}, Exp: &BigInt{}}},
		{decimal.New(1, 0), &BigDecimal{Digits: MustBuildBigInt(1), Exp: &BigInt{}}},
		{decimal.New(1, 1), &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(1)}},
		{decimal.New(1, 2), &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(2)}},
		{decimal.New(1, -1), &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(-1)}},
		{decimal.New(11, -2), &BigDecimal{Digits: MustBuildBigInt(11), Exp: MustBuildBigInt(-2)}},
		{decimal.New(11, -1), &BigDecimal{Digits: MustBuildBigInt(11), Exp: MustBuildBigInt(-1)}},
		{decimal.New(99, -1), &BigDecimal{Digits: MustBuildBigInt(99), Exp: MustBuildBigInt(-1)}},
		{
			decimal.New(999999999999999999, -9),
			&BigDecimal{Digits: MustBuildBigInt(999999999999999999), Exp: MustBuildBigInt(-9)},
		},
		{decimal.New(1, 8), &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(8)}},
		{
			decimal.New(100000000000000001, -9),
			&BigDecimal{Digits: MustBuildBigInt(100000000000000001), Exp: MustBuildBigInt(-9)},
		},
		{decimal.New(1, -9), &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(-9)}},
	}
	for i, testcase := range testcases {
		val := testcase[0].(decimal.Decimal)
		v := BuildBigDecimal(val)
		assert.Equal(t, testcase[1], v, fmt.Sprintf("case #%d: %v", i, val))
		assert.Equal(t, val, testcase[1].(*BigDecimal).ToDecimal(), fmt.Sprintf("case #%d: %v", i, val))
	}
}

func Test_BigDecimalToString(t *testing.T) {
	testcases := [][]any{
		{"0", &BigDecimal{Digits: &BigInt{}, Exp: &BigInt{}}},
		{"1", &BigDecimal{Digits: MustBuildBigInt(1), Exp: &BigInt{}}},
		{"10", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(1)}},
		{"100", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(2)}},
		{"100", &BigDecimal{Digits: MustBuildBigInt(10), Exp: MustBuildBigInt(1)}},
		{"100", &BigDecimal{Digits: MustBuildBigInt(100), Exp: MustBuildBigInt(0)}},
		{"100.0", &BigDecimal{Digits: MustBuildBigInt(1000), Exp: MustBuildBigInt(-1)}},
		{"0.1", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(-1)}},
		{"0.11", &BigDecimal{Digits: MustBuildBigInt(11), Exp: MustBuildBigInt(-2)}},
		{"0.011", &BigDecimal{Digits: MustBuildBigInt(11), Exp: MustBuildBigInt(-3)}},
		{"1.1", &BigDecimal{Digits: MustBuildBigInt(11), Exp: MustBuildBigInt(-1)}},
		{"9.9", &BigDecimal{Digits: MustBuildBigInt(99), Exp: MustBuildBigInt(-1)}},
		{
			"999999999.999999999",
			&BigDecimal{Digits: MustBuildBigInt(999999999999999999), Exp: MustBuildBigInt(-9)},
		},
		{"100000000", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(8)}},
		{
			"100000000.000000001",
			&BigDecimal{Digits: MustBuildBigInt(100000000000000001), Exp: MustBuildBigInt(-9)},
		},
		{"0.000000001", &BigDecimal{Digits: MustBuildBigInt(1), Exp: MustBuildBigInt(-9)}},
	}
	for i, testcase := range testcases {
		str := testcase[0].(string)
		assert.Equal(t, str, testcase[1].(*BigDecimal).String(), fmt.Sprintf("case #%d: %v", i, str))
	}
}

func Test_truncate(t *testing.T) {
	testcases := [][]int64{
		{0, 100, 0, 0},
		{0, -100, 0, 0},
		{0, 0, 0, 0},

		{2, 100, 2, 100},
		{2, -100, 2, -100},
		{2, 0, 2, 0},

		{20, 100, 2, 101},
		{20, -100, 2, -99},
		{20, 0, 2, 1},

		{1200, 100, 12, 102},
		{1200, 1000000, 12, 1000002},
		{1200, -100, 12, -98},
		{1200, -1000000, 12, -999998},
		{1200, 0, 12, 2},
	}
	for i, testcase := range testcases {
		x := BuildBigDecimal(decimal.New(testcase[0], int32(testcase[1]))).TruncateDigits()
		assert.Equal(t, MustBuildBigInt(testcase[2]), x.Digits, fmt.Sprintf("#%d %v", i, testcase))
		assert.Equal(t, MustBuildBigInt(testcase[3]), x.Exp, fmt.Sprintf("#%d %v", i, testcase))
	}

	var x *BigDecimal

	x = BuildBigDecimalFromBigInt(
		MustBuildBigInt(
			"11111111112222222222333333333344444444445555555555"+
				"11111111112222222222333333333344444444445555555555"+
				"11111111112222222222333333335500000000001111111111"), // len = 150
		1000000,
	).TruncateDigits()
	assert.Equal(t, MustBuildBigInt(
		"11111111112222222222333333333344444444445555555555"+
			"11111111112222222222333333333344444444445555555555"+
			"1111111111222222222233333334"), x.Digits) // len = 128
	assert.Equal(t, MustBuildBigInt(1000022), x.Exp)

	x = BuildBigDecimalFromBigInt(
		MustBuildBigInt(
			"11111111112222222222333333333344444444445555555555"+
				"11111111112222222222333333333344444444445555555555"+
				"11111111112222222222000000005500000000001111111111"), // len = 150
		1000000,
	).TruncateDigits()
	assert.Equal(t, MustBuildBigInt(
		"11111111112222222222333333333344444444445555555555"+
			"11111111112222222222333333333344444444445555555555"+
			"1111111111222222222200000001"), x.Digits) // len = 128
	assert.Equal(t, MustBuildBigInt(1000022), x.Exp)

	x = BuildBigDecimalFromBigInt(
		MustBuildBigInt(
			"99999999999999999999999999999999999999999999999999"+
				"99999999999999999999999999999999999999999999999999"+
				"99999999999999999999999999995500000000001111111111"), // len = 150
		1000000,
	).TruncateDigits()
	assert.Equal(t, MustBuildBigInt(1), x.Digits) // len = 128, and
	assert.Equal(t, MustBuildBigInt(1000150), x.Exp)

	x = BuildBigDecimalFromBigInt(
		MustBuildBigInt(
			"11111111112222222222333333333344444444445555555555"+
				"11111111112222222222333333333344444444445555555555"+
				"11111111112222222222333333333344444444445555555555"), // len = 150
		1000000,
	).TruncateDigits()
	assert.Equal(t, MustBuildBigInt(
		"11111111112222222222333333333344444444445555555555"+
			"11111111112222222222333333333344444444445555555555"+
			"1111111111222222222233333333"), x.Digits) // len = 128
	assert.Equal(t, MustBuildBigInt(1000022), x.Exp)

	x = BuildBigDecimalFromBigInt(
		MustBuildBigInt(
			"11111111112222222222333333333344444444445555555555"+
				"11111111112222222222333333333344444444445555555555"+
				"11111111112222222222000000004400000000001111111111"), // len = 150
		1000000,
	).TruncateDigits()
	assert.Equal(t, MustBuildBigInt(
		"11111111112222222222333333333344444444445555555555"+
			"11111111112222222222333333333344444444445555555555"+
			"11111111112222222222"), x.Digits) // len = 120
	assert.Equal(t, MustBuildBigInt(1000030), x.Exp)

	x = BuildBigDecimalFromBigInt(
		MustBuildBigInt(
			"11111111112222222222333333333344444444445555555555"+
				"00000000000000000000000000000000000000000000000000"+
				"00000000000000000000000000004400000000001111111111"), // len = 150
		1000000,
	).TruncateDigits()
	assert.Equal(t, MustBuildBigInt(
		"11111111112222222222333333333344444444445555555555"), x.Digits) // len = 120
	assert.Equal(t, MustBuildBigInt(1000100), x.Exp)
}
