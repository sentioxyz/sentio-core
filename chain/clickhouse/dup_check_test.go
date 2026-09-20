package clickhouse

import (
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/chx"
	rg "sentioxyz/sentio-core/common/range"
)

func Test_checkUniqueKey(t *testing.T) {
	type row struct {
		Number uint64 `clickhouse:"number" number_field:"true"`
		Index  uint64 `clickhouse:"index"`
		Data   string `clickhouse:"data"`
	}
	build := func() TableSchema {
		return BuildTable("tbl", &row{}, chx.TableConfig{}, "")
	}

	assert.NoError(t, build().WithUniqueKey("number", "index").checkUniqueKey())

	// a table must say what identifies its rows, so that none is silently left out of the check
	assert.ErrorContains(t, build().checkUniqueKey(), "declares neither a unique key nor a reason")

	assert.ErrorContains(t, build().WithUniqueKey("number", "missing").checkUniqueKey(),
		`unique key column "missing" is not a column of table tbl`)

	// a table whose rows carry no identity worth checking has to say so, and why
	assert.NoError(t, build().WithoutUniqueKey("append-only, a repeated row changes no answer").checkUniqueKey())
	assert.ErrorContains(t,
		build().WithUniqueKey("number").WithoutUniqueKey("both").checkUniqueKey(),
		"declares both a unique key and a reason to go without one")

	// a key the check could never scope a window for is worse than none: it reads as coverage
	outside := build().WithUniqueKey("number", "index")
	outside.NumberField = ""
	assert.ErrorContains(t, outside.checkUniqueKey(), "no number field to scope a duplicate check window")
}

func Test_duplicateCheckSQL(t *testing.T) {
	type row struct {
		Number uint64 `clickhouse:"number" number_field:"true"`
		Index  uint64 `clickhouse:"index"`
		Data   string `clickhouse:"data"`
	}
	table := BuildTable("tbl", &row{}, chx.TableConfig{}, "").WithUniqueKey("number", "index")
	assert.Equal(t,
		"SELECT count(), toUInt64(sum(c - 1)), toUInt64(min(n)), toUInt64(max(n)) FROM ("+
			"SELECT count() AS c, min(`number`) AS n FROM `db`.`tbl` WHERE number >= 10 AND number <= 20 "+
			"GROUP BY `number`, `index` HAVING c > 1)",
		table.duplicateCheckSQL("`db`.`tbl`", "number >= 10 AND number <= 20"))

	// a key column added to the table later is NULL on the rows written before it existed: their
	// identity is unknown, not shared, so they are left out instead of counted as one duplicate
	type latecomer struct {
		Number uint64  `clickhouse:"number" number_field:"true"`
		Index  *uint64 `clickhouse:"index"`
		Data   string  `clickhouse:"data"`
	}
	late := BuildTable("tbl", &latecomer{}, chx.TableConfig{}, "").WithUniqueKey("number", "index")
	assert.Equal(t, []string{"index"}, late.nullableUniqueKeyColumns())
	assert.Contains(t, late.duplicateCheckSQL("`db`.`tbl`", "number >= 10"),
		"WHERE number >= 10 AND `index` IS NOT NULL GROUP BY `number`, `index`")
}

func Test_packDuplicateCheckPages(t *testing.T) {
	window := rg.NewRange(1000, 1999)
	const bucket = 100

	// a window light enough for one scan stays one page, and it ends where its rows end
	light := []bucketRows{{Bucket: 10, Rows: 3}, {Bucket: 12, Rows: 4}}
	assert.Equal(t, []rg.Range{rg.NewRange(1000, 1299)}, packDuplicateCheckPages(window, bucket, light))

	// buckets are packed until a page is full, so a busy stretch makes shorter pages rather than
	// heavier ones: the first three buckets fill a page on their own here
	heavy := []bucketRows{
		{Bucket: 10, Rows: 2_000_000},
		{Bucket: 11, Rows: 2_000_000},
		{Bucket: 12, Rows: 2_000_000},
		{Bucket: 13, Rows: 1},
		{Bucket: 19, Rows: 1},
	}
	assert.Equal(t, []rg.Range{rg.NewRange(1000, 1299), rg.NewRange(1300, 1999)},
		packDuplicateCheckPages(window, bucket, heavy))

	// a bucket heavier than a page is a page of its own, since it cannot be cut any finer
	assert.Equal(t, []rg.Range{rg.NewRange(1000, 1099), rg.NewRange(1100, 1199)},
		packDuplicateCheckPages(window, bucket, []bucketRows{
			{Bucket: 10, Rows: 50_000_000},
			{Bucket: 11, Rows: 50_000_000},
		}))

	// an empty window is not scanned at all
	assert.Empty(t, packDuplicateCheckPages(window, bucket, nil))
}

func Test_duplicateCheckWindow(t *testing.T) {
	cur := rg.NewRange(100, 999)

	// nothing recorded yet, or nothing held: there is no window to scan
	assert.True(t, duplicateCheckWindow(500, false, cur).IsEmpty())
	assert.True(t, duplicateCheckWindow(500, true, rg.EmptyRange).IsEmpty())

	// from the oldest recorded end to what the destination holds now
	assert.Equal(t, rg.NewRange(900, 999), duplicateCheckWindow(900, true, cur))

	// never below the range the destination holds, never above its end
	assert.Equal(t, rg.NewRange(100, 999), duplicateCheckWindow(50, true, cur))
	assert.Equal(t, rg.NewRange(999, 999), duplicateCheckWindow(5000, true, cur))

	// a fork records an end below the ones before it, and the window follows it down
	assert.Equal(t, rg.NewRange(149, 299), duplicateCheckWindow(149, true, rg.NewRange(100, 299)))

	// a destination that started empty records its first range already ending at the last slot of
	// the batch that filled it, so that batch is behind the window: it needs one look by hand
	assert.Equal(t, rg.NewRange(199, 199), duplicateCheckWindow(199, true, rg.NewRange(100, 199)))
}

func Test_deleteNeedsReplicaSync(t *testing.T) {
	clean := rg.EmptyRangeSet
	dirty := rg.EmptyRangeSet.Union(rg.NewRange(500, 599))

	// taking back everything above a point: a killed process's leftovers at startup, and the slots
	// a fork is about to have written again
	assert.True(t, deleteNeedsReplicaSync(rg.Range{Start: 100}, clean))

	// the retention cut, far below anything being written, puts nothing back
	assert.False(t, deleteNeedsReplicaSync(rg.NewRange(0, 99), clean))

	// unless it reaches into a range a save left unfinished, where rows the count cannot see may
	// still be landing
	assert.True(t, deleteNeedsReplicaSync(rg.NewRange(0, 599), dirty))

	// a cut clear of every such range is still free to go
	assert.False(t, deleteNeedsReplicaSync(rg.NewRange(0, 99), dirty))
}

func Test_truncateNeedsReplicaSync(t *testing.T) {
	clean := rg.EmptyRangeSet
	dirty := rg.EmptyRangeSet.Union(rg.NewRange(500, 599))

	// an append above everything written so far: nothing of this store's is in flight there
	assert.False(t, truncateNeedsReplicaSync(rg.NewRange(200, 299), true, 199, clean))

	// an overwrite, reaching back over rows that may be as new as the last save: a repair filling a
	// gap, or a copy asked to overwrite
	assert.True(t, truncateNeedsReplicaSync(rg.NewRange(130, 170), true, 199, clean))
	assert.True(t, truncateNeedsReplicaSync(rg.NewRange(199, 299), true, 199, clean))

	// a store that has saved nothing yet knows nothing about what is there
	assert.True(t, truncateNeedsReplicaSync(rg.NewRange(200, 299), false, 0, clean))

	// a range a save left unfinished sits above the high-water mark, which would otherwise wave its
	// retry through as an append
	assert.True(t, truncateNeedsReplicaSync(rg.NewRange(500, 599), true, 199, dirty))
	assert.True(t, truncateNeedsReplicaSync(rg.NewRange(550, 650), true, 199, dirty))

	// an append somewhere else is still an append
	assert.False(t, truncateNeedsReplicaSync(rg.NewRange(600, 699), true, 199, dirty))
}

func Test_recordSave(t *testing.T) {
	store := SimpleSlotStore[*testSlot]{dirty: rg.EmptyRangeSet}

	// a finished save is what lets the next one tell an append from an overwrite
	store.recordSave(rg.NewRange(100, 199), true, nil)
	assert.False(t, store.truncateNeedsReplicaSync(rg.NewRange(200, 299)))
	assert.True(t, store.truncateNeedsReplicaSync(rg.NewRange(150, 299)))

	// a save that did not finish leaves its range behind, above everything saved so far
	store.recordSave(rg.NewRange(500, 599), false, errors.New("boom"))
	assert.True(t, store.truncateNeedsReplicaSync(rg.NewRange(500, 599)))

	// a save running alongside it finishes without having waited for the replicas
	store.recordSave(rg.NewRange(300, 399), false, nil)
	assert.True(t, store.truncateNeedsReplicaSync(rg.NewRange(500, 599)))

	// and so does one that did wait, but for its own range: its wait ran before those rows were
	// written and its truncate never touched them, so the range stays
	store.recordSave(rg.NewRange(400, 499), true, nil)
	assert.True(t, store.truncateNeedsReplicaSync(rg.NewRange(500, 599)))

	// only the save that covered the range itself, having waited, takes it out
	store.recordSave(rg.NewRange(500, 599), true, nil)
	assert.False(t, store.truncateNeedsReplicaSync(rg.NewRange(600, 699)))

	// and the high-water mark only ever moves up
	assert.True(t, store.truncateNeedsReplicaSync(rg.NewRange(599, 699)))
}

// testSlot is the smallest thing that satisfies chain.Slot, for the store's type parameter.
type testSlot struct{}

func (t *testSlot) GetNumber() uint64     { return 0 }
func (t *testSlot) GetHash() string       { return "" }
func (t *testSlot) GetParentHash() string { return "" }
func (t *testSlot) Features() []string    { return nil }
func (t *testSlot) Linked() bool          { return false }
