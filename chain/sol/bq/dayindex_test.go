package bq

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func day(s string) time.Time {
	t, err := time.ParseInLocation(dateLayout, s, time.UTC)
	if err != nil {
		panic(err)
	}
	return t
}

// index with a non-contiguous gap (199->210) between days and ascending slot ranges.
func sampleIndex() *DaySlotIndex {
	return &DaySlotIndex{
		Days: []DayEntry{
			{Date: "2026-05-26", MinSlot: 100, MaxSlot: 199},
			{Date: "2026-05-27", MinSlot: 210, MaxSlot: 299}, // gap 200..209 vs previous max
			{Date: "2026-05-28", MinSlot: 300, MaxSlot: 399},
		},
		CompleteThrough: "2026-05-28",
	}
}

func TestDaySlotIndexWindow(t *testing.T) {
	ix := sampleIndex()

	// Single slot inside a day.
	lo, hi, ok := ix.window(150, 150)
	require.True(t, ok)
	assert.Equal(t, day("2026-05-26"), lo)
	assert.Equal(t, day("2026-05-26").Add(24*time.Hour-time.Nanosecond), hi)

	// Range spanning the gap across two days: lower day start to upper day end.
	lo, hi, ok = ix.window(150, 250)
	require.True(t, ok)
	assert.Equal(t, day("2026-05-26"), lo)
	assert.Equal(t, day("2026-05-27").Add(24*time.Hour-time.Nanosecond), hi)

	// from in the skipped gap (199 < 205 < 210): lower bound is the next day holding blocks.
	lo, _, ok = ix.window(205, 250)
	require.True(t, ok)
	assert.Equal(t, day("2026-05-27"), lo)

	// Whole [from,to] inside a gap → no blocks → not resolvable (caller falls back).
	_, _, ok = ix.window(200, 209)
	assert.False(t, ok)

	// Slot newer than the indexed history (today) → not resolvable.
	_, _, ok = ix.window(500, 500)
	assert.False(t, ok)

	// Full span.
	lo, hi, ok = ix.window(100, 399)
	require.True(t, ok)
	assert.Equal(t, day("2026-05-26"), lo)
	assert.Equal(t, day("2026-05-28").Add(24*time.Hour-time.Nanosecond), hi)

	// Empty index.
	_, _, ok = (&DaySlotIndex{}).window(100, 100)
	assert.False(t, ok)
}

func TestDaySlotIndexPreviousWindow(t *testing.T) {
	ix := sampleIndex() // days: [100,199], [210,299], [300,399]

	// before inside day 1 → previous block is day 0.
	lo, _, ok := ix.previousWindow(150)
	require.True(t, ok)
	assert.Equal(t, day("2026-05-26"), lo)

	// before in the gap (205) → previous block is day 0's max (199), in day 0.
	lo, _, ok = ix.previousWindow(205)
	require.True(t, ok)
	assert.Equal(t, day("2026-05-26"), lo)

	// before inside day 2 → previous block is day 1.
	lo, _, ok = ix.previousWindow(250)
	require.True(t, ok)
	assert.Equal(t, day("2026-05-27"), lo)

	// before newer than everything → previous block is the last day.
	lo, _, ok = ix.previousWindow(1000)
	require.True(t, ok)
	assert.Equal(t, day("2026-05-28"), lo)

	// before at/below the earliest slot → no previous block.
	_, _, ok = ix.previousWindow(100)
	assert.False(t, ok)
	_, _, ok = ix.previousWindow(50)
	assert.False(t, ok)
}

func TestDaySlotIndexMergeForward(t *testing.T) {
	ix := &DaySlotIndex{
		Days:            []DayEntry{{Date: "2026-05-26", MinSlot: 100, MaxSlot: 199}},
		CompleteThrough: "2026-05-26",
	}
	ix.mergeForward([]DayEntry{
		{Date: "2026-05-27", MinSlot: 210, MaxSlot: 299},
		{Date: "2026-05-28", MinSlot: 300, MaxSlot: 399},
	}, "2026-05-28")

	require.Len(t, ix.Days, 3)
	assert.Equal(t, "2026-05-28", ix.CompleteThrough)
	// stays sorted ascending by slot
	for i := 1; i < len(ix.Days); i++ {
		assert.Less(t, ix.Days[i-1].MinSlot, ix.Days[i].MinSlot)
	}
}
