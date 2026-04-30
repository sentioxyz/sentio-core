package chv2

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/clickhouse"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/objectx"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

const (
	BlockTableIdx       = 0
	TransactionTableIdx = 1
	EventTableIdx       = 2
	ChangeTableIdx      = 3
	ResourceTableIdx    = 4
	ModuleTableIdx      = 5
	TableItemTableIdx   = 6
)

type ClickhouseSchemaMgr struct {
	tablesMeta         clickhouse.TablesMeta
	convertConcurrency uint
}

func tableNames(database, tableNamePrefix string) []chx.FullName {
	return utils.MapSliceNoError([]string{
		"blocks",
		"transactions",
		"events",
		"changes",
		"resources",
		"modules",
		"table_items",
	}, func(suffix string) chx.FullName {
		return chx.FullName{
			Database: database,
			Name:     tableNamePrefix + "." + suffix,
		}
	})
}

func NewClickhouseSchemaMgr(
	cluster string,
	database string,
	tableNamePrefix string,
	blockPartitionSize uint64,
	txnPartitionSize uint64,
	convertConcurrency uint,
) *ClickhouseSchemaMgr {
	tablesName := tableNames(database, tableNamePrefix)
	engine := chx.NewDefaultMergeTreeEngine(cluster != "")
	blockPartitionBy := fmt.Sprintf("intDiv(block_height, %d)", blockPartitionSize)
	txnPartitionBy := fmt.Sprintf("intDiv(transaction_version, %d)", txnPartitionSize)
	tableSettings := make(map[string]string)
	chx.WithLightDeleteTableSettings(tableSettings)
	chx.WithProjectionTableSettings(tableSettings)
	createTableSchema := func(name chx.FullName, tblObj any, partitionBy string, orderBy ...string) clickhouse.TableSchema {
		config := chx.TableConfig{
			Engine:      engine,
			PartitionBy: partitionBy,
			OrderBy:     orderBy,
			Settings:    tableSettings,
		}
		return clickhouse.BuildTable(name, tblObj, config, "")
	}
	// the order should match to const *TableIdx
	tables := []clickhouse.TableSchema{
		createTableSchema(tablesName[0], &Block{}, blockPartitionBy, "block_height"),
		createTableSchema(tablesName[1], &Transaction{}, txnPartitionBy, "transaction_version"),
		createTableSchema(tablesName[2], &Event{}, txnPartitionBy, "transaction_version", "event_index"),
		createTableSchema(tablesName[3], &Change{}, txnPartitionBy, "transaction_version", "change_index"),
		createTableSchema(tablesName[4], &Resource{}, txnPartitionBy, "transaction_version", "change_index"),
		createTableSchema(tablesName[5], &Module{}, txnPartitionBy, "transaction_version", "change_index"),
		createTableSchema(tablesName[6], &TableItem{}, txnPartitionBy, "transaction_version", "change_index"),
	}
	return &ClickhouseSchemaMgr{
		tablesMeta: clickhouse.TablesMeta{
			Tables:                      tables,
			LinkTableIndex:              -1,
			BlockTableIndex:             0,
			BlockTableMinSubNumberField: "first_version",
			BlockTableMaxSubNumberField: "last_version",
		},
		convertConcurrency: convertConcurrency,
	}
}

func (m *ClickhouseSchemaMgr) GetTablesMeta() clickhouse.TablesMeta {
	return m.tablesMeta
}

func (m *ClickhouseSchemaMgr) convert(slot *aptos.Slot) (
	block Block,
	transactions []Transaction,
	events []Event,
	changes []Change,
	modules []Module,
	resources []Resource,
	tableItems []TableItem,
	err error,
) {
	blockIndex := BlockIndex{
		BlockHeight:    slot.BlockHeight,
		BlockTimestamp: time.UnixMicro(int64(slot.BlockTimestamp)),
		BlockHash:      slot.BlockHash,
	}
	block = Block{
		BlockIndex:        blockIndex,
		FirstVersion:      slot.FirstVersion,
		LastVersion:       slot.LastVersion,
		TransactionsCount: int64(len(slot.Transactions)),
	}
	for i, tx := range slot.Transactions {
		txIndex := TransactionIndex{
			TransactionHash:    tx.Hash(),
			TxIndex:            uint64(i),
			TransactionVersion: tx.Version(),
		}
		if bmtx, is := tx.BlockMetadataTransaction(); is == nil {
			block.Epoch = bmtx.Epoch
			block.Round = bmtx.Round
			block.PreviousBlockVotesBitvec = hexutil.Bytes(bmtx.PreviousBlockVotesBitvec).String()
			block.Proposer = accountAddressToString(bmtx.Proposer)
		}
		var transaction Transaction
		if err = transaction.fromRawTransaction(blockIndex, txIndex, *tx); err != nil {
			return
		}
		for evIndex, ev := range aptos.GetTransactionEvents(tx) {
			var event Event
			event.fromRawEvent(blockIndex, txIndex, uint64(evIndex), *ev)
			events = append(events, event)
		}
		for wcIndex, wc := range aptos.GetTransactionChanges(tx) {
			changeIndex := ChangeIndex{
				ChangeIndex: uint64(wcIndex),
				ChangeType:  string(wc.Type),
			}
			var change Change
			if err = change.fromRawChange(blockIndex, txIndex, changeIndex, *wc); err != nil {
				return
			}
			changes = append(changes, change)

			var module Module
			if module.fromRawChange(blockIndex, txIndex, changeIndex, *wc) {
				modules = append(modules, module)
				transaction.ModuleChangesCount++
				continue
			}
			var resource Resource
			if resource.fromRawChange(blockIndex, txIndex, changeIndex, *wc) {
				resources = append(resources, resource)
				transaction.ResourceChangesCount++
				continue
			}
			var tableItem TableItem
			if tableItem.fromRawChange(blockIndex, txIndex, changeIndex, *wc) {
				tableItems = append(tableItems, tableItem)
				transaction.TableItemChangesCount++
			}
		}
		transactions = append(transactions, transaction)
	}
	return
}

func (m *ClickhouseSchemaMgr) Convert(_ context.Context, slot *aptos.Slot) (clickhouse.Chunk, error) {
	block, transactions, events, changes, modules, resources, tableItems, err := m.convert(slot)
	if err != nil {
		return clickhouse.Chunk{}, err
	}
	fieldFilter := objectx.HasTag("clickhouse")
	var values [][]any
	values = append(values, objectx.CollectFieldValues(block, fieldFilter))
	for _, tx := range transactions {
		values = append(values, objectx.CollectFieldValues(tx, fieldFilter))
	}
	for _, ev := range events {
		values = append(values, objectx.CollectFieldValues(ev, fieldFilter))
	}
	for _, cg := range changes {
		values = append(values, objectx.CollectFieldValues(cg, fieldFilter))
	}
	for _, re := range resources {
		values = append(values, objectx.CollectFieldValues(re, fieldFilter))
	}
	for _, md := range modules {
		values = append(values, objectx.CollectFieldValues(md, fieldFilter))
	}
	for _, ti := range tableItems {
		values = append(values, objectx.CollectFieldValues(ti, fieldFilter))
	}
	counts := []int{1, len(transactions), len(events), len(changes), len(resources), len(modules), len(tableItems)}
	return clickhouse.Chunk{RowNum: counts, RowData: values}, nil
}

func (m *ClickhouseSchemaMgr) ConvertConcurrency() uint {
	return m.convertConcurrency
}

func (m *ClickhouseSchemaMgr) Done(r rg.Range) error {
	return nil
}
