package clickhouse

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/chx"
	rg "sentioxyz/sentio-core/common/range"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serverTable is a table of the connected server.
type serverTable struct {
	engine     string
	engineFull string
	fields     []chx.Field
}

// fakeServer answers what a store asks at startup: the system tables of the connected server,
// where the facades or the old physical tables are, and of the replicas of the data clusters.
type fakeServer struct {
	mu sync.Mutex
	// tables of the connected server by name, prefix included
	tables map[string]serverTable
	// local tables by cluster, replica and name: engine
	locals map[string]map[string]map[string]string
	// queries and execs are every statement in the order issued
	queries []string
	execs   []string
}

func newFakeServer() *fakeServer {
	return &fakeServer{tables: map[string]serverTable{}, locals: map[string]map[string]map[string]string{}}
}

const (
	loadTablesSQL = "SELECT name, engine, engine_full, create_table_query, as_select, partition_key, sorting_key, comment " +
		"FROM system.tables WHERE database = ? AND name = ? AND is_temporary = 0"
	loadColumnsSQL = "SELECT table, name, type, default_expression, comment, compression_codec " +
		"FROM system.columns WHERE database = ? AND table = ? ORDER BY table, position"
	loadIndicesSQL = "SELECT table, name, type_full, expr, granularity " +
		"FROM system.data_skipping_indices WHERE database = ? AND table = ? ORDER BY table, name"
	loadProjectionsSQL = "SELECT table, name, query " +
		"FROM system.projections WHERE database = ? AND table = ? ORDER BY table, name"
)

var loadSQLs = []string{loadTablesSQL, loadColumnsSQL, loadIndicesSQL, loadProjectionsSQL}

func (s *fakeServer) addPhysical(name string, fields []chx.Field) {
	s.tables[name] = serverTable{
		engine:     "ReplicatedMergeTree",
		engineFull: "ReplicatedMergeTree('/clickhouse/tables/{shard}/" + name + "', '{replica}') ORDER BY number",
		fields:     fields,
	}
}

func (s *fakeServer) addFacade(name, cluster, localDatabase, localTable string, fields []chx.Field, hosts ...string) {
	s.tables[name] = serverTable{
		engine:     "Distributed",
		engineFull: "Distributed('" + cluster + "', '" + localDatabase + "', '" + localTable + "', rand())",
		fields:     fields,
	}
	if s.locals[cluster] == nil {
		s.locals[cluster] = map[string]map[string]string{}
	}
	for _, h := range hosts {
		if s.locals[cluster][h] == nil {
			s.locals[cluster][h] = map[string]string{}
		}
		s.locals[cluster][h][localTable] = "ReplicatedMergeTree"
	}
}

func clusterOf(sql string) string {
	_, rest, _ := strings.Cut(sql, "clusterAllReplicas('")
	name, _, _ := strings.Cut(rest, "'")
	return name
}

func (s *fakeServer) query(sql string, args []any) ([][]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries = append(s.queries, sql)
	var rows [][]any
	switch {
	case sql == loadTablesSQL:
		if t, has := s.tables[args[1].(string)]; has {
			rows = append(rows, []any{args[1], t.engine, t.engineFull, "", "", "", "number", ""})
		}
	case sql == loadColumnsSQL:
		for _, f := range s.tables[args[1].(string)].fields {
			rows = append(rows, []any{args[1], f.Name, f.Type.String(), "", "", ""})
		}
	case sql == loadIndicesSQL, sql == loadProjectionsSQL:
	case strings.HasPrefix(sql, "SELECT engine FROM system.tables"):
		if t, has := s.tables[args[1].(string)]; has {
			rows = append(rows, []any{t.engine})
		}
	case strings.HasPrefix(sql, "SELECT engine, engine_full FROM system.tables"):
		if t, has := s.tables[args[1].(string)]; has {
			rows = append(rows, []any{t.engine, t.engineFull})
		}
	case strings.HasPrefix(sql, "SELECT count() FROM system.clusters"):
		rows = append(rows, []any{uint64(len(s.locals[args[0].(string)]))})
	case strings.HasPrefix(sql, "SELECT hostName() FROM clusterAllReplicas"):
		for h := range s.locals[clusterOf(sql)] {
			rows = append(rows, []any{h})
		}
	case strings.HasPrefix(sql, "SELECT hostName(), engine FROM clusterAllReplicas"):
		for h, tables := range s.locals[clusterOf(sql)] {
			if engine, has := tables[args[1].(string)]; has && args[0] == "chain_db" {
				rows = append(rows, []any{h, engine})
			}
		}
	case strings.HasPrefix(sql, "SELECT left, right FROM "):
	case strings.HasPrefix(sql, "SELECT min(right), count() FROM "):
		rows = append(rows, []any{uint64(0), uint64(0)})
	default:
		return nil, errors.Errorf("unexpected query %s", sql)
	}
	return rows, nil
}

func (s *fakeServer) exec(sql string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.execs = append(s.execs, sql)
	for _, kind := range []string{"CREATE TABLE ", "CREATE VIEW "} {
		if !strings.HasPrefix(sql, kind) {
			continue
		}
		_, rest, _ := strings.Cut(sql, "`chain_db`.`")
		name, _, _ := strings.Cut(rest, "`")
		if _, has := s.tables[name]; has {
			return errors.Errorf("code: 57, message: Table chain_db.%s already exists", name)
		}
		s.tables[name] = serverTable{engine: "ReplicatedMergeTree"}
	}
	return nil
}

func (s *fakeServer) all(prefix string) []string {
	var r []string
	for _, q := range append(append([]string{}, s.queries...), s.execs...) {
		if strings.Contains(q, prefix) {
			r = append(r, q)
		}
	}
	return r
}

type fakeServerConn struct{ s *fakeServer }

func (c fakeServerConn) GetClickhouseConn() clickhouse.Conn { return nil }
func (c fakeServerConn) GetDatabase() string                { return "chain_db" }
func (c fakeServerConn) GetCluster() string                 { return "" }
func (c fakeServerConn) GetHost() string                    { return "" }
func (c fakeServerConn) GetPassword() string                { return "" }
func (c fakeServerConn) GetUsername() string                { return "" }
func (c fakeServerConn) GetSettings() clickhouse.Settings   { return nil }
func (c fakeServerConn) Close()                             {}
func (c fakeServerConn) Exec(_ context.Context, sql string, _ ...any) error {
	return c.s.exec(sql)
}
func (c fakeServerConn) Query(_ context.Context, sql string, args ...any) (driver.Rows, error) {
	rows, err := c.s.query(sql, args)
	if err != nil {
		return nil, err
	}
	return &fakeServerRows{rows: rows, at: -1}, nil
}
func (c fakeServerConn) QueryRow(context.Context, string, ...any) driver.Row { return nil }
func (c fakeServerConn) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	return nil, errors.New("not supported")
}

type fakeServerRows struct {
	rows [][]any
	at   int
}

func (r *fakeServerRows) Next() bool {
	r.at++
	return r.at < len(r.rows)
}

func (r *fakeServerRows) Scan(dest ...any) error {
	row := r.rows[r.at]
	if len(dest) != len(row) {
		return errors.Errorf("scan %d values from a row of %d", len(dest), len(row))
	}
	for i, d := range dest {
		v := reflect.ValueOf(d).Elem()
		v.Set(reflect.ValueOf(row[i]).Convert(v.Type()))
	}
	return nil
}

func (r *fakeServerRows) ScanStruct(any) error             { return errors.New("not supported") }
func (r *fakeServerRows) ColumnTypes() []driver.ColumnType { return nil }
func (r *fakeServerRows) Totals(...any) error              { return nil }
func (r *fakeServerRows) Columns() []string                { return nil }
func (r *fakeServerRows) Close() error                     { return nil }
func (r *fakeServerRows) Err() error                       { return nil }
func (r *fakeServerRows) HasData() bool                    { return len(r.rows) > 0 }

type testSchemaMgr struct{ meta TablesMeta }

func (m testSchemaMgr) GetTablesMeta() TablesMeta                         { return m.meta }
func (m testSchemaMgr) Convert(context.Context, *testSlot) (Chunk, error) { return Chunk{}, nil }
func (m testSchemaMgr) ConvertConcurrency() uint                          { return 1 }
func (m testSchemaMgr) Done(rg.Range) error                               { return nil }

type testRow struct {
	Number uint64 `clickhouse:"number"`
	Hash   string `clickhouse:"hash"`
}

var testRoute = chx.MaintenanceRoute{
	LocalDatabase:    "chain_db",
	LocalTableSuffix: "_local",
	DataCluster:      "data",
	ComputeCluster:   "compute",
}

// testCtrl is the controller of a store as the launcher builds it: one connection, and a copy of
// the controller with the route of the store, nil for none.
func testCtrl(t *testing.T, s *fakeServer, route *chx.MaintenanceRoute) chx.Controller {
	ctrl, err := chx.New(fakeServerConn{s}, chx.WithCluster("compute"), chx.WithTableNamePrefix("zircuit.")).
		WithMaintenanceRoute(route)
	require.NoError(t, err)
	return ctrl
}

func testMeta(ctrl chx.Controller, views ...chx.View) TablesMeta {
	meta := TablesMeta{LinkTableIndex: -1, BlockTableIndex: -1, Views: views}
	for _, name := range []string{"blocks", "logs"} {
		sch := BuildTable(name, &testRow{}, chx.TableConfig{
			Engine:  ctrl.NewDefaultMergeTreeEngine(),
			OrderBy: []string{"number"},
		}, "")
		sch.UniqueKeyExemption = "test"
		meta.Tables = append(meta.Tables, sch)
	}
	return meta
}

func newTestSlotStore(ctx context.Context, ctrl chx.Controller, meta TablesMeta) (*SimpleSlotStore[*testSlot], error) {
	return NewSimpleSlotStore[*testSlot](ctx, ctrl, testSchemaMgr{meta: meta}, 100, time.Second, 1, 1)
}

func newTestRangeStore(ctx context.Context, ctrl chx.Controller) (*RangeStore, error) {
	return NewRangeStore(ctx, ctrl, "range", time.Hour, 7, false)
}

func rangeFields(ctrl chx.Controller) []chx.Field {
	return BuildTable("range", &rangeObject{}, chx.TableConfig{Engine: ctrl.NewDefaultMergeTreeEngine()}, "").Table.Fields
}

// newStatements are the statements only the routed startup issues, which a store without a route
// must never issue before something has failed.
var newStatements = []string{"SELECT engine FROM", "SELECT engine, engine_full FROM", "clusterAllReplicas", "system.clusters"}

func assertLegacyOnly(t *testing.T, s *fakeServer) {
	for _, stmt := range newStatements {
		assert.Empty(t, s.all(stmt), stmt)
	}
}

// TestLegacyStartupExisting pins the startup of stores without a route over the old
// ReplicatedMergeTree tables: the tables are loaded and found in sync, as they always were, and
// nothing else is asked or done.
func TestLegacyStartupExisting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newFakeServer()
	ctrl := testCtrl(t, s, nil)
	meta := testMeta(ctrl)
	for _, table := range meta.Tables {
		s.addPhysical("zircuit."+table.Table.Name, table.Table.Fields)
	}
	s.addPhysical("zircuit.range", rangeFields(ctrl))

	_, err := newTestSlotStore(ctx, ctrl, meta)
	require.NoError(t, err)
	_, err = newTestRangeStore(ctx, ctrl)
	require.NoError(t, err)

	assert.Equal(t, append(append(append([]string{}, loadSQLs...), loadSQLs...), loadSQLs...), s.queries)
	assert.Empty(t, s.execs)
	assertLegacyOnly(t, s)
}

// TestLegacyStartupPlainMergeTree checks that stores without a route still start over tables that
// are not replicated: only a route needs Replicated*MergeTree, and its check is not run without one.
func TestLegacyStartupPlainMergeTree(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newFakeServer()
	ctrl := testCtrl(t, s, nil)
	meta := testMeta(ctrl)
	for _, table := range meta.Tables {
		s.addPhysical("zircuit."+table.Table.Name, table.Table.Fields)
	}
	s.addPhysical("zircuit.range", rangeFields(ctrl))
	for name, table := range s.tables {
		table.engine, table.engineFull = "MergeTree", "MergeTree ORDER BY number"
		s.tables[name] = table
	}

	_, err := newTestSlotStore(ctx, ctrl, meta)
	require.NoError(t, err)
	_, err = newTestRangeStore(ctx, ctrl)
	require.NoError(t, err)
	assert.Empty(t, s.execs)
	assertLegacyOnly(t, s)
}

// TestLegacyStartupMissing pins the startup of stores without a route over an empty database: the
// tables and the view are created, with the statements they always were.
func TestLegacyStartupMissing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newFakeServer()
	ctrl := testCtrl(t, s, nil)
	view := chx.View{Name: "recent", Select: "SELECT 1"}
	meta := testMeta(ctrl, view)

	_, err := newTestSlotStore(ctx, ctrl, meta)
	require.NoError(t, err)
	_, err = newTestRangeStore(ctx, ctrl)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"CREATE TABLE `chain_db`.`zircuit.blocks` ON CLUSTER 'compute' (`number` UInt64, `hash` String) " +
			"ENGINE = ReplicatedMergeTree('/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}','{replica}') ORDER BY (`number`)",
		"CREATE TABLE `chain_db`.`zircuit.logs` ON CLUSTER 'compute' (`number` UInt64, `hash` String) " +
			"ENGINE = ReplicatedMergeTree('/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}','{replica}') ORDER BY (`number`)",
		"CREATE OR REPLACE VIEW `chain_db`.`zircuit.recent` ON CLUSTER 'compute' AS (SELECT 1) COMMENT ''",
		"CREATE TABLE `chain_db`.`zircuit.range` ON CLUSTER 'compute' " +
			"(`left` UInt64, `right` UInt64, `create_at` DateTime64(3, 'UTC')) " +
			"ENGINE = ReplicatedMergeTree('/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}','{replica}') " +
			"PARTITION BY toYYYYMMDD(create_at) ORDER BY (`create_at`)",
	}, s.execs)
	assert.Len(t, s.queries, 4*len(loadSQLs))
	assertLegacyOnly(t, s)
}

// TestLegacyStartupDistributed checks that a store without a route over a Distributed facade
// fails as it always did, the CREATE finding the facade there, and now says why.
func TestLegacyStartupDistributed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newFakeServer()
	ctrl := testCtrl(t, s, nil)
	meta := testMeta(ctrl)
	s.addPhysical("zircuit.blocks", meta.Tables[0].Table.Fields)
	s.addFacade("zircuit.logs", "data", "chain_db", "zircuit.logs_local", meta.Tables[1].Table.Fields, "d1")
	s.addFacade("zircuit.range", "control", "chain_db", "zircuit.range_local", rangeFields(ctrl), "r1")

	_, err := newTestSlotStore(ctx, ctrl, meta)
	assert.ErrorContains(t, err, "create table logs failed: "+
		"`chain_db`.`zircuit.logs` is a Distributed table but its store has no maintenance route")
	assert.ErrorContains(t, err, "already exists")
	_, err = newTestRangeStore(ctx, ctrl)
	assert.ErrorContains(t, err, "`chain_db`.`zircuit.range` is a Distributed table but its store has no maintenance route")

	// The engine is only asked once a CREATE has failed, and nothing is changed.
	assert.Len(t, s.all("SELECT engine FROM system.tables"), 2)
	assert.Empty(t, s.all("ALTER"))
	assert.Empty(t, s.all("clusterAllReplicas"))
}

// TestRoutedStartup checks the startup of stores with a route, slot and range each with its own
// data cluster, over Distributed facades that match the route.
func TestRoutedStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newFakeServer()
	rangeRoute := testRoute
	rangeRoute.DataCluster = "control"
	slotCtrl, rangeCtrl := testCtrl(t, s, &testRoute), testCtrl(t, s, &rangeRoute)
	meta := testMeta(slotCtrl, chx.View{Name: "recent", Select: "SELECT 1"})
	for _, table := range meta.Tables {
		name := "zircuit." + table.Table.Name
		s.addFacade(name, "data", "chain_db", name+"_local", table.Table.Fields, "d1", "d2")
	}
	s.addFacade("zircuit.range", "control", "chain_db", "zircuit.range_local", rangeFields(rangeCtrl), "r1", "r2")
	s.addFacade("zircuit.recent", "data", "chain_db", "zircuit.recent_local", nil, "d1", "d2")

	_, err := newTestSlotStore(ctx, slotCtrl, meta)
	require.NoError(t, err)
	_, err = newTestRangeStore(ctx, rangeCtrl)
	require.NoError(t, err)

	assert.Empty(t, s.execs, "nothing is created or changed")
	assert.Empty(t, s.all(loadColumnsSQL), "the facades are not loaded or synced")
	// slot: 2 facades, 2 to reach the data cluster, 2 local tables, the view; range: 1 + 2 + 1
	assert.Len(t, s.queries, 7+4)
}

// TestRoutedStartupRefused checks that a store with a route fails at startup when the route and
// the tables do not match, before anything is created or changed.
func TestRoutedStartupRefused(t *testing.T) {
	for name, tc := range map[string]struct {
		setup func(s *fakeServer, fields []chx.Field, rangeFields []chx.Field)
		slot  string
		rng   string
	}{
		"facade forwards to another cluster": {
			setup: func(s *fakeServer, fields, rangeFields []chx.Field) {
				s.addFacade("zircuit.blocks", "data", "chain_db", "zircuit.blocks_local", fields, "d1")
				s.addFacade("zircuit.logs", "control", "chain_db", "zircuit.logs_local", fields, "d1")
				// the range facade forwards to the slot data cluster, its route to the control one
				s.addFacade("zircuit.range", "data", "chain_db", "zircuit.range_local", rangeFields, "d1")
			},
			slot: `Distributed table ` + "`chain_db`.`zircuit.logs`" + ` forwards to cluster "control" database "chain_db" ` +
				`table "zircuit.logs_local", but its maintenance route expects cluster "data" database "chain_db" ` +
				`table "zircuit.logs_local"`,
			rng: `forwards to cluster "data" database "chain_db" table "zircuit.range_local", ` +
				`but its maintenance route expects cluster "control"`,
		},
		"facade forwards to another database and table": {
			setup: func(s *fakeServer, fields, rangeFields []chx.Field) {
				s.addFacade("zircuit.blocks", "data", "storage", "zircuit.blocks_local", fields, "d1")
				s.addFacade("zircuit.logs", "data", "chain_db", "zircuit.logs_shard", fields, "d1")
				s.addFacade("zircuit.range", "control", "chain_db", "range_local", rangeFields, "r1")
			},
			slot: `forwards to cluster "data" database "storage"`,
			rng:  `table "range_local", but its maintenance route expects cluster "control" database "chain_db" table "zircuit.range_local"`,
		},
		"old physical tables": {
			setup: func(s *fakeServer, fields, rangeFields []chx.Field) {
				s.addPhysical("zircuit.blocks", fields)
				s.addPhysical("zircuit.logs", fields)
				s.addPhysical("zircuit.range", rangeFields)
			},
			slot: "table `chain_db`.`zircuit.blocks` is ReplicatedMergeTree, not Distributed, " +
				"but a maintenance route is configured for it",
			rng: "table `chain_db`.`zircuit.range` is ReplicatedMergeTree, not Distributed",
		},
		"local tables not replicated": {
			setup: func(s *fakeServer, fields, rangeFields []chx.Field) {
				s.addFacade("zircuit.blocks", "data", "chain_db", "zircuit.blocks_local", fields, "d1", "d2")
				s.addFacade("zircuit.logs", "data", "chain_db", "zircuit.logs_local", fields, "d1", "d2")
				s.locals["data"]["d1"]["zircuit.logs_local"] = "MergeTree"
				s.addFacade("zircuit.range", "control", "chain_db", "zircuit.range_local", rangeFields, "r1")
				s.locals["control"]["r1"]["zircuit.range_local"] = "MergeTree"
			},
			slot: "local table `chain_db`.`zircuit.logs_local` of logs is not of the Replicated*MergeTree family on [d1:MergeTree]",
			rng:  "local table `chain_db`.`zircuit.range_local` of range is not of the Replicated*MergeTree family on [r1:MergeTree]",
		},
		"no tables": {
			setup: func(*fakeServer, []chx.Field, []chx.Field) {},
			slot:  "table `chain_db`.`zircuit.blocks` does not exist, but a maintenance route is configured for it",
			rng:   "table `chain_db`.`zircuit.range` does not exist",
		},
		"local tables missing": {
			setup: func(s *fakeServer, fields, rangeFields []chx.Field) {
				s.addFacade("zircuit.blocks", "data", "chain_db", "zircuit.blocks_local", fields, "d1", "d2")
				s.addFacade("zircuit.logs", "data", "chain_db", "zircuit.logs_local", fields, "d1")
				s.locals["data"]["d2"]["zircuit.blocks_local"] = "Distributed"
				s.addFacade("zircuit.range", "control", "chain_db", "zircuit.range_local", rangeFields)
				s.locals["control"]["r1"] = map[string]string{}
			},
			slot: "local table `chain_db`.`zircuit.blocks_local` of blocks is not of the Replicated*MergeTree family on [d2:Distributed]",
			rng:  "local table `chain_db`.`zircuit.range_local` of range is missing on replicas [r1] of cluster control",
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := newFakeServer()
			rangeRoute := testRoute
			rangeRoute.DataCluster = "control"
			slotCtrl, rangeCtrl := testCtrl(t, s, &testRoute), testCtrl(t, s, &rangeRoute)
			meta := testMeta(slotCtrl)
			tc.setup(s, meta.Tables[0].Table.Fields, rangeFields(rangeCtrl))

			_, err := newTestSlotStore(ctx, slotCtrl, meta)
			assert.ErrorContains(t, err, tc.slot)
			_, err = newTestRangeStore(ctx, rangeCtrl)
			assert.ErrorContains(t, err, tc.rng)
			assert.Empty(t, s.execs, "nothing is created or changed")
		})
	}
}

// TestReadOnlyRangeStoreUnchecked checks that a read-only range store asks nothing at startup,
// with a route or without, whatever the table is.
func TestReadOnlyRangeStoreUnchecked(t *testing.T) {
	s := newFakeServer()
	s.addPhysical("zircuit.range", nil)
	assert.NotNil(t, NewReadOnlyRangeStore(testCtrl(t, s, &testRoute), "range"))
	assert.NotNil(t, NewReadOnlyRangeStore(testCtrl(t, s, nil), "range"))
	assert.Empty(t, s.queries)
	assert.Empty(t, s.execs)
}

// TestRangeStoreDataSQL checks that the range store reads and writes its rows the same way with a
// route or without: through the logic name, which with a route is the facade, never the local
// table, so the INSERT and SELECT of a routed store still go through the Distributed table.
func TestRangeStoreDataSQL(t *testing.T) {
	statements := func(route *chx.MaintenanceRoute, cluster string) []string {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s := newFakeServer()
		ctrl := testCtrl(t, s, route)
		if route == nil {
			s.addPhysical("zircuit.range", rangeFields(ctrl))
		} else {
			s.addFacade("zircuit.range", cluster, "chain_db", "zircuit.range_local", rangeFields(ctrl), "r1", "r2")
		}
		store, err := newTestRangeStore(ctx, ctrl)
		require.NoError(t, err)
		s.queries, s.execs = nil, nil

		r, err := store.Update(ctx, rg.RangeSetter(rg.NewRange(10, 20)))
		require.NoError(t, err)
		assert.Equal(t, rg.NewRange(10, 20), r)
		_, _, err = store.OldestRecordedEnd(ctx)
		require.NoError(t, err)
		return append(append([]string{}, s.queries...), s.execs...)
	}
	want := []string{
		"SELECT left, right FROM `chain_db`.`zircuit.range` ORDER BY create_at DESC LIMIT 1",
		"SELECT min(right), count() FROM `chain_db`.`zircuit.range`",
		"INSERT INTO `chain_db`.`zircuit.range` (left, right, create_at) VALUES(10, 20, NOW64())",
	}
	rangeRoute := testRoute
	rangeRoute.DataCluster = "control"
	assert.Equal(t, want, statements(nil, ""))
	assert.Equal(t, want, statements(&testRoute, "data"))
	assert.Equal(t, want, statements(&rangeRoute, "control"))
}
