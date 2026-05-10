package rg

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_greaterNilAsInf(t *testing.T) {
	one := uint64(1)
	two := uint64(2)
	testcases := []struct {
		a, b *uint64
		gt   bool
		ge   bool
	}{
		{a: nil, b: nil, gt: false, ge: true},
		{a: nil, b: &one, gt: true, ge: true},
		{a: &one, b: nil, gt: false, ge: false},
		{a: &two, b: &one, gt: true, ge: true},
		{a: &one, b: &two, gt: false, ge: false},
		{a: &two, b: &two, gt: false, ge: true},
	}
	for i, tc := range testcases {
		assert.Equalf(t, tc.gt, GreaterNilAsInf(tc.a, tc.b), "testcase #%d GreaterNilAsInf", i)
		assert.Equalf(t, tc.ge, GreaterEqualNilAsInf(tc.a, tc.b), "testcase #%d GreaterEqualNilAsInf", i)
	}
}

func Test_nilAsInfEdgeCases(t *testing.T) {
	one, two, three := uint64(1), uint64(2), uint64(3)

	// MinNilAsInf with no args → nil
	assert.Nil(t, MinNilAsInf())

	// MinNilAsInf: all nil → nil
	assert.Nil(t, MinNilAsInf(nil, nil))

	// MinNilAsInf with 3 args
	assert.Equal(t, uint64(1), *MinNilAsInf(&one, &two, &three))
	assert.Equal(t, uint64(1), *MinNilAsInf(&three, &one, &two))

	// MaxNilAsInf with no args → pointer to 0 (zero value)
	r := MaxNilAsInf()
	assert.NotNil(t, r)
	assert.Equal(t, uint64(0), *r)

	// MaxNilAsInf: one nil → nil
	assert.Nil(t, MaxNilAsInf(nil, &one))
	assert.Nil(t, MaxNilAsInf(&one, nil))

	// MaxNilAsInf with 3 non-nil args
	assert.Equal(t, uint64(3), *MaxNilAsInf(&one, &two, &three))
	assert.Equal(t, uint64(3), *MaxNilAsInf(&three, &one, &two))
}

func Test_cmpNilAsInf(t *testing.T) {
	one := uint64(1)
	two := uint64(2)
	testcases := []struct {
		a, b, max, min *uint64
		eq, lt, le     bool
	}{
		{a: nil, b: nil, max: nil, min: nil, eq: true, lt: false, le: true},
		{a: nil, b: &one, max: nil, min: &one, eq: false, lt: false, le: false},
		{a: &one, b: nil, max: nil, min: &one, eq: false, lt: true, le: true},
		{a: &one, b: &two, max: &two, min: &one, eq: false, lt: true, le: true},
		{a: &two, b: &one, max: &two, min: &one, eq: false, lt: false, le: false},
		{a: &two, b: &two, max: &two, min: &two, eq: true, lt: false, le: true},
	}
	for i, tc := range testcases {
		assert.Equalf(t, tc.max, MaxNilAsInf(tc.a, tc.b), "testcase #%d: %v", i, tc)
		assert.Equalf(t, tc.min, MinNilAsInf(tc.a, tc.b), "testcase #%d: %v", i, tc)
		assert.Equalf(t, tc.eq, EqualNilAsInf(tc.a, tc.b), "testcase #%d: %v", i, tc)
		assert.Equalf(t, tc.lt, LessNilAsInf(tc.a, tc.b), "testcase #%d: %v", i, tc)
		assert.Equalf(t, tc.le, LessEqualNilAsInf(tc.a, tc.b), "testcase #%d: %v", i, tc)
	}
}
