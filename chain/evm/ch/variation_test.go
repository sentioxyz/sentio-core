package ch

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/chx"
)

// The chain tables are plain MergeTree with no deduplication, so a re-ingested block leaves
// byte-identical rows behind (production incident: Arbitrum blocks 466790099+ had duplicate
// rows in blocks / logs / transactions). Every read must collapse them at the source, otherwise
// callers see the same record twice — or, for the Ex queries, a duplicated region trips a
// record cap and stalls a driver.
func TestQuerySQLIsDeduplicated(t *testing.T) {
	c := NewEthVarCtrl("1", chx.Controller{}).(*EthVariationController[*Block, *Transaction])

	// value = the output order callers rely on. It is NOT what makes the dedup streaming -- that
	// comes from the read order and is covered by TestQuerySQLDistinctCoversSortKey; logs and
	// blockTxHashes deliberately order by something other than a sort-key prefix.
	for name, tc := range map[string]struct{ sql, orderBy string }{
		"blocks":        {c.blocksSQL("block_number = 1"), "ORDER BY block_number"},
		"blockLinks":    {c.blockLinksSQL(1, 2), "ORDER BY block_number"},
		"blockTxHashes": {c.blockTxHashesSQL(), "ORDER BY transaction_index"},
		"txs":           {c.txsSQL("block_number = 1"), "ORDER BY block_number, transaction_index"},
		"logs":          {c.logsSQL("block_number = 1", 0), "ORDER BY block_number, log_index"},
		"traces": {
			c.tracesSQL("block_number = 1", 0),
			"ORDER BY block_number, transaction_index, trace_index",
		},
	} {
		assert.Truef(t, strings.HasPrefix(tc.sql, "SELECT DISTINCT "), "%s must deduplicate: %s", name, tc.sql)
		assert.Containsf(t, tc.sql, tc.orderBy, "%s must return rows in the order callers expect: %s", name, tc.sql)
	}
}

// The streaming dedup only holds while every sort-key column is either selected into the DISTINCT
// list or pinned to a single value by the WHERE: that is what lets ClickHouse reuse the MergeTree
// read order and clear its seen-set as the key advances, instead of hashing the whole result. The
// sort keys come from BuildTablesMeta so adding or reordering one fails here rather than silently
// turning every read into a full hash.
func TestQuerySQLDistinctCoversSortKey(t *testing.T) {
	c := NewEthVarCtrl("1", chx.Controller{}).(*EthVariationController[*Block, *Transaction])
	sortKeys := make(map[string][]string)
	for _, tbl := range c.BuildTablesMeta(500000).Tables {
		sortKeys[tbl.Table.Name] = tbl.Table.Config.OrderBy
	}
	for name, tc := range map[string]struct{ sql, table string }{
		// a WHERE that pins nothing, so the DISTINCT list itself has to carry the sort key;
		// blockTxHashes is the one intended exception and pins block_number on its own
		"blocks":        {c.blocksSQL("block_number > 1"), tableNameBlocks},
		"blockLinks":    {c.blockLinksSQL(1, 2), tableNameBlocks},
		"blockTxHashes": {c.blockTxHashesSQL(), tableNameTransactions},
		"txs":           {c.txsSQL("block_number > 1"), tableNameTransactions},
		"logs":          {c.logsSQL("block_number > 1", 0), tableNameLogs},
		"traces":        {c.tracesSQL("block_number > 1", 0), tableNameTraces},
	} {
		sortKey := sortKeys[tc.table]
		assert.NotEmptyf(t, sortKey, "%s: no sort key declared for table %s", name, tc.table)
		selected := distinctColumns(t, tc.sql)
		for _, col := range sortKey {
			pinned := strings.Contains(tc.sql, col+" = ")
			assert.Truef(t, slices.Contains(selected, col) || pinned,
				"%s: sort-key column %s is neither selected nor pinned to one value: %s", name, col, tc.sql)
		}
	}
}

// distinctColumns returns the column list of a SELECT DISTINCT, stripped of quoting.
func distinctColumns(t *testing.T, sql string) []string {
	t.Helper()
	const prefix = "SELECT DISTINCT "
	rest, ok := strings.CutPrefix(sql, prefix)
	if !ok {
		t.Fatalf("not a SELECT DISTINCT: %s", sql)
	}
	list, _, ok := strings.Cut(rest, " FROM ")
	if !ok {
		t.Fatalf("no FROM clause: %s", sql)
	}
	cols := strings.Split(strings.ReplaceAll(list, "`", ""), ",")
	for i := range cols {
		cols[i] = strings.TrimSpace(cols[i])
	}
	return cols
}

func TestBlockTxHashesSQLKeepsOrderColumnInDistinct(t *testing.T) {
	c := NewEthVarCtrl("1", chx.Controller{}).(*EthVariationController[*Block, *Transaction])
	sql := c.blockTxHashesSQL()
	// ClickHouse rejects an ORDER BY column that is not part of a SELECT DISTINCT list, and
	// deduplicating on the bare hash would also drop a hash that legitimately repeats at
	// another index, so transaction_index has to ride along
	assert.Contains(t, sql, "SELECT DISTINCT transaction_index, transaction_hash")
	assert.Contains(t, sql, "ORDER BY transaction_index")
}

func TestQuerySQLKeepsRecordCap(t *testing.T) {
	c := NewEthVarCtrl("1", chx.Controller{}).(*EthVariationController[*Block, *Transaction])
	// the cap now bounds DEDUPLICATED rows
	assert.Contains(t, c.logsSQL("block_number = 1", 4001), " LIMIT 4001")
	assert.Contains(t, c.tracesSQL("block_number = 1", 4001), " LIMIT 4001")
	assert.NotContains(t, c.logsSQL("block_number = 1", 0), "LIMIT")
	assert.NotContains(t, c.tracesSQL("block_number = 1", 0), "LIMIT")
}
