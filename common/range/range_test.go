package rg

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewSingleRange(t *testing.T) {
	r := NewSingleRange(5)
	assert.Equal(t, uint64(5), r.Start)
	assert.Equal(t, uint64(5), *r.End)
	assert.False(t, r.IsEmpty())
	assert.True(t, r.Contains(5))
	assert.False(t, r.Contains(4))
	assert.False(t, r.Contains(6))
}

func Test_NewRangeBySize(t *testing.T) {
	// nil size → infinite range
	r := NewRangeBySize(10, nil)
	assert.Equal(t, uint64(10), r.Start)
	assert.Nil(t, r.End)

	// zero size → EmptyRange
	zero := uint64(0)
	r = NewRangeBySize(10, &zero)
	assert.True(t, r.IsEmpty())

	// normal case: start=10, size=5 → [10,14]
	size := uint64(5)
	r = NewRangeBySize(10, &size)
	assert.Equal(t, uint64(10), r.Start)
	assert.Equal(t, uint64(14), *r.End)
}

func Test_NewRangeByEndAndSize(t *testing.T) {
	// zero size → EmptyRange
	r := NewRangeByEndAndSize(10, 0)
	assert.True(t, r.IsEmpty())

	// end+1 < size → start clamped to 0
	r = NewRangeByEndAndSize(2, 10)
	assert.Equal(t, uint64(0), r.Start)
	assert.Equal(t, uint64(2), *r.End)

	// normal case: end=9, size=5 → [5,9]
	r = NewRangeByEndAndSize(9, 5)
	assert.Equal(t, uint64(5), r.Start)
	assert.Equal(t, uint64(9), *r.End)
}

func Test_EndOrZero(t *testing.T) {
	assert.Equal(t, uint64(0), Range{Start: 5}.EndOrZero())
	assert.Equal(t, uint64(10), NewRange(5, 10).EndOrZero())
}

func Test_EndOr(t *testing.T) {
	// nil End → returns the supplied default
	assert.Equal(t, uint64(42), Range{Start: 5}.EndOr(42))
	assert.Equal(t, uint64(0), Range{Start: 5}.EndOr(0))

	// non-nil End → returns *End regardless of default
	assert.Equal(t, uint64(10), NewRange(5, 10).EndOr(42))
	assert.Equal(t, uint64(10), NewRange(5, 10).EndOr(0))
}

func Test_EndOrMaxUInt64(t *testing.T) {
	assert.Equal(t, uint64(math.MaxUint64), Range{Start: 5}.EndOrMaxUInt64())
	assert.Equal(t, uint64(10), NewRange(5, 10).EndOrMaxUInt64())
}

func Test_rangeSize(t *testing.T) {
	// nil end → nil
	assert.Nil(t, Range{Start: 5}.Size())

	// empty range → 0
	s := EmptyRange.Size()
	assert.NotNil(t, s)
	assert.Equal(t, uint64(0), *s)

	// single element → 1
	s = NewSingleRange(5).Size()
	assert.NotNil(t, s)
	assert.Equal(t, uint64(1), *s)

	// [5,9] → 5
	s = NewRange(5, 9).Size()
	assert.NotNil(t, s)
	assert.Equal(t, uint64(5), *s)
}

func Test_rangeContains(t *testing.T) {
	assert.False(t, EmptyRange.Contains(0))
	assert.False(t, EmptyRange.Contains(5))

	r := NewRange(3, 7)
	assert.False(t, r.Contains(2))
	assert.True(t, r.Contains(3))
	assert.True(t, r.Contains(5))
	assert.True(t, r.Contains(7))
	assert.False(t, r.Contains(8))

	// infinite range
	inf := Range{Start: 5}
	assert.False(t, inf.Contains(4))
	assert.True(t, inf.Contains(5))
	assert.True(t, inf.Contains(1000))
}

func Test_rangeString(t *testing.T) {
	assert.Equal(t, "[0,INF]", Range{Start: 0}.String())
	assert.Equal(t, "[5,INF]", Range{Start: 5}.String())
	assert.Equal(t, "[1,3/3]", NewRange(1, 3).String())
	assert.Contains(t, EmptyRange.String(), "EMPTY")
}

func Test_MoveLeftBorder(t *testing.T) {
	r := NewRange(5, 10)

	// normal shift: 5-3=2
	moved := r.MoveLeftBorder(3)
	assert.Equal(t, uint64(2), moved.Start)
	assert.Equal(t, uint64(10), *moved.End)

	// clamp to 0: 5-7 underflows
	moved = r.MoveLeftBorder(7)
	assert.Equal(t, uint64(0), moved.Start)
	assert.Equal(t, uint64(10), *moved.End)

	// infinite end preserved
	inf := Range{Start: 10}
	moved = inf.MoveLeftBorder(4)
	assert.Equal(t, uint64(6), moved.Start)
	assert.Nil(t, moved.End)
}

func Test_rangeIntersection(t *testing.T) {
	assert.True(t, EmptyRange.Intersection(EmptyRange).IsEmpty())
	assert.True(t, EmptyRange.Intersection(NewRange(1, 5)).IsEmpty())
	assert.True(t, NewRange(1, 5).Intersection(EmptyRange).IsEmpty())

	// non-overlapping
	assert.True(t, NewRange(1, 3).Intersection(NewRange(5, 7)).IsEmpty())

	// adjacent but not overlapping: [1,4] ∩ [5,9] = empty
	assert.True(t, NewRange(1, 4).Intersection(NewRange(5, 9)).IsEmpty())

	// partial overlap
	assert.Equal(t, NewRange(3, 5), NewRange(1, 5).Intersection(NewRange(3, 7)))

	// one contains the other
	assert.Equal(t, NewRange(3, 7), NewRange(1, 10).Intersection(NewRange(3, 7)))

	// with infinite end
	assert.Equal(t, NewRange(8, 10), NewRange(5, 10).Intersection(Range{Start: 8}))
	assert.Equal(t, NewRange(5, 7), Range{Start: 5}.Intersection(NewRange(3, 7)))
}

func Test_Cover(t *testing.T) {
	assert.True(t, EmptyRange.Cover(EmptyRange).IsEmpty())
	assert.Equal(t, NewRange(1, 5), EmptyRange.Cover(NewRange(1, 5)))
	assert.Equal(t, NewRange(1, 5), NewRange(1, 5).Cover(EmptyRange))

	// union of non-overlapping ranges
	r := NewRange(1, 5).Cover(NewRange(3, 10))
	assert.Equal(t, NewRange(1, 10), r)

	r = NewRange(3, 10).Cover(NewRange(1, 5))
	assert.Equal(t, NewRange(1, 10), r)

	// infinite end
	r = NewRange(1, 5).Cover(Range{Start: 3})
	assert.Equal(t, uint64(1), r.Start)
	assert.Nil(t, r.End)
}

func Test_GetDistance(t *testing.T) {
	assert.Equal(t, uint64(0), EmptyRange.GetDistance(NewRange(1, 5)))
	assert.Equal(t, uint64(0), NewRange(1, 5).GetDistance(EmptyRange))

	// overlapping → 0
	assert.Equal(t, uint64(0), NewRange(1, 5).GetDistance(NewRange(3, 7)))

	// adjacent (no gap) → 0
	assert.Equal(t, uint64(0), NewRange(1, 5).GetDistance(NewRange(6, 10)))

	// gap of 1
	assert.Equal(t, uint64(1), NewRange(1, 5).GetDistance(NewRange(7, 10)))
	assert.Equal(t, uint64(1), NewRange(7, 10).GetDistance(NewRange(1, 5)))

	// larger gap
	assert.Equal(t, uint64(4), NewRange(1, 5).GetDistance(NewRange(10, 15)))
}

func Test_rangeCutByFixedSize(t *testing.T) {
	// infinite range panics regardless of num
	assert.Panics(t, func() {
		Range{Start: 0}.CutByFixedSize(0, 10, 0)
	})
	assert.Equal(t, []Range{
		NewRange(0, 9),
		NewRange(10, 19),
		NewRange(20, 29),
		NewRange(30, 39),
		NewRange(40, 49),
	}, Range{Start: 0}.CutByFixedSize(0, 10, 5))

	assert.Nil(t, EmptyRange.CutByFixedSize(0, 10, 0))

	// size=0 → returns the whole range unchanged
	r := NewRange(5, 10)
	assert.Equal(t, []Range{r}, r.CutByFixedSize(0, 0, 0))

	// aligned from base=0, size=5: [5,9], [10,14]
	result := NewRange(5, 14).CutByFixedSize(0, 5, 0)
	assert.Equal(t, []Range{NewRange(5, 9), NewRange(10, 14)}, result)

	// num=1 limits output
	result = NewRange(5, 14).CutByFixedSize(0, 5, 1)
	assert.Equal(t, []Range{NewRange(5, 9)}, result)
}

func Test_RangeSetter(t *testing.T) {
	fixed := NewRange(10, 20)
	op := RangeSetter(fixed)
	assert.Equal(t, fixed, op(NewRange(1, 5)))
	assert.Equal(t, fixed, op(EmptyRange))
	assert.Equal(t, fixed, op(Range{Start: 0}))
}

func Test_remove(t *testing.T) {
	for si := uint64(0); si <= 10; si++ {
		for ei := si - 1; ei <= 10; ei++ {
			for sj := uint64(0); sj <= 10; sj++ {
				for ej := sj - 1; ej <= 10; ej++ {
					ri := NewRange(si, ei)
					rj := NewRange(sj, ej)
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
	assert.Equal(t, true, EmptyRange.Equal(NewRange(1, 0)))
	assert.Equal(t, true, NewRange(1, 0).Equal(EmptyRange))

	assert.Equal(t, true, Range{Start: 1}.Equal(Range{Start: 1}))
	assert.Equal(t, false, Range{Start: 1}.Equal(Range{Start: 2}))

	for si := uint64(0); si <= 10; si++ {
		for ei := si; ei <= 10; ei++ {
			for sj := uint64(0); sj <= 10; sj++ {
				for ej := sj; ej <= 10; ej++ {
					ri := NewRange(si, ei)
					rj := NewRange(sj, ej)
					eq := si == sj && ei == ej
					assert.Equalf(t, eq, ri.Equal(rj), "invalid result ri:%s, rj:%s, eq:%v", ri, rj, eq)
				}
			}
		}
	}
}

func Test_include(t *testing.T) {
	assert.Equal(t, true, EmptyRange.Include(NewRange(1, 0)))
	assert.Equal(t, true, NewRange(1, 0).Include(EmptyRange))

	assert.Equal(t, false, EmptyRange.Include(Range{Start: 0}))
	assert.Equal(t, true, Range{Start: 0}.Include(EmptyRange))

	assert.Equal(t, true, Range{Start: 0}.Include(NewRange(0, 0)))
	assert.Equal(t, true, Range{Start: 0}.Include(NewRange(0, 1)))
	assert.Equal(t, true, Range{Start: 0}.Include(NewRange(1, 1)))
	assert.Equal(t, true, Range{Start: 0}.Include(NewRange(1, 2)))
	assert.Equal(t, false, NewRange(0, 0).Include(Range{Start: 0}))
	assert.Equal(t, false, NewRange(0, 1).Include(Range{Start: 0}))
	assert.Equal(t, false, NewRange(1, 1).Include(Range{Start: 0}))
	assert.Equal(t, false, NewRange(1, 2).Include(Range{Start: 0}))

	assert.Equal(t, true, NewRange(1, 3).Include(NewRange(1, 2)))
	assert.Equal(t, true, NewRange(1, 3).Include(NewRange(1, 3)))
	assert.Equal(t, true, NewRange(1, 3).Include(NewRange(2, 3)))
	assert.Equal(t, false, NewRange(1, 2).Include(NewRange(1, 3)))
	assert.Equal(t, true, NewRange(1, 3).Include(NewRange(1, 3)))
	assert.Equal(t, false, NewRange(2, 3).Include(NewRange(1, 3)))
}
