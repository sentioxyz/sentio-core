package clickhouse

import (
	"context"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/chx"
	rg "sentioxyz/sentio-core/common/range"
)

type TableSchema struct {
	Table          chx.Table
	NumberField    string // if NumberField is empty, delete in range will ignore this table
	SubNumberField string
	// UniqueKey lists the columns that identify a row: two rows of the table never share these
	// values unless the data is broken (e.g. a flush written twice). CheckDuplicates only scans
	// tables that declare one; leave it empty when rows have no identity of their own.
	UniqueKey []string
}

// WithUniqueKey declares the identity columns of the table, see TableSchema.UniqueKey.
func (t TableSchema) WithUniqueKey(columns ...string) TableSchema {
	t.UniqueKey = columns
	return t
}

type TablesMeta struct {
	Tables []TableSchema

	// Views are auxiliary views created alongside the tables. They are not write targets:
	// save, delete and verification only apply to Tables. A view is skipped (with a warning)
	// if a physical table with the same name still exists, so replacing a table with a view
	// requires dropping the legacy table manually first.
	Views []chx.View

	// LinkTableIndex >=0 means the chain need to check the link between blocks
	LinkTableIndex           int
	LinkTableNumberField     string
	LinkTableHashField       string
	LinkTableParentHashField string

	// BlockTableIndex >= 0 means some of the tables partition by sub-block number, range query in these tables
	// need to use sub-block number instead of block number, or it will have performances issue, and the range of
	// sub-blocks contained in each block depends on the MinSubNumberField and MaxSubNumberField fields.
	BlockTableIndex             int
	BlockTableMinSubNumberField string
	BlockTableMaxSubNumberField string
}

type Chunk struct {
	SlotNum uint64
	RowNum  []int
	RowData [][]any
}

type tableRows [][]any

type SchemaMgr[SLOT chain.Slot] interface {
	GetTablesMeta() TablesMeta

	// Convert will be called in order, with concurrency got by ConvertConcurrency()
	Convert(ctx context.Context, st SLOT) (Chunk, error)
	ConvertConcurrency() uint
	Done(r rg.Range) error
}
