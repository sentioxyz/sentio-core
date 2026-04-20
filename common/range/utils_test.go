package rg

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

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
