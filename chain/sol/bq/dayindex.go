package bq

import (
	"sort"
	"time"
)

// DayEntry records, for one UTC day, the min/max unskipped slot seen in the Blocks table. A day with
// no blocks produces no entry; its absence within [genesis, CompleteThrough] means "empty day", not
// "not yet queried".
//
// Exported only because it is the persisted cache payload (the launcher constructs the typed
// kvstore); it is not part of the Storage contract.
type DayEntry struct {
	Date    time.Time `json:"date"` // UTC midnight of the day
	MinSlot uint64    `json:"minSlot"`
	MaxSlot uint64    `json:"maxSlot"`
}

// DaySlotIndex maps historical UTC days to their unskipped-slot ranges so that a slot range can be
// resolved to a block_timestamp window without querying BigQuery. Days holds only days whose data is
// COMPLETE (fully ingested) — kept sorted ascending by date. The most recent still-ingesting day (and
// today) are deliberately excluded: BigQuery is not real-time, so the latest day with rows may still
// be growing, and is only admitted once a strictly-later day has appeared.
//
// CompleteThrough is the UTC midnight of the latest complete day (the zero value means the index has
// not been built yet). The MaxSlot of that latest complete day is the authoritative upper bound of
// the BigQuery data (see maxValidSlot); it plays the same role as the ClickHouse range store — a slot
// query above it cannot be answered from complete data and must error rather than be served from a
// partially-ingested tail. Stored under a single cache key.
//
// Invariant (relied on by window/previousWindow, which binary-search Days by slot even though it is
// sorted by date): slot ranges increase monotonically with date — for adjacent days,
// Days[i].MaxSlot < Days[i+1].MinSlot. This holds for Solana (slots increase with time) and for the
// build query (GROUP BY day ORDER BY day over Blocks).
type DaySlotIndex struct {
	Days            []DayEntry `json:"days"`
	CompleteThrough time.Time  `json:"completeThrough"`
}

// endOfDay returns the last instant of the UTC day that starts at d.
func endOfDay(d time.Time) time.Time { return d.Add(24*time.Hour - time.Nanosecond) }

// maxValidSlot returns the inclusive upper slot bound of the complete (authoritative) BigQuery data:
// the MaxSlot of the latest complete day. ok is false when no complete day is recorded yet (the index
// is empty), in which case the store cannot serve any slot.
func (ix *DaySlotIndex) maxValidSlot() (uint64, bool) {
	n := len(ix.Days)
	if n == 0 {
		return 0, false
	}
	return ix.Days[n-1].MaxSlot, true
}

// retentionFloor returns the lower slot bound (and its UTC day) below which BigQuery queries are
// refused: the MinSlot of the earliest complete day still within `days` days of the latest complete
// day. This caps how far back (and thus how much data / cost) the BigQuery tier will serve. ok is
// false when no complete day is recorded yet. When the index spans fewer than `days` days, the floor
// is simply the earliest recorded day.
func (ix *DaySlotIndex) retentionFloor(days int) (minSlot uint64, date time.Time, ok bool) {
	n := len(ix.Days)
	if n == 0 {
		return 0, time.Time{}, false
	}
	cutoff := ix.Days[n-1].Date.AddDate(0, 0, -days)
	// first recorded day whose Date >= cutoff (the last day's Date >= cutoff, so i < n always).
	i := sort.Search(n, func(i int) bool { return !ix.Days[i].Date.Before(cutoff) })
	if i == n {
		i = n - 1
	}
	return ix.Days[i].MinSlot, ix.Days[i].Date, true
}

// clone returns a deep copy (the Days slice is copied), so a merge can be staged and only swapped in
// after it has been persisted — see ensureDayIndexLocked.
func (ix *DaySlotIndex) clone() *DaySlotIndex {
	days := make([]DayEntry, len(ix.Days))
	copy(days, ix.Days)
	return &DaySlotIndex{Days: days, CompleteThrough: ix.CompleteThrough}
}

// window returns the [lo, hi] block_timestamp window (inclusive, UTC) covering every block whose slot
// is in [from, to], using only the day index. ok is false when [from, to] lies entirely in a skipped
// gap between recorded days (no blocks to cover).
//
// Precondition (enforced by the caller, resolveTimeRange): the index is built (n > 0) and
// from <= to <= maxValidSlot, so both bounds fall within the recorded complete days. No "today" /
// open-ended handling is needed here — slots above the complete data are rejected before this call.
//
// Day boundaries are handled robustly: day(N).MaxSlot and day(N+1).MinSlot may be non-contiguous
// (skipped slots between days), and empty days are simply absent from Days. The lower bound is the
// first recorded day whose MaxSlot >= from (the day holding `from`, or the next one if `from` sits in
// a gap); the upper bound is the last day whose MinSlot <= to.
func (ix *DaySlotIndex) window(from, to uint64) (lo, hi time.Time, ok bool) {
	n := len(ix.Days)
	if n == 0 {
		return time.Time{}, time.Time{}, false
	}
	loIdx := sort.Search(n, func(i int) bool { return ix.Days[i].MaxSlot >= from })
	if loIdx == n {
		// `from` is above every recorded day; only reachable if the caller skipped the bound check.
		return time.Time{}, time.Time{}, false
	}
	loDay := ix.Days[loIdx].Date
	hiIdx := sort.Search(n, func(i int) bool { return ix.Days[i].MinSlot > to }) - 1
	if hiIdx < loIdx {
		return time.Time{}, time.Time{}, false // [from, to] covers no recorded blocks (skip gap)
	}
	return loDay, endOfDay(ix.Days[hiIdx].Date), true
}

// previousWindow returns the [lo, hi] window (inclusive, UTC) of the day that holds the nearest block
// with slot < before, used to bound QueryPreviousUnskipped. That block lives in the last recorded day
// whose MinSlot < before. ok is false when no block precedes before.
//
// Precondition (enforced by previousDayWindow): the index is built (n > 0) and before <=
// maxValidSlot+1, so the searched-for predecessor (slot < before) is within the complete data.
func (ix *DaySlotIndex) previousWindow(before uint64) (lo, hi time.Time, ok bool) {
	n := len(ix.Days)
	if n == 0 {
		return time.Time{}, time.Time{}, false
	}
	j := sort.Search(n, func(i int) bool { return ix.Days[i].MinSlot >= before }) - 1
	if j < 0 {
		return time.Time{}, time.Time{}, false // before is at/below the earliest recorded slot
	}
	return ix.Days[j].Date, endOfDay(ix.Days[j].Date), true
}

// snapshot returns a compact, display-friendly view of the index for Store.Snapshot: the completeness
// boundary, the data boundary slot, the day count, and the day entries — capped at the oldest 100 plus
// the newest 100 (with the elided count) so the output stays bounded for a multi-thousand-day index.
func (ix *DaySlotIndex) snapshot() map[string]any {
	n := len(ix.Days)
	out := map[string]any{
		"completeThrough": ix.CompleteThrough,
		"dayCount":        n,
	}
	if maxSlot, ok := ix.maxValidSlot(); ok {
		out["maxValidSlot"] = maxSlot
	}
	const k = 100
	cp := func(src []DayEntry) []DayEntry {
		dst := make([]DayEntry, len(src))
		copy(dst, src)
		return dst
	}
	if n <= 2*k {
		out["days"] = cp(ix.Days)
	} else {
		out["oldestDays"] = cp(ix.Days[:k])
		out["newestDays"] = cp(ix.Days[n-k:])
		out["elidedDays"] = n - 2*k
	}
	return out
}

// mergeForward folds newDays into the index and records the new completeness boundary. Entries for a
// day already present are replaced (refreshed); new days are appended; the result is kept sorted
// ascending by date. (The sole caller passes only days strictly newer than the current
// CompleteThrough, so in practice this just appends — but de-duplicating by date keeps the index
// correct if a day is ever re-queried.)
func (ix *DaySlotIndex) mergeForward(newDays []DayEntry, completeThrough time.Time) {
	if len(newDays) > 0 {
		pos := make(map[int64]int, len(ix.Days))
		for i, d := range ix.Days {
			pos[d.Date.Unix()] = i
		}
		for _, d := range newDays {
			if i, exists := pos[d.Date.Unix()]; exists {
				ix.Days[i] = d
			} else {
				pos[d.Date.Unix()] = len(ix.Days)
				ix.Days = append(ix.Days, d)
			}
		}
		sort.Slice(ix.Days, func(i, j int) bool { return ix.Days[i].Date.Before(ix.Days[j].Date) })
	}
	ix.CompleteThrough = completeThrough
}
