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

func (f *fakeConn) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	return nil, errors.New("not supported")
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
