package chv3

import (
	"context"
	"fmt"
	"sentioxyz/sentio-core/chain/clickhouse"
	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/common/chx"
)

type ClickhouseSchemaMgrV3 struct {
	tablesMeta clickhouse.TablesMeta
	SlotConverter
}

const (
	tableNameTransactions    = "transactions"
	tableNameEvents          = "events"
	tableNameMoveCalls       = "move_calls"
	tableNameBalanceChanges  = "balance_changes"
	tableNameObjectChanges   = "object_changes"
	tableNameObjectPositions = "object_positions"
)

func NewClickhouseSchemaMgr(
	ctrl chx.Controller,
	slotConverter SlotConverter,
	checkpointPartitionSize uint64,
) *ClickhouseSchemaMgrV3 {
	engine := ctrl.NewDefaultMergeTreeEngine()
	tableSettings := make(map[string]string)
	chx.WithLightDeleteTableSettings(tableSettings)
	chx.WithProjectionTableSettings(tableSettings)
	partitionBy := fmt.Sprintf("intDiv(checkpoint, %d)", checkpointPartitionSize)
	createTableSchema := func(name string, tblObj any, orderBy ...string) clickhouse.TableSchema {
		config := chx.TableConfig{
			Engine:      engine,
			PartitionBy: partitionBy,
			OrderBy:     orderBy,
			Settings:    tableSettings,
		}
		if obj, is := tblObj.(*CHUObjectPosition); is {
			config.PartitionBy = obj.PartitionBy()
		}
		return clickhouse.BuildTable(name, tblObj, config, "")
	}
	tables := []clickhouse.TableSchema{
		createTableSchema(tableNameTransactions, &CHUTransaction{}, "checkpoint", "checkpoint_timestamp_ms", "digest"),
		createTableSchema(tableNameEvents, &CHUEvent{}, "checkpoint", "timestamp_ms", "digest"),
		createTableSchema(tableNameMoveCalls, &CHUMoveCall{}, "checkpoint", "timestamp_ms", "digest"),
		createTableSchema(tableNameBalanceChanges, &CHUBalanceChange{}, "checkpoint", "timestamp_ms", "digest"),
		createTableSchema(tableNameObjectChanges, &CHUObjectChange{}, "checkpoint", "timestamp_ms", "digest"),
		createTableSchema(tableNameObjectPositions, &CHUObjectPosition{}, "object_id", "object_version", "checkpoint"),
	}
	return &ClickhouseSchemaMgrV3{
		tablesMeta: clickhouse.TablesMeta{
			Tables:          tables,
			LinkTableIndex:  -1,
			BlockTableIndex: -1,
		},
		SlotConverter: slotConverter,
	}
}

func (m *ClickhouseSchemaMgrV3) Convert(ctx context.Context, slot *sui.Slot) (clickhouse.Chunk, error) {
	checkpoint, err := m.ConvertSlot(ctx, slot)
	if err != nil {
		return clickhouse.Chunk{}, err
	}
	return checkpoint.Values(), nil
}

func (m *ClickhouseSchemaMgrV3) GetTablesMeta() clickhouse.TablesMeta {
	return m.tablesMeta
}
