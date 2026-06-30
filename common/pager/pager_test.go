package pager

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func cfg() Config {
	return Config{Target: 50000, Min: 100, Max: 5000, Step: 100, Initial: 500}
}

func Test_NextSize(t *testing.T) {
	c := cfg()
	cases := []struct {
		name          string
		span, records uint64
		want          uint64
	}{
		{"on target", 500, 50000, 500},
		{"too dense -> shrink", 500, 100000, 300},
		{"way too dense -> clamp min", 100, 10000000, 100},
		{"too sparse -> grow & clamp max", 500, 2500, 5000},
		{"empty page -> max", 500, 0, 5000},
		{"rounds to nearest hundred", 500, 66667, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := c.NextSize(tc.span, tc.records)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, uint64(0), got%c.Step, "must stay on the step grid")
			assert.GreaterOrEqual(t, got, c.Min)
			assert.LessOrEqual(t, got, c.Max)
		})
	}
}

func Test_NextSize_targetZeroGoesMax(t *testing.T) {
	c := Config{Target: 0, Min: 100, Max: 5000, Step: 100}
	assert.Equal(t, uint64(5000), c.NextSize(500, 12345))
}

func Test_Walk_coversRangeExactlyOnceContiguous(t *testing.T) {
	// density: 100 records per unit -> target 50000 wants ~500-unit pages.
	const density = 100
	var visited []uint64 // every unit, in order
	err := Walk(0, 4999, cfg(), func(start, end uint64) (uint64, bool, error) {
		assert.LessOrEqual(t, start, end)
		for u := start; u <= end; u++ {
			visited = append(visited, u)
		}
		return (end - start + 1) * density, false, nil
	})
	assert.NoError(t, err)
	assert.Len(t, visited, 5000)
	for i, u := range visited {
		assert.Equal(t, uint64(i), u, "units must be contiguous and in order, no gaps/overlaps")
	}
}

func Test_Walk_pagesStayWithinBoundsAndOnGrid(t *testing.T) {
	c := cfg()
	var sizes []uint64
	// alternating dense/sparse to exercise both shrink and grow paths.
	toggle := true
	err := Walk(0, 99999, c, func(start, end uint64) (uint64, bool, error) {
		size := end - start + 1
		sizes = append(sizes, size)
		assert.GreaterOrEqual(t, size, uint64(1))
		assert.LessOrEqual(t, size, c.Max)
		toggle = !toggle
		if toggle {
			return size * 1000, false, nil // very dense -> next page shrinks toward min
		}
		return 0, false, nil // empty -> next page jumps to max
	})
	assert.NoError(t, err)
	// after an empty page the next page must be Max; after a very dense page it must shrink to Min.
	assert.Contains(t, sizes, c.Max)
	assert.Contains(t, sizes, c.Min)
}

func Test_Walk_emptyRange(t *testing.T) {
	calls := 0
	err := Walk(10, 9, cfg(), func(start, end uint64) (uint64, bool, error) {
		calls++
		return 0, false, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, calls)
}

func Test_Walk_singleUnitRange(t *testing.T) {
	var spans [][2]uint64
	err := Walk(7, 7, cfg(), func(start, end uint64) (uint64, bool, error) {
		spans = append(spans, [2]uint64{start, end})
		return 1, false, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, [][2]uint64{{7, 7}}, spans)
}

func Test_Walk_stopsOnError(t *testing.T) {
	boom := errors.New("boom")
	calls := 0
	err := Walk(0, 100000, cfg(), func(start, end uint64) (uint64, bool, error) {
		calls++
		if calls == 2 {
			return 0, false, boom
		}
		return 50000, false, nil
	})
	assert.ErrorIs(t, err, boom)
	assert.Equal(t, 2, calls)
}

func Test_Walk_tooBigShrinksAndRetriesThenCoversRange(t *testing.T) {
	// Any page wider than maxOK units reports tooBig; otherwise it succeeds. Walk must shrink past
	// the grid/Min as needed, never advance on a tooBig page, and still cover the range exactly once.
	const maxOK = 30 // narrower than Min (100) to force shrinking below the grid floor
	var visited []uint64
	tooBigSpans := 0
	err := Walk(0, 199, cfg(), func(start, end uint64) (uint64, bool, error) {
		span := end - start + 1
		if span > maxOK {
			tooBigSpans++
			return 0, true, nil // not done; do not record these units
		}
		for u := start; u <= end; u++ {
			visited = append(visited, u)
		}
		return span * 100, false, nil
	})
	assert.NoError(t, err)
	assert.Greater(t, tooBigSpans, 0, "the oversized initial/grown pages must have triggered shrinking")
	assert.Len(t, visited, 200)
	for i, u := range visited {
		assert.Equal(t, uint64(i), u, "units must be contiguous and in order after shrink-retries")
	}
}

func Test_Walk_tooBigOnSingleUnitErrors(t *testing.T) {
	// A process that reports tooBig even once the span is a single unit cannot make progress;
	// Walk must surface an error instead of looping forever.
	err := Walk(0, 9, cfg(), func(start, end uint64) (uint64, bool, error) {
		return 0, true, nil
	})
	assert.Error(t, err)
}

func Test_normalize_defaults(t *testing.T) {
	c := Config{}.normalize()
	assert.Equal(t, uint64(1), c.Step)
	assert.Equal(t, c.Step, c.Min)
	assert.GreaterOrEqual(t, c.Max, c.Min)
	assert.GreaterOrEqual(t, c.Initial, c.Min)
}
