package bq

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func day(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		panic(err)
	}
	return t
}

// index with a non-contiguous gap (199->210) between days and ascending slot ranges.
func sampleIndex() *DaySlotIndex {
	return &DaySlotIndex{
		Days: []DayEntry{
			{Date: day("2026-05-26"), MinSlot: 100, MaxSlot: 199},
			{Date: day("2026-05-27"), MinSlot: 210, MaxSlot: 299}, // gap 200..209 vs previous max
			{Date: day("2026-05-28"), MinSlot: 300, MaxSlot: 399},
		},
		CompleteThrough: day("2026-05-28"),
	}
}

func TestDaySlotIndexMaxValidSlot(t *testing.T) {
	maxSlot, ok := sampleIndex().maxValidSlot()
	require.True(t, ok)
	assert.Equal(t, uint64(399), maxSlot) // MaxSlot of the latest complete day

	_, ok = (&DaySlotIndex{}).maxValidSlot()
	assert.False(t, ok) // no complete day → not serveable
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

	// Full span: lower day start to the latest complete day's end.
	lo, hi, ok = ix.window(100, 399)
	require.True(t, ok)
	assert.Equal(t, day("2026-05-26"), lo)
	assert.Equal(t, day("2026-05-28").Add(24*time.Hour-time.Nanosecond), hi)

	// `from` above every recorded day (the store rejects this via maxValidSlot before calling window;
	// window defensively returns not-ok).
	_, _, ok = ix.window(500, 500)
	assert.False(t, ok)

	// Empty Days → nothing to resolve.
	_, _, ok = (&DaySlotIndex{CompleteThrough: day("2026-05-28")}).window(100, 100)
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

	// before just past the last complete slot (maxValid+1=400) → predecessor is in the last day.
	lo, hi, ok := ix.previousWindow(400)
	require.True(t, ok)
	assert.Equal(t, day("2026-05-28"), lo)
	assert.Equal(t, day("2026-05-28").Add(24*time.Hour-time.Nanosecond), hi)

	// before at/below the earliest slot → no previous block.
	_, _, ok = ix.previousWindow(100)
	assert.False(t, ok)
	_, _, ok = ix.previousWindow(50)
	assert.False(t, ok)

	// Empty Days → no previous block.
	_, _, ok = (&DaySlotIndex{CompleteThrough: day("2026-05-28")}).previousWindow(100)
	assert.False(t, ok)
}

func TestDaySlotIndexMergeForward(t *testing.T) {
	ix := &DaySlotIndex{
		Days:            []DayEntry{{Date: day("2026-05-26"), MinSlot: 100, MaxSlot: 199}},
		CompleteThrough: day("2026-05-26"),
	}
	ix.mergeForward([]DayEntry{
		{Date: day("2026-05-27"), MinSlot: 210, MaxSlot: 299},
		{Date: day("2026-05-28"), MinSlot: 300, MaxSlot: 399},
	}, day("2026-05-28"))

	require.Len(t, ix.Days, 3)
	assert.Equal(t, day("2026-05-28"), ix.CompleteThrough)
	// stays sorted ascending by date
	for i := 1; i < len(ix.Days); i++ {
		assert.True(t, ix.Days[i-1].Date.Before(ix.Days[i].Date))
	}
}

// A re-queried day replaces the existing entry rather than duplicating it.
func TestDaySlotIndexMergeForwardDedup(t *testing.T) {
	ix := &DaySlotIndex{
		Days: []DayEntry{
			{Date: day("2026-05-26"), MinSlot: 100, MaxSlot: 199},
			{Date: day("2026-05-27"), MinSlot: 210, MaxSlot: 290}, // stale max
		},
		CompleteThrough: day("2026-05-27"),
	}
	ix.mergeForward([]DayEntry{
		{Date: day("2026-05-27"), MinSlot: 210, MaxSlot: 299}, // refreshed, larger max
		{Date: day("2026-05-28"), MinSlot: 300, MaxSlot: 399},
	}, day("2026-05-28"))

	require.Len(t, ix.Days, 3) // 05-27 replaced, not duplicated
	assert.Equal(t, uint64(299), ix.Days[1].MaxSlot)
	maxSlot, ok := ix.maxValidSlot()
	require.True(t, ok)
	assert.Equal(t, uint64(399), maxSlot)
}

func TestDaySlotIndexSnapshot(t *testing.T) {
	// Small index: all days shown.
	snap := sampleIndex().snapshot()
	assert.Equal(t, 3, snap["dayCount"])
	assert.Equal(t, uint64(399), snap["maxValidSlot"])
	assert.Equal(t, day("2026-05-28"), snap["completeThrough"])
	require.Len(t, snap["days"], 3)
	_, hasOldest := snap["oldestDays"]
	assert.False(t, hasOldest)

	// Large index: capped at oldest 100 + newest 100, with the elided count.
	big := &DaySlotIndex{CompleteThrough: day("2026-05-28")}
	for i := 0; i < 250; i++ {
		s := uint64(i*100 + 1)
		big.Days = append(big.Days, DayEntry{Date: day("2026-05-26").AddDate(0, 0, i), MinSlot: s, MaxSlot: s + 99})
	}
	snap = big.snapshot()
	assert.Equal(t, 250, snap["dayCount"])
	assert.Equal(t, 50, snap["elidedDays"])
	require.Len(t, snap["oldestDays"], 100)
	require.Len(t, snap["newestDays"], 100)
	_, hasDays := snap["days"]
	assert.False(t, hasDays)
	// oldest starts at the first day, newest ends at the last.
	assert.Equal(t, uint64(1), snap["oldestDays"].([]DayEntry)[0].MinSlot)
	assert.Equal(t, uint64(249*100+1), snap["newestDays"].([]DayEntry)[99].MinSlot)
}

func TestDaySlotIndexClone(t *testing.T) {
	ix := sampleIndex()
	c := ix.clone()
	c.Days[0].MaxSlot = 12345
	c.CompleteThrough = day("2026-06-01")
	// Mutating the clone must not affect the original.
	assert.Equal(t, uint64(199), ix.Days[0].MaxSlot)
	assert.Equal(t, day("2026-05-28"), ix.CompleteThrough)
}
