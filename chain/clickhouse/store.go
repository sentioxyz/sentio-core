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
}

type TablesMeta struct {
	Tables []TableSchema

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
