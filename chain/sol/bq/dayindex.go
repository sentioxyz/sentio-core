package bq

import (
	"sort"
	"time"
)

const dateLayout = "2006-01-02"

// DayEntry records, for one UTC day, the min/max unskipped slot seen in the Blocks table. A day with
// no blocks produces no entry; its absence within [genesis, CompleteThrough] means "empty day", not
// "not yet queried".
//
// Exported only because it is the persisted cache payload (the launcher constructs the typed
// kvstore); it is not part of the Storage contract.
type DayEntry struct {
	Date    string `json:"date"` // UTC calendar day, "2006-01-02"
	MinSlot uint64 `json:"minSlot"`
	MaxSlot uint64 `json:"maxSlot"`
}

// DaySlotIndex maps historical UTC days to their unskipped-slot ranges so that a slot range can be
// resolved to a block_timestamp window without querying BigQuery. It is complete for
// [genesis, CompleteThrough]; CompleteThrough is the latest fully-ingested UTC day — today is never
// recorded since it is still growing (matching BigQuery's UTC DAY partitioning). Days is kept sorted
// ascending (by date, equivalently by slot — both monotonic). Stored under a single cache key.
type DaySlotIndex struct {
	Days            []DayEntry `json:"days"`
	CompleteThrough string     `json:"completeThrough"` // UTC date "2006-01-02"; "" when empty
}

// maxIndexTime is the open-ended upper bound used when a slot range reaches into "today" (and
// beyond) — the part the index never records. As a block_timestamp predicate it satisfies
// requirePartitionFilter and prunes to the existing partitions from the lower bound onward.
var maxIndexTime = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

// completeThroughTime parses CompleteThrough; ok is false when the index has not been built yet.
func (ix *DaySlotIndex) completeThroughTime() (time.Time, bool) {
	if ix.CompleteThrough == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(dateLayout, ix.CompleteThrough, time.UTC)
	return t, err == nil
}

func parseDay(s string) (time.Time, bool) {
	t, err := time.ParseInLocation(dateLayout, s, time.UTC)
	return t, err == nil
}

// window returns the [lo, hi] block_timestamp window (inclusive, UTC) covering every block whose slot
// is in [from, to], using only the day index. ok is false only when the index has not been built, or
// when [from, to] lies entirely in a skipped gap between recorded days (no blocks to cover).
//
// "Today" handling: the index records complete days up to CompleteThrough; anything newer lives in
// today (or later). When `from` is past the last recorded slot (including the empty-index case: no
// unskipped slots through CompleteThrough), the window is [start of the day after CompleteThrough,
// maxIndexTime]. When only `to` reaches past the last recorded slot, the upper bound is maxIndexTime.
//
// Day boundaries are handled robustly: day(N).MaxSlot and day(N+1).MinSlot may be non-contiguous
// (skipped slots between days), and empty days are simply absent from Days. The lower bound is the
// first recorded day whose MaxSlot >= from (the day holding `from`, or the next one if `from` sits in
// a gap); the upper bound is the last day whose MinSlot <= to.
func (ix *DaySlotIndex) window(from, to uint64) (lo, hi time.Time, ok bool) {
	ct, ctOK := ix.completeThroughTime()
	if !ctOK {
		return time.Time{}, time.Time{}, false
	}
	today := ct.Add(24 * time.Hour) // start of the day after CompleteThrough

	n := len(ix.Days)
	// No recorded slots at all, or `from` is newer than every recorded day ⇒ today and beyond.
	if n == 0 || from > ix.Days[n-1].MaxSlot {
		return today, maxIndexTime, true
	}

	loIdx := sort.Search(n, func(i int) bool { return ix.Days[i].MaxSlot >= from })
	loDay, dayOK := parseDay(ix.Days[loIdx].Date)
	if !dayOK {
		return time.Time{}, time.Time{}, false
	}

	if to > ix.Days[n-1].MaxSlot {
		// `to` reaches into today; cover everything from loDay onward.
		return loDay, maxIndexTime, true
	}
	hiIdx := sort.Search(n, func(i int) bool { return ix.Days[i].MinSlot > to }) - 1
	if hiIdx < loIdx {
		return time.Time{}, time.Time{}, false // [from, to] covers no actual blocks (gap)
	}
	hiDay, dayOK := parseDay(ix.Days[hiIdx].Date)
	if !dayOK {
		return time.Time{}, time.Time{}, false
	}
	return loDay, hiDay.Add(24*time.Hour - time.Nanosecond), true
}

// previousWindow returns the [lo, hi] window (inclusive, UTC) of the day that holds the nearest block
// with slot < before, used to bound QueryPreviousUnskipped. That block lives in the last recorded day
// whose MinSlot < before. ok is false only when the index has not been built, or when no block
// precedes before.
//
// "Today" handling: with no recorded slots, the previous block (if any) is in today, so the window is
// [start of the day after CompleteThrough, maxIndexTime]. When `before` is past the last recorded
// slot, the previous block may be in today, so the upper bound is maxIndexTime.
func (ix *DaySlotIndex) previousWindow(before uint64) (lo, hi time.Time, ok bool) {
	ct, ctOK := ix.completeThroughTime()
	if !ctOK {
		return time.Time{}, time.Time{}, false
	}
	n := len(ix.Days)
	if n == 0 {
		return ct.Add(24 * time.Hour), maxIndexTime, true
	}
	j := sort.Search(n, func(i int) bool { return ix.Days[i].MinSlot >= before }) - 1
	if j < 0 {
		return time.Time{}, time.Time{}, false // before is at/below the earliest recorded slot
	}
	day, dayOK := parseDay(ix.Days[j].Date)
	if !dayOK {
		return time.Time{}, time.Time{}, false
	}
	if before > ix.Days[n-1].MaxSlot {
		// before is in today; the previous block may be the last recorded block or a today block.
		return day, maxIndexTime, true
	}
	return day, day.Add(24*time.Hour - time.Nanosecond), true
}

// mergeForward appends day entries that are strictly newer than the current ones (extension toward
// the present) and records the new completeness boundary. newDays must be sorted ascending.
func (ix *DaySlotIndex) mergeForward(newDays []DayEntry, completeThrough string) {
	ix.Days = append(ix.Days, newDays...)
	sort.Slice(ix.Days, func(i, j int) bool { return ix.Days[i].MinSlot < ix.Days[j].MinSlot })
	ix.CompleteThrough = completeThrough
}
