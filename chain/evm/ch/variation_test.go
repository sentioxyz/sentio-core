package ch

import (
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

	// value = the ORDER BY that makes the read follow the table's sort key, which is what lets
	// ClickHouse deduplicate streaming (DistinctSortedStreamTransform) instead of hashing the
	// whole result; blockTxHashes reads a single block, so its key order starts at
	// transaction_index
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
		assert.Containsf(t, tc.sql, tc.orderBy, "%s must read in sort-key order: %s", name, tc.sql)
	}
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
