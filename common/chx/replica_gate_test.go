package chx

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gateRecorder stands in for the controller behind a gate and counts the waits per table.
type gateRecorder struct {
	mu      sync.Mutex
	syncs   map[string]int
	syncErr error
	onSync  func(table string)
	deleted uint64
	delErr  error
}

func (r *gateRecorder) sync(_ context.Context, table string) error {
	r.mu.Lock()
	if r.syncs == nil {
		r.syncs = make(map[string]int)
	}
	r.syncs[table]++
	err, onSync := r.syncErr, r.onSync
	r.mu.Unlock()
	if onSync != nil {
		onSync(table)
	}
	return err
}

func (r *gateRecorder) del(ctx context.Context, table string, _ string, _ bool) (uint64, error) {
	// the real delete waits for the replicas before it counts
	if err := r.sync(ctx, table); err != nil {
		return 0, err
	}
	return r.deleted, r.delErr
}

func (r *gateRecorder) count(table string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.syncs[table]
}

func TestReplicaGateWaitsOnlyForWrittenTables(t *testing.T) {
	ctx := context.Background()
	rec := &gateRecorder{}
	g := newReplicaGate(rec.sync, rec.del)

	// a table the gate has not seen may have been written by an earlier run
	require.NoError(t, g.WaitSynced(ctx, "a", "b"))
	assert.Equal(t, 1, rec.count("a"))
	assert.Equal(t, 1, rec.count("b"))

	// nothing was written since: the reads are free
	require.NoError(t, g.WaitSynced(ctx, "a", "b"))
	require.NoError(t, g.WaitSynced(ctx, "a"))
	assert.Equal(t, 1, rec.count("a"))
	assert.Equal(t, 1, rec.count("b"))

	// several writes before a read cost one wait, and only on the table written
	g.Written("a")
	g.Written("a")
	require.NoError(t, g.WaitSynced(ctx, "a", "b"))
	require.NoError(t, g.WaitSynced(ctx, "a", "b"))
	assert.Equal(t, 2, rec.count("a"))
	assert.Equal(t, 1, rec.count("b"))
}

func TestReplicaGateFailedWaitLeavesTableWritten(t *testing.T) {
	ctx := context.Background()
	rec := &gateRecorder{syncErr: errors.New("replica is away")}
	g := newReplicaGate(rec.sync, rec.del)

	err := g.WaitSynced(ctx, "a")
	assert.ErrorContains(t, err, "replica is away")
	assert.ErrorContains(t, err, "replicas of a")

	rec.mu.Lock()
	rec.syncErr = nil
	rec.mu.Unlock()
	require.NoError(t, g.WaitSynced(ctx, "a"))
	require.NoError(t, g.WaitSynced(ctx, "a"))
	assert.Equal(t, 2, rec.count("a"))
}

// A write that lands while the wait is running is not covered by it: the next read waits again.
func TestReplicaGateWriteDuringWaitNeedsAnotherWait(t *testing.T) {
	ctx := context.Background()
	rec := &gateRecorder{}
	g := newReplicaGate(rec.sync, rec.del)
	once := sync.Once{}
	rec.onSync = func(table string) {
		once.Do(func() { g.Written(table) })
	}

	require.NoError(t, g.WaitSynced(ctx, "a"))
	require.NoError(t, g.WaitSynced(ctx, "a"))
	require.NoError(t, g.WaitSynced(ctx, "a"))
	assert.Equal(t, 2, rec.count("a"))
}

func TestReplicaGateConcurrentReadersShareOneWait(t *testing.T) {
	ctx := context.Background()
	rec := &gateRecorder{}
	g := newReplicaGate(rec.sync, rec.del)
	entered, release := make(chan struct{}), make(chan struct{})
	rec.onSync = func(string) {
		close(entered) // a second wait would close it again and panic
		<-release
	}

	const readers = 8
	var wg, started sync.WaitGroup
	errs := make(chan error, readers)
	read := func() {
		defer wg.Done()
		started.Done()
		errs <- g.WaitSynced(ctx, "a")
	}
	wg.Add(readers)
	started.Add(readers)
	go read()
	// the others arrive while the first one is waiting for the replicas, and have to queue behind it
	<-entered
	for i := 1; i < readers; i++ {
		go read()
	}
	started.Wait()
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, 1, rec.count("a"))
}

func TestReplicaGateWaitHonoursContextWhileQueued(t *testing.T) {
	rec := &gateRecorder{}
	g := newReplicaGate(rec.sync, rec.del)
	entered, release := make(chan struct{}), make(chan struct{})
	rec.onSync = func(string) {
		close(entered)
		<-release
	}
	done := make(chan error, 1)
	go func() { done <- g.WaitSynced(context.Background(), "a") }()
	<-entered

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, g.WaitSynced(ctx, "a"), context.Canceled)

	close(release)
	require.NoError(t, <-done)
}

func TestReplicaGateDelete(t *testing.T) {
	ctx := context.Background()
	rec := &gateRecorder{}
	g := newReplicaGate(rec.sync, rec.del)

	// nothing deleted: the wait the delete ran itself leaves the table synced
	deleted, err := g.DeleteAfterReplicaSync(ctx, "a", "x > 1", true)
	require.NoError(t, err)
	assert.Zero(t, deleted)
	require.NoError(t, g.WaitSynced(ctx, "a"))
	assert.Equal(t, 1, rec.count("a"))

	// rows deleted: the delete has to reach the replicas before the next read
	rec.deleted = 3
	deleted, err = g.DeleteAfterReplicaSync(ctx, "a", "x > 1", true)
	require.NoError(t, err)
	assert.EqualValues(t, 3, deleted)
	require.NoError(t, g.WaitSynced(ctx, "a"))
	require.NoError(t, g.WaitSynced(ctx, "a"))
	assert.Equal(t, 3, rec.count("a"))

	// a failed delete may have run in part
	rec.deleted, rec.delErr = 0, errors.New("timeout")
	_, err = g.DeleteAfterReplicaSync(ctx, "a", "x > 1", true)
	assert.ErrorContains(t, err, "timeout")
	require.NoError(t, g.WaitSynced(ctx, "a"))
	assert.Equal(t, 5, rec.count("a"))
}

// The gate of a real controller waits with the statement the delete uses, and a store whose
// tables are not replicated never sends it.
func TestReplicaGateUsesControllerSync(t *testing.T) {
	ctx := context.Background()
	conn := &fakeConn{}
	g := NewReplicaGate(New(conn, WithCluster("c1"), WithDatabase("db")))
	require.NoError(t, g.WaitSynced(ctx, "entity_X"))
	require.NoError(t, g.WaitSynced(ctx, "entity_X"))
	syncs := conn.statements("SYSTEM SYNC REPLICA")
	require.Len(t, syncs, 1)
	// the wait is only one once every host has run it: a statement that returns while it is merely
	// queued, or reports a host that has not run it as done, would leave the table marked as synced
	assert.Equal(t, "throw", conn.settings[0]["distributed_ddl_output_mode"])
	assert.EqualValues(t, onClusterDDLTimeoutSeconds, conn.settings[0]["distributed_ddl_task_timeout"])
	assert.True(t, strings.Contains(syncs[0], "ON CLUSTER 'c1'") && strings.Contains(syncs[0], "entity_X"), syncs[0])

	conn = &fakeConn{}
	g = NewReplicaGate(New(conn, WithDatabase("db")))
	require.NoError(t, g.WaitSynced(ctx, "entity_X"))
	assert.Empty(t, conn.statements("SYSTEM"))
}
