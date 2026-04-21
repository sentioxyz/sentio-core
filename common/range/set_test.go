package rg

import (
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/utils"
	"testing"
)

func Test_setContains(t *testing.T) {
	set := RangeSet{ // [1][4-6][10]
		Range: NewRange(1, 10),
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
	set := RangeSet{ // [1][4-6][10]
		Range: NewRange(1, 10),
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
			assert.Equalf(t, ok, set.Include(NewRange(s, e)), "invalid testcase: [%s,%s], ok: %v", s, e, ok)
		}
	}
}

func Test_setLast(t *testing.T) {
	assert.Equal(t, EmptyRange, EmptyRangeSet.Last())

	set := RangeSet{
		Range: NewRange(1, 10),
	}
	assert.Equal(t, NewRange(1, 10), set.Last())

	set.Holes = [][2]uint64{
		{2, 3},
		{7, 9},
	}
	assert.Equal(t, NewRange(10, 10), set.Last())
}

func Test_setIntersection(t *testing.T) {
	assert.Equal(t, EmptyRangeSet, EmptyRangeSet.Intersection(EmptyRange))
	assert.Equal(t, EmptyRangeSet, EmptyRangeSet.Intersection(Range{Start: 1}))

	// [1][4-6][10]
	set := RangeSet{
		Range: NewRange(1, 10),
		Holes: [][2]uint64{{2, 3}, {7, 9}},
	}
	assert.Equal(t, EmptyRangeSet, set.Intersection(EmptyRange))
	assert.Equal(t, set, set.Intersection(Range{Start: 0}))

	assert.Equal(t, RangeSet{Range: NewRange(1, 1)}, set.Intersection(NewRange(1, 1)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 1)}, set.Intersection(NewRange(1, 2)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 1)}, set.Intersection(NewRange(1, 3)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 4), Holes: [][2]uint64{{2, 3}}}, set.Intersection(NewRange(1, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 5), Holes: [][2]uint64{{2, 3}}}, set.Intersection(NewRange(1, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(NewRange(1, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(NewRange(1, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(NewRange(1, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(NewRange(1, 9)))

	assert.Equal(t, set, set.Intersection(NewRange(1, 10)))
	assert.Equal(t, set, set.Intersection(NewRange(1, 11)))
	assert.Equal(t, set, set.Intersection(NewRange(0, 10)))
	assert.Equal(t, set, set.Intersection(NewRange(0, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(NewRange(2, 2)))
	assert.Equal(t, EmptyRangeSet, set.Intersection(NewRange(2, 3)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 4)}, set.Intersection(NewRange(2, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 5)}, set.Intersection(NewRange(2, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(2, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(2, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(2, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(2, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(NewRange(2, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(NewRange(2, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(NewRange(3, 3)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 4)}, set.Intersection(NewRange(3, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 5)}, set.Intersection(NewRange(3, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(3, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(3, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(3, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(3, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(NewRange(3, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(NewRange(3, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(4, 4)}, set.Intersection(NewRange(4, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 5)}, set.Intersection(NewRange(4, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(4, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(4, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(4, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 6)}, set.Intersection(NewRange(4, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(NewRange(4, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(NewRange(4, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(5, 5)}, set.Intersection(NewRange(5, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(5, 6)}, set.Intersection(NewRange(5, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(5, 6)}, set.Intersection(NewRange(5, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(5, 6)}, set.Intersection(NewRange(5, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(5, 6)}, set.Intersection(NewRange(5, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(NewRange(5, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(NewRange(5, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(6, 6)}, set.Intersection(NewRange(6, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(6, 6)}, set.Intersection(NewRange(6, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(6, 6)}, set.Intersection(NewRange(6, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(6, 6)}, set.Intersection(NewRange(6, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(NewRange(6, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(NewRange(6, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(NewRange(7, 7)))
	assert.Equal(t, EmptyRangeSet, set.Intersection(NewRange(7, 8)))
	assert.Equal(t, EmptyRangeSet, set.Intersection(NewRange(7, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Intersection(NewRange(7, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Intersection(NewRange(7, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(NewRange(8, 8)))
	assert.Equal(t, EmptyRangeSet, set.Intersection(NewRange(8, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Intersection(NewRange(8, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Intersection(NewRange(8, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(NewRange(9, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Intersection(NewRange(9, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Intersection(NewRange(9, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Intersection(NewRange(10, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Intersection(NewRange(10, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(NewRange(11, 11)))
	assert.Equal(t, EmptyRangeSet, set.Intersection(NewRange(0, 0)))
}

func Test_setRemove(t *testing.T) {
	assert.Equal(t, EmptyRangeSet, EmptyRangeSet.Remove(EmptyRange))
	assert.Equal(t, EmptyRangeSet, EmptyRangeSet.Remove(Range{Start: 1}))

	set := RangeSet{ // [1][4-6][10]
		Range: NewRange(1, 10),
		Holes: [][2]uint64{
			{2, 3},
			{7, 9},
		},
	}
	assert.Equal(t, set, set.Remove(EmptyRange))
	assert.Equal(t, EmptyRangeSet, set.Remove(Range{Start: 0}))

	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(NewRange(0, 0)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(NewRange(0, 1)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(NewRange(0, 2)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(NewRange(0, 3)))
	assert.Equal(t, RangeSet{Range: NewRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(NewRange(0, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(NewRange(0, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Remove(NewRange(0, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Remove(NewRange(0, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Remove(NewRange(0, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Remove(NewRange(0, 9)))
	assert.Equal(t, EmptyRangeSet, set.Remove(NewRange(0, 10)))
	assert.Equal(t, EmptyRangeSet, set.Remove(NewRange(0, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(NewRange(1, 1)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(NewRange(1, 2)))
	assert.Equal(t, RangeSet{Range: NewRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(NewRange(1, 3)))
	assert.Equal(t, RangeSet{Range: NewRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(NewRange(1, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(NewRange(1, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Remove(NewRange(1, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Remove(NewRange(1, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Remove(NewRange(1, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(10, 10)}, set.Remove(NewRange(1, 9)))
	assert.Equal(t, EmptyRangeSet, set.Remove(NewRange(1, 10)))
	assert.Equal(t, EmptyRangeSet, set.Remove(NewRange(1, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(NewRange(2, 2)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(NewRange(2, 3)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 4}, {7, 9}}}, set.Remove(NewRange(2, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 5}, {7, 9}}}, set.Remove(NewRange(2, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(2, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(2, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(2, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(2, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 1)}, set.Remove(NewRange(2, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 1)}, set.Remove(NewRange(2, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(NewRange(3, 3)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 4}, {7, 9}}}, set.Remove(NewRange(3, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 5}, {7, 9}}}, set.Remove(NewRange(3, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(3, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(3, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(3, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(3, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 1)}, set.Remove(NewRange(3, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 1)}, set.Remove(NewRange(3, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 4}, {7, 9}}}, set.Remove(NewRange(4, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 5}, {7, 9}}}, set.Remove(NewRange(4, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(4, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(4, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(4, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(NewRange(4, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 1)}, set.Remove(NewRange(4, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 1)}, set.Remove(NewRange(4, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 5}, {7, 9}}}, set.Remove(NewRange(5, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(NewRange(5, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(NewRange(5, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(NewRange(5, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(NewRange(5, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 4), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(5, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 4), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(5, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(NewRange(6, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(NewRange(6, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(NewRange(6, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(NewRange(6, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 5), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(6, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 5), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(6, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(NewRange(7, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(NewRange(7, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(NewRange(7, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(7, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(7, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(NewRange(8, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(NewRange(8, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(8, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(8, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(NewRange(9, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(9, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(9, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(10, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(NewRange(10, 11)))

	assert.Equal(t, RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(NewRange(11, 11)))
}

func Test_setUnion(t *testing.T) {
	assert.Equal(t, EmptyRangeSet, EmptyRangeSet.Union(EmptyRange))
	assert.Equal(t, RangeSet{Range: Range{Start: 1}}, RangeSet{Range: Range{Start: 1}}.Union(EmptyRange))
	assert.Equal(t, RangeSet{Range: Range{Start: 1}}, EmptyRangeSet.Union(Range{Start: 1}))

	set := RangeSet{ // [2][5-7][11]
		Range: NewRange(2, 11),
		Holes: [][2]uint64{
			{3, 4},
			{8, 10},
		},
	}
	assert.Equal(t, set, set.Union(EmptyRange))

	assert.Equal(t, RangeSet{Range: NewRange(0, 11), Holes: [][2]uint64{{1, 1}, {3, 4}, {8, 10}}}, set.Union(NewRange(0, 0)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(0, 1)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(0, 2)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(NewRange(0, 3)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(0, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(0, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(0, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(0, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(NewRange(0, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(NewRange(0, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 11)}, set.Union(NewRange(0, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 11)}, set.Union(NewRange(0, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(0, 12)}, set.Union(NewRange(0, 12)))
	assert.Equal(t, RangeSet{Range: Range{Start: 0}}, set.Union(Range{Start: 0}))

	assert.Equal(t, RangeSet{Range: NewRange(1, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(1, 1)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(1, 2)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(NewRange(1, 3)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(1, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(1, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(1, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(1, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(NewRange(1, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(NewRange(1, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 11)}, set.Union(NewRange(1, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 11)}, set.Union(NewRange(1, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(1, 12)}, set.Union(NewRange(1, 12)))

	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(2, 2)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(NewRange(2, 3)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(2, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(2, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(2, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(2, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(NewRange(2, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(NewRange(2, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11)}, set.Union(NewRange(2, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11)}, set.Union(NewRange(2, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 12)}, set.Union(NewRange(2, 12)))

	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(NewRange(3, 3)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(3, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(3, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(3, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(NewRange(3, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(NewRange(3, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(NewRange(3, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11)}, set.Union(NewRange(3, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11)}, set.Union(NewRange(3, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 12)}, set.Union(NewRange(3, 12)))

	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(NewRange(4, 4)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(NewRange(4, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(NewRange(4, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(NewRange(4, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 3}, {9, 10}}}, set.Union(NewRange(4, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 3}, {10, 10}}}, set.Union(NewRange(4, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 3}}}, set.Union(NewRange(4, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 3}}}, set.Union(NewRange(4, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 12), Holes: [][2]uint64{{3, 3}}}, set.Union(NewRange(4, 12)))

	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(5, 5)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(5, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(5, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(NewRange(5, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(NewRange(5, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(5, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(5, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(5, 12)))

	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(6, 6)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(6, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(NewRange(6, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(NewRange(6, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(6, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(6, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(6, 12)))

	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(7, 7)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(NewRange(7, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(NewRange(7, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(7, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(7, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(7, 12)))

	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(NewRange(8, 8)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(NewRange(8, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(8, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(8, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(NewRange(8, 12)))

	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 8}, {10, 10}}}, set.Union(NewRange(9, 9)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 8}}}, set.Union(NewRange(9, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 8}}}, set.Union(NewRange(9, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 8}}}, set.Union(NewRange(9, 12)))

	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 9}}}, set.Union(NewRange(10, 10)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 9}}}, set.Union(NewRange(10, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 9}}}, set.Union(NewRange(10, 12)))

	assert.Equal(t, RangeSet{Range: NewRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(11, 11)))
	assert.Equal(t, RangeSet{Range: NewRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(11, 12)))

	assert.Equal(t, RangeSet{Range: NewRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(NewRange(12, 12)))
	assert.Equal(t, RangeSet{Range: Range{Start: 2}, Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(Range{Start: 12}))

	assert.Equal(t, RangeSet{Range: NewRange(2, 13), Holes: [][2]uint64{{3, 4}, {8, 10}, {12, 12}}}, set.Union(NewRange(13, 13)))
	assert.Equal(t, RangeSet{Range: Range{Start: 2}, Holes: [][2]uint64{{3, 4}, {8, 10}, {12, 12}}}, set.Union(Range{Start: 13}))
}

func Test_NewRangeSet(t *testing.T) {
	assert.Equal(t, EmptyRangeSet, NewRangeSet())
	assert.Equal(t, RangeSet{Range: NewRange(1, 5)}, NewRangeSet(NewRange(1, 5)))

	// two non-overlapping ranges with gap
	rs := NewRangeSet(NewRange(1, 3), NewRange(5, 7))
	assert.Equal(t, uint64(1), rs.Start)
	assert.Equal(t, uint64(7), *rs.End)
	assert.Equal(t, [][2]uint64{{4, 4}}, rs.Holes)

	// overlapping ranges → merged
	rs = NewRangeSet(NewRange(1, 5), NewRange(3, 7))
	assert.Equal(t, NewRange(1, 7), rs.Range)
	assert.Empty(t, rs.Holes)

	// adjacent ranges → merged
	rs = NewRangeSet(NewRange(1, 4), NewRange(5, 7))
	assert.Equal(t, NewRange(1, 7), rs.Range)
	assert.Empty(t, rs.Holes)

	// one includes the other
	rs = NewRangeSet(NewRange(1, 10), NewRange(3, 7))
	assert.Equal(t, NewRange(1, 10), rs.Range)
	assert.Empty(t, rs.Holes)

	// three ranges with gaps
	rs = NewRangeSet(NewRange(1, 2), NewRange(5, 6), NewRange(9, 10))
	assert.Equal(t, uint64(1), rs.Start)
	assert.Equal(t, uint64(10), *rs.End)
	assert.Equal(t, [][2]uint64{{3, 4}, {7, 8}}, rs.Holes)
}

func Test_setEqual(t *testing.T) {
	assert.True(t, EmptyRangeSet.Equal(EmptyRangeSet))

	set := RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{3, 4}}}
	assert.True(t, set.Equal(set))

	// different range
	assert.False(t, set.Equal(RangeSet{Range: NewRange(1, 9), Holes: [][2]uint64{{3, 4}}}))

	// missing holes
	assert.False(t, set.Equal(RangeSet{Range: NewRange(1, 10)}))

	// different hole values
	assert.False(t, set.Equal(RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{4, 5}}}))
}

func Test_setFirst(t *testing.T) {
	assert.Equal(t, EmptyRange, EmptyRangeSet.First())

	// no holes → whole range is first
	assert.Equal(t, NewRange(1, 10), RangeSet{Range: NewRange(1, 10)}.First())

	// [1][4-6][10] → first segment is [1,1]
	set := RangeSet{
		Range: NewRange(1, 10),
		Holes: [][2]uint64{{2, 3}, {7, 9}},
	}
	assert.Equal(t, NewRange(1, 1), set.First())
}

func Test_GetRanges(t *testing.T) {
	// empty set returns one empty range
	ranges := EmptyRangeSet.GetRanges()
	assert.Len(t, ranges, 1)
	assert.True(t, ranges[0].IsEmpty())

	// no holes
	assert.Equal(t, []Range{NewRange(1, 5)}, RangeSet{Range: NewRange(1, 5)}.GetRanges())

	// [1][4-6][10]
	set := RangeSet{
		Range: NewRange(1, 10),
		Holes: [][2]uint64{{2, 3}, {7, 9}},
	}
	assert.Equal(t, []Range{NewRange(1, 1), NewRange(4, 6), NewRange(10, 10)}, set.GetRanges())

	// infinite end
	infSet := RangeSet{Range: Range{Start: 5}, Holes: [][2]uint64{{7, 9}}}
	r := infSet.GetRanges()
	assert.Len(t, r, 2)
	assert.Equal(t, NewRange(5, 6), r[0])
	assert.Equal(t, uint64(10), r[1].Start)
	assert.Nil(t, r[1].End)
}

func Test_FindContains(t *testing.T) {
	_, ok := EmptyRangeSet.FindContains(NewRange(1, 5))
	assert.False(t, ok)

	// set: [1][4-6][10]
	set := RangeSet{
		Range: NewRange(1, 10),
		Holes: [][2]uint64{{2, 3}, {7, 9}},
	}

	// found in first segment
	r, ok := set.FindContains(NewRange(1, 1))
	assert.True(t, ok)
	assert.Equal(t, NewRange(1, 1), r)

	// found in middle segment
	r, ok = set.FindContains(NewRange(4, 6))
	assert.True(t, ok)
	assert.Equal(t, NewRange(4, 6), r)

	// spans a hole → not found
	_, ok = set.FindContains(NewRange(1, 4))
	assert.False(t, ok)

	// range entirely in a hole → not found
	_, ok = set.FindContains(NewRange(2, 3))
	assert.False(t, ok)

	// infinite query against finite set → not found
	_, ok = set.FindContains(Range{Start: 1})
	assert.False(t, ok)

	// infinite query against infinite set → found in last segment
	infSet := RangeSet{Range: Range{Start: 1}, Holes: [][2]uint64{{2, 3}}}
	r, ok = infSet.FindContains(Range{Start: 4})
	assert.True(t, ok)
	assert.Equal(t, uint64(4), r.Start)
	assert.Nil(t, r.End)
}

func Test_setCutByFixedSize(t *testing.T) {
	assert.Panics(t, func() {
		RangeSet{Range: Range{Start: 0}}.CutByFixedSize(10, true)
	})

	// [1][4-6][10], size=2, not aligned
	// GetRanges: [1,1], [4,6], [10,10]
	// [1,1] base=1: [1,2]∩[1,1]=[1,1]
	// [4,6] base=4: [4,5], [6,7]∩[4,6]=[6,6]
	// [10,10] base=10: [10,11]∩[10,10]=[10,10]
	set := RangeSet{
		Range: NewRange(1, 10),
		Holes: [][2]uint64{{2, 3}, {7, 9}},
	}
	result := set.CutByFixedSize(2, false)
	assert.Equal(t, []Range{NewRange(1, 1), NewRange(4, 5), NewRange(6, 6), NewRange(10, 10)}, result)

	// aligned to zero, size=5
	// [1,1] base=0: [0,4]∩[1,1]=[1,1]
	// [4,6] base=0: [0,4]∩[4,6]=[4,4], [5,9]∩[4,6]=[5,6]
	// [10,10] base=0: [10,14]∩[10,10]=[10,10]
	result = set.CutByFixedSize(5, true)
	assert.Equal(t, []Range{NewRange(1, 1), NewRange(4, 4), NewRange(5, 6), NewRange(10, 10)}, result)
}

func Test_CutRangeSet(t *testing.T) {
	assert.Nil(t, CutRangeSet(0, nil))
	assert.Nil(t, CutRangeSet(0, []Range{}))

	src := []Range{
		{Start: 100},
		{Start: 100},
		{Start: 100},
	}
	assert.Equal(t, []Range{{Start: 100}}, CutRangeSet(0, src))
	assert.Equal(t, []Range{{Start: 100}}, CutRangeSet(100, src))
	assert.Equal(t, []Range{{Start: 110}}, CutRangeSet(110, src))

	src = []Range{
		{Start: 100},
		{Start: 200},
		{Start: 300},
	}
	assert.Equal(t, []Range{
		{Start: 100, End: utils.WrapPointer[uint64](199)},
		{Start: 200, End: utils.WrapPointer[uint64](299)},
		{Start: 300},
	}, CutRangeSet(0, src))
	assert.Equal(t, []Range{
		{Start: 100, End: utils.WrapPointer[uint64](199)},
		{Start: 200, End: utils.WrapPointer[uint64](299)},
		{Start: 300},
	}, CutRangeSet(100, src))
	assert.Equal(t, []Range{
		{Start: 150, End: utils.WrapPointer[uint64](199)},
		{Start: 200, End: utils.WrapPointer[uint64](299)},
		{Start: 300},
	}, CutRangeSet(150, src))
	assert.Equal(t, []Range{
		{Start: 199, End: utils.WrapPointer[uint64](199)},
		{Start: 200, End: utils.WrapPointer[uint64](299)},
		{Start: 300},
	}, CutRangeSet(199, src))
	assert.Equal(t, []Range{
		{Start: 200, End: utils.WrapPointer[uint64](299)},
		{Start: 300},
	}, CutRangeSet(200, src))
	assert.Equal(t, []Range{
		{Start: 250, End: utils.WrapPointer[uint64](299)},
		{Start: 300},
	}, CutRangeSet(250, src))
	assert.Equal(t, []Range{
		{Start: 299, End: utils.WrapPointer[uint64](299)},
		{Start: 300},
	}, CutRangeSet(299, src))
	assert.Equal(t, []Range{{Start: 300}}, CutRangeSet(300, src))
	assert.Equal(t, []Range{{Start: 301}}, CutRangeSet(301, src))

	assert.Equal(t, []Range{
		{Start: 100, End: utils.WrapPointer[uint64](150)},
		{Start: 151, End: utils.WrapPointer[uint64](199)},
		{Start: 200, End: utils.WrapPointer[uint64](299)},
		{Start: 300, End: utils.WrapPointer[uint64](350)},
		{Start: 351, End: utils.WrapPointer[uint64](550)},
	}, CutRangeSet(0, []Range{
		{Start: 100, End: utils.WrapPointer[uint64](150)},
		{Start: 200, End: utils.WrapPointer[uint64](350)},
		{Start: 300, End: utils.WrapPointer[uint64](550)},
	}))

	src = []Range{
		{Start: 100, End: utils.WrapPointer[uint64](199)},
		{Start: 300, End: utils.WrapPointer[uint64](399)},
	}
	assert.Equal(t, []Range{
		{Start: 100, End: utils.WrapPointer[uint64](199)},
		{Start: 200, End: utils.WrapPointer[uint64](299)},
		{Start: 300, End: utils.WrapPointer[uint64](399)},
	}, CutRangeSet(0, src))
	assert.Equal(t, []Range{
		{Start: 100, End: utils.WrapPointer[uint64](199)},
		{Start: 200, End: utils.WrapPointer[uint64](299)},
		{Start: 300, End: utils.WrapPointer[uint64](399)},
	}, CutRangeSet(100, src))
	assert.Equal(t, []Range{
		{Start: 199, End: utils.WrapPointer[uint64](199)},
		{Start: 200, End: utils.WrapPointer[uint64](299)},
		{Start: 300, End: utils.WrapPointer[uint64](399)},
	}, CutRangeSet(199, src))
	assert.Equal(t, []Range{{Start: 300, End: utils.WrapPointer[uint64](399)}}, CutRangeSet(200, src))
	assert.Equal(t, []Range{{Start: 300, End: utils.WrapPointer[uint64](399)}}, CutRangeSet(299, src))
	assert.Equal(t, []Range{{Start: 300, End: utils.WrapPointer[uint64](399)}}, CutRangeSet(300, src))
	assert.Equal(t, []Range{{Start: 350, End: utils.WrapPointer[uint64](399)}}, CutRangeSet(350, src))
	assert.Equal(t, []Range{{Start: 399, End: utils.WrapPointer[uint64](399)}}, CutRangeSet(399, src))
	assert.Equal(t, []Range(nil), CutRangeSet(400, src))
	assert.Equal(t, []Range(nil), CutRangeSet(401, src))
}
