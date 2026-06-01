package ch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/clickhouse"
	"sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/common/chx"
	rg "sentioxyz/sentio-core/common/range"
)

const (
	tableNameBlocks       = "blocks"
	tableNameTransactions = "transactions"
)

type ClickhouseSchemaMgr struct {
	tablesMeta         clickhouse.TablesMeta
	convertConcurrency uint
}

func NewClickhouseSchemaMgr(
	ctrl chx.Controller,
	blockPartitionSize uint64,
	convertConcurrency uint,
) *ClickhouseSchemaMgr {
	blockSettings := make(map[string]string)
	chx.WithLightDeleteTableSettings(blockSettings)
	blockTable := clickhouse.BuildTable(
		tableNameBlocks,
		&ClickhouseBlock{},
		chx.TableConfig{
			Engine:      ctrl.NewDefaultMergeTreeEngine(),
			PartitionBy: fmt.Sprintf("intDiv(slot, %d)", blockPartitionSize),
			OrderBy:     []string{"slot"},
			Settings:    blockSettings,
		},
		"",
	)

	txSettings := make(map[string]string)
	chx.WithLightDeleteTableSettings(txSettings)
	chx.WithProjectionTableSettings(txSettings)
	txTable := clickhouse.BuildTable(
		tableNameTransactions,
		&ClickhouseTransaction{},
		chx.TableConfig{
			Engine:      ctrl.NewDefaultMergeTreeEngine(),
			PartitionBy: fmt.Sprintf("intDiv(slot, %d)", blockPartitionSize),
			OrderBy:     []string{"slot", "transaction_index"},
			Settings:    txSettings,
		},
		"",
	)

	return &ClickhouseSchemaMgr{
		tablesMeta: clickhouse.TablesMeta{
			// blocks is index 0 so CheckMissing uses its dense one-row-per-slot count.
			Tables:          []clickhouse.TableSchema{blockTable, txTable},
			LinkTableIndex:  -1,
			BlockTableIndex: -1,
		},
		convertConcurrency: convertConcurrency,
	}
}

func (m *ClickhouseSchemaMgr) GetTablesMeta() clickhouse.TablesMeta {
	return m.tablesMeta
}

func (m *ClickhouseSchemaMgr) Convert(_ context.Context, st *sol.Slot) (clickhouse.Chunk, error) {
	var blockTime time.Time
	if st.BlockTime != nil {
		blockTime = st.BlockTime.Time()
	}
	var blockHeight uint64
	if st.BlockHeight != nil {
		blockHeight = *st.BlockHeight
	}

	block := ClickhouseBlock{
		Slot:              st.SlotNumber,
		Skipped:           st.Skipped,
		Blockhash:         st.Blockhash.String(),
		PreviousBlockhash: st.PreviousBlockhash.String(),
		ParentSlot:        st.ParentSlot,
		BlockHeight:       blockHeight,
		BlockTime:         blockTime,
	}

	txRows := make([][]any, 0, len(st.Transactions))
	for i, tx := range st.Transactions {
		if tx.Transaction == nil || len(tx.Transaction.Signatures) == 0 {
			continue
		}
		txnJSON, err := json.Marshal(tx)
		if err != nil {
			return clickhouse.Chunk{}, errors.Wrapf(err, "marshal transaction %d of slot %d failed", i, st.SlotNumber)
		}
		txRows = append(txRows, transactionValues(ClickhouseTransaction{
			Slot:             st.SlotNumber,
			BlockTime:        blockTime,
			TransactionIndex: uint32(i),
			Signature:        tx.Transaction.Signatures[0].String(),
			AccountKeys:      sol.CollectAccountKeys(tx.Transaction, tx.Meta),
			Version:          int32(tx.Version),
			Err:              tx.Meta != nil && tx.Meta.Err != nil,
			TransactionJSON:  string(txnJSON),
		}))
	}

	rows := make([][]any, 0, 1+len(txRows))
	rows = append(rows, blockValues(block))
	rows = append(rows, txRows...)
	return clickhouse.Chunk{
		RowNum:  []int{1, len(txRows)},
		RowData: rows,
	}, nil
}

func (m *ClickhouseSchemaMgr) ConvertConcurrency() uint {
	return m.convertConcurrency
}

func (m *ClickhouseSchemaMgr) Done(r rg.Range) error {
	return nil
}
