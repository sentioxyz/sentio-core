package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/chx"
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

	// a table taking part in the slot-range operations must say what identifies its rows, so that
	// no table is silently left out of the duplicate check
	assert.ErrorContains(t, build().checkUniqueKey(), "declares no unique key")

	assert.ErrorContains(t, build().WithUniqueKey("number", "missing").checkUniqueKey(),
		`unique key column "missing" is not a column of table tbl`)

	// a table outside the slot-range operations has no number field to scope a window with
	outside := build()
	outside.NumberField = ""
	assert.NoError(t, outside.checkUniqueKey())
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
