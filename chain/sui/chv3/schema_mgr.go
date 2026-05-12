package chv3

import (
	"context"
	"fmt"
	"sentioxyz/sentio-core/chain/clickhouse"
	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/utils"
)

type ClickhouseSchemaMgrV3 struct {
	tablesMeta clickhouse.TablesMeta
	SlotConverter
}

const (
	TransactionsTableIdx   = 0
	ObjectChangeTableIdx   = 4
	ObjectPositionTableIdx = 5
)

func tableNames(database, tableNamePrefix string) []chx.FullName {
	return utils.MapSliceNoError([]string{
		"transactions",
		"events",
		"move_calls",
		"balance_changes",
		"object_changes",
		"object_positions",
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
	slotConverter SlotConverter,
	checkpointPartitionSize uint64,
) *ClickhouseSchemaMgrV3 {
	tablesName := tableNames(database, tableNamePrefix)
	engine := chx.NewDefaultMergeTreeEngine(cluster != "")
	tableSettings := make(map[string]string)
	chx.WithLightDeleteTableSettings(tableSettings)
	chx.WithProjectionTableSettings(tableSettings)
	partitionBy := fmt.Sprintf("intDiv(checkpoint, %d)", checkpointPartitionSize)
	createTableSchema := func(name chx.FullName, tblObj any, orderBy ...string) clickhouse.TableSchema {
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
		createTableSchema(tablesName[0], &CHUTransaction{}, "checkpoint", "checkpoint_timestamp_ms", "digest"),
		createTableSchema(tablesName[1], &CHUEvent{}, "checkpoint", "timestamp_ms", "digest"),
		createTableSchema(tablesName[2], &CHUMoveCall{}, "checkpoint", "timestamp_ms", "digest"),
		createTableSchema(tablesName[3], &CHUBalanceChange{}, "checkpoint", "timestamp_ms", "digest"),
		createTableSchema(tablesName[4], &CHUObjectChange{}, "checkpoint", "timestamp_ms", "digest"),
		createTableSchema(tablesName[5], &CHUObjectPosition{}, "object_id", "object_version", "checkpoint"),
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
