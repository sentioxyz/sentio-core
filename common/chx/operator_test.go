package chx

import (
	"context"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHouseGateController is a controller wired the way the Sentio driver wires a network_v1
// processor behind HouseGate: tables are `phys`.`proc_0.<name>` in ClickHouse and
// `proc_0`.`<name>` in its statements.
func newHouseGateController(conn *fakeConn) Controller {
	return New(conn,
		WithDatabase("phys"),
		WithTableNamePrefix("proc_0."),
		WithLogicDatabase("proc_0"),
		WithLogicTableNamePrefix(""),
	)
}

// systemTableRows answers the metadata reads of load from one shape of system tables: the
// physical one HouseGate used to pass through, or the logical one it serves filtered to the
// caller's databases. Each answer only holds rows the scope of the query matches.
func systemTableRows(logical bool) func(sql string, args []any) ([][]any, error) {
	db, prefix := "phys", "proc_0."
	if logical {
		db, prefix = "proc_0", ""
	}
	return func(sql string, args []any) ([][]any, error) {
		switch {
		case strings.Contains(sql, "FROM system.tables"):
			return [][]any{
				{db, prefix + "events", "MergeTree", "MergeTree ORDER BY id SETTINGS index_granularity = 8192",
					"CREATE TABLE ...", "", "", "id", "events table"},
				{db, prefix + "events_mv", "MaterializedView", "",
					"CREATE MATERIALIZED VIEW " + db + ".`" + prefix + "events_mv` TO " + db + ".`" + prefix +
						"events` AS SELECT id FROM src", "SELECT id FROM src", "", "", ""},
			}, nil
		case strings.Contains(sql, "FROM system.columns"):
			return [][]any{
				{db, prefix + "events", "id", "UInt64", "", "", ""},
				{db, prefix + "events_mv", "id", "UInt64", "", "", ""},
			}, nil
		case strings.Contains(sql, "FROM system.data_skipping_indices"):
			return [][]any{{db, prefix + "events", "idx_id", "minmax", "id", uint64(1)}}, nil
		case strings.Contains(sql, "FROM system.projections"):
			return [][]any{{db, prefix + "events", "p_id", "SELECT id ORDER BY id"}}, nil
		}
		return nil, errors.Errorf("unexpected query %s", sql)
	}
}

// TestLoadReadsBothShapes pins that a controller behind HouseGate reads its tables whether
// HouseGate shows the system tables under the physical names or under the logical ones.
func TestLoadReadsBothShapes(t *testing.T) {
	for _, logical := range []bool{false, true} {
		conn := &fakeConn{onQuery: systemTableRows(logical)}
		c := newHouseGateController(conn)
		got, err := c.LoadAll(context.Background(), false)
		require.NoError(t, err, "logical=%v", logical)
		require.Len(t, got, 2, "logical=%v", logical)

		table, ok := got[0].(Table)
		require.True(t, ok, "logical=%v: %T", logical, got[0])
		assert.Equal(t, "events", table.Name)
		assert.Equal(t, "events table", table.Comment)
		require.Len(t, table.Fields, 1)
		assert.Equal(t, "id", table.Fields[0].Name)
		require.Len(t, table.Indexes, 1)
		assert.Equal(t, "idx_id", table.Indexes[0].Name)
		require.Len(t, table.Projections, 1)
		assert.Equal(t, "p_id", table.Projections[0].Name)

		view, ok := got[1].(MaterializedView)
		require.True(t, ok, "logical=%v: %T", logical, got[1])
		assert.Equal(t, "events_mv", view.Name)
		assert.Equal(t, "events", view.To, "logical=%v", logical)
		require.Len(t, view.Fields, 1)

		for i, sql := range conn.log {
			assert.Contains(t, sql, "((database = ? AND ", "query %d", i)
			assert.Equal(t, []any{"phys", "proc\\_0.%", "proc_0", "%"}, conn.args[i], "query %d: %s", i, sql)
		}
	}
}

// TestLoadOneScopesBothShapes pins the arguments of a one-table load behind HouseGate.
func TestLoadOneScopesBothShapes(t *testing.T) {
	conn := &fakeConn{onQuery: func(sql string, args []any) ([][]any, error) {
		return [][]any{{"proc_0", "events", "MergeTree", "events table"}}, nil
	}}
	c := newHouseGateController(conn)
	got, ok, err := c.LoadOne(context.Background(), "events", true)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, Table{Name: "events", Comment: "events table"}, got)
	require.Len(t, conn.log, 1)
	assert.Equal(t, "SELECT database, name, engine, comment FROM system.tables "+
		"WHERE ((database = ? AND name = ?) OR (database = ? AND name = ?)) AND is_temporary = 0", conn.log[0])
	assert.Equal(t, []any{"phys", "proc_0.events", "proc_0", "events"}, conn.args[0])
}

// TestLoadKeepsSingleShapeWhenNamesAgree pins that a controller whose statement names are its
// ClickHouse names (the Sentio platform) keeps reading the system tables in the one shape.
func TestLoadKeepsSingleShapeWhenNamesAgree(t *testing.T) {
	conn := &fakeConn{onQuery: func(sql string, args []any) ([][]any, error) {
		if strings.Contains(sql, "FROM system.tables") {
			return [][]any{{"chain_db", "proc_events", "MergeTree", "MergeTree ORDER BY id", "", "", "", "id", ""}}, nil
		}
		return nil, nil
	}}
	c := New(conn, WithTableNamePrefix("proc_"))
	got, err := c.LoadAll(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "events", got[0].(Table).Name)
	for i, sql := range conn.log {
		assert.NotContains(t, sql, " OR ", "query %d", i)
		assert.Equal(t, []any{"chain_db", "proc\\_%"}, conn.args[i], "query %d: %s", i, sql)
	}
}

// TestMaterializedViewTarget pins the TO table of a materialized view in either shape, and the
// refusal of one outside the controller's tables.
func TestMaterializedViewTarget(t *testing.T) {
	c := newHouseGateController(&fakeConn{})
	for raw, want := range map[string]string{
		"proc_0.`events`":        "events",
		"`proc_0`.`events`":      "events",
		"phys.`proc_0.events`":   "events",
		"`phys`.`proc_0.events`": "events",
	} {
		got, err := c.materializedViewTarget("mv", raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}
	for _, raw := range []string{"phys.`proc_1.events`", "other.`events`", "events"} {
		_, err := c.materializedViewTarget("mv", raw)
		assert.Error(t, err, raw)
	}
}

// TestWaitMutationsStopsOnRefusal pins that a refused poll of system.mutations fails the wait at
// once instead of polling until ctx expires.
func TestWaitMutationsStopsOnRefusal(t *testing.T) {
	conn := &fakeConn{onQuery: func(sql string, args []any) ([][]any, error) {
		return nil, &clickhouse.Exception{Code: 403, Message: "system table system.mutations is not accessible"}
	}}
	c := newHouseGateController(conn)
	err := c.waitMutations(context.Background(), "events", "delete", "%DELETE%WHERE%")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not accessible")
	require.Len(t, conn.log, 1)
	assert.Equal(t, []any{"phys", "proc_0.events", "proc_0", "events", "%DELETE%WHERE%"}, conn.args[0])
}

// TestWaitMutationsRetriesTransientErrors pins that an error that is not a refusal keeps the wait
// going.
func TestWaitMutationsRetriesTransientErrors(t *testing.T) {
	calls := 0
	conn := &fakeConn{onQuery: func(sql string, args []any) ([][]any, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("connection reset")
		}
		return [][]any{{uint64(0), ""}}, nil
	}}
	c := newHouseGateController(conn)
	waitMutationsPollInterval = 0
	defer func() { waitMutationsPollInterval = defaultWaitMutationsPollInterval }()
	require.NoError(t, c.waitMutations(context.Background(), "events", "delete", "%DELETE%WHERE%"))
	assert.Equal(t, 2, calls)
}

// TestPartsReadsScopeBothShapes pins the arguments of the per-table reads of system.parts.
func TestPartsReadsScopeBothShapes(t *testing.T) {
	conn := &fakeConn{}
	c := newHouseGateController(conn)
	_, err := c.ListPartitions(context.Background(), "events")
	require.NoError(t, err)
	require.Len(t, conn.log, 1)
	assert.Contains(t, conn.log[0], "WHERE ((database = ? AND table = ?) OR (database = ? AND table = ?)) AND active = 1")
	assert.Equal(t, []any{"phys", "proc_0.events", "proc_0", "events"}, conn.args[0])
}
