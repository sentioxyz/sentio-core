package chx

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
)

// fakeConn answers the statements of a controller from a script, recording every one of them.
type fakeConn struct {
	mu       sync.Mutex
	log      []string
	args     [][]any
	settings []clickhouse.Settings
	onExec   func(sql string) error
	onQuery  func(sql string, args []any) ([][]any, error)
}

func (f *fakeConn) record(ctx context.Context, sql string, args []any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, sql)
	f.args = append(f.args, args)
	f.settings = append(f.settings, ckhmanager.ContextSettings(ctx))
}

func (f *fakeConn) statements(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var r []string
	for _, s := range f.log {
		if strings.HasPrefix(s, prefix) {
			r = append(r, s)
		}
	}
	return r
}

func (f *fakeConn) GetClickhouseConn() clickhouse.Conn { return nil }
func (f *fakeConn) GetDatabase() string                { return "chain_db" }
func (f *fakeConn) GetCluster() string                 { return "" }
func (f *fakeConn) GetHost() string                    { return "" }
func (f *fakeConn) GetPassword() string                { return "" }
func (f *fakeConn) GetUsername() string                { return "" }
func (f *fakeConn) GetSettings() clickhouse.Settings   { return nil }
func (f *fakeConn) Close()                             {}

func (f *fakeConn) Exec(ctx context.Context, sql string, args ...any) error {
	f.record(ctx, sql, args)
	if f.onExec == nil {
		return nil
	}
	return f.onExec(sql)
}

func (f *fakeConn) Query(ctx context.Context, sql string, args ...any) (driver.Rows, error) {
	f.record(ctx, sql, args)
	if f.onQuery == nil {
		return &fakeRows{}, nil
	}
	rows, err := f.onQuery(sql, args)
	if err != nil {
		return nil, err
	}
	return &fakeRows{rows: rows, at: -1}, nil
}

func (f *fakeConn) QueryRow(context.Context, string, ...any) driver.Row { return nil }

// PrepareBatch records the INSERT with the settings it is prepared with, which is when
// clickhouse-go sends them, and the batch records its Send after it.
func (f *fakeConn) PrepareBatch(ctx context.Context, sql string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	f.record(ctx, sql, nil)
	return &fakeBatch{f: f}, nil
}

// fakeBatch is the part of driver.Batch BatchInsert uses.
type fakeBatch struct {
	driver.Batch
	f *fakeConn
}

func (b *fakeBatch) Append(...any) error { return nil }
func (b *fakeBatch) Flush() error        { return nil }
func (b *fakeBatch) Abort() error        { return nil }
func (b *fakeBatch) Send() error {
	b.f.record(context.Background(), "SEND", nil)
	return nil
}

type fakeRows struct {
	rows [][]any
	at   int
}

func (r *fakeRows) Next() bool {
	r.at++
	return r.at < len(r.rows)
}

func (r *fakeRows) Scan(dest ...any) error {
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

func (r *fakeRows) ScanStruct(any) error             { return errors.New("not supported") }
func (r *fakeRows) ColumnTypes() []driver.ColumnType { return nil }
func (r *fakeRows) Totals(...any) error              { return nil }
func (r *fakeRows) Columns() []string                { return nil }
func (r *fakeRows) Close() error                     { return nil }
func (r *fakeRows) Err() error                       { return nil }
func (r *fakeRows) HasData() bool                    { return len(r.rows) > 0 }

func init() {
	maintPollInterval = time.Millisecond
}

var testRoute = MaintenanceRoute{
	LocalDatabase:    "chain_db",
	LocalTableSuffix: "_local",
	DataCluster:      "data",
	ComputeCluster:   "compute",
}

func newTestController(t *testing.T, conn *fakeConn, route *MaintenanceRoute) Controller {
	c := New(conn, WithCluster("compute"), WithTableNamePrefix("zircuit."))
	c, err := c.WithMaintenanceRoute(route)
	require.NoError(t, err)
	return c
}

// TestLegacySQLUnchanged pins the statements of a controller without a route to the ones it has
// always issued.
func TestLegacySQLUnchanged(t *testing.T) {
	conn := &fakeConn{onQuery: func(sql string, args []any) ([][]any, error) {
		switch {
		case strings.HasPrefix(sql, "SELECT COUNT(*)"):
			return [][]any{{uint64(3)}}, nil
		case strings.HasPrefix(sql, "SELECT count(), max(latest_fail_reason)"):
			return [][]any{{uint64(0), ""}}, nil
		case strings.HasPrefix(sql, "SELECT count(), sum(data_uncompressed_bytes)"):
			return [][]any{{uint64(0), uint64(0)}}, nil
		}
		return nil, nil
	}}
	c := newTestController(t, conn, nil)
	ctx := context.Background()

	require.NoError(t, c.EndRangeInserts(ctx, "blocks", 10, 20))
	_, err := c.DeleteAfterReplicaSync(ctx, "blocks", "number >= 10", false)
	require.NoError(t, err)
	_, err = c.Delete(ctx, "logs", "number >= 10", true)
	require.NoError(t, err)
	_, err = c.ListPartitionsForMaintenance(ctx, "range")
	require.NoError(t, err)
	require.NoError(t, c.DropPartitionForMaintenance(ctx, "range", PartitionMeta{Partition: "20261018"}))

	assert.Equal(t, []string{
		"KILL QUERY ON CLUSTER 'compute' WHERE query_kind = 'Insert' " +
			"AND startsWith(query_id, 'rangeinsert:') AND endsWith(query_id, ':chain_db.zircuit.blocks') " +
			"AND toUInt64OrZero(splitByChar(':', query_id)[2]) <= 20 " +
			"AND toUInt64OrZero(splitByChar(':', query_id)[3]) >= 10 SYNC",
		"SELECT count(), max(latest_fail_reason) FROM system.mutations " +
			"WHERE database = ? AND table = ? AND NOT is_done AND command LIKE ?",
		"SYSTEM SYNC REPLICA ON CLUSTER 'compute' `chain_db`.`zircuit.blocks` LIGHTWEIGHT",
		"SELECT COUNT(*) FROM `chain_db`.`zircuit.blocks` WHERE number >= 10",
		"ALTER TABLE `chain_db`.`zircuit.blocks` DELETE WHERE number >= 10",
		"SELECT count(), max(latest_fail_reason) FROM system.mutations " +
			"WHERE database = ? AND table = ? AND NOT is_done AND command LIKE ?",
		"SELECT count(), sum(data_uncompressed_bytes) FROM system.parts " +
			"WHERE database = ? AND table = ? AND active AND startsWith(name, 'patch-')",
		"SELECT COUNT(*) FROM `chain_db`.`zircuit.logs` WHERE number >= 10",
		"DELETE FROM `chain_db`.`zircuit.logs` WHERE number >= 10",
		"SELECT partition,sum(rows),sum(bytes_on_disk),sum(data_compressed_bytes),sum(data_uncompressed_bytes) " +
			"FROM system.parts WHERE database = ? AND table = ? AND active = 1 GROUP BY partition ORDER BY partition",
		"ALTER TABLE `chain_db`.`zircuit.range` ON CLUSTER 'compute' DROP PARTITION ?",
	}, conn.log)
	assert.Equal(t, []any{"20261018"}, conn.args[len(conn.args)-1])
	for i, s := range conn.settings {
		assert.NotContains(t, s, "skip_unavailable_shards", conn.log[i])
		assert.NotContains(t, s, "use_query_cache", conn.log[i])
	}
}

func TestMaintenanceRouteValidation(t *testing.T) {
	for name, mutate := range map[string]func(r *MaintenanceRoute){
		"no local database":  func(r *MaintenanceRoute) { r.LocalDatabase = "" },
		"no suffix":          func(r *MaintenanceRoute) { r.LocalTableSuffix = "" },
		"no data cluster":    func(r *MaintenanceRoute) { r.DataCluster = "" },
		"no compute cluster": func(r *MaintenanceRoute) { r.ComputeCluster = "" },
		"quote in cluster":   func(r *MaintenanceRoute) { r.DataCluster = "data' OR 1" },
		"backtick in db":     func(r *MaintenanceRoute) { r.LocalDatabase = "db`x" },
		"dot in suffix":      func(r *MaintenanceRoute) { r.LocalTableSuffix = ".local" },
		"space in cluster":   func(r *MaintenanceRoute) { r.ComputeCluster = "compute cluster" },
	} {
		t.Run(name, func(t *testing.T) {
			r := testRoute
			mutate(&r)
			assert.Error(t, r.Validate())
			c := New(&fakeConn{}, WithTableNamePrefix("zircuit."))
			routed, err := c.WithMaintenanceRoute(&r)
			assert.Error(t, err)
			assert.Nil(t, routed.MaintenanceRoute())
		})
	}
	assert.NoError(t, testRoute.Validate())

	_, err := New(&fakeConn{}, WithTableNamePrefix("zircuit`.")).WithMaintenanceRoute(&testRoute)
	assert.Error(t, err)
	_, err = New(nil, WithTableNamePrefix("zircuit.")).WithMaintenanceRoute(&testRoute)
	assert.Error(t, err, "a controller without a database")
	_, err = New(&fakeConn{}, WithTableNamePrefix("zircuit."), WithLogicDatabase("other")).WithMaintenanceRoute(&testRoute)
	assert.Error(t, err, "logic database differs from the physical one")
	_, err = New(&fakeConn{}, WithTableNamePrefix("zircuit."), WithLogicTableNamePrefix("x.")).WithMaintenanceRoute(&testRoute)
	assert.Error(t, err, "logic prefix differs from the physical one")
}

// TestMaintenanceRouteCopies checks that a store can take its own route without changing the one
// of the controller it was copied from.
func TestMaintenanceRouteCopies(t *testing.T) {
	conn := &fakeConn{}
	base := New(conn, WithTableNamePrefix("zircuit."))
	slot, err := base.WithMaintenanceRoute(&testRoute)
	require.NoError(t, err)
	rangeRoute := *slot.MaintenanceRoute()
	rangeRoute.DataCluster = "control"
	rng, err := slot.WithMaintenanceRoute(&rangeRoute)
	require.NoError(t, err)

	assert.Nil(t, base.MaintenanceRoute())
	assert.Equal(t, "data", slot.MaintenanceRoute().DataCluster)
	assert.Equal(t, "control", rng.MaintenanceRoute().DataCluster)
	slot.MaintenanceRoute().DataCluster = "changed"
	assert.Equal(t, "data", slot.MaintenanceRoute().DataCluster, "the getter returns a copy")

	unrouted, err := slot.WithMaintenanceRoute(nil)
	require.NoError(t, err)
	assert.Nil(t, unrouted.MaintenanceRoute())

	ctx := context.Background()
	require.NoError(t, rng.DropPartitionForMaintenance(ctx, "range", PartitionMeta{PartitionID: "20261018"}))
	require.NoError(t, slot.DropPartitionForMaintenance(ctx, "blocks", PartitionMeta{PartitionID: "1"}))
	assert.Error(t, rng.DropPartitionForMaintenance(ctx, "range", PartitionMeta{PartitionID: "x'; DROP"}))
	assert.Equal(t, []string{
		"ALTER TABLE `chain_db`.`zircuit.range_local` ON CLUSTER 'control' DROP PARTITION ID '20261018'",
		"ALTER TABLE `chain_db`.`zircuit.blocks_local` ON CLUSTER 'data' DROP PARTITION ID '1'",
	}, conn.log)
}

// cluster simulates the replicas a routed controller reads the state of.
type cluster struct {
	mu sync.Mutex
	// hosts are the replicas of each cluster as system.clusters has them, answering are the ones
	// that answer clusterAllReplicas.
	hosts     map[string][]string
	answering map[string][]string
	// mutations by host
	mutations map[string][]mutationRow
	// count is what SELECT COUNT(*) over the facade returns, one per call; the last one sticks.
	counts        []uint64
	ddlUnfinished uint64
	submitting    uint64
	processes     uint64
	queued        uint64
	failObserve   error
	// onSubmit runs when the delete is submitted, and what it returns is the result of the submit.
	onSubmit func(c *cluster) error
	// onPoll runs before every read of system.mutations.
	onPoll func(c *cluster, poll int)
	polls  int
}

func newCluster() *cluster {
	data := []string{"d1", "d2", "d3", "d4"}
	return &cluster{
		hosts:     map[string][]string{"data": data, "compute": {"c1", "c2"}},
		answering: map[string][]string{"data": data, "compute": {"c1", "c2"}},
		mutations: map[string][]mutationRow{
			"d1": {{Host: "d1", ID: "0000000001", Command: "DELETE WHERE old", Done: true}},
		},
	}
}

func (cl *cluster) addMutationEverywhere(id, command string, done, killed bool) {
	for _, h := range cl.hosts["data"] {
		cl.mutations[h] = append(cl.mutations[h], mutationRow{Host: h, ID: id, Command: command, Done: done, Killed: killed})
	}
}

func clusterArg(sql string) string {
	_, rest, ok := strings.Cut(sql, "clusterAllReplicas('")
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(rest, "'")
	return name
}

func (cl *cluster) conn() *fakeConn {
	f := &fakeConn{}
	f.onExec = func(sql string) error {
		cl.mu.Lock()
		defer cl.mu.Unlock()
		if strings.HasPrefix(sql, "ALTER TABLE") && strings.Contains(sql, " DELETE WHERE ") && cl.onSubmit != nil {
			return cl.onSubmit(cl)
		}
		return nil
	}
	f.onQuery = func(sql string, args []any) ([][]any, error) {
		cl.mu.Lock()
		defer cl.mu.Unlock()
		switch {
		case strings.HasPrefix(sql, "SELECT count() FROM system.clusters"):
			return [][]any{{uint64(len(cl.hosts[args[0].(string)]))}}, nil
		case strings.HasPrefix(sql, "SELECT hostName() FROM clusterAllReplicas"):
			var rows [][]any
			for _, h := range cl.answering[clusterArg(sql)] {
				rows = append(rows, []any{h})
			}
			return rows, nil
		case strings.Contains(sql, "system.processes") && strings.Contains(sql, "query_kind = 'Insert'"):
			return [][]any{{cl.processes}}, nil
		case strings.Contains(sql, "system.processes"):
			return [][]any{{cl.submitting}}, nil
		case strings.Contains(sql, "system.distribution_queue"):
			return [][]any{{cl.queued}}, nil
		case strings.Contains(sql, "system.distributed_ddl_queue"):
			return [][]any{{cl.ddlUnfinished}}, nil
		case strings.Contains(sql, "system.mutations"):
			cl.polls++
			if cl.onPoll != nil {
				cl.onPoll(cl, cl.polls)
			}
			if cl.failObserve != nil {
				return nil, cl.failObserve
			}
			var rows [][]any
			for _, h := range cl.answering["data"] {
				for _, m := range cl.mutations[h] {
					var done, killed uint8
					if m.Done {
						done = 1
					}
					if m.Killed {
						killed = 1
					}
					rows = append(rows, []any{m.Host, m.ID, m.Command, done, killed, m.FailReason})
				}
			}
			return rows, nil
		case strings.HasPrefix(sql, "SELECT COUNT(*)"):
			n := cl.counts[0]
			if len(cl.counts) > 1 {
				cl.counts = cl.counts[1:]
			}
			return [][]any{{n}}, nil
		}
		return nil, errors.Errorf("unexpected query %s", sql)
	}
	return f
}

func TestRoutedEndRangeInserts(t *testing.T) {
	cl := newCluster()
	conn := cl.conn()
	c := newTestController(t, conn, &testRoute)
	require.NoError(t, c.EndRangeInserts(context.Background(), "blocks", 10, 20))

	kills := conn.statements("KILL QUERY")
	require.Len(t, kills, 2)
	filter := "WHERE query_kind = 'Insert' AND (" +
		"(startsWith(query_id, 'rangeinsert:') AND endsWith(query_id, ':chain_db.zircuit.blocks') " +
		"AND toUInt64OrZero(splitByChar(':', query_id)[2]) <= 20 " +
		"AND toUInt64OrZero(splitByChar(':', query_id)[3]) >= 10) OR " +
		"(startsWith(initial_query_id, 'rangeinsert:') AND endsWith(initial_query_id, ':chain_db.zircuit.blocks') " +
		"AND toUInt64OrZero(splitByChar(':', initial_query_id)[2]) <= 20 " +
		"AND toUInt64OrZero(splitByChar(':', initial_query_id)[3]) >= 10)) SYNC"
	assert.Equal(t, "KILL QUERY ON CLUSTER 'compute' "+filter, kills[0])
	assert.Equal(t, "KILL QUERY ON CLUSTER 'data' "+filter, kills[1])

	// An insert still listed after the kill is an error, not a drained range.
	cl.processes = 1
	assert.ErrorContains(t, c.EndRangeInserts(context.Background(), "blocks", 10, 20), "still running")

	// So is a cluster whose replicas do not all answer.
	cl.processes = 0
	cl.answering["data"] = []string{"d1", "d2", "d3"}
	assert.Error(t, c.EndRangeInserts(context.Background(), "blocks", 10, 20))

	// And a kill that fails.
	cl.answering["data"] = cl.hosts["data"]
	conn.onExec = func(string) error { return errors.New("timeout") }
	assert.ErrorContains(t, c.EndRangeInserts(context.Background(), "blocks", 10, 20), "timeout")
	assertRoutedSettings(t, conn)
}

func deleteSucceeds(cl *cluster) error {
	cl.addMutationEverywhere("0000000002", "DELETE WHERE number >= 10", true, false)
	return nil
}

func TestRoutedDeleteTargets(t *testing.T) {
	cl := newCluster()
	cl.counts = []uint64{3, 0}
	cl.onSubmit = deleteSucceeds
	conn := cl.conn()
	c := newTestController(t, conn, &testRoute)

	n, err := c.DeleteAfterReplicaSync(context.Background(), "blocks", "number >= 10", true)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), n)

	assert.Equal(t, []string{
		"SYSTEM FLUSH DISTRIBUTED `chain_db`.`zircuit.blocks` ON CLUSTER 'compute'",
		"SYSTEM SYNC REPLICA ON CLUSTER 'data' `chain_db`.`zircuit.blocks_local` LIGHTWEIGHT",
	}, conn.statements("SYSTEM"))
	assert.Equal(t, []string{
		"ALTER TABLE `chain_db`.`zircuit.blocks_local` ON CLUSTER 'data' DELETE WHERE number >= 10",
	}, conn.statements("ALTER"))
	assert.Equal(t, []string{
		"SELECT COUNT(*) FROM `chain_db`.`zircuit.blocks` WHERE number >= 10",
		"SELECT COUNT(*) FROM `chain_db`.`zircuit.blocks` WHERE number >= 10",
	}, conn.statements("SELECT COUNT(*)"), "counted through the facade before and after")
	assert.Empty(t, conn.statements("DELETE FROM"), "light is not honoured")
	assertRoutedSettings(t, conn)
}

// assertRoutedSettings checks that every read of a routed controller fails on a replica or shard
// that does not answer and bypasses the query cache, whatever the connection defaults to, and that
// every distributed DDL fails unless all hosts have run it.
func assertRoutedSettings(t *testing.T, conn *fakeConn) {
	t.Helper()
	conn.mu.Lock()
	defer conn.mu.Unlock()
	require.NotEmpty(t, conn.log)
	for i, sql := range conn.log {
		s := conn.settings[i]
		if strings.HasPrefix(sql, "SELECT") {
			assert.Equal(t, 0, s["skip_unavailable_shards"], sql)
			assert.Equal(t, 0, s["use_query_cache"], sql)
		} else if strings.Contains(sql, " ON CLUSTER ") {
			assert.Equal(t, "throw", s["distributed_ddl_output_mode"], sql)
			assert.Equal(t, maintDDLTaskTimeoutSeconds, s["distributed_ddl_task_timeout"], sql)
			assert.Equal(t, 0, s["skip_unavailable_shards"], sql)
			assert.Equal(t, 0, s["use_query_cache"], sql)
		}
	}
}

func TestRoutedDeleteNothingToDelete(t *testing.T) {
	cl := newCluster()
	cl.counts = []uint64{0}
	conn := cl.conn()
	c := newTestController(t, conn, &testRoute)
	n, err := c.Delete(context.Background(), "blocks", "number >= 10", false)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, conn.statements("ALTER"))
	assert.Empty(t, conn.statements("SYSTEM"), "a delete without sync does not sync")
}

func TestRoutedDeleteFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		setup   func(cl *cluster)
		wantErr string
		submits int
	}{
		"no mutation appears": {
			setup:   func(cl *cluster) { cl.onSubmit = func(*cluster) error { return nil } },
			wantErr: "outcome unknown",
			submits: 1,
		},
		"failed submit not applied": {
			setup: func(cl *cluster) {
				cl.onSubmit = func(*cluster) error { return errors.New("code: 159, timeout") }
			},
			wantErr: "not seen applied on any replica",
			submits: 1,
		},
		"applied on some replicas only": {
			setup: func(cl *cluster) {
				cl.onSubmit = func(cl *cluster) error {
					for _, h := range []string{"d1", "d2"} {
						cl.mutations[h] = append(cl.mutations[h],
							mutationRow{Host: h, ID: "0000000002", Command: "DELETE WHERE x", Done: true})
					}
					return errors.New("connection reset")
				}
			},
			wantErr: "[d3 d4]",
			submits: 1,
		},
		"ddl task cleaned before every replica ran it": {
			setup: func(cl *cluster) {
				cl.onSubmit = func(cl *cluster) error {
					cl.ddlUnfinished = 2
					for _, h := range []string{"d1", "d2"} {
						cl.mutations[h] = append(cl.mutations[h],
							mutationRow{Host: h, ID: "0000000002", Command: "DELETE WHERE x", Done: true})
					}
					return errors.New("code: 159, timeout")
				}
				cl.onPoll = func(cl *cluster, poll int) {
					if poll == 6 {
						cl.ddlUnfinished = 0
					}
				}
			},
			wantErr: "[d3 d4]",
			submits: 1,
		},
		"mutation cancelled and removed": {
			setup: func(cl *cluster) {
				cl.onSubmit = func(cl *cluster) error {
					cl.addMutationEverywhere("0000000002", "DELETE WHERE x", false, false)
					return nil
				}
				cl.onPoll = func(cl *cluster, poll int) {
					if poll == 6 {
						for h := range cl.mutations {
							cl.mutations[h] = cl.mutations[h][:len(cl.mutations[h])-1]
						}
					}
				}
			},
			wantErr: "outcome unknown",
			submits: 1,
		},
		"killed": {
			setup: func(cl *cluster) {
				cl.onSubmit = func(cl *cluster) error {
					cl.addMutationEverywhere("0000000002", "DELETE WHERE x", false, true)
					return nil
				}
			},
			wantErr: "was killed",
			submits: 1,
		},
		"keeps failing": {
			setup: func(cl *cluster) {
				cl.onSubmit = func(cl *cluster) error {
					cl.addMutationEverywhere("0000000002", "DELETE WHERE x", false, false)
					cl.mutations["d3"][len(cl.mutations["d3"])-1].FailReason = "Memory limit exceeded"
					return nil
				}
			},
			wantErr: "Memory limit exceeded",
			submits: 1,
		},
		"rows left": {
			setup: func(cl *cluster) {
				cl.counts = []uint64{3, 1}
				cl.onSubmit = deleteSucceeds
			},
			wantErr: "still in",
			submits: 1,
		},
		"replica unreachable before submit": {
			setup: func(cl *cluster) {
				cl.failObserve = errors.New("code: 279, all connection tries failed")
			},
			wantErr: "state of the cluster unknown",
			submits: 0,
		},
		"replica missing from cluster": {
			setup: func(cl *cluster) {
				cl.answering["data"] = []string{"d1", "d2", "d3"}
			},
			wantErr: "state of the cluster unknown",
			submits: 0,
		},
		"replica unreachable after submit": {
			setup: func(cl *cluster) {
				cl.onSubmit = func(cl *cluster) error {
					cl.failObserve = errors.New("code: 210, connection refused")
					return nil
				}
			},
			wantErr: "state of the cluster unknown",
			submits: 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			cl := newCluster()
			cl.counts = []uint64{3, 0}
			tc.setup(cl)
			conn := cl.conn()
			c := newTestController(t, conn, &testRoute)
			_, err := c.Delete(context.Background(), "blocks", "number >= 10", false)
			assert.ErrorContains(t, err, tc.wantErr)
			assert.Len(t, conn.statements("ALTER"), tc.submits, "a delete is never submitted twice")
		})
	}
}

// TestRoutedDeleteRecovers checks that a submit returning an error is not taken as failed when the
// delete is then seen through on every replica.
func TestRoutedDeleteRecovers(t *testing.T) {
	cl := newCluster()
	cl.counts = []uint64{3, 0}
	cl.onSubmit = func(cl *cluster) error {
		cl.ddlUnfinished = 4
		cl.submitting = 1
		return errors.New("code: 159, watching task is executing longer than distributed_ddl_task_timeout")
	}
	cl.onPoll = func(cl *cluster, poll int) {
		if poll == 5 {
			cl.submitting = 0
			cl.ddlUnfinished = 0
			cl.addMutationEverywhere("0000000002", "DELETE WHERE number >= 10", true, false)
		}
	}
	conn := cl.conn()
	c := newTestController(t, conn, &testRoute)
	n, err := c.Delete(context.Background(), "blocks", "number >= 10", false)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), n)
	assert.Len(t, conn.statements("ALTER"), 1)
}

// TestRoutedDeleteWaitsForOthers checks that a delete waits for every unfinished mutation and DDL
// task on the table first, including ones it did not start.
func TestRoutedDeleteWaitsForOthers(t *testing.T) {
	cl := newCluster()
	cl.counts = []uint64{3, 0}
	cl.ddlUnfinished = 1
	cl.mutations["d2"] = []mutationRow{{Host: "d2", ID: "0000000007", Command: "UPDATE x = 1 WHERE manual"}}
	var submittedAt int
	cl.onPoll = func(cl *cluster, poll int) {
		if poll == 3 {
			cl.ddlUnfinished = 0
		}
		if poll == 6 {
			cl.mutations["d2"][0].Done = true
		}
	}
	cl.onSubmit = func(cl *cluster) error {
		submittedAt = cl.polls
		return deleteSucceeds(cl)
	}
	c := newTestController(t, cl.conn(), &testRoute)
	_, err := c.Delete(context.Background(), "blocks", "number >= 10", false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, submittedAt, 6)
}

func TestRoutedDeleteCancel(t *testing.T) {
	cl := newCluster()
	cl.counts = []uint64{3}
	ctx, cancel := context.WithCancel(context.Background())
	cl.onSubmit = func(cl *cluster) error {
		cl.addMutationEverywhere("0000000002", "DELETE WHERE x", false, false)
		cancel()
		return nil
	}
	c := newTestController(t, cl.conn(), &testRoute)
	_, err := c.Delete(ctx, "blocks", "number >= 10", false)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRoutedSyncRefusesQueuedData(t *testing.T) {
	cl := newCluster()
	cl.counts = []uint64{3, 0}
	cl.queued = 1
	conn := cl.conn()
	c := newTestController(t, conn, &testRoute)
	_, err := c.DeleteAfterReplicaSync(context.Background(), "blocks", "number >= 10", false)
	assert.ErrorContains(t, err, "distributed queue")
	assert.Empty(t, conn.statements("ALTER"))
	assert.Empty(t, conn.statements("SYSTEM SYNC REPLICA"))
}

func TestJudgeDeleteSettled(t *testing.T) {
	hosts := []string{"a", "b"}
	old := mutationRow{Host: "a", ID: "1", Command: "DELETE WHERE old", Done: true}
	snapshot := newMutationSnapshot(maintObservation{Hosts: hosts, Mutations: []mutationRow{old}})
	newOn := func(done bool, hs ...string) []mutationRow {
		rows := []mutationRow{old}
		for _, h := range hs {
			rows = append(rows, mutationRow{Host: h, ID: "2", Command: "DELETE WHERE x", Done: done})
		}
		return rows
	}

	v := judgeDeleteSettled(snapshot, maintObservation{Hosts: hosts}, false)
	assert.False(t, v.Done, "an empty list is not done")
	assert.Equal(t, hosts, v.Missing)

	v = judgeDeleteSettled(snapshot, maintObservation{Hosts: hosts, Mutations: []mutationRow{old}}, false)
	assert.False(t, v.Done, "the mutations seen before the submit do not count")

	v = judgeDeleteSettled(snapshot, maintObservation{Hosts: hosts, Mutations: newOn(false, "a", "b")}, false)
	assert.False(t, v.Done)
	assert.Empty(t, v.Missing)

	v = judgeDeleteSettled(snapshot, maintObservation{Hosts: hosts, Mutations: newOn(true, "a")}, false)
	assert.False(t, v.Done)
	assert.Equal(t, []string{"b"}, v.Missing)

	v = judgeDeleteSettled(snapshot, maintObservation{Hosts: hosts, Mutations: newOn(true, "a", "b"), DDLUnfinished: 1}, false)
	assert.False(t, v.Done, "a DDL task left could still create a mutation")

	v = judgeDeleteSettled(snapshot, maintObservation{Hosts: hosts, Mutations: newOn(true, "a", "b"), Submitting: 1}, false)
	assert.False(t, v.Done, "the submit is still running")

	v = judgeDeleteSettled(snapshot, maintObservation{Hosts: hosts, Mutations: newOn(true, "a", "b")}, true)
	assert.True(t, v.Done, "seen through, whatever the submit returned")

	update := append(newOn(true, "a"), mutationRow{Host: "b", ID: "2", Command: "UPDATE x = 1 WHERE y", Done: true})
	v = judgeDeleteSettled(snapshot, maintObservation{Hosts: hosts, Mutations: update}, false)
	assert.Equal(t, []string{"b"}, v.Missing, "only a DELETE counts as the delete")
}

func TestMergeReplicaPartitions(t *testing.T) {
	part := func(host, partition string, rows uint64) replicaPart {
		return replicaPart{Host: host, Partition: partition, PartitionID: partition, Rows: rows, BytesOnDisk: rows * 10}
	}
	merged, err := mergeReplicaPartitions([]string{"r1", "r2"}, []replicaPart{
		part("r1", "20261017", 5), part("r1", "20261017", 2), part("r2", "20261017", 7),
		part("r1", "20261018", 1), part("r2", "20261018", 3),
		part("r2", "20261016", 4),
	})
	require.NoError(t, err)
	assert.Equal(t, []PartitionMeta{
		{Partition: "20261016", PartitionID: "20261016", Rows: 4, BytesOnDisk: 40, Replicas: 1},
		{Partition: "20261017", PartitionID: "20261017", Rows: 7, BytesOnDisk: 70, Replicas: 2},
		{Partition: "20261018", PartitionID: "20261018", Rows: 3, BytesOnDisk: 30, Replicas: 2},
	}, merged)

	_, err = mergeReplicaPartitions([]string{"r1"}, []replicaPart{part("r9", "20261017", 1)})
	assert.Error(t, err, "a host that is not a replica")
	bad := part("r2", "20261017", 1)
	bad.PartitionID = "other"
	_, err = mergeReplicaPartitions([]string{"r1", "r2"}, []replicaPart{part("r1", "20261017", 1), bad})
	assert.Error(t, err, "the same partition under two ids")
}

func TestRoutedListPartitions(t *testing.T) {
	conn := &fakeConn{onQuery: func(sql string, args []any) ([][]any, error) {
		switch {
		case strings.HasPrefix(sql, "SELECT count() FROM system.clusters"):
			return [][]any{{uint64(2)}}, nil
		case strings.HasPrefix(sql, "SELECT hostName() FROM clusterAllReplicas"):
			return [][]any{{"r1"}, {"r2"}}, nil
		case strings.Contains(sql, "system.parts"):
			return [][]any{
				{"r1", "20261017", "20261017", uint64(1), uint64(1), uint64(1), uint64(1)},
				{"r2", "20261017", "20261017", uint64(1), uint64(1), uint64(1), uint64(1)},
			}, nil
		}
		return nil, errors.Errorf("unexpected %s", sql)
	}}
	route := testRoute
	route.DataCluster = "control"
	c := newTestController(t, conn, &route)
	partitions, err := c.ListPartitionsForMaintenance(context.Background(), "range")
	require.NoError(t, err)
	require.Len(t, partitions, 1)
	assert.Equal(t, "SELECT hostName(), partition, partition_id, rows, bytes_on_disk, "+
		"data_compressed_bytes, data_uncompressed_bytes "+
		"FROM clusterAllReplicas('control', system.parts) WHERE database = ? AND table = ? AND active = 1",
		conn.log[len(conn.log)-1])
	assert.Equal(t, []any{"chain_db", "zircuit.range_local"}, conn.args[len(conn.args)-1])
	assertRoutedSettings(t, conn)
}

func TestQuote(t *testing.T) {
	assert.Equal(t, "`a\\`b\\\\c`", quoteIdent("a`b\\c"))
	assert.Equal(t, `'a\'b\\c'`, quoteString(`a'b\c`))
}

func TestDistributedTarget(t *testing.T) {
	for engineFull, want := range map[string][3]string{
		"Distributed('data', 'chain_db', 'zircuit.blocks_local', cityHash64(block_number))": {
			"data", "chain_db", "zircuit.blocks_local"},
		"Distributed(data, chain_db, `zircuit.blocks_local`)":                         {"data", "chain_db", "zircuit.blocks_local"},
		"Distributed(\"data\",'chain_db','t',rand()) SETTINGS fsync_after_insert = 0": {"data", "chain_db", "t"},
		// commas, brackets and quotes inside quotes and inside the sharding key are not arguments
		"Distributed('a,b', 'db(', 't''x', sipHash64(tuple(a, b), [1, 2]), 'policy')": {"a,b", "db(", "t'x"},
		"Distributed('a\\'b', `d\\`b`, 't\\\\x')":                                     {"a'b", "d`b", "t\\x"},
		"Distributed('{cluster}', 'chain_db', 't')":                                   {"{cluster}", "chain_db", "t"},
	} {
		got, err := distributedTarget(engineFull)
		if assert.NoError(t, err, engineFull) {
			assert.Equal(t, want, got, engineFull)
		}
	}
	for _, engineFull := range []string{
		"ReplicatedMergeTree('/clickhouse/tables/{shard}/t', '{replica}')",
		"Distributed",
		"Distributed('data', 'chain_db')",
		"Distributed('data', currentDatabase(), 't')",
		"Distributed('data', 'chain_db', 't' || '_local')",
		"Distributed('data', 'chain_db', db.t)",
		"Distributed('data', 'chain_db', 't'",
		"Distributed('data', 'chain_db, 't')",
		"Distributed('data', 'chain_db', 't', f(x])",
	} {
		_, err := distributedTarget(engineFull)
		assert.Error(t, err, engineFull)
	}
}

// routeSchema is what the system tables say about the facades and the local tables behind them.
type routeSchema struct {
	// facades by table name on the connected server: engine and engine_full
	facades map[string][2]string
	// locals by host of the data cluster and table name: engine
	locals map[string]map[string]string
	// answering are the replicas of the data cluster, as system.clusters has them and as they answer
	answering []string
}

func newRouteSchema() *routeSchema {
	s := &routeSchema{
		facades:   map[string][2]string{},
		locals:    map[string]map[string]string{},
		answering: []string{"d1", "d2"},
	}
	for _, table := range []string{"blocks", "logs"} {
		s.facades["zircuit."+table] = [2]string{"Distributed",
			"Distributed('data', 'chain_db', 'zircuit." + table + "_local', cityHash64(number))"}
	}
	for _, h := range s.answering {
		s.locals[h] = map[string]string{
			"zircuit.blocks_local": "ReplicatedMergeTree",
			"zircuit.logs_local":   "ReplicatedReplacingMergeTree",
		}
	}
	return s
}

func (s *routeSchema) conn() *fakeConn {
	return &fakeConn{
		onExec: func(sql string) error { return errors.Errorf("unexpected exec %s", sql) },
		onQuery: func(sql string, args []any) ([][]any, error) {
			switch {
			case strings.HasPrefix(sql, "SELECT engine, engine_full FROM system.tables"):
				if f, has := s.facades[args[1].(string)]; has {
					return [][]any{{f[0], f[1]}}, nil
				}
				return nil, nil
			case strings.HasPrefix(sql, "SELECT count() FROM system.clusters"):
				return [][]any{{uint64(2)}}, nil
			case strings.HasPrefix(sql, "SELECT hostName() FROM clusterAllReplicas('data', system.one)"):
				var rows [][]any
				for _, h := range s.answering {
					rows = append(rows, []any{h})
				}
				return rows, nil
			case strings.HasPrefix(sql, "SELECT hostName(), engine FROM clusterAllReplicas('data', system.tables)"):
				var rows [][]any
				for _, h := range s.answering {
					if engine, has := s.locals[h][args[1].(string)]; has && args[0] == "chain_db" {
						rows = append(rows, []any{h, engine})
					}
				}
				return rows, nil
			}
			return nil, errors.Errorf("unexpected query %s", sql)
		},
	}
}

func TestVerifyMaintenanceRoute(t *testing.T) {
	ctx := context.Background()
	s := newRouteSchema()
	conn := s.conn()
	c := newTestController(t, conn, &testRoute)
	require.NoError(t, c.VerifyMaintenanceRoute(ctx, "blocks", "logs"))
	assert.Empty(t, conn.statements("ALTER"))
	assert.Empty(t, conn.statements("CREATE"))
	// 2 facades, 2 to reach the data cluster, 2 local tables
	assert.Len(t, conn.log, 6)
	for i, sql := range conn.log {
		if strings.Contains(sql, "clusterAllReplicas") {
			assert.Equal(t, 0, conn.settings[i]["skip_unavailable_shards"], sql)
		}
	}

	err := newTestController(t, newRouteSchema().conn(), nil).VerifyMaintenanceRoute(ctx, "blocks")
	assert.ErrorContains(t, err, "no maintenance route")

	for name, tc := range map[string]struct {
		mutate func(s *routeSchema)
		route  func(r *MaintenanceRoute)
		msg    string
	}{
		"facade forwards to another cluster": {
			mutate: func(s *routeSchema) {
				s.facades["zircuit.logs"] = [2]string{"Distributed", "Distributed('control', 'chain_db', 'zircuit.logs_local')"}
			},
			msg: `forwards to cluster "control" database "chain_db" table "zircuit.logs_local", ` +
				`but its maintenance route expects cluster "data" database "chain_db" table "zircuit.logs_local"`,
		},
		"route names another data cluster": {
			route: func(r *MaintenanceRoute) { r.DataCluster = "control" },
			msg:   `forwards to cluster "data"`,
		},
		"route names another database": {
			route: func(r *MaintenanceRoute) { r.LocalDatabase = "storage_db" },
			msg:   `expects cluster "data" database "storage_db"`,
		},
		"route names another suffix": {
			route: func(r *MaintenanceRoute) { r.LocalTableSuffix = "_shard" },
			msg:   `table "zircuit.blocks_shard"`,
		},
		"old physical table": {
			mutate: func(s *routeSchema) {
				s.facades["zircuit.blocks"] = [2]string{"ReplicatedMergeTree",
					"ReplicatedMergeTree('/clickhouse/tables/{shard}/blocks', '{replica}') ORDER BY number"}
			},
			msg: "table `chain_db`.`zircuit.blocks` is ReplicatedMergeTree, not Distributed, " +
				"but a maintenance route is configured for it",
		},
		"facade missing": {
			mutate: func(s *routeSchema) { delete(s.facades, "zircuit.logs") },
			msg:    "table `chain_db`.`zircuit.logs` does not exist",
		},
		"facade argument an expression": {
			mutate: func(s *routeSchema) {
				s.facades["zircuit.logs"] = [2]string{"Distributed", "Distributed('data', currentDatabase(), 'zircuit.logs_local')"}
			},
			msg: "currentDatabase() is an expression",
		},
		"local table missing on a replica": {
			mutate: func(s *routeSchema) { delete(s.locals["d2"], "zircuit.logs_local") },
			msg:    "local table `chain_db`.`zircuit.logs_local` of logs is missing on replicas [d2] of cluster data",
		},
		"local table not MergeTree": {
			mutate: func(s *routeSchema) { s.locals["d1"]["zircuit.blocks_local"] = "Distributed" },
			msg:    "is not of the Replicated*MergeTree family on [d1:Distributed]",
		},
		"local table plain MergeTree": {
			mutate: func(s *routeSchema) { s.locals["d2"]["zircuit.logs_local"] = "MergeTree" },
			msg:    "local table `chain_db`.`zircuit.logs_local` of logs is not of the Replicated*MergeTree family on [d2:MergeTree]",
		},
		"local table plain ReplacingMergeTree": {
			mutate: func(s *routeSchema) { s.locals["d1"]["zircuit.logs_local"] = "ReplacingMergeTree" },
			msg:    "is not of the Replicated*MergeTree family on [d1:ReplacingMergeTree]",
		},
		"data replica not answering": {
			mutate: func(s *routeSchema) { s.answering = []string{"d1"} },
			msg:    "cluster data has 2 replicas but 1 answered",
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := newRouteSchema()
			if tc.mutate != nil {
				tc.mutate(s)
			}
			r := testRoute
			if tc.route != nil {
				tc.route(&r)
			}
			conn := s.conn()
			err := newTestController(t, conn, &r).VerifyMaintenanceRoute(ctx, "blocks", "logs")
			assert.ErrorContains(t, err, tc.msg)
			assert.Empty(t, conn.statements("ALTER"))
			assert.Empty(t, conn.statements("CREATE"))
		})
	}
}

// unsafeCallerCtx carries the settings a caller or a profile could hand a routed controller that
// would each break one of its guarantees, along with ones it has to leave alone, a deadline and a
// value of its own.
func unsafeCallerCtx(t *testing.T) (context.Context, any) {
	type key struct{}
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), key{}, "mine"), time.Hour)
	t.Cleanup(cancel)
	return ckhmanager.ContextMergeSettings(ctx, map[string]any{
		"distributed_foreground_insert": 0,
		"async_insert":                  1,
		"distributed_ddl_task_timeout":  0,
		"distributed_ddl_output_mode":   "none",
		"skip_unavailable_shards":       1,
		"use_query_cache":               1,
		"max_threads":                   4,
	}), key{}
}

// TestFacadeInsertCtx checks that the INSERT context of a routed controller is in the foreground
// and not asynchronous whatever the caller asked for, keeps everything else of the caller, and
// that a controller without a route returns the context of the caller as it is.
func TestFacadeInsertCtx(t *testing.T) {
	ctx, key := unsafeCallerCtx(t)
	deadline, _ := ctx.Deadline()

	legacy := newTestController(t, &fakeConn{}, nil).FacadeInsertCtx(ctx)
	assert.True(t, legacy == ctx, "a controller without a route returns ctx itself")

	routed := newTestController(t, &fakeConn{}, &testRoute).FacadeInsertCtx(ctx)
	s := ckhmanager.ContextSettings(routed)
	assert.Equal(t, 1, s["distributed_foreground_insert"])
	assert.Equal(t, 0, s["async_insert"])
	assert.Equal(t, 4, s["max_threads"])
	assert.Equal(t, "none", s["distributed_ddl_output_mode"], "only the INSERT settings are touched")
	assert.Equal(t, "mine", routed.Value(key))
	got, _ := routed.Deadline()
	assert.Equal(t, deadline, got)
	assert.Equal(t, 0, ckhmanager.ContextSettings(ctx)["distributed_foreground_insert"], "ctx itself is not changed")
}

// TestBatchInsertSettings checks the settings a BatchInsert is prepared with: a routed controller
// adds the foreground ones over the caller's, on the same INSERT that Send then ends; a controller
// without a route sends what it always sent, the caller's settings and the deduplication token.
func TestBatchInsertSettings(t *testing.T) {
	ctx, _ := unsafeCallerCtx(t)
	rows := [][]any{{uint64(1)}, {uint64(2)}, {uint64(3)}}
	insert := func(route *MaintenanceRoute) (*fakeConn, clickhouse.Settings) {
		conn := &fakeConn{}
		c := newTestController(t, conn, route)
		require.NoError(t, c.BatchInsert(ctx, "INSERT INTO `chain_db`.`zircuit.blocks` (`number`)", 2, NewGetter(rows,
			func(row []any) []any { return row })))
		require.Equal(t, []string{"INSERT INTO `chain_db`.`zircuit.blocks` (`number`)", "SEND"}, conn.log,
			"one INSERT, through the facade, ended by Send")
		return conn, conn.settings[0]
	}

	_, routed := insert(&testRoute)
	assert.Equal(t, 1, routed["distributed_foreground_insert"])
	assert.Equal(t, 0, routed["async_insert"])
	assert.Equal(t, 4, routed["max_threads"])
	assert.NotEmpty(t, routed["insert_deduplication_token"])

	_, legacy := insert(nil)
	want := ckhmanager.ContextSettings(ctx)
	want["insert_deduplication_token"] = legacy["insert_deduplication_token"]
	assert.Equal(t, want, legacy)
}

// TestRoutedDDLWaitsWhateverInherited checks that every distributed DDL of a routed controller
// waits for all hosts, bounded, failing otherwise, even when the caller's settings say not to wait
// or not to fail. The plain reads are held to their settings the same way.
func TestRoutedDDLWaitsWhateverInherited(t *testing.T) {
	ctx, _ := unsafeCallerCtx(t)
	cl := newCluster()
	cl.counts = []uint64{3, 0}
	cl.onSubmit = deleteSucceeds
	conn := cl.conn()
	c := newTestController(t, conn, &testRoute)
	require.NoError(t, c.EndRangeInserts(ctx, "blocks", 10, 20))
	_, err := c.DeleteAfterReplicaSync(ctx, "blocks", "number >= 10", false)
	require.NoError(t, err)
	require.NoError(t, c.DropPartitionForMaintenance(ctx, "blocks", PartitionMeta{Partition: "20260101", PartitionID: "20260101"}))

	var ddl []string
	for i, sql := range conn.log {
		if strings.Contains(sql, " ON CLUSTER ") {
			ddl = append(ddl, sql)
			assert.Equal(t, 4, conn.settings[i]["max_threads"], "the other settings of the caller are kept: %s", sql)
		}
	}
	// 2 kills, the flush and the sync, the delete, the partition drop
	assert.Len(t, ddl, 6)
	assertRoutedSettings(t, conn)
}

// TestRoutedSyncFailureStopsDelete checks that a replica sync that does not finish, such as one
// whose distributed DDL times out on a replica that is behind, fails the delete before it counts:
// a count made then could read zero over rows the replica has not fetched yet, and the delete
// would be skipped.
func TestRoutedSyncFailureStopsDelete(t *testing.T) {
	for name, failing := range map[string]string{
		"flush": "SYSTEM FLUSH DISTRIBUTED",
		"sync":  "SYSTEM SYNC REPLICA",
	} {
		t.Run(name, func(t *testing.T) {
			ctx, _ := unsafeCallerCtx(t)
			cl := newCluster()
			cl.counts = []uint64{0}
			conn := cl.conn()
			onExec := conn.onExec
			conn.onExec = func(sql string) error {
				if strings.HasPrefix(sql, failing) {
					return errors.New("code: 159, Distributed DDL task is not finished on 1 of 4 hosts")
				}
				return onExec(sql)
			}
			c := newTestController(t, conn, &testRoute)
			n, err := c.DeleteAfterReplicaSync(ctx, "blocks", "number >= 10", false)
			assert.ErrorContains(t, err, "is not finished on 1 of 4 hosts")
			assert.Zero(t, n)
			assert.Empty(t, conn.statements("SELECT COUNT(*)"), "nothing is counted")
			assert.Empty(t, conn.statements("ALTER"))
		})
	}
}
