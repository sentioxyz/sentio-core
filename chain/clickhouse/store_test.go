package clickhouse

import (
	"testing"

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
