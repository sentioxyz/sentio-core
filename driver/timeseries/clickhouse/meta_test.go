// file: driver/timeseries/clickhouse/meta_hash_test.go
package clickhouse

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sentioxyz/sentio-core/common/chx"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/period"
	"sentioxyz/sentio-core/driver/timeseries"
)

type testMetaOption struct {
	extra  bool
	agg    bool
	nested bool
	token  bool
	array  bool

	nestedSchema map[string]timeseries.FieldType
}

func newMeta(name string, option testMetaOption) timeseries.Meta {
	fields := map[string]timeseries.Field{
		"timestamp":    {Name: "timestamp", Type: timeseries.FieldTypeTime, Role: timeseries.FieldRoleTimestamp},
		"chain_id":     {Name: "chain_id", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleChainID},
		"block_number": {Name: "block_number", Type: timeseries.FieldTypeInt, Role: timeseries.FieldRoleSlotNumber},
		"value":        {Name: "value", Type: timeseries.FieldTypeInt, Role: timeseries.FieldRoleSeriesValue},
	}
	if option.extra {
		fields["token"] = timeseries.Field{Name: "token", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleSeriesLabel}
	}
	if option.nested {
		fields["nested_struct"] = timeseries.Field{
			Name:               "nested_struct",
			Type:               timeseries.FieldTypeJSON,
			NestedStructSchema: option.nestedSchema,
		}
	}
	if option.token {
		fields["token_price"] = timeseries.Field{Name: "token_price", Type: timeseries.FieldTypeToken}
	}
	if option.array {
		fields["array"] = timeseries.Field{Name: "array", Type: timeseries.FieldTypeArray}
	}
	m := timeseries.Meta{
		Name:   name,
		Type:   timeseries.MetaTypeGauge,
		Fields: fields,
	}
	if option.agg {
		m.Aggregation = &timeseries.Aggregation{
			Source: name,
			Intervals: []period.Period{
				period.Day,
			},
			Fields: map[string]timeseries.AggregationField{
				"value": {Name: "value", Function: "sum", Expression: "value"},
			},
		}
	}
	m.HashData = m.CalculateHash()
	return m
}

func newDataset(name string, option testMetaOption) timeseries.Dataset {
	return timeseries.Dataset{
		Meta: newMeta(name, option),
	}
}

func newTestStore(conn ckhmanager.Conn) *Store {
	opts := []chx.Option{
		chx.WithTableNamePrefix("proc_"),
		chx.WithLogicTableNamePrefix("proc_"),
	}
	if conn == nil {
		opts = append(opts,
			chx.WithDatabase("testdb"),
			chx.WithLogicDatabase("testdb"),
		)
	}
	ctrl := chx.New(conn, opts...)
	return NewStore(ctrl, Option{}, nil)
}

func TestCalculateMetasHash_BasicStability(t *testing.T) {
	store := newTestStore(nil)

	// Initial (empty) hash
	h1 := store.Meta().GetHash()
	require.NotEmpty(t, h1)

	// Recompute without change
	h2 := store.Meta().GetHash()
	require.Equal(t, h1, h2)
}

func TestCalculateMetasHash_OrderIndependence(t *testing.T) {
	metaA := newMeta("alpha", testMetaOption{})
	tableA := chx.Table{Name: metaA.GetTableName()}
	metaB := newMeta("beta", testMetaOption{})
	tableB := chx.Table{Name: metaB.GetTableName()}

	// Store A insertion order A then B
	a := newTestStore(nil)
	a.meta = newStoreMeta([]metaAndTable{
		{meta: metaA, table: tableA},
		{meta: metaB, table: tableB},
	})
	hashA := a.Meta().GetHash()

	// Store B insertion order reversed
	b := newTestStore(nil)
	b.meta = newStoreMeta([]metaAndTable{
		{meta: metaB, table: tableB},
		{meta: metaA, table: tableA},
	})
	hashB := b.Meta().GetHash()

	require.Equal(t, hashA, hashB, "hash must be deterministic irrespective of map insertion order")
}

func TestCalculateMetasHash_FieldChangeAffectsHash(t *testing.T) {
	store := newTestStore(nil)
	store.meta = newStoreMeta([]metaAndTable{{
		meta: newMeta("alpha", testMetaOption{}),
	}})
	h1 := store.Meta().GetHash()

	// Add a new field
	store.meta = newStoreMeta([]metaAndTable{{
		meta: newMeta("alpha", testMetaOption{extra: true}),
	}})
	h2 := store.Meta().GetHash()

	require.NotEqual(t, h1, h2)
}

func TestCalculateMetasHash_AggregationAffectsHash(t *testing.T) {
	store := newTestStore(nil)
	store.meta = newStoreMeta([]metaAndTable{{
		meta: newMeta("alpha", testMetaOption{}),
	}})
	h1 := store.Meta().GetHash()

	// Add aggregation config
	metaWithAgg := newMeta("alpha", testMetaOption{agg: true})
	store.meta = newStoreMeta([]metaAndTable{{
		meta: metaWithAgg,
	}})
	h2 := store.Meta().GetHash()

	require.NotEqual(t, h1, h2)

	// Re-run without further change
	h3 := store.Meta().GetHash()
	require.Equal(t, h2, h3)
}

func TestMeta_BuildSQL(t *testing.T) {
	t.Skip("need to run manually")

	conn := ckhmanager.NewConn(localClickhouseDSN)
	store := newTestStore(conn)

	require.Nil(t, store.Init(context.Background()))
}
