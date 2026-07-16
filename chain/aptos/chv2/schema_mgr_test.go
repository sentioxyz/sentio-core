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
		tableNameChanges, tableNameResources, tableNameModules,
	}, tableNames)

	// table_items is a view over transactions instead of a physical table
	assert.Len(t, meta.Views, 1)
	view := meta.Views[0]
	assert.Equal(t, tableNameTableItems, view.Name)
	assert.Contains(t, view.Select, "`db`.`aptos-test.transactions`")
	// the view exposes the storage-slot identifier that the legacy physical table was missing
	assert.Contains(t, view.Select, "JSONExtractString(changes[ci], 'state_key_hash') AS state_key_hash")

	// Chunk.RowNum must stay aligned with Tables
	chunk, err := m.Convert(context.Background(), &aptos.Slot{})
	assert.NoError(t, err)
	assert.Len(t, chunk.RowNum, len(meta.Tables))
}
