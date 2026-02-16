package common

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	zero        = []byte{}
	pn1         = []byte{1}
	pn127       = []byte{127}
	pn128       = []byte{128, 0}
	pn255       = []byte{255, 0}
	pn256       = []byte{0, 1}
	pn32767     = []byte{255, 127}
	pn32768     = []byte{0, 128, 0}
	pn65535     = []byte{255, 255, 0}
	pn65536     = []byte{0, 0, 1}
	nn1         = []byte{0xff}
	nn127       = []byte{0x81}
	nn128       = []byte{0x80}
	nn255       = []byte{1, 0xff}
	nn256       = []byte{0, 0xff}
	nn32767     = []byte{1, 0x80}
	nn32768     = []byte{0, 0x80}
	nn65535     = []byte{1, 0, 0xff}
	nn65536     = []byte{0, 0, 0xff}
	nnMinInt64  = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80}
	pnMaxInt64  = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f}
	pnMaxUint64 = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0}
)

func Test_fromBytes(t *testing.T) {
	testcases := [][]any{
		{int64(0), zero},
		{int64(1), pn1},
		{int64(127), pn127},
		{int64(128), pn128},
		{int64(255), pn255},
		{int64(256), pn256},
		{int64(32767), pn32767},
		{int64(32768), pn32768},
		{int64(65535), pn65535},
		{int64(65536), pn65536},
		{int64(-1), nn1},
		{int64(-127), nn127},
		{int64(-128), nn128},
		{int64(-255), nn255},
		{int64(-256), nn256},
		{int64(-32767), nn32767},
		{int64(-32768), nn32768},
		{int64(-65535), nn65535},
		{int64(-65536), nn65536},
	}

	for i, testcase := range testcases {
		val := testcase[0].(int64)
		bv := MustBuildBigInt(testcase[1].([]byte))
		assert.Equal(t, val, bv.Int64(), fmt.Sprintf("case #%d: %v", i, testcase))
	}
	assert.Equal(t, uint64(math.MaxUint64), MustBuildBigInt(pnMaxUint64).Uint64())
	assert.Equal(t, int64(math.MaxInt64), MustBuildBigInt(pnMaxInt64).Int64())
	assert.Equal(t, int64(math.MinInt64), MustBuildBigInt(nnMinInt64).Int64())
}

func Test_toBytes(t *testing.T) {
	testcases := [][]any{
		{int64(0), zero},
		{int64(1), pn1},
		{int64(127), pn127},
		{int64(128), pn128},
		{int64(255), pn255},
		{int64(256), pn256},
		{int64(32767), pn32767},
		{int64(32768), pn32768},
		{int64(65535), pn65535},
		{int64(65536), pn65536},
		{int64(-1), nn1},
		{int64(-127), nn127},
		{int64(-128), nn128},
		{int64(-255), nn255},
		{int64(-256), nn256},
		{int64(-32767), nn32767},
		{int64(-32768), nn32768},
		{int64(-65535), nn65535},
		{int64(-65536), nn65536},
	}

	for i, testcase := range testcases {
		bv := MustBuildBigInt(testcase[0].(int64))
		assert.Equal(t, testcase[1].([]byte), bv.toBytes(), fmt.Sprintf("case #%d: %v", i, testcase))
	}

	assert.Equal(t, pnMaxUint64, MustBuildBigInt(uint64(math.MaxUint64)).toBytes())
	assert.Equal(t, pnMaxInt64, MustBuildBigInt(int64(math.MaxInt64)).toBytes())
	assert.Equal(t, nnMinInt64, MustBuildBigInt(int64(math.MinInt64)).toBytes())
}

func Test_BuildBigIntFromString(t *testing.T) {
	testcases := [][]any{
		{"", zero},
		{"0", zero},
		{"255", pn255},
		{"256", pn256},
		{"-255", nn255},
		{"-256", nn256},
		{"32767", pn32767},
		{"-32767", nn32767},
		{"65535", pn65535},
		{"-65535", nn65535},
		{"18446744073709551615", pnMaxUint64},
		{"9223372036854775807", pnMaxInt64},
		{"-9223372036854775808", nnMinInt64},
		{"000009", []byte{9}},
	}
	for i, testcase := range testcases {
		val, err := BuildBigInt(testcase[0].(string))
		assert.NoError(t, err)
		assert.Equal(t, testcase[1].([]byte), val.toBytes(), fmt.Sprintf("case #%d: %v", i, testcase))
	}
}

func Test_BuildBigIntFromHex(t *testing.T) {
	testcases := [][]any{
		{"", zero, zero},
		{"1", pn1, nn1},
		{"7f", pn127, nn127},
		{"80", pn128, nn128},
		{"ff", pn255, nn255},
		{"100", pn256, nn256},
		{"7fff", pn32767, nn32767},
		{"8000", pn32768, nn32768},
		{"ffff", pn65535, nn65535},
		{"10000", pn65536, nn65536},
	}

	check := func(hex string, exp *BigInt, caseNum int) {
		v, err := BuildBigInt(hex)
		assert.NoError(t, err)
		// Use Cmp instead of Equal to avoid issues with internal big.Int representation changes in Go 1.25.7+
		assert.Equal(t, 0, v.Cmp(exp), fmt.Sprintf("case #%d: %s, expected %s but got %s", caseNum, hex, exp.String(), v.String()))
	}

	for i, testcase := range testcases {
		base := testcase[0].(string)
		pn := MustBuildBigInt(testcase[1].([]byte))
		nn := MustBuildBigInt(testcase[2].([]byte))

		check("0x"+base, pn, i)
		check("0x0"+base, pn, i)
		check("0x00"+base, pn, i)
		check("0x000"+base, pn, i)

		check("-0x"+base, nn, i)
		check("-0x0"+base, nn, i)
		check("-0x00"+base, nn, i)
		check("-0x000"+base, nn, i)

		check("0x-"+base, nn, i)
		check("0x-0"+base, nn, i)
		check("0x-00"+base, nn, i)
		check("0x-000"+base, nn, i)
	}
}

func Test_BigIntToStringHex(t *testing.T) {
	testcases := [][]any{
		{"0", "0x0", zero},
		{"1", "0x1", pn1},
		{"127", "0x7f", pn127},
		{"128", "0x80", pn128},
		{"255", "0xff", pn255},
		{"256", "0x100", pn256},
		{"32767", "0x7fff", pn32767},
		{"32768", "0x8000", pn32768},
		{"65535", "0xffff", pn65535},
		{"65536", "0x10000", pn65536},
		{"-1", "-0x1", nn1},
		{"-127", "-0x7f", nn127},
		{"-128", "-0x80", nn128},
		{"-255", "-0xff", nn255},
		{"-256", "-0x100", nn256},
		{"-32767", "-0x7fff", nn32767},
		{"-32768", "-0x8000", nn32768},
		{"-65535", "-0xffff", nn65535},
		{"-65536", "-0x10000", nn65536},
	}
	for i, testcase := range testcases {
		val := MustBuildBigInt(testcase[2].([]byte))
		assert.Equal(t, testcase[0], val.String(), fmt.Sprintf("case #%d: %v", i, testcase))
		assert.Equal(t, testcase[1], val.ToHex(), fmt.Sprintf("case #%d: %v", i, testcase))
	}
}

func Test_BigIntCmp(t *testing.T) {
	numbers := []*BigInt{
		MustBuildBigInt(nn65536),
		MustBuildBigInt(nn65535),
		MustBuildBigInt(nn32768),
		MustBuildBigInt(nn32767),
		MustBuildBigInt(nn256),
		MustBuildBigInt(nn255),
		MustBuildBigInt(nn128),
		MustBuildBigInt(nn127),
		MustBuildBigInt(nn1),
		MustBuildBigInt(zero),
		MustBuildBigInt(pn1),
		MustBuildBigInt(pn127),
		MustBuildBigInt(pn128),
		MustBuildBigInt(pn255),
		MustBuildBigInt(pn256),
		MustBuildBigInt(pn32767),
		MustBuildBigInt(pn32768),
		MustBuildBigInt(pn65535),
		MustBuildBigInt(pn65536),
	}
	cmp := func(a, b int) int {
		if a < b {
			return -1
		} else if a > b {
			return 1
		} else {
			return 0
		}
	}

	for i := 0; i < len(numbers); i++ {
		for j := 0; j < len(numbers); j++ {
			assert.Equal(t, cmp(i, j), numbers[i].Cmp(numbers[j]),
				fmt.Sprintf("case n[%d]:%s n[%d]:%s", i, numbers[i], j, numbers[j]))
		}
	}
}
