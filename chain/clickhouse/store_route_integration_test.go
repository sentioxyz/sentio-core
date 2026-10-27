//go:build integration

package clickhouse

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/chx"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	rg "sentioxyz/sentio-core/common/range"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteIntegration runs routed stores against real servers: the topology of
// testdata/route_integration/fixture.sh, whose `up` prints the environment this reads. It is
// skipped without it, and only built with -tags integration.
//
// The connection of the stores carries, in its DSN, the settings that would each break a
// guarantee of the route if the route left them alone: background and asynchronous inserts, DDL
// that does not wait and does not fail, shards that may be skipped. The facades reach the replicas
// through a proxy that the test holds right before an INSERT goes out, which is how a shard that
// is slow or unreachable looks to the node taking the INSERT.
func TestRouteIntegration(t *testing.T) {
	env := map[string]string{}
	for _, k := range []string{"SFRO_COMPUTE", "SFRO_D1", "SFRO_D2", "SFRO_R1", "SFRO_STATE"} {
		if env[k] = os.Getenv(k); env[k] == "" {
			t.Skipf("%s not set, see testdata/route_integration/fixture.sh", k)
		}
	}
	it := &routeIT{t: t, state: env["SFRO_STATE"]}
	it.d1, it.d2, it.r1, it.c1 = it.open(env["SFRO_D1"]), it.open(env["SFRO_D2"]), it.open(env["SFRO_R1"]),
		it.open(env["SFRO_COMPUTE"])
	it.dsn = "clickhouse://" + env["SFRO_COMPUTE"] + "/chain_db?async_insert=1&distributed_foreground_insert=0" +
		"&distributed_ddl_task_timeout=0&distributed_ddl_output_mode=none&skip_unavailable_shards=1"
	it.exec(it.d1, "CREATE DATABASE IF NOT EXISTS chain_db ON CLUSTER 'data'")
	it.exec(it.r1, "CREATE DATABASE IF NOT EXISTS chain_db ON CLUSTER 'control'")
	it.exec(it.c1, "CREATE DATABASE IF NOT EXISTS chain_db")

	t.Run("save", func(t *testing.T) {
		for _, legacy := range []bool{true, false} {
			t.Run(fmt.Sprintf("old_settings=%v", legacy), func(t *testing.T) { it.save(t, legacy) })
		}
	})
	t.Run("rollback", func(t *testing.T) {
		for _, legacy := range []bool{true, false} {
			t.Run(fmt.Sprintf("old_settings=%v", legacy), func(t *testing.T) { it.rollback(t, legacy) })
		}
	})
	t.Run("rollback_range_fails", it.rollbackRangeFails)
	t.Run("ddl_barrier", it.ddlBarrier)
	t.Run("engine", it.engine)
}

type routeIT struct {
	t              *testing.T
	state, dsn     string
	c1, d1, d2, r1 driver.Conn
}

func (it *routeIT) open(addr string) driver.Conn {
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: []string{addr}})
	require.NoError(it.t, err)
	it.t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func (it *routeIT) exec(conn driver.Conn, sql string) {
	it.t.Helper()
	require.NoError(it.t, conn.Exec(context.Background(), sql), sql)
}

func (it *routeIT) count(conn driver.Conn, sql string) uint64 {
	it.t.Helper()
	var n uint64
	require.NoError(it.t, conn.QueryRow(context.Background(), sql).Scan(&n), sql)
	return n
}

// hold stops the proxy forwarding to the replica listening on port, until the returned release.
func (it *routeIT) hold(port int) (release func()) {
	file := filepath.Join(it.state, fmt.Sprintf("hold-%d", port))
	require.NoError(it.t, os.WriteFile(file, nil, 0o644))
	var once sync.Once
	release = func() { once.Do(func() { _ = os.Remove(file) }) }
	it.t.Cleanup(release)
	return release
}

const (
	portD1 = 39101
	portD2 = 39102
	portR1 = 39103
)

// tables creates the local tables, Replicated of engine, and the facades of a store whose table
// name prefix is prefix, dropping what an earlier run left.
func (it *routeIT) tables(prefix, engine string) {
	for _, table := range []string{"blocks", "logs"} {
		local := fmt.Sprintf("chain_db.`%s%s_local`", prefix, table)
		it.exec(it.c1, fmt.Sprintf("DROP TABLE IF EXISTS chain_db.`%s%s` SYNC", prefix, table))
		it.exec(it.d1, fmt.Sprintf("DROP TABLE IF EXISTS %s ON CLUSTER 'data' SYNC", local))
		engineFull := "MergeTree"
		if engine == "ReplicatedMergeTree" {
			engineFull = fmt.Sprintf("ReplicatedMergeTree('/sfro/%s%s/%d', '{replica}')", prefix, table, time.Now().UnixNano())
		}
		it.exec(it.d1, fmt.Sprintf("CREATE TABLE %s ON CLUSTER 'data' (`number` UInt64, `hash` String) ENGINE = %s ORDER BY number",
			local, engineFull))
		it.exec(it.c1, fmt.Sprintf("CREATE TABLE chain_db.`%s%s` (`number` UInt64, `hash` String) "+
			"ENGINE = Distributed('data', 'chain_db', '%s%s_local', rand())", prefix, table, prefix, table))
	}
	local := fmt.Sprintf("chain_db.`%srange_local`", prefix)
	it.exec(it.c1, fmt.Sprintf("DROP TABLE IF EXISTS chain_db.`%srange` SYNC", prefix))
	it.exec(it.r1, fmt.Sprintf("DROP TABLE IF EXISTS %s ON CLUSTER 'control' SYNC", local))
	columns := "(`left` UInt64, `right` UInt64, `create_at` DateTime64(3, 'UTC'))"
	it.exec(it.r1, fmt.Sprintf("CREATE TABLE %s ON CLUSTER 'control' %s ENGINE = ReplicatedMergeTree('/sfro/%srange/%d', '{replica}') "+
		"PARTITION BY toYYYYMMDD(create_at) ORDER BY create_at", local, columns, prefix, time.Now().UnixNano()))
	it.exec(it.c1, fmt.Sprintf("CREATE TABLE chain_db.`%srange` %s ENGINE = Distributed('control', 'chain_db', '%srange_local')",
		prefix, columns, prefix))
}

// faultConn is the connection of the stores: every statement goes to the server as it is, except
// that beforeInsert runs right before an INSERT is sent, and, with oldSettings, the INSERT goes
// with the settings the stores used before they set any, those of the DSN.
type faultConn struct {
	ckhmanager.Conn
	mu           sync.Mutex
	oldSettings  bool
	beforeInsert func(sql string)
}

func (f *faultConn) insert(ctx context.Context, sql string) context.Context {
	if !strings.HasPrefix(sql, "INSERT INTO") {
		return ctx
	}
	f.mu.Lock()
	hook := f.beforeInsert
	f.mu.Unlock()
	if hook != nil {
		hook(sql)
	}
	if f.oldSettings {
		return ckhmanager.ContextMergeSettings(ctx, map[string]any{"distributed_foreground_insert": 0, "async_insert": 1})
	}
	return ctx
}

func (f *faultConn) setBeforeInsert(hook func(sql string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.beforeInsert = hook
}

func (f *faultConn) Exec(ctx context.Context, sql string, args ...any) error {
	return f.Conn.Exec(f.insert(ctx, sql), sql, args...)
}

func (f *faultConn) PrepareBatch(ctx context.Context, sql string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return f.Conn.PrepareBatch(f.insert(ctx, sql), sql, opts...)
}

func (it *routeIT) dimension(t *testing.T, ctx context.Context, prefix string, conn *faultConn) *chain.SimpleDimension[*numSlot] {
	rangeRoute := testRoute
	rangeRoute.DataCluster = "control"
	newCtrl := func(route chx.MaintenanceRoute) chx.Controller {
		ctrl, err := chx.New(conn, chx.WithCluster("compute"), chx.WithTableNamePrefix(prefix)).WithMaintenanceRoute(&route)
		require.NoError(t, err)
		return ctrl
	}
	slotCtrl := newCtrl(testRoute)
	meta := TablesMeta{LinkTableIndex: -1, BlockTableIndex: -1}
	for _, name := range []string{"blocks", "logs"} {
		sch := BuildTable(name, &testRow{}, chx.TableConfig{Engine: slotCtrl.NewDefaultMergeTreeEngine(), OrderBy: []string{"number"}}, "")
		sch.NumberField, sch.UniqueKeyExemption = "number", "test"
		meta.Tables = append(meta.Tables, sch)
	}
	slotStore, err := NewSimpleSlotStore[*numSlot](ctx, slotCtrl, numSchemaMgr{meta: meta}, 100, time.Second, 1, 1)
	require.NoError(t, err)
	rangeStore, err := NewRangeStore(ctx, newCtrl(rangeRoute), "range", time.Hour, 7, false)
	require.NoError(t, err)
	return chain.NewSimpleDimension[*numSlot](rangeStore, slotStore)
}

func (it *routeIT) newConn() *faultConn {
	return &faultConn{Conn: ckhmanager.NewConn(it.dsn)}
}

// sample is what the replicas hold at one point.
type sample struct {
	at time.Duration
	// rows holds, per slot table, the most rows from first to last any data replica holds: the
	// rows a reader of the facade can be answered with, once written to one replica of the shard
	rows map[string]uint64
	// leastRows is the same but the least, which is 0 once any replica has deleted them
	leastRows map[string]uint64
	// last is the range last recorded, as the range store reads it
	last    rg.Range
	hasLast bool
}

func (it *routeIT) sample(prefix string, start time.Time, first, last uint64) sample {
	s := sample{at: time.Since(start), rows: map[string]uint64{}, leastRows: map[string]uint64{}}
	for _, table := range []string{"blocks", "logs"} {
		sql := fmt.Sprintf("SELECT count() FROM chain_db.`%s%s_local` WHERE number BETWEEN %d AND %d", prefix, table, first, last)
		a, b := it.count(it.d1, sql), it.count(it.d2, sql)
		s.rows[table], s.leastRows[table] = max(a, b), min(a, b)
	}
	rows, err := it.r1.Query(context.Background(),
		fmt.Sprintf("SELECT left, right FROM chain_db.`%srange_local` ORDER BY create_at DESC LIMIT 1", prefix))
	require.NoError(it.t, err)
	for rows.Next() {
		var l, r uint64
		require.NoError(it.t, rows.Scan(&l, &r))
		s.last, s.hasLast = rg.NewRange(l, r), true
	}
	require.NoError(it.t, rows.Close())
	return s
}

func (s sample) String() string {
	last := "none"
	if s.hasLast {
		last = fmt.Sprintf("%d..%d", s.last.Start, *s.last.End)
	}
	return fmt.Sprintf("%6.2fs range=%s rows(max)=%v rows(min)=%v", s.at.Seconds(), last, s.rows, s.leastRows)
}

// watch samples every 100ms until done is closed and the hold is over by a margin, so that what
// was sent on in the background while it lasted is seen arriving.
func (it *routeIT) watch(prefix string, first, last uint64, done <-chan struct{}) []sample {
	start := time.Now()
	var samples []sample
	finished := false
	for {
		samples = append(samples, it.sample(prefix, start, first, last))
		if finished && time.Since(start) >= holdFor+2*time.Second {
			return samples
		}
		select {
		case <-done:
			finished = true
		case <-time.After(100 * time.Millisecond):
		}
	}
}

const holdFor = 5 * time.Second

// save saves slots 0..5 with the shard of the slot tables held from right before their first
// INSERT for holdFor, and checks that the range covering them is never seen before the rows.
func (it *routeIT) save(t *testing.T, oldSettings bool) {
	prefix := fmt.Sprintf("sfro_save_%v.", oldSettings)
	it.tables(prefix, "ReplicatedMergeTree")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn := it.newConn()
	conn.oldSettings = oldSettings
	dim := it.dimension(t, ctx, prefix, conn)

	var releaseOnce sync.Once
	var release func()
	conn.setBeforeInsert(func(sql string) {
		if strings.Contains(sql, "range") {
			return
		}
		releaseOnce.Do(func() {
			r1, r2 := it.hold(portD1), it.hold(portD2)
			release = func() { r1(); r2() }
			time.AfterFunc(holdFor, release)
		})
	})
	done := make(chan struct{})
	var saveErr error
	var savedAt time.Duration
	start := time.Now()
	go func() {
		defer close(done)
		saveErr = dim.Save(ctx, rg.NewRange(0, 5), feedSlots(0, 5))
		savedAt = time.Since(start)
	}()
	samples := it.watch(prefix, 0, 5, done)
	require.NoError(t, saveErr)
	// Rows sent on in the background may still be on their way: wait for them so the next run
	// starts clean, and so the samples below end with everything written.
	require.Eventually(t, func() bool {
		s := it.sample(prefix, start, 0, 5)
		return s.rows["blocks"] == 6 && s.rows["logs"] == 6
	}, time.Minute, 200*time.Millisecond)

	var early []sample
	for _, s := range samples {
		if s.hasLast && *s.last.End == 5 && (s.rows["blocks"] < 6 || s.rows["logs"] < 6) {
			early = append(early, s)
		}
	}
	t.Logf("old settings %v: Save returned after %s", oldSettings, savedAt)
	logSamples(t, samples)
	if oldSettings {
		assert.NotEmpty(t, early, "the old settings record the range before the rows are written")
		assert.Less(t, savedAt, holdFor, "and return while the shard is still held")
	} else {
		assert.Empty(t, early, "the range is never visible before the rows it covers")
		assert.GreaterOrEqual(t, savedAt, holdFor-200*time.Millisecond, "Save waits for the held shard")
	}
}

// rollback saves 0..5, then rolls 3.. back with the range shard held from right before the INSERT
// shrinking the range, and checks that no row is deleted while the old range is still visible.
func (it *routeIT) rollback(t *testing.T, oldSettings bool) {
	prefix := fmt.Sprintf("sfro_rollback_%v.", oldSettings)
	it.tables(prefix, "ReplicatedMergeTree")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	conn := it.newConn()
	dim := it.dimension(t, ctx, prefix, conn)
	require.NoError(t, dim.Save(ctx, rg.NewRange(0, 5), feedSlots(0, 5)))
	require.Eventually(t, func() bool {
		s := it.sample(prefix, time.Now(), 3, 5)
		return s.leastRows["blocks"] == 3 && s.leastRows["logs"] == 3
	}, time.Minute, 100*time.Millisecond, "both replicas hold the rows before the rollback")

	conn.oldSettings = oldSettings
	var releaseOnce sync.Once
	conn.setBeforeInsert(func(sql string) {
		if strings.Contains(sql, "range") {
			releaseOnce.Do(func() { time.AfterFunc(holdFor, it.hold(portR1)) })
		}
	})
	done := make(chan struct{})
	var deleteErr error
	var deletedAt time.Duration
	start := time.Now()
	go func() {
		defer close(done)
		deleteErr = dim.Delete(ctx, rg.Range{Start: 3})
		deletedAt = time.Since(start)
	}()
	samples := it.watch(prefix, 3, 5, done)
	require.NoError(t, deleteErr)
	require.Eventually(t, func() bool {
		s := it.sample(prefix, start, 3, 5)
		return s.hasLast && *s.last.End == 2
	}, time.Minute, 200*time.Millisecond)

	var early []sample
	for _, s := range samples {
		if (s.leastRows["blocks"] < 3 || s.leastRows["logs"] < 3) && s.hasLast && *s.last.End >= 3 {
			early = append(early, s)
		}
	}
	t.Logf("old settings %v: Delete returned after %s", oldSettings, deletedAt)
	logSamples(t, samples)
	final := samples[len(samples)-1]
	assert.Zero(t, final.rows["blocks"]+final.rows["logs"], "the rows rolled back are gone from every replica")
	if oldSettings {
		assert.NotEmpty(t, early, "the old settings delete rows while the old range is still visible")
	} else {
		assert.Empty(t, early, "no row is deleted before the range shrunk over it is visible")
		assert.GreaterOrEqual(t, deletedAt, holdFor-200*time.Millisecond, "Delete waits for the held range shard")
	}
}

// rollbackRangeFails rolls back while the range shard stays held until the call gives up, and
// checks that nothing is deleted.
func (it *routeIT) rollbackRangeFails(t *testing.T) {
	prefix := "sfro_rollback_fails."
	it.tables(prefix, "ReplicatedMergeTree")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	conn := it.newConn()
	dim := it.dimension(t, ctx, prefix, conn)
	require.NoError(t, dim.Save(ctx, rg.NewRange(0, 5), feedSlots(0, 5)))
	require.Eventually(t, func() bool {
		return it.sample(prefix, time.Now(), 0, 5).leastRows["blocks"] == 6
	}, time.Minute, 100*time.Millisecond)
	mutations := fmt.Sprintf("SELECT count() FROM clusterAllReplicas('data', system.mutations) "+
		"WHERE database = 'chain_db' AND table LIKE '%s%%'", prefix)
	before := it.count(it.d1, mutations)

	var release func()
	conn.setBeforeInsert(func(sql string) {
		if strings.Contains(sql, "range") && release == nil {
			release = it.hold(portR1)
		}
	})
	deleteCtx, deleteCancel := context.WithTimeout(ctx, holdFor)
	defer deleteCancel()
	err := dim.Delete(deleteCtx, rg.Range{Start: 3})
	t.Logf("Delete with the range shard held: %v", err)
	assert.Error(t, err)
	s := it.sample(prefix, time.Now(), 3, 5)
	assert.Equal(t, uint64(3), s.leastRows["blocks"], "nothing deleted: %s", s)
	assert.Equal(t, uint64(3), s.leastRows["logs"], "nothing deleted: %s", s)
	assert.Equal(t, before, it.count(it.d1, mutations), "no delete submitted")
	release()
}

// ddlBarrier checks the replica sync of a routed delete over a connection whose DSN says not to
// wait for distributed DDL: with the fetches of one replica stopped, the same SYNC REPLICA
// returns at once under the settings of the DSN, but blocks the delete until the replica has the
// rows, which are then counted and deleted; and a replica that never catches up fails the delete,
// which counts nothing and deletes nothing.
func (it *routeIT) ddlBarrier(t *testing.T) {
	prefix := "sfro_ddl."
	it.tables(prefix, "ReplicatedMergeTree")
	ctx := context.Background()
	conn := it.newConn()
	ctrl, err := chx.New(conn, chx.WithCluster("compute"), chx.WithTableNamePrefix(prefix)).WithMaintenanceRoute(&testRoute)
	require.NoError(t, err)
	local := fmt.Sprintf("chain_db.`%sblocks_local`", prefix)
	syncSQL := fmt.Sprintf("SYSTEM SYNC REPLICA ON CLUSTER 'data' %s LIGHTWEIGHT", local)
	onD2 := fmt.Sprintf("SELECT count() FROM %s WHERE number BETWEEN 100 AND 299", local)

	it.exec(it.d2, "SYSTEM STOP FETCHES "+local)
	it.exec(it.d1, fmt.Sprintf("INSERT INTO %s SELECT number + 100, 'x' FROM numbers(3)", local))

	// The settings of the DSN: the statement is queued and returns, the replica still has nothing.
	start := time.Now()
	require.NoError(t, conn.Exec(ctx, syncSQL))
	t.Logf("SYNC REPLICA with the DSN settings returned after %s, d2 holds %d rows", time.Since(start), it.count(it.d2, onD2))
	assert.Less(t, time.Since(start), 3*time.Second)
	assert.Zero(t, it.count(it.d2, onD2))

	// The route: the delete waits for the sync, which waits for the fetches.
	time.AfterFunc(holdFor, func() { _ = it.d2.Exec(context.Background(), "SYSTEM START FETCHES "+local) })
	start = time.Now()
	n, err := ctrl.DeleteAfterReplicaSync(ctx, "blocks", "number >= 100 AND number <= 199", false)
	t.Logf("routed delete returned after %s: %d rows, %v", time.Since(start), n, err)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(start), holdFor-200*time.Millisecond)
	assert.Equal(t, uint64(3), n)
	assert.Zero(t, it.count(it.d1, onD2)+it.count(it.d2, onD2))

	// A replica that does not catch up fails the delete, and nothing is counted or deleted.
	it.exec(it.d2, "SYSTEM STOP FETCHES "+local)
	it.exec(it.d1, fmt.Sprintf("INSERT INTO %s SELECT number + 200, 'x' FROM numbers(3)", local))
	mutations := fmt.Sprintf("SELECT count() FROM clusterAllReplicas('data', system.mutations) "+
		"WHERE database = 'chain_db' AND table = '%sblocks_local'", prefix)
	before := it.count(it.d1, mutations)
	start = time.Now()
	n, err = ctrl.DeleteAfterReplicaSync(ctx, "blocks", "number >= 200 AND number <= 299", false)
	t.Logf("routed delete with d2 never catching up returned after %s: %d rows, %v", time.Since(start), n, err)
	assert.Error(t, err)
	assert.Zero(t, n)
	assert.Equal(t, before, it.count(it.d1, mutations), "no delete submitted")
	assert.Equal(t, uint64(3), it.count(it.d1, onD2))
	it.exec(it.d2, "SYSTEM START FETCHES "+local)
}

// engine checks that a route refuses local tables that are not replicated, and that a store
// without a route still takes them.
func (it *routeIT) engine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	prefix := "sfro_plain."
	it.tables(prefix, "MergeTree")
	ctrl, err := chx.New(it.newConn(), chx.WithCluster("compute"), chx.WithTableNamePrefix(prefix)).WithMaintenanceRoute(&testRoute)
	require.NoError(t, err)
	err = ctrl.VerifyMaintenanceRoute(ctx, "blocks", "logs")
	t.Logf("routed over plain MergeTree: %v", err)
	assert.ErrorContains(t, err, "is not of the Replicated*MergeTree family on [sfro-d1:MergeTree sfro-d2:MergeTree]")

	// Without a route, on a server of its own, the store creates and uses plain MergeTree tables.
	legacyPrefix := fmt.Sprintf("sfro_legacy_%d.", time.Now().UnixNano())
	legacy := chx.New(ckhmanager.NewConn("clickhouse://"+os.Getenv("SFRO_D1")+"/chain_db"), chx.WithTableNamePrefix(legacyPrefix))
	meta := TablesMeta{LinkTableIndex: -1, BlockTableIndex: -1}
	for _, name := range []string{"blocks", "logs"} {
		sch := BuildTable(name, &testRow{}, chx.TableConfig{Engine: legacy.NewDefaultMergeTreeEngine(), OrderBy: []string{"number"}}, "")
		sch.NumberField, sch.UniqueKeyExemption = "number", "test"
		meta.Tables = append(meta.Tables, sch)
	}
	slotStore, err := NewSimpleSlotStore[*numSlot](ctx, legacy, numSchemaMgr{meta: meta}, 100, time.Second, 1, 1)
	require.NoError(t, err)
	rangeStore, err := NewRangeStore(ctx, legacy, "range", time.Hour, 7, false)
	require.NoError(t, err)
	dim := chain.NewSimpleDimension[*numSlot](rangeStore, slotStore)
	require.NoError(t, dim.Save(ctx, rg.NewRange(0, 5), feedSlots(0, 5)))
	require.NoError(t, dim.Delete(ctx, rg.Range{Start: 3}))
	cur, err := dim.GetRange(ctx)
	require.NoError(t, err)
	assert.Equal(t, rg.NewRange(0, 2), cur)
	engine := it.count(it.d1, fmt.Sprintf("SELECT count() FROM system.tables WHERE database = 'chain_db' "+
		"AND name LIKE '%s%%' AND engine = 'MergeTree'", legacyPrefix))
	assert.Equal(t, uint64(3), engine, "the legacy store created its three tables as plain MergeTree")
	assert.Zero(t, it.count(it.d1, fmt.Sprintf("SELECT count() FROM chain_db.`%sblocks` WHERE number >= 3", legacyPrefix)))
	for _, table := range []string{"blocks", "logs", "range"} {
		it.exec(it.d1, fmt.Sprintf("DROP TABLE chain_db.`%s%s` SYNC", legacyPrefix, table))
	}
}

func logSamples(t *testing.T, samples []sample) {
	var prev string
	for _, s := range samples {
		// Only the samples where something changed.
		key := fmt.Sprintf("%v%v%v%v", s.last, s.hasLast, s.rows, s.leastRows)
		if key != prev {
			t.Log(s.String())
			prev = key
		}
	}
}
