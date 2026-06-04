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

// window returns the [lo, hi] block_timestamp window (inclusive, UTC) covering every block whose slot
// is in [from, to], using only the day index. ok is false when the index cannot resolve the range —
// the slot is newer than the indexed history (i.e. today), or [from, to] lies entirely in a skipped
// gap between days — and the caller should fall back to a direct query.
//
// Day boundaries are handled robustly: day(N).MaxSlot and day(N+1).MinSlot may be non-contiguous
// (skipped slots between days), and empty days are simply absent from Days. The lower bound is the
// first day whose MaxSlot >= from (the day holding `from`, or the next one if `from` sits in a gap),
// and the upper bound is the last day whose MinSlot <= to.
func (ix *DaySlotIndex) window(from, to uint64) (lo, hi time.Time, ok bool) {
	n := len(ix.Days)
	if n == 0 {
		return time.Time{}, time.Time{}, false
	}
	loIdx := sort.Search(n, func(i int) bool { return ix.Days[i].MaxSlot >= from })
	if loIdx == n {
		return time.Time{}, time.Time{}, false // `from` is newer than all indexed days (today)
	}
	hiIdx := sort.Search(n, func(i int) bool { return ix.Days[i].MinSlot > to }) - 1
	if hiIdx < 0 || loIdx > hiIdx {
		return time.Time{}, time.Time{}, false // [from, to] covers no actual blocks (gap)
	}
	loDay, err1 := time.ParseInLocation(dateLayout, ix.Days[loIdx].Date, time.UTC)
	hiDay, err2 := time.ParseInLocation(dateLayout, ix.Days[hiIdx].Date, time.UTC)
	if err1 != nil || err2 != nil {
		return time.Time{}, time.Time{}, false
	}
	// Inclusive window: start of the lower day to the very end of the upper UTC day.
	return loDay, hiDay.Add(24*time.Hour - time.Nanosecond), true
}

// previousWindow returns the [lo, hi] window (inclusive, UTC) of the day that holds the nearest block
// with slot < before. That block lives in the last day whose MinSlot < before: days after it start
// at slot >= before, and within that day the largest slot < before is present (its MinSlot < before).
// ok is false when no indexed day precedes before.
func (ix *DaySlotIndex) previousWindow(before uint64) (lo, hi time.Time, ok bool) {
	n := len(ix.Days)
	if n == 0 {
		return time.Time{}, time.Time{}, false
	}
	j := sort.Search(n, func(i int) bool { return ix.Days[i].MinSlot >= before }) - 1
	if j < 0 {
		return time.Time{}, time.Time{}, false
	}
	d, err := time.ParseInLocation(dateLayout, ix.Days[j].Date, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return d, d.Add(24*time.Hour - time.Nanosecond), true
}

// mergeForward appends day entries that are strictly newer than the current ones (extension toward
// the present) and records the new completeness boundary. newDays must be sorted ascending.
func (ix *DaySlotIndex) mergeForward(newDays []DayEntry, completeThrough string) {
	ix.Days = append(ix.Days, newDays...)
	sort.Slice(ix.Days, func(i, j int) bool { return ix.Days[i].MinSlot < ix.Days[j].MinSlot })
	ix.CompleteThrough = completeThrough
}
