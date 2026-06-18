package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/utils"
)

func newBlockRange(a, b uint64) BlockRange {
	return BlockRange{StartBlock: a, EndBlock: &b}
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

func Test_remove(t *testing.T) {
	for si := uint64(0); si <= 10; si++ {
		for ei := si - 1; ei <= 10; ei++ {
			for sj := uint64(0); sj <= 10; sj++ {
				for ej := sj - 1; ej <= 10; ej++ {
					ri := newBlockRange(si, ei)
					rj := newBlockRange(sj, ej)
					rk := ri.Remove(rj)
					for x := uint64(0); x <= 11; x++ {
						ok := si <= x && x <= ei && !(sj <= x && x <= ej)
						//log.Debugf("!!! ri:%s, rj:%s, ri.Remove(rj):%s, x:%d, ok:%v\n", ri, rj, rk, x, ok)
						assert.Equalf(t, ok, rk.Contains(x), "invalid result ri:%s, rj:%s, ri.Remove(rj):%s, x:%d, ok:%v", rj, rj, rk, x, ok)
					}
				}
			}
		}
	}
}

func Test_equal(t *testing.T) {
	assert.Equal(t, true, EmptyBlockRange.Equal(newBlockRange(1, 0)))
	assert.Equal(t, true, newBlockRange(1, 0).Equal(EmptyBlockRange))

	for si := uint64(0); si <= 10; si++ {
		for ei := si; ei <= 10; ei++ {
			for sj := uint64(0); sj <= 10; sj++ {
				for ej := sj; ej <= 10; ej++ {
					ri := newBlockRange(si, ei)
					rj := newBlockRange(sj, ej)
					eq := si == sj && ei == ej
					assert.Equalf(t, eq, ri.Equal(rj), "invalid result ri:%s, rj:%s, eq:%v", ri, rj, eq)
				}
			}
		}
	}
}

func Test_include(t *testing.T) {
	assert.Equal(t, true, EmptyBlockRange.Include(newBlockRange(1, 0)))
	assert.Equal(t, true, newBlockRange(1, 0).Include(EmptyBlockRange))
	assert.Equal(t, false, EmptyBlockRange.Include(BlockRange{StartBlock: 0}))
	assert.Equal(t, true, BlockRange{StartBlock: 0}.Include(EmptyBlockRange))
}

func Test_setContains(t *testing.T) {
	set := BlockRangeSet{ // [1][4-6][10]
		BlockRange: newBlockRange(1, 10),
		Holes: [][2]uint64{
			{2, 3},
			{7, 9},
		},
	}
	contains := []uint64{1, 4, 5, 6, 10}
	for x := uint64(0); x <= 11; x++ {
		ok := utils.IndexOf(contains, x) >= 0
		assert.Equalf(t, ok, set.Contains(x), "invalid result contains:%v", x)
	}
}

func Test_setInclude(t *testing.T) {
	set := BlockRangeSet{ // [1][4-6][10]
		BlockRange: newBlockRange(1, 10),
		Holes: [][2]uint64{
			{2, 3},
			{7, 9},
		},
	}
	okPairs := [][2]uint64{
		{1, 1},
		{4, 4},
		{4, 5},
		{4, 6},
		{5, 5},
		{5, 6},
		{6, 6},
		{10, 10},
	}
	for s := uint64(1); s <= 10; s++ {
		for e := s; e <= 10; e++ {
			ok := utils.IndexOf(okPairs, [2]uint64{s, e}) >= 0
			assert.Equalf(t, ok, set.Include(newBlockRange(s, e)), "invalid testcase: [%s,%s], ok: %v", s, e, ok)
		}
	}
}

func Test_setLast(t *testing.T) {
	assert.Equal(t, EmptyBlockRange, EmptyBlockRangeSet.Last())

	set := BlockRangeSet{
		BlockRange: newBlockRange(1, 10),
	}
	assert.Equal(t, newBlockRange(1, 10), set.Last())

	set.Holes = [][2]uint64{
		{2, 3},
		{7, 9},
	}
	assert.Equal(t, newBlockRange(10, 10), set.Last())
}

func Test_setIntersection(t *testing.T) {
	assert.Equal(t, EmptyBlockRangeSet, EmptyBlockRangeSet.Intersection(EmptyBlockRange))
	assert.Equal(t, EmptyBlockRangeSet, EmptyBlockRangeSet.Intersection(BlockRange{StartBlock: 1}))

	// [1][4-6][10]
	set := BlockRangeSet{
		BlockRange: newBlockRange(1, 10),
		Holes:      [][2]uint64{{2, 3}, {7, 9}},
	}
	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(EmptyBlockRange))
	assert.Equal(t, set, set.Intersection(BlockRange{StartBlock: 0}))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 1)}, set.Intersection(newBlockRange(1, 1)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 1)}, set.Intersection(newBlockRange(1, 2)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 1)}, set.Intersection(newBlockRange(1, 3)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 4), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newBlockRange(1, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 5), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newBlockRange(1, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newBlockRange(1, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newBlockRange(1, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newBlockRange(1, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newBlockRange(1, 9)))

	assert.Equal(t, set, set.Intersection(newBlockRange(1, 10)))
	assert.Equal(t, set, set.Intersection(newBlockRange(1, 11)))
	assert.Equal(t, set, set.Intersection(newBlockRange(0, 10)))
	assert.Equal(t, set, set.Intersection(newBlockRange(0, 11)))

	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(newBlockRange(2, 2)))
	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(newBlockRange(2, 3)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 4)}, set.Intersection(newBlockRange(2, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 5)}, set.Intersection(newBlockRange(2, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(2, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(2, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(2, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(2, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newBlockRange(2, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newBlockRange(2, 11)))

	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(newBlockRange(3, 3)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 4)}, set.Intersection(newBlockRange(3, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 5)}, set.Intersection(newBlockRange(3, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(3, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(3, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(3, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(3, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newBlockRange(3, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newBlockRange(3, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 4)}, set.Intersection(newBlockRange(4, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 5)}, set.Intersection(newBlockRange(4, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(4, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(4, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(4, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 6)}, set.Intersection(newBlockRange(4, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newBlockRange(4, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newBlockRange(4, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(5, 5)}, set.Intersection(newBlockRange(5, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(5, 6)}, set.Intersection(newBlockRange(5, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(5, 6)}, set.Intersection(newBlockRange(5, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(5, 6)}, set.Intersection(newBlockRange(5, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(5, 6)}, set.Intersection(newBlockRange(5, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newBlockRange(5, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newBlockRange(5, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(6, 6)}, set.Intersection(newBlockRange(6, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(6, 6)}, set.Intersection(newBlockRange(6, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(6, 6)}, set.Intersection(newBlockRange(6, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(6, 6)}, set.Intersection(newBlockRange(6, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newBlockRange(6, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newBlockRange(6, 11)))

	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(newBlockRange(7, 7)))
	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(newBlockRange(7, 8)))
	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(newBlockRange(7, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Intersection(newBlockRange(7, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Intersection(newBlockRange(7, 11)))

	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(newBlockRange(8, 8)))
	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(newBlockRange(8, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Intersection(newBlockRange(8, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Intersection(newBlockRange(8, 11)))

	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(newBlockRange(9, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Intersection(newBlockRange(9, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Intersection(newBlockRange(9, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Intersection(newBlockRange(10, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Intersection(newBlockRange(10, 11)))

	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(newBlockRange(11, 11)))
	assert.Equal(t, EmptyBlockRangeSet, set.Intersection(newBlockRange(0, 0)))
}

func Test_setRemove(t *testing.T) {
	assert.Equal(t, EmptyBlockRangeSet, EmptyBlockRangeSet.Remove(EmptyBlockRange))
	assert.Equal(t, EmptyBlockRangeSet, EmptyBlockRangeSet.Remove(BlockRange{StartBlock: 1}))

	set := BlockRangeSet{ // [1][4-6][10]
		BlockRange: newBlockRange(1, 10),
		Holes: [][2]uint64{
			{2, 3},
			{7, 9},
		},
	}
	assert.Equal(t, set, set.Remove(EmptyBlockRange))
	assert.Equal(t, EmptyBlockRangeSet, set.Remove(BlockRange{StartBlock: 0}))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newBlockRange(0, 0)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newBlockRange(0, 1)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newBlockRange(0, 2)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newBlockRange(0, 3)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newBlockRange(0, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newBlockRange(0, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Remove(newBlockRange(0, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Remove(newBlockRange(0, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Remove(newBlockRange(0, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Remove(newBlockRange(0, 9)))
	assert.Equal(t, EmptyBlockRangeSet, set.Remove(newBlockRange(0, 10)))
	assert.Equal(t, EmptyBlockRangeSet, set.Remove(newBlockRange(0, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newBlockRange(1, 1)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newBlockRange(1, 2)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newBlockRange(1, 3)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newBlockRange(1, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newBlockRange(1, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Remove(newBlockRange(1, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Remove(newBlockRange(1, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Remove(newBlockRange(1, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(10, 10)}, set.Remove(newBlockRange(1, 9)))
	assert.Equal(t, EmptyBlockRangeSet, set.Remove(newBlockRange(1, 10)))
	assert.Equal(t, EmptyBlockRangeSet, set.Remove(newBlockRange(1, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newBlockRange(2, 2)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newBlockRange(2, 3)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 4}, {7, 9}}}, set.Remove(newBlockRange(2, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 5}, {7, 9}}}, set.Remove(newBlockRange(2, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(2, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(2, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(2, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(2, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 1)}, set.Remove(newBlockRange(2, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 1)}, set.Remove(newBlockRange(2, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newBlockRange(3, 3)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 4}, {7, 9}}}, set.Remove(newBlockRange(3, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 5}, {7, 9}}}, set.Remove(newBlockRange(3, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(3, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(3, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(3, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(3, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 1)}, set.Remove(newBlockRange(3, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 1)}, set.Remove(newBlockRange(3, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 4}, {7, 9}}}, set.Remove(newBlockRange(4, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 5}, {7, 9}}}, set.Remove(newBlockRange(4, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(4, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(4, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(4, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newBlockRange(4, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 1)}, set.Remove(newBlockRange(4, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 1)}, set.Remove(newBlockRange(4, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 5}, {7, 9}}}, set.Remove(newBlockRange(5, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(newBlockRange(5, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(newBlockRange(5, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(newBlockRange(5, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(newBlockRange(5, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 4), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(5, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 4), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(5, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(newBlockRange(6, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(newBlockRange(6, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(newBlockRange(6, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(newBlockRange(6, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 5), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(6, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 5), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(6, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newBlockRange(7, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newBlockRange(7, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newBlockRange(7, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(7, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(7, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newBlockRange(8, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newBlockRange(8, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(8, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(8, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newBlockRange(9, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(9, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(9, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(10, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newBlockRange(10, 11)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newBlockRange(11, 11)))
}

func Test_setUnion(t *testing.T) {
	assert.Equal(t, EmptyBlockRangeSet, EmptyBlockRangeSet.Union(EmptyBlockRange))
	assert.Equal(t, BlockRangeSet{BlockRange: BlockRange{StartBlock: 1}}, BlockRangeSet{BlockRange: BlockRange{StartBlock: 1}}.Union(EmptyBlockRange))
	assert.Equal(t, BlockRangeSet{BlockRange: BlockRange{StartBlock: 1}}, EmptyBlockRangeSet.Union(BlockRange{StartBlock: 1}))

	set := BlockRangeSet{ // [2][5-7][11]
		BlockRange: newBlockRange(2, 11),
		Holes: [][2]uint64{
			{3, 4},
			{8, 10},
		},
	}
	assert.Equal(t, set, set.Union(EmptyBlockRange))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11), Holes: [][2]uint64{{1, 1}, {3, 4}, {8, 10}}}, set.Union(newBlockRange(0, 0)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(0, 1)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(0, 2)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(newBlockRange(0, 3)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(0, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(0, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(0, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(0, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(newBlockRange(0, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(newBlockRange(0, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11)}, set.Union(newBlockRange(0, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 11)}, set.Union(newBlockRange(0, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(0, 12)}, set.Union(newBlockRange(0, 12)))
	assert.Equal(t, BlockRangeSet{BlockRange: BlockRange{StartBlock: 0}}, set.Union(BlockRange{StartBlock: 0}))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(1, 1)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(1, 2)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(newBlockRange(1, 3)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(1, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(1, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(1, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(1, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(newBlockRange(1, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(newBlockRange(1, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 11)}, set.Union(newBlockRange(1, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 11)}, set.Union(newBlockRange(1, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(1, 12)}, set.Union(newBlockRange(1, 12)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(2, 2)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(newBlockRange(2, 3)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(2, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(2, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(2, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(2, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(newBlockRange(2, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(newBlockRange(2, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11)}, set.Union(newBlockRange(2, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11)}, set.Union(newBlockRange(2, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 12)}, set.Union(newBlockRange(2, 12)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(newBlockRange(3, 3)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(3, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(3, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(3, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newBlockRange(3, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(newBlockRange(3, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(newBlockRange(3, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11)}, set.Union(newBlockRange(3, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11)}, set.Union(newBlockRange(3, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 12)}, set.Union(newBlockRange(3, 12)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(newBlockRange(4, 4)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(newBlockRange(4, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(newBlockRange(4, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(newBlockRange(4, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 3}, {9, 10}}}, set.Union(newBlockRange(4, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 3}, {10, 10}}}, set.Union(newBlockRange(4, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 3}}}, set.Union(newBlockRange(4, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 3}}}, set.Union(newBlockRange(4, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 12), Holes: [][2]uint64{{3, 3}}}, set.Union(newBlockRange(4, 12)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(5, 5)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(5, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(5, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(newBlockRange(5, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(newBlockRange(5, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(5, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(5, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(5, 12)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(6, 6)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(6, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(newBlockRange(6, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(newBlockRange(6, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(6, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(6, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(6, 12)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(7, 7)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(newBlockRange(7, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(newBlockRange(7, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(7, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(7, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(7, 12)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(newBlockRange(8, 8)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(newBlockRange(8, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(8, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(8, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(newBlockRange(8, 12)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 8}, {10, 10}}}, set.Union(newBlockRange(9, 9)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 8}}}, set.Union(newBlockRange(9, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 8}}}, set.Union(newBlockRange(9, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 8}}}, set.Union(newBlockRange(9, 12)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 9}}}, set.Union(newBlockRange(10, 10)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 9}}}, set.Union(newBlockRange(10, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 9}}}, set.Union(newBlockRange(10, 12)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(11, 11)))
	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(11, 12)))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newBlockRange(12, 12)))
	assert.Equal(t, BlockRangeSet{BlockRange: BlockRange{StartBlock: 2}, Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(BlockRange{StartBlock: 12}))

	assert.Equal(t, BlockRangeSet{BlockRange: newBlockRange(2, 13), Holes: [][2]uint64{{3, 4}, {8, 10}, {12, 12}}}, set.Union(newBlockRange(13, 13)))
	assert.Equal(t, BlockRangeSet{BlockRange: BlockRange{StartBlock: 2}, Holes: [][2]uint64{{3, 4}, {8, 10}, {12, 12}}}, set.Union(BlockRange{StartBlock: 13}))
}

func Test_CutRangeSet(t *testing.T) {
	assert.Nil(t, CutRangeSet(0, nil))
	assert.Nil(t, CutRangeSet(0, []BlockRange{}))

	src := []BlockRange{
		{StartBlock: 100},
		{StartBlock: 100},
		{StartBlock: 100},
	}
	assert.Equal(t, []BlockRange{{StartBlock: 100}}, CutRangeSet(0, src))
	assert.Equal(t, []BlockRange{{StartBlock: 100}}, CutRangeSet(100, src))
	assert.Equal(t, []BlockRange{{StartBlock: 110}}, CutRangeSet(110, src))

	src = []BlockRange{
		{StartBlock: 100},
		{StartBlock: 200},
		{StartBlock: 300},
	}
	assert.Equal(t, []BlockRange{
		{StartBlock: 100, EndBlock: utils.WrapPointer[uint64](199)},
		{StartBlock: 200, EndBlock: utils.WrapPointer[uint64](299)},
		{StartBlock: 300},
	}, CutRangeSet(0, src))
	assert.Equal(t, []BlockRange{
		{StartBlock: 100, EndBlock: utils.WrapPointer[uint64](199)},
		{StartBlock: 200, EndBlock: utils.WrapPointer[uint64](299)},
		{StartBlock: 300},
	}, CutRangeSet(100, src))
	assert.Equal(t, []BlockRange{
		{StartBlock: 150, EndBlock: utils.WrapPointer[uint64](199)},
		{StartBlock: 200, EndBlock: utils.WrapPointer[uint64](299)},
		{StartBlock: 300},
	}, CutRangeSet(150, src))
	assert.Equal(t, []BlockRange{
		{StartBlock: 199, EndBlock: utils.WrapPointer[uint64](199)},
		{StartBlock: 200, EndBlock: utils.WrapPointer[uint64](299)},
		{StartBlock: 300},
	}, CutRangeSet(199, src))
	assert.Equal(t, []BlockRange{
		{StartBlock: 200, EndBlock: utils.WrapPointer[uint64](299)},
		{StartBlock: 300},
	}, CutRangeSet(200, src))
	assert.Equal(t, []BlockRange{
		{StartBlock: 250, EndBlock: utils.WrapPointer[uint64](299)},
		{StartBlock: 300},
	}, CutRangeSet(250, src))
	assert.Equal(t, []BlockRange{
		{StartBlock: 299, EndBlock: utils.WrapPointer[uint64](299)},
		{StartBlock: 300},
	}, CutRangeSet(299, src))
	assert.Equal(t, []BlockRange{{StartBlock: 300}}, CutRangeSet(300, src))
	assert.Equal(t, []BlockRange{{StartBlock: 301}}, CutRangeSet(301, src))

	assert.Equal(t, []BlockRange{
		{StartBlock: 100, EndBlock: utils.WrapPointer[uint64](150)},
		{StartBlock: 151, EndBlock: utils.WrapPointer[uint64](199)},
		{StartBlock: 200, EndBlock: utils.WrapPointer[uint64](299)},
		{StartBlock: 300, EndBlock: utils.WrapPointer[uint64](350)},
		{StartBlock: 351, EndBlock: utils.WrapPointer[uint64](550)},
	}, CutRangeSet(0, []BlockRange{
		{StartBlock: 100, EndBlock: utils.WrapPointer[uint64](150)},
		{StartBlock: 200, EndBlock: utils.WrapPointer[uint64](350)},
		{StartBlock: 300, EndBlock: utils.WrapPointer[uint64](550)},
	}))

	src = []BlockRange{
		{StartBlock: 100, EndBlock: utils.WrapPointer[uint64](199)},
		{StartBlock: 300, EndBlock: utils.WrapPointer[uint64](399)},
	}
	assert.Equal(t, []BlockRange{
		{StartBlock: 100, EndBlock: utils.WrapPointer[uint64](199)},
		{StartBlock: 200, EndBlock: utils.WrapPointer[uint64](299)},
		{StartBlock: 300, EndBlock: utils.WrapPointer[uint64](399)},
	}, CutRangeSet(0, src))
	assert.Equal(t, []BlockRange{
		{StartBlock: 100, EndBlock: utils.WrapPointer[uint64](199)},
		{StartBlock: 200, EndBlock: utils.WrapPointer[uint64](299)},
		{StartBlock: 300, EndBlock: utils.WrapPointer[uint64](399)},
	}, CutRangeSet(100, src))
	assert.Equal(t, []BlockRange{
		{StartBlock: 199, EndBlock: utils.WrapPointer[uint64](199)},
		{StartBlock: 200, EndBlock: utils.WrapPointer[uint64](299)},
		{StartBlock: 300, EndBlock: utils.WrapPointer[uint64](399)},
	}, CutRangeSet(199, src))
	assert.Equal(t, []BlockRange{{StartBlock: 300, EndBlock: utils.WrapPointer[uint64](399)}}, CutRangeSet(200, src))
	assert.Equal(t, []BlockRange{{StartBlock: 300, EndBlock: utils.WrapPointer[uint64](399)}}, CutRangeSet(299, src))
	assert.Equal(t, []BlockRange{{StartBlock: 300, EndBlock: utils.WrapPointer[uint64](399)}}, CutRangeSet(300, src))
	assert.Equal(t, []BlockRange{{StartBlock: 350, EndBlock: utils.WrapPointer[uint64](399)}}, CutRangeSet(350, src))
	assert.Equal(t, []BlockRange{{StartBlock: 399, EndBlock: utils.WrapPointer[uint64](399)}}, CutRangeSet(399, src))
	assert.Equal(t, []BlockRange(nil), CutRangeSet(400, src))
	assert.Equal(t, []BlockRange(nil), CutRangeSet(401, src))
}
