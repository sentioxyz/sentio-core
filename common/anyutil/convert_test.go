package anyutil

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ToString(t *testing.T) {
	assert.Equal(t, "abc", ToString("abc"))
	assert.Equal(t, "abc", ToString([]byte("abc")))
	assert.Equal(t, "123", ToString(123))
	assert.Equal(t, "123456", ToString(uint64(123456)))
}

func Test_ParseInt(t *testing.T) {
	testcases := []any{
		int(10000000), int64(10000000), nil,
		int8(123), int64(123), nil,
		int16(30000), int64(30000), nil,
		int32(10000000), int64(10000000), nil,
		int64(10000000), int64(10000000), nil,
		uint(10000000), int64(10000000), nil,
		uint8(123), int64(123), nil,
		uint16(30000), int64(30000), nil,
		uint32(10000000), int64(10000000), nil,
		uint64(10000000), int64(10000000), nil,
		float32(10000000), int64(10000000), nil,
		float64(10000000), int64(10000000), nil,
		"10000000", int64(10000000), nil,
		uint64(math.MaxUint64), int64(0), errors.New("too big"),
	}
	for i := 0; i < len(testcases); i += 3 {
		r, err := ParseInt(testcases[i])
		assert.Equal(t, testcases[i+1], r, "case[%d]: %T %v", i/3, testcases[i], testcases[i])
		assert.Equal(t, testcases[i+2], err, "case[%d]: %v", i/3, testcases[i])
	}
}

func Test_ParseUint(t *testing.T) {
	testcases := []any{
		int(10000000), uint64(10000000), nil,
		int8(123), uint64(123), nil,
		int16(30000), uint64(30000), nil,
		int32(10000000), uint64(10000000), nil,
		int64(10000000), uint64(10000000), nil,
		uint(10000000), uint64(10000000), nil,
		uint8(123), uint64(123), nil,
		uint16(30000), uint64(30000), nil,
		uint32(10000000), uint64(10000000), nil,
		uint64(10000000), uint64(10000000), nil,
		float32(10000000), uint64(10000000), nil,
		float32(10000001), uint64(10000001), nil,
		float32(9999999), uint64(9999999), nil,
		float64(10000000), uint64(10000000), nil,
		"10000000", uint64(10000000), nil,
		int(-1), uint64(0), errors.New("negative number"),
		int8(-1), uint64(0), errors.New("negative number"),
		int16(-1), uint64(0), errors.New("negative number"),
		int32(-1), uint64(0), errors.New("negative number"),
		int64(-1), uint64(0), errors.New("negative number"),
	}
	for i := 0; i < len(testcases); i += 3 {
		r, err := ParseUint(testcases[i])
		assert.Equal(t, testcases[i+1], r, "case[%d]: %T %v", i/3, testcases[i], testcases[i])
		assert.Equal(t, testcases[i+2], err, "case[%d]: %v", i/3, testcases[i])
	}
}

func Test_ParseInt32(t *testing.T) {
	testcases := []any{
		int(10000000), int32(10000000), nil,
		int32(10000000), int32(10000000), nil,
		int64(10000000), int32(10000000), nil,
		uint(10000000), int32(10000000), nil,
		uint32(10000000), int32(10000000), nil,
		uint64(10000000), int32(10000000), nil,
		float32(10000000), int32(10000000), nil,
		float64(10000000), int32(10000000), nil,
		"10000000", int32(10000000), nil,
		int64(math.MinInt64), int32(0), errors.New("too small"),
		int64(math.MaxInt64), int32(0), errors.New("too big"),
	}
	for i := 0; i < len(testcases); i += 3 {
		r, err := ParseInt32(testcases[i])
		assert.Equal(t, testcases[i+1], r, "case[%d]: %T %v", i/3, testcases[i], testcases[i])
		assert.Equal(t, testcases[i+2], err, "case[%d]: %v", i/3, testcases[i])
	}
}

func Test_ParseFloat64(t *testing.T) {
	testcases := []any{
		int(10000000), float64(10000000), nil,
		int8(123), float64(123), nil,
		int16(30000), float64(30000), nil,
		int32(10000000), float64(10000000), nil,
		int64(10000000), float64(10000000), nil,
		uint(10000000), float64(10000000), nil,
		uint8(123), float64(123), nil,
		uint16(30000), float64(30000), nil,
		uint32(10000000), float64(10000000), nil,
		uint64(10000000), float64(10000000), nil,
		float32(0), float64(0), nil,
		float32(10), float64(10), nil,
		float32(10.01), float64(10.010000228881836), nil,
		float32(0.01), float64(0.009999999776482582), nil,
		float64(10000000), float64(10000000), nil,
		float64(10000000.0000000001), float64(10000000.0000000001), nil,
		float64(10000000.000000000001), float64(10000000.000000000001), nil,
		"10000000", float64(10000000), nil,
		"1.1e10", float64(1.1e10), nil,
	}
	for i := 0; i < len(testcases); i += 3 {
		r, err := ParseFloat64(testcases[i])
		assert.Equal(t, testcases[i+1], r, "case[%d]: %T %v", i/3, testcases[i], testcases[i])
		assert.Equal(t, testcases[i+2], err, "case[%d]: %v", i/3, testcases[i])
	}
}

func Test_strEqual(t *testing.T) {
	var a = "abc"
	var b = "abc"
	var c = "abcd"

	var xa any = a
	var xb any = b
	var xc any = c

	assert.True(t, xa == xb)
	assert.True(t, xa != xc)
	assert.True(t, xb != xc)

	assert.True(t, a == xb)
	assert.True(t, a != xc)
	assert.True(t, b != xc)

	assert.True(t, xa == b)
	assert.True(t, xa != c)
	assert.True(t, xb != c)
}

func Test_intEqual(t *testing.T) {
	var a int64 = 11
	var b int64 = 11
	var c int64 = 222

	var xa any = a
	var xb any = b
	var xc any = c

	assert.True(t, xa == xb)
	assert.True(t, xa != xc)
	assert.True(t, xb != xc)

	assert.True(t, a == xb)
	assert.True(t, a != xc)
	assert.True(t, b != xc)

	assert.True(t, xa == b)
	assert.True(t, xa != c)
	assert.True(t, xb != c)
}

func Test_crossTypeEqual(t *testing.T) {
	var a int64 = 11
	var b int64 = 11
	var c = "11"
	var d int32 = 11

	var xa any = a
	var xb any = b
	var xc any = c
	var xd any = d

	assert.True(t, xa == xb)
	assert.True(t, xa != xc)
	assert.True(t, xa != xd)
	assert.True(t, xb != xc)
	assert.True(t, xb != xd)
	assert.True(t, xc != xd)

	assert.True(t, a == xb)
	assert.True(t, a != xc)
	assert.True(t, a != xd)
	assert.True(t, b != xc)
	assert.True(t, b != xd)
	assert.True(t, c != xd)

	assert.True(t, xa == b)
	assert.True(t, xa != c)
	assert.True(t, xa != d)
	assert.True(t, xb != c)
	assert.True(t, xb != d)
	assert.True(t, xc != d)
}
