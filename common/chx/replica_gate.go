package chx

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/envconf"
)

// readAfterWriteReplicaSync turns the waits of ReplicaGate.WaitSynced off when set to false, which
// brings back reads that may miss what was just written. It is there for the case the waits load
// the distributed DDL queue of a cluster more than it can take.
var readAfterWriteReplicaSync = envconf.LoadBool("SENTIO_READ_AFTER_WRITE_REPLICA_SYNC", true)

// ReplicaGate makes the reads of a store wait for the replicas to hold what the store itself has
// written. A write is acknowledged by one replica and reaches the others afterwards, while the
// read that follows is answered by whichever replica the connection pool picks, so without the
// wait a read can miss rows the same process wrote moments ago, and it misses them silently.
//
// The wait is the one DeleteAfterReplicaSync runs, which is a distributed DDL statement and too
// heavy to pay on every read (see the comment there). The gate therefore remembers which tables
// were written since they were last waited for: Written marks a table, WaitSynced waits for the
// marked ones only and is free for the others. A table the gate has not seen yet counts as
// written, since an earlier run of the process may have been the writer.
//
// What it waits for is replication of data a replica already holds. An INSERT that is still
// running on the server after its client gave up is not covered, see InsertScope for that.
//
// A gate belongs to one store, which must be the only writer of the tables it reads.
type ReplicaGate struct {
	sync func(ctx context.Context, table string) error
	del  func(ctx context.Context, table string, condition string, light bool) (uint64, error)

	mu     sync.Mutex
	tables map[string]*replicaGateTable
}

type replicaGateTable struct {
	// syncing holds one token, taken by the reader that runs the wait; the others queue for it
	// and usually find the table synced when they get it
	syncing chan struct{}
	// written counts the writes to the table, synced is the value written had when the last
	// successful wait started. The table needs a wait while they differ.
	written uint64
	synced  uint64
}

func NewReplicaGate(ctrl Controller) *ReplicaGate {
	wait := ctrl.syncReplicas
	if !readAfterWriteReplicaSync {
		wait = func(context.Context, string) error { return nil }
	}
	return newReplicaGate(wait, ctrl.DeleteAfterReplicaSync)
}

func newReplicaGate(
	sync func(ctx context.Context, table string) error,
	del func(ctx context.Context, table string, condition string, light bool) (uint64, error),
) *ReplicaGate {
	return &ReplicaGate{sync: sync, del: del, tables: make(map[string]*replicaGateTable)}
}

// table returns the state of a table, must be called with g.mu held.
func (g *ReplicaGate) table(name string) *replicaGateTable {
	t, has := g.tables[name]
	if !has {
		t = &replicaGateTable{syncing: make(chan struct{}, 1), written: 1}
		g.tables[name] = t
	}
	return t
}

// Written records that the tables were written to. Call it once the write has returned, whether
// it succeeded or not: a failed write may have landed in part.
func (g *ReplicaGate) Written(tables ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, name := range tables {
		g.table(name).written++
	}
}

// WaitSynced returns once every replica holds what was written to the tables before the call. It
// costs nothing for a table that was not written since it was last waited for.
func (g *ReplicaGate) WaitSynced(ctx context.Context, tables ...string) error {
	for _, name := range tables {
		if err := g.waitSynced(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

func (g *ReplicaGate) waitSynced(ctx context.Context, name string) error {
	g.mu.Lock()
	t := g.table(name)
	dirty := t.synced != t.written
	g.mu.Unlock()
	if !dirty {
		return nil
	}

	select {
	case t.syncing <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-t.syncing }()

	// whoever held the token may have waited for everything this call needs
	g.mu.Lock()
	written := t.written
	dirty = t.synced != written
	g.mu.Unlock()
	if !dirty {
		return nil
	}
	if err := g.sync(ctx, name); err != nil {
		return errors.Wrapf(err, "wait for the replicas of %s to hold the latest writes failed", name)
	}
	g.markSynced(t, written)
	return nil
}

func (g *ReplicaGate) markSynced(t *replicaGateTable, written uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if written > t.synced {
		t.synced = written
	}
}

// DeleteAfterReplicaSync is Controller.DeleteAfterReplicaSync, keeping the gate informed: the
// delete waits for the replicas itself, so one that found nothing to remove leaves the table
// synced, and one that removed rows is a write like any other.
func (g *ReplicaGate) DeleteAfterReplicaSync(
	ctx context.Context,
	table string,
	condition string,
	light bool,
) (uint64, error) {
	g.mu.Lock()
	t := g.table(table)
	written := t.written
	g.mu.Unlock()

	deleted, err := g.del(ctx, table, condition, light)
	if err != nil || deleted > 0 {
		g.Written(table)
		return deleted, err
	}
	g.markSynced(t, written)
	return 0, nil
}
