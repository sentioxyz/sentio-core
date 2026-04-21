package rg

import (
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/utils"
	"testing"
)

func Test_setContains(t *testing.T) {
	set := RangeSet{ // [1][4-6][10]
		Range: newRange(1, 10),
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
		Range: newRange(1, 10),
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
			assert.Equalf(t, ok, set.Include(newRange(s, e)), "invalid testcase: [%s,%s], ok: %v", s, e, ok)
		}
	}
}

func Test_setLast(t *testing.T) {
	assert.Equal(t, EmptyRange, EmptyRangeSet.Last())

	set := RangeSet{
		Range: newRange(1, 10),
	}
	assert.Equal(t, newRange(1, 10), set.Last())

	set.Holes = [][2]uint64{
		{2, 3},
		{7, 9},
	}
	assert.Equal(t, newRange(10, 10), set.Last())
}

func Test_setIntersection(t *testing.T) {
	assert.Equal(t, EmptyRangeSet, EmptyRangeSet.Intersection(EmptyRange))
	assert.Equal(t, EmptyRangeSet, EmptyRangeSet.Intersection(Range{Start: 1}))

	// [1][4-6][10]
	set := RangeSet{
		Range: newRange(1, 10),
		Holes: [][2]uint64{{2, 3}, {7, 9}},
	}
	assert.Equal(t, EmptyRangeSet, set.Intersection(EmptyRange))
	assert.Equal(t, set, set.Intersection(Range{Start: 0}))

	assert.Equal(t, RangeSet{Range: newRange(1, 1)}, set.Intersection(newRange(1, 1)))
	assert.Equal(t, RangeSet{Range: newRange(1, 1)}, set.Intersection(newRange(1, 2)))
	assert.Equal(t, RangeSet{Range: newRange(1, 1)}, set.Intersection(newRange(1, 3)))
	assert.Equal(t, RangeSet{Range: newRange(1, 4), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newRange(1, 4)))
	assert.Equal(t, RangeSet{Range: newRange(1, 5), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newRange(1, 5)))
	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newRange(1, 6)))
	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newRange(1, 7)))
	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newRange(1, 8)))
	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Intersection(newRange(1, 9)))

	assert.Equal(t, set, set.Intersection(newRange(1, 10)))
	assert.Equal(t, set, set.Intersection(newRange(1, 11)))
	assert.Equal(t, set, set.Intersection(newRange(0, 10)))
	assert.Equal(t, set, set.Intersection(newRange(0, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(newRange(2, 2)))
	assert.Equal(t, EmptyRangeSet, set.Intersection(newRange(2, 3)))
	assert.Equal(t, RangeSet{Range: newRange(4, 4)}, set.Intersection(newRange(2, 4)))
	assert.Equal(t, RangeSet{Range: newRange(4, 5)}, set.Intersection(newRange(2, 5)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(2, 6)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(2, 7)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(2, 8)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(2, 9)))
	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newRange(2, 10)))
	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newRange(2, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(newRange(3, 3)))
	assert.Equal(t, RangeSet{Range: newRange(4, 4)}, set.Intersection(newRange(3, 4)))
	assert.Equal(t, RangeSet{Range: newRange(4, 5)}, set.Intersection(newRange(3, 5)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(3, 6)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(3, 7)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(3, 8)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(3, 9)))
	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newRange(3, 10)))
	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newRange(3, 11)))

	assert.Equal(t, RangeSet{Range: newRange(4, 4)}, set.Intersection(newRange(4, 4)))
	assert.Equal(t, RangeSet{Range: newRange(4, 5)}, set.Intersection(newRange(4, 5)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(4, 6)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(4, 7)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(4, 8)))
	assert.Equal(t, RangeSet{Range: newRange(4, 6)}, set.Intersection(newRange(4, 9)))
	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newRange(4, 10)))
	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newRange(4, 11)))

	assert.Equal(t, RangeSet{Range: newRange(5, 5)}, set.Intersection(newRange(5, 5)))
	assert.Equal(t, RangeSet{Range: newRange(5, 6)}, set.Intersection(newRange(5, 6)))
	assert.Equal(t, RangeSet{Range: newRange(5, 6)}, set.Intersection(newRange(5, 7)))
	assert.Equal(t, RangeSet{Range: newRange(5, 6)}, set.Intersection(newRange(5, 8)))
	assert.Equal(t, RangeSet{Range: newRange(5, 6)}, set.Intersection(newRange(5, 9)))
	assert.Equal(t, RangeSet{Range: newRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newRange(5, 10)))
	assert.Equal(t, RangeSet{Range: newRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newRange(5, 11)))

	assert.Equal(t, RangeSet{Range: newRange(6, 6)}, set.Intersection(newRange(6, 6)))
	assert.Equal(t, RangeSet{Range: newRange(6, 6)}, set.Intersection(newRange(6, 7)))
	assert.Equal(t, RangeSet{Range: newRange(6, 6)}, set.Intersection(newRange(6, 8)))
	assert.Equal(t, RangeSet{Range: newRange(6, 6)}, set.Intersection(newRange(6, 9)))
	assert.Equal(t, RangeSet{Range: newRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newRange(6, 10)))
	assert.Equal(t, RangeSet{Range: newRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Intersection(newRange(6, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(newRange(7, 7)))
	assert.Equal(t, EmptyRangeSet, set.Intersection(newRange(7, 8)))
	assert.Equal(t, EmptyRangeSet, set.Intersection(newRange(7, 9)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Intersection(newRange(7, 10)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Intersection(newRange(7, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(newRange(8, 8)))
	assert.Equal(t, EmptyRangeSet, set.Intersection(newRange(8, 9)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Intersection(newRange(8, 10)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Intersection(newRange(8, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(newRange(9, 9)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Intersection(newRange(9, 10)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Intersection(newRange(9, 11)))

	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Intersection(newRange(10, 10)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Intersection(newRange(10, 11)))

	assert.Equal(t, EmptyRangeSet, set.Intersection(newRange(11, 11)))
	assert.Equal(t, EmptyRangeSet, set.Intersection(newRange(0, 0)))
}

func Test_setRemove(t *testing.T) {
	assert.Equal(t, EmptyRangeSet, EmptyRangeSet.Remove(EmptyRange))
	assert.Equal(t, EmptyRangeSet, EmptyRangeSet.Remove(Range{Start: 1}))

	set := RangeSet{ // [1][4-6][10]
		Range: newRange(1, 10),
		Holes: [][2]uint64{
			{2, 3},
			{7, 9},
		},
	}
	assert.Equal(t, set, set.Remove(EmptyRange))
	assert.Equal(t, EmptyRangeSet, set.Remove(Range{Start: 0}))

	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newRange(0, 0)))
	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newRange(0, 1)))
	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newRange(0, 2)))
	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newRange(0, 3)))
	assert.Equal(t, RangeSet{Range: newRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newRange(0, 4)))
	assert.Equal(t, RangeSet{Range: newRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newRange(0, 5)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Remove(newRange(0, 6)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Remove(newRange(0, 7)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Remove(newRange(0, 8)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Remove(newRange(0, 9)))
	assert.Equal(t, EmptyRangeSet, set.Remove(newRange(0, 10)))
	assert.Equal(t, EmptyRangeSet, set.Remove(newRange(0, 11)))

	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newRange(1, 1)))
	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newRange(1, 2)))
	assert.Equal(t, RangeSet{Range: newRange(4, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newRange(1, 3)))
	assert.Equal(t, RangeSet{Range: newRange(5, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newRange(1, 4)))
	assert.Equal(t, RangeSet{Range: newRange(6, 10), Holes: [][2]uint64{{7, 9}}}, set.Remove(newRange(1, 5)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Remove(newRange(1, 6)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Remove(newRange(1, 7)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Remove(newRange(1, 8)))
	assert.Equal(t, RangeSet{Range: newRange(10, 10)}, set.Remove(newRange(1, 9)))
	assert.Equal(t, EmptyRangeSet, set.Remove(newRange(1, 10)))
	assert.Equal(t, EmptyRangeSet, set.Remove(newRange(1, 11)))

	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newRange(2, 2)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newRange(2, 3)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 4}, {7, 9}}}, set.Remove(newRange(2, 4)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 5}, {7, 9}}}, set.Remove(newRange(2, 5)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(2, 6)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(2, 7)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(2, 8)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(2, 9)))
	assert.Equal(t, RangeSet{Range: newRange(1, 1)}, set.Remove(newRange(2, 10)))
	assert.Equal(t, RangeSet{Range: newRange(1, 1)}, set.Remove(newRange(2, 11)))

	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newRange(3, 3)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 4}, {7, 9}}}, set.Remove(newRange(3, 4)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 5}, {7, 9}}}, set.Remove(newRange(3, 5)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(3, 6)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(3, 7)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(3, 8)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(3, 9)))
	assert.Equal(t, RangeSet{Range: newRange(1, 1)}, set.Remove(newRange(3, 10)))
	assert.Equal(t, RangeSet{Range: newRange(1, 1)}, set.Remove(newRange(3, 11)))

	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 4}, {7, 9}}}, set.Remove(newRange(4, 4)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 5}, {7, 9}}}, set.Remove(newRange(4, 5)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(4, 6)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(4, 7)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(4, 8)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 9}}}, set.Remove(newRange(4, 9)))
	assert.Equal(t, RangeSet{Range: newRange(1, 1)}, set.Remove(newRange(4, 10)))
	assert.Equal(t, RangeSet{Range: newRange(1, 1)}, set.Remove(newRange(4, 11)))

	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 5}, {7, 9}}}, set.Remove(newRange(5, 5)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(newRange(5, 6)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(newRange(5, 7)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(newRange(5, 8)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {5, 9}}}, set.Remove(newRange(5, 9)))
	assert.Equal(t, RangeSet{Range: newRange(1, 4), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(5, 10)))
	assert.Equal(t, RangeSet{Range: newRange(1, 4), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(5, 11)))

	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(newRange(6, 6)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(newRange(6, 7)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(newRange(6, 8)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {6, 9}}}, set.Remove(newRange(6, 9)))
	assert.Equal(t, RangeSet{Range: newRange(1, 5), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(6, 10)))
	assert.Equal(t, RangeSet{Range: newRange(1, 5), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(6, 11)))

	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newRange(7, 7)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newRange(7, 8)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newRange(7, 9)))
	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(7, 10)))
	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(7, 11)))

	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newRange(8, 8)))
	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newRange(8, 9)))
	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(8, 10)))
	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(8, 11)))

	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newRange(9, 9)))
	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(9, 10)))
	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(9, 11)))

	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(10, 10)))
	assert.Equal(t, RangeSet{Range: newRange(1, 6), Holes: [][2]uint64{{2, 3}}}, set.Remove(newRange(10, 11)))

	assert.Equal(t, RangeSet{Range: newRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}, set.Remove(newRange(11, 11)))
}

func Test_setUnion(t *testing.T) {
	assert.Equal(t, EmptyRangeSet, EmptyRangeSet.Union(EmptyRange))
	assert.Equal(t, RangeSet{Range: Range{Start: 1}}, RangeSet{Range: Range{Start: 1}}.Union(EmptyRange))
	assert.Equal(t, RangeSet{Range: Range{Start: 1}}, EmptyRangeSet.Union(Range{Start: 1}))

	set := RangeSet{ // [2][5-7][11]
		Range: newRange(2, 11),
		Holes: [][2]uint64{
			{3, 4},
			{8, 10},
		},
	}
	assert.Equal(t, set, set.Union(EmptyRange))

	assert.Equal(t, RangeSet{Range: newRange(0, 11), Holes: [][2]uint64{{1, 1}, {3, 4}, {8, 10}}}, set.Union(newRange(0, 0)))
	assert.Equal(t, RangeSet{Range: newRange(0, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(0, 1)))
	assert.Equal(t, RangeSet{Range: newRange(0, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(0, 2)))
	assert.Equal(t, RangeSet{Range: newRange(0, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(newRange(0, 3)))
	assert.Equal(t, RangeSet{Range: newRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(0, 4)))
	assert.Equal(t, RangeSet{Range: newRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(0, 5)))
	assert.Equal(t, RangeSet{Range: newRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(0, 6)))
	assert.Equal(t, RangeSet{Range: newRange(0, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(0, 7)))
	assert.Equal(t, RangeSet{Range: newRange(0, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(newRange(0, 8)))
	assert.Equal(t, RangeSet{Range: newRange(0, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(newRange(0, 9)))
	assert.Equal(t, RangeSet{Range: newRange(0, 11)}, set.Union(newRange(0, 10)))
	assert.Equal(t, RangeSet{Range: newRange(0, 11)}, set.Union(newRange(0, 11)))
	assert.Equal(t, RangeSet{Range: newRange(0, 12)}, set.Union(newRange(0, 12)))
	assert.Equal(t, RangeSet{Range: Range{Start: 0}}, set.Union(Range{Start: 0}))

	assert.Equal(t, RangeSet{Range: newRange(1, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(1, 1)))
	assert.Equal(t, RangeSet{Range: newRange(1, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(1, 2)))
	assert.Equal(t, RangeSet{Range: newRange(1, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(newRange(1, 3)))
	assert.Equal(t, RangeSet{Range: newRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(1, 4)))
	assert.Equal(t, RangeSet{Range: newRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(1, 5)))
	assert.Equal(t, RangeSet{Range: newRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(1, 6)))
	assert.Equal(t, RangeSet{Range: newRange(1, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(1, 7)))
	assert.Equal(t, RangeSet{Range: newRange(1, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(newRange(1, 8)))
	assert.Equal(t, RangeSet{Range: newRange(1, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(newRange(1, 9)))
	assert.Equal(t, RangeSet{Range: newRange(1, 11)}, set.Union(newRange(1, 10)))
	assert.Equal(t, RangeSet{Range: newRange(1, 11)}, set.Union(newRange(1, 11)))
	assert.Equal(t, RangeSet{Range: newRange(1, 12)}, set.Union(newRange(1, 12)))

	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(2, 2)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(newRange(2, 3)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(2, 4)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(2, 5)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(2, 6)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(2, 7)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(newRange(2, 8)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(newRange(2, 9)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11)}, set.Union(newRange(2, 10)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11)}, set.Union(newRange(2, 11)))
	assert.Equal(t, RangeSet{Range: newRange(2, 12)}, set.Union(newRange(2, 12)))

	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{4, 4}, {8, 10}}}, set.Union(newRange(3, 3)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(3, 4)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(3, 5)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(3, 6)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{8, 10}}}, set.Union(newRange(3, 7)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{9, 10}}}, set.Union(newRange(3, 8)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{10, 10}}}, set.Union(newRange(3, 9)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11)}, set.Union(newRange(3, 10)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11)}, set.Union(newRange(3, 11)))
	assert.Equal(t, RangeSet{Range: newRange(2, 12)}, set.Union(newRange(3, 12)))

	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(newRange(4, 4)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(newRange(4, 5)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(newRange(4, 6)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 3}, {8, 10}}}, set.Union(newRange(4, 7)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 3}, {9, 10}}}, set.Union(newRange(4, 8)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 3}, {10, 10}}}, set.Union(newRange(4, 9)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 3}}}, set.Union(newRange(4, 10)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 3}}}, set.Union(newRange(4, 11)))
	assert.Equal(t, RangeSet{Range: newRange(2, 12), Holes: [][2]uint64{{3, 3}}}, set.Union(newRange(4, 12)))

	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(5, 5)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(5, 6)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(5, 7)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(newRange(5, 8)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(newRange(5, 9)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(5, 10)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(5, 11)))
	assert.Equal(t, RangeSet{Range: newRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(5, 12)))

	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(6, 6)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(6, 7)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(newRange(6, 8)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(newRange(6, 9)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(6, 10)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(6, 11)))
	assert.Equal(t, RangeSet{Range: newRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(6, 12)))

	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(7, 7)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(newRange(7, 8)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(newRange(7, 9)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(7, 10)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(7, 11)))
	assert.Equal(t, RangeSet{Range: newRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(7, 12)))

	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {9, 10}}}, set.Union(newRange(8, 8)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {10, 10}}}, set.Union(newRange(8, 9)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(8, 10)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(8, 11)))
	assert.Equal(t, RangeSet{Range: newRange(2, 12), Holes: [][2]uint64{{3, 4}}}, set.Union(newRange(8, 12)))

	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 8}, {10, 10}}}, set.Union(newRange(9, 9)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 8}}}, set.Union(newRange(9, 10)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 8}}}, set.Union(newRange(9, 11)))
	assert.Equal(t, RangeSet{Range: newRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 8}}}, set.Union(newRange(9, 12)))

	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 9}}}, set.Union(newRange(10, 10)))
	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 9}}}, set.Union(newRange(10, 11)))
	assert.Equal(t, RangeSet{Range: newRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 9}}}, set.Union(newRange(10, 12)))

	assert.Equal(t, RangeSet{Range: newRange(2, 11), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(11, 11)))
	assert.Equal(t, RangeSet{Range: newRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(11, 12)))

	assert.Equal(t, RangeSet{Range: newRange(2, 12), Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(newRange(12, 12)))
	assert.Equal(t, RangeSet{Range: Range{Start: 2}, Holes: [][2]uint64{{3, 4}, {8, 10}}}, set.Union(Range{Start: 12}))

	assert.Equal(t, RangeSet{Range: newRange(2, 13), Holes: [][2]uint64{{3, 4}, {8, 10}, {12, 12}}}, set.Union(newRange(13, 13)))
	assert.Equal(t, RangeSet{Range: Range{Start: 2}, Holes: [][2]uint64{{3, 4}, {8, 10}, {12, 12}}}, set.Union(Range{Start: 13}))
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
