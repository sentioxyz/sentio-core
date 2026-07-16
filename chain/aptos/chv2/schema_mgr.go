package chv2

import (
	"context"
	"fmt"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/clickhouse"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/objectx"
	rg "sentioxyz/sentio-core/common/range"
	"time"
)

type ClickhouseSchemaMgr struct {
	tablesMeta         clickhouse.TablesMeta
	convertConcurrency uint
}

const (
	tableNameBlocks       = "blocks"
	tableNameTransactions = "transactions"
	tableNameEvents       = "events"
	tableNameChanges      = "changes"
	tableNameResources    = "resources"
	tableNameModules      = "modules"
	tableNameTableItems   = "table_items"
)

func NewClickhouseSchemaMgr(
	ctrl chx.Controller,
	blockPartitionSize uint64,
	txnPartitionSize uint64,
	convertConcurrency uint,
) *ClickhouseSchemaMgr {
	engine := ctrl.NewDefaultMergeTreeEngine()
	blockPartitionBy := fmt.Sprintf("intDiv(block_height, %d)", blockPartitionSize)
	txnPartitionBy := fmt.Sprintf("intDiv(transaction_version, %d)", txnPartitionSize)
	tableSettings := make(map[string]string)
	chx.WithLightDeleteTableSettings(tableSettings)
	chx.WithProjectionTableSettings(tableSettings)
	createTableSchema := func(name string, tblObj any, partitionBy string, orderBy ...string) clickhouse.TableSchema {
		config := chx.TableConfig{
			Engine:      engine,
			PartitionBy: partitionBy,
			OrderBy:     orderBy,
			Settings:    tableSettings,
		}
		return clickhouse.BuildTable(name, tblObj, config, "")
	}
	tables := []clickhouse.TableSchema{
		createTableSchema(tableNameBlocks, &Block{}, blockPartitionBy, "block_height"),
		createTableSchema(tableNameTransactions, &Transaction{}, txnPartitionBy, "transaction_version"),
		createTableSchema(tableNameEvents, &Event{}, txnPartitionBy, "transaction_version", "event_index"),
		createTableSchema(tableNameChanges, &Change{}, txnPartitionBy, "transaction_version", "change_index"),
		createTableSchema(tableNameResources, &Resource{}, txnPartitionBy, "transaction_version", "change_index"),
		createTableSchema(tableNameModules, &Module{}, txnPartitionBy, "transaction_version", "change_index"),
	}
	views := []chx.View{
		buildTableItemsView(ctrl),
	}
	return &ClickhouseSchemaMgr{
		tablesMeta: clickhouse.TablesMeta{
			Tables:                      tables,
			Views:                       views,
			LinkTableIndex:              -1,
			BlockTableIndex:             0,
			BlockTableMinSubNumberField: "first_version",
			BlockTableMaxSubNumberField: "last_version",
		},
		convertConcurrency: convertConcurrency,
	}
}

// buildTableItemsView builds the table_items view over the transactions table. Table item
// changes are fully contained in the embedded `changes` JSON array of transactions, so they
// are exposed as a view instead of being persisted a second time.
//
// The ARRAY JOIN intentionally only unnests arrayEnumerate(changes) and every column takes
// changes[ci] lazily, so queries that do not touch the JSON-derived columns never read the
// big `changes` column. Column order matches the legacy physical table, with the previously
// missing state_key_hash (the identifier of the storage slot in the state tree, uniform
// across all change types) appended at the end.
func buildTableItemsView(ctrl chx.Controller) chx.View {
	return chx.View{
		Name: tableNameTableItems,
		Select: fmt.Sprintf(`SELECT
    block_height,
    block_timestamp,
    block_hash,
    transaction_hash,
    transaction_index,
    transaction_version,
    JSONExtractString(changes[ci], 'type') AS type,
    toUInt64(ci - 1) AS change_index,
    JSONExtractString(changes[ci], 'handle') AS table_item_handle,
    JSONExtractString(changes[ci], 'key') AS table_item_key,
    if(startsWith(JSONExtractString(changes[ci], 'type'), 'delete'), '', JSONExtractString(changes[ci], 'value')) AS table_item_value,
    JSONExtractRaw(changes[ci], 'data') AS table_item_data,
    JSONExtractString(changes[ci], 'state_key_hash') AS state_key_hash
FROM %s
ARRAY JOIN arrayEnumerate(changes) AS ci
WHERE JSONExtractString(changes[ci], 'type') IN ('write_table_item', 'delete_table_item')`,
			ctrl.FullLogicName(tableNameTransactions)),
		Comment: "table item changes extracted from the embedded changes of the transactions table",
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
			// table item changes are served by the table_items view over transactions,
			// only the per-transaction counter is still maintained here
			switch wc.Type {
			case api.WriteSetChangeVariantWriteTableItem, api.WriteSetChangeVariantDeleteTableItem:
				transaction.TableItemChangesCount++
			}
		}
		transactions = append(transactions, transaction)
	}
	return
}

func (m *ClickhouseSchemaMgr) Convert(_ context.Context, slot *aptos.Slot) (clickhouse.Chunk, error) {
	block, transactions, events, changes, modules, resources, err := m.convert(slot)
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
	counts := []int{1, len(transactions), len(events), len(changes), len(resources), len(modules)}
	return clickhouse.Chunk{RowNum: counts, RowData: values}, nil
}

func (m *ClickhouseSchemaMgr) ConvertConcurrency() uint {
	return m.convertConcurrency
}

func (m *ClickhouseSchemaMgr) Done(r rg.Range) error {
	return nil
}
