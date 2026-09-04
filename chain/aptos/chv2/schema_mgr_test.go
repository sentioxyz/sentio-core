package chv2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/common/chx"
)

func TestTablesMetaAndConvertAlignment(t *testing.T) {
	ctrl := chx.New(nil,
		chx.WithDatabase("db"),
		chx.WithTableNamePrefix("aptos-test."),
	)
	m := NewClickhouseSchemaMgr(ctrl, 1000000, 10000000, 1)
	meta := m.GetTablesMeta()

	var tableNames []string
	for _, tbl := range meta.Tables {
		tableNames = append(tableNames, tbl.Table.Name)
	}
	assert.Equal(t, []string{
		tableNameBlocks, tableNameTransactions, tableNameEvents,
	}, tableNames)

	// the per-change tables are views over transactions instead of physical tables
	var viewNames []string
	for _, view := range meta.Views {
		viewNames = append(viewNames, view.Name)
		assert.Contains(t, view.Select, "`db`.`aptos-test.transactions`")
	}
	assert.Equal(t, []string{
		tableNameChanges, tableNameResources, tableNameModules, tableNameTableItems,
	}, viewNames)

	// the table_items view exposes the storage-slot identifier that the legacy physical table
	// was missing
	assert.Contains(t, meta.Views[3].Select,
		"JSONExtractString(changes[ci], 'state_key_hash') AS state_key_hash")
	// the changes view unnests the small change_addresses column, not the big changes column
	assert.Contains(t, meta.Views[0].Select, "ARRAY JOIN arrayEnumerate(change_addresses) AS ci")

	// Chunk.RowNum must stay aligned with Tables
	chunk, err := m.Convert(context.Background(), &aptos.Slot{})
	assert.NoError(t, err)
	assert.Len(t, chunk.RowNum, len(meta.Tables))
}
