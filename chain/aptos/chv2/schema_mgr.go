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
	}
	views := []chx.View{
		buildChangesView(ctrl),
		buildResourcesView(ctrl),
		buildModulesView(ctrl),
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

// The changes / resources / modules / table_items views below replace the physical tables of the
// same names: every change of a transaction is fully contained in its embedded `changes` JSON
// array (with the element-aligned change_addresses / resource_type companion arrays), so the
// per-change tables duplicated that data and are exposed as views instead. The definitions were
// validated against the legacy physical tables on aptos-mainnet full history with per-column
// hashes; the only intended differences are noted on each view.
//
// Every view ARRAY JOINs arrayEnumerate(...) and takes changes[ci] lazily per column, so a query
// only reads the big `changes` column for the columns and rows it actually touches. The
// `_addresses` passthrough column (and `_resource_types` on the resources view) is appended so
// query layers can reach the bloom-filter index of the underlying array column via
// has(_addresses, ?) — an element-equality predicate on a computed column such as address = ?
// has no automatic index path through a view.

// buildChangesView builds the changes view over the transactions table. It unnests
// arrayEnumerate(change_addresses) — a small column — instead of the changes array, so queries
// not touching `data` never read the big column at all. Intended difference from the legacy
// table: `data` is the whole change JSON including its type, while the legacy column stripped
// the type prefix.
func buildChangesView(ctrl chx.Controller) chx.View {
	return chx.View{
		Name: tableNameChanges,
		Select: fmt.Sprintf(`SELECT
    block_height,
    block_timestamp,
    block_hash,
    transaction_hash,
    transaction_index,
    transaction_version,
    JSONExtractString(changes[ci], 'type') AS type,
    toUInt64(ci - 1) AS change_index,
    JSONExtractString(changes[ci], 'state_key_hash') AS state_key_hash,
    CAST(startsWith(JSONExtractString(changes[ci], 'type'), 'delete'), 'Bool') AS is_deletion,
    CAST(nullIf(change_addresses[ci], ''), 'Nullable(FixedString(66))') AS address,
    changes[ci] AS data,
    change_addresses AS _addresses
FROM %s
ARRAY JOIN arrayEnumerate(change_addresses) AS ci`,
			ctrl.FullLogicName(tableNameTransactions)),
		Comment: "change records extracted from the embedded changes of the transactions table",
	}
}

// buildResourcesView builds the resources view over the transactions table. The row filter and
// the delete-row resource_type must come from the change JSON: the resource_type companion array
// of the base table is only populated for write_resource changes (empty for delete_resource, see
// aptos.GetChangeResourceType), which is also why the base table needs the `t` alias — the view's
// own resource_type output column would shadow it otherwise. The JSON `resource` field of a
// delete row was validated to equal what move.TrimTypeString produced for the legacy table.
func buildResourcesView(ctrl chx.Controller) chx.View {
	return chx.View{
		Name: tableNameResources,
		Select: fmt.Sprintf(`SELECT
    block_height,
    block_timestamp,
    block_hash,
    transaction_hash,
    transaction_index,
    transaction_version,
    JSONExtractString(changes[ci], 'type') AS type,
    toUInt64(ci - 1) AS change_index,
    CAST(nullIf(change_addresses[ci], ''), 'Nullable(FixedString(66))') AS address,
    if(startsWith(JSONExtractString(changes[ci], 'type'), 'delete'),
       JSONExtractString(changes[ci], 'resource'), t.resource_type[ci]) AS resource_type,
    if(startsWith(JSONExtractString(changes[ci], 'type'), 'delete'),
       '', JSONExtractRaw(changes[ci], 'data', 'data')) AS resource_data,
    CAST(startsWith(JSONExtractString(changes[ci], 'type'), 'delete'), 'Bool') AS is_delete,
    t.resource_type AS _resource_types,
    change_addresses AS _addresses
FROM %s AS t
ARRAY JOIN arrayEnumerate(changes) AS ci
WHERE JSONExtractString(changes[ci], 'type') IN ('write_resource', 'delete_resource')`,
			ctrl.FullLogicName(tableNameTransactions)),
		Comment: "resource changes extracted from the embedded changes of the transactions table",
	}
}

// buildModulesView builds the modules view over the transactions table. Intended difference from
// the legacy table: module_bytecode is always the hex form from the change JSON, while the legacy
// table stored raw binary for rows written after the encoding change of the writer.
func buildModulesView(ctrl chx.Controller) chx.View {
	return chx.View{
		Name: tableNameModules,
		Select: fmt.Sprintf(`SELECT
    block_height,
    block_timestamp,
    block_hash,
    transaction_hash,
    transaction_index,
    transaction_version,
    JSONExtractString(changes[ci], 'type') AS type,
    toUInt64(ci - 1) AS change_index,
    CAST(nullIf(change_addresses[ci], ''), 'Nullable(FixedString(66))') AS address,
    if(startsWith(JSONExtractString(changes[ci], 'type'), 'delete'),
       JSONExtractString(changes[ci], 'module'), JSONExtractString(changes[ci], 'data', 'abi', 'name')) AS module_name,
    if(startsWith(JSONExtractString(changes[ci], 'type'), 'delete'),
       '', JSONExtractString(changes[ci], 'data', 'bytecode')) AS module_bytecode,
    if(startsWith(JSONExtractString(changes[ci], 'type'), 'delete'),
       '', JSONExtractRaw(changes[ci], 'data', 'abi')) AS abi,
    CAST(startsWith(JSONExtractString(changes[ci], 'type'), 'delete'), 'Bool') AS is_delete,
    change_addresses AS _addresses
FROM %s
ARRAY JOIN arrayEnumerate(changes) AS ci
WHERE JSONExtractString(changes[ci], 'type') IN ('write_module', 'delete_module')`,
			ctrl.FullLogicName(tableNameTransactions)),
		Comment: "module changes extracted from the embedded changes of the transactions table",
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
		// change records are served by the changes/resources/modules/table_items views over
		// transactions, only the per-transaction counters are still maintained here
		for _, wc := range aptos.GetTransactionChanges(tx) {
			switch wc.Type {
			case api.WriteSetChangeVariantWriteModule, api.WriteSetChangeVariantDeleteModule:
				transaction.ModuleChangesCount++
			case api.WriteSetChangeVariantWriteResource, api.WriteSetChangeVariantDeleteResource:
				transaction.ResourceChangesCount++
			case api.WriteSetChangeVariantWriteTableItem, api.WriteSetChangeVariantDeleteTableItem:
				transaction.TableItemChangesCount++
			}
		}
		transactions = append(transactions, transaction)
	}
	return
}

func (m *ClickhouseSchemaMgr) Convert(_ context.Context, slot *aptos.Slot) (clickhouse.Chunk, error) {
	block, transactions, events, err := m.convert(slot)
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
	counts := []int{1, len(transactions), len(events)}
	return clickhouse.Chunk{RowNum: counts, RowData: values}, nil
}

func (m *ClickhouseSchemaMgr) ConvertConcurrency() uint {
	return m.convertConcurrency
}

func (m *ClickhouseSchemaMgr) Done(r rg.Range) error {
	return nil
}
