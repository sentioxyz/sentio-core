// file: driver/timeseries/clickhouse/meta_hash_test.go
package clickhouse

import (
	"context"
	"testing"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/period"
	"sentioxyz/sentio-core/driver/timeseries"

	"github.com/stretchr/testify/require"
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

func TestCalculateMetasHash_BasicStability(t *testing.T) {
	store := NewStore(nil, "", "", "proc", Option{})

	// Initial (empty) hash
	h1 := store.Meta().GetHash()
	require.NotEmpty(t, h1)

	// Recompute without change
	h2 := store.Meta().GetHash()
	require.Equal(t, h1, h2)
}

func TestCalculateMetasHash_OrderIndependence(t *testing.T) {
	// Store A insertion order A then B
	a := NewStore(nil, "", "", "proc", Option{})
	a.meta.Metas = map[timeseries.MetaType]map[string]timeseries.Meta{
		timeseries.MetaTypeGauge: {
			"alpha": newMeta("alpha", testMetaOption{}),
			"beta":  newMeta("beta", testMetaOption{}),
		},
	}
	hashA := a.Meta().GetHash()

	// Store B insertion order reversed
	b := NewStore(nil, "", "", "proc", Option{})
	b.meta.Metas = map[timeseries.MetaType]map[string]timeseries.Meta{
		timeseries.MetaTypeGauge: {
			"beta":  newMeta("beta", testMetaOption{}),
			"alpha": newMeta("alpha", testMetaOption{}),
		},
	}
	hashB := b.Meta().GetHash()

	require.Equal(t, hashA, hashB, "hash must be deterministic irrespective of map insertion order")
}

func TestCalculateMetasHash_FieldChangeAffectsHash(t *testing.T) {
	store := NewStore(nil, "", "", "proc", Option{})
	store.meta.Metas = map[timeseries.MetaType]map[string]timeseries.Meta{
		timeseries.MetaTypeGauge: {
			"alpha": newMeta("alpha", testMetaOption{}),
		},
	}
	h1 := store.Meta().GetHash()

	// Add a new field
	store.meta.Metas[timeseries.MetaTypeGauge]["alpha"] = newMeta("alpha", testMetaOption{extra: true})
	h2 := store.Meta().GetHash()

	require.NotEqual(t, h1, h2)
}

func TestCalculateMetasHash_AggregationAffectsHash(t *testing.T) {
	store := NewStore(nil, "", "", "proc", Option{})
	store.meta.Metas = map[timeseries.MetaType]map[string]timeseries.Meta{
		timeseries.MetaTypeGauge: {
			"alpha": newMeta("alpha", testMetaOption{}),
		},
	}
	h1 := store.Meta().GetHash()

	// Add aggregation config
	metaWithAgg := newMeta("alpha", testMetaOption{agg: true})
	store.meta.Metas[timeseries.MetaTypeGauge]["alpha"] = metaWithAgg
	h2 := store.Meta().GetHash()

	require.NotEqual(t, h1, h2)

	// Re-run without further change
	h3 := store.Meta().GetHash()
	require.Equal(t, h2, h3)
}

func TestMeta_BuildSQL(t *testing.T) {
	t.Skip("need to run manually")

	conn := ckhmanager.NewConn(localClickhouseDSN)

	store := NewStore(conn, "", conn.GetDatabase(), "proc", Option{})
	require.Nil(t, store.Init(context.Background(), true))
}

func TestMeta_RWMeta(t *testing.T) {
	t.Skip("need to run manually")

	conn := ckhmanager.NewConn(localClickhouseDSN)

	ctx := context.Background()
	store := NewStore(conn, "", conn.GetDatabase(), "proc", Option{})
	store.meta.Metas = map[timeseries.MetaType]map[string]timeseries.Meta{}

	_ = conn.Exec(ctx, "DROP TABLE IF EXISTS proc_gauge_beta, proc_gauge_alpha, proc_gauge_sigma, proc_gauge_gamma, proc_gauge_zetta")

	require.Nil(t, store.syncMeta(ctx, newDataset("beta", testMetaOption{})))
	require.Nil(t, store.syncMeta(ctx, newDataset("alpha", testMetaOption{token: true, array: true})))
	require.Nil(t, store.syncMeta(ctx, newDataset("sigma", testMetaOption{extra: true})))
	loaded1, err := store.loadMeta(ctx)
	require.Nil(t, err)
	require.Equal(t, store.meta.GetHash(), loaded1.GetHash())

	require.Nil(t, store.syncMeta(ctx, newDataset("gamma", testMetaOption{extra: true})))
	loaded2, err := store.loadMeta(ctx)
	require.Nil(t, err)
	require.NotEqual(t, store.meta.GetHash(), loaded1.GetHash())
	require.Equal(t, store.meta.GetHash(), loaded2.GetHash())

	require.Nil(t, store.syncMeta(ctx, newDataset("zetta", testMetaOption{nested: true, nestedSchema: map[string]timeseries.FieldType{"nested": timeseries.FieldTypeString}})))
	loaded3, err := store.loadMeta(ctx)
	require.Nil(t, err)
	require.Equal(t, store.meta.GetHash(), loaded3.GetHash())

	require.Nil(t, store.syncMeta(ctx, newDataset("zetta", testMetaOption{
		nested: true,
		nestedSchema: map[string]timeseries.FieldType{
			"nested":      timeseries.FieldTypeString,
			"middle":      timeseries.FieldTypeJSON,
			"middle.leaf": timeseries.FieldTypeArray,
		}})))
	loaded4, err := store.loadMeta(ctx)
	require.Nil(t, err)
	require.Equal(t, store.meta.GetHash(), loaded4.GetHash())

	require.Nil(t, store.syncMeta(ctx, newDataset("zetta", testMetaOption{
		nested: true,
		nestedSchema: map[string]timeseries.FieldType{
			"nested_array": timeseries.FieldTypeArray,
		}})))
	loaded5, err := store.loadMeta(ctx)
	require.Nil(t, err)
	require.Equal(t, store.meta.GetHash(), loaded5.GetHash())

	m, ok := loaded5.Meta(timeseries.MetaTypeGauge, "zetta")
	require.True(t, ok)
	m.Fields["nested_struct"].NestedStructSchema["nested_array"] = timeseries.FieldTypeArray
	m.Fields["nested_struct"].NestedStructSchema["middle.leaf"] = timeseries.FieldTypeArray
	m.Fields["nested_struct"].NestedStructSchema["middle"] = timeseries.FieldTypeJSON
	m.Fields["nested_struct"].NestedStructSchema["nested"] = timeseries.FieldTypeString
}
