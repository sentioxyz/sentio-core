package persistent

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	rsh "sentioxyz/sentio-core/common/richstructhelper"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	entityProtos "sentioxyz/sentio-core/processor/protos"
)

// Resolving pending operators needs the store version of every entity's first pending version,
// and a read without an uncommitted change needs the store version as its answer. These tests
// check that the controller reads the store before it takes its lock (so writers are never
// stalled behind a store round trip), in parallel, once per entity, for the commit as well as for
// the get and list paths, and that a write arriving during such a read is picked up.

func prefetchFixture(t *testing.T) (*schema.Schema, *schema.Entity) {
	t.Helper()
	sch, err := schema.ParseAndVerifySchema(testSchema)
	assert.NoError(t, err)
	return sch, sch.GetEntity("EntityE1")
}

func seedE1(ps *mockChainStore, id string, propB int32) {
	utils.PutIntoK2Map(ps.data, "EntityE1", id, &EntityBox{
		Entity: "EntityE1", ID: id, GenBlockNumber: 10, GenBlockHash: "0x1234",
		Data: map[string]any{"id": id, "propA": "a", "propB": propB},
	})
}

func addBox(t *testing.T, e *schema.Entity, id string, block uint64, add int32) UncommittedEntityBox {
	t.Helper()
	box := UncommittedEntityBox{EntityBox: EntityBox{ID: id, GenBlockNumber: block, GenBlockHash: "0x1234"}}
	assert.NoError(t, box.FromEntityUpdateData(e, &entityProtos.EntityUpdateData{
		Fields: map[string]*entityProtos.EntityUpdateData_FieldValue{
			"propB": {Op: entityProtos.EntityUpdateData_ADD, Value: rsh.NewIntValue(add)},
		},
	}))
	return box
}

func TestController_CommitPrefetchesPreviousVersions(t *testing.T) {
	sch, e := prefetchFixture(t)
	ctx := context.Background()
	ps, s := newTestStore(sch, "mainnet")
	for i := range 10 {
		seedE1(ps, fmt.Sprintf("e%d", i), int32(i))
	}
	ctrl, _ := newCtrl(s)

	// e0..e9: pending ADD at block 11, the first version needs the store
	for i := range 10 {
		assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, fmt.Sprintf("e%d", i), 11, 100)))
	}
	// e0 again at block 12: still one store read, the second version reads the first
	assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e0", 12, 1000)))
	// new: not in the store, the prefetch records "no previous version"
	assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "new", 11, 7)))
	// up: an upsert is concrete and needs nothing from the store
	assert.NoError(t, ctrl.SetEntity(ctx, e, UncommittedEntityBox{EntityBox: EntityBox{
		ID: "up", GenBlockNumber: 11, GenBlockHash: "0x1234",
		Data: map[string]any{"id": "up", "propA": "u", "propB": int32(1)},
	}}))
	// e3 is read before the commit, which resolves it, so the commit does not read it again
	got, err := ctrl.GetEntity(ctx, e, "e3", 11)
	assert.NoError(t, err)
	assert.Equal(t, int32(103), got.Data["propB"])

	ps.getEntityCalls.Store(0)
	ps.getEntityMaxInFlight.Store(0)
	ps.getEntityHook = func(_ *schema.Entity, _ string) error {
		time.Sleep(20 * time.Millisecond) // long enough for the reads to overlap
		return nil
	}
	created, updated, err := ctrl.Commit(ctx, 12, time.Now())
	assert.NoError(t, err)
	assert.Equal(t, map[string]int{"EntityE1": 0}, created)
	assert.Equal(t, map[string]int{"EntityE1": 13}, updated) // 10 + e0@12 + new + up

	// one read per entity with a pending first version: e0..e9 minus e3, plus new
	assert.Equal(t, int64(10), ps.getEntityCalls.Load())
	assert.Greater(t, ps.getEntityMaxInFlight.Load(), int64(1), "the prefetch reads must run in parallel")

	for i := range 10 {
		id := fmt.Sprintf("e%d", i)
		want := int32(i + 100)
		if i == 0 {
			want += 1000
		}
		assert.Equal(t, want, ps.data["EntityE1"][id].Data["propB"], id)
	}
	assert.Equal(t, map[string]any{"id": "new", "propA": "", "propB": int32(7)}, ps.data["EntityE1"]["new"].Data)
	assert.Equal(t, int32(1), ps.data["EntityE1"]["up"].Data["propB"])
}

func TestController_CommitPrefetchDoesNotHoldTheLock(t *testing.T) {
	sch, e := prefetchFixture(t)
	ctx := context.Background()
	ps, s := newTestStore(sch, "mainnet")
	seedE1(ps, "e0", 1)
	ctrl, _ := newCtrl(s)
	assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e0", 11, 1)))

	// a handler writes (at a later block) while the commit is reading the store; with the read
	// under the controller lock this would deadlock
	written := make(chan error, 1)
	ps.getEntityHook = func(_ *schema.Entity, id string) error {
		if id == "e0" {
			done := make(chan error, 1)
			go func() { done <- ctrl.SetEntity(ctx, e, addBox(t, e, "other", 13, 5)) }()
			select {
			case err := <-done:
				written <- err
			case <-time.After(2 * time.Second):
				written <- fmt.Errorf("SetEntity blocked while the commit was reading the store")
			}
		}
		return nil
	}
	_, _, err := ctrl.Commit(ctx, 12, time.Now())
	assert.NoError(t, err)
	assert.NoError(t, <-written)
	assert.Equal(t, int32(2), ps.data["EntityE1"]["e0"].Data["propB"])
	// the write that arrived during the prefetch is still pending for a later commit
	assert.Len(t, ctrl.changes["EntityE1"]["other"], 1)
	assert.False(t, ctrl.changes["EntityE1"]["other"][0].Resolved())
}

func TestController_CommitPrefetchFailure(t *testing.T) {
	sch, e := prefetchFixture(t)
	ctx := context.Background()
	ps, s := newTestStore(sch, "mainnet")
	seedE1(ps, "e0", 1)
	seedE1(ps, "e1", 1)
	ctrl, _ := newCtrl(s)
	assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e0", 11, 1)))
	assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e1", 11, 1)))

	ps.getEntityHook = func(_ *schema.Entity, id string) error {
		if id == "e1" {
			return fmt.Errorf("store is down")
		}
		return nil
	}
	_, _, err := ctrl.Commit(ctx, 11, time.Now())
	assert.ErrorContains(t, err, `prefetch entity "EntityE1" with id e1 failed: store is down`)
	// nothing was committed or resolved: both entities are still pending and the store untouched
	assert.Equal(t, int32(1), ps.data["EntityE1"]["e0"].Data["propB"])
	for _, id := range []string{"e0", "e1"} {
		assert.False(t, ctrl.changes["EntityE1"][id][0].Resolved(), id)
	}

	// once the store is back, the same commit succeeds
	ps.getEntityHook = nil
	_, updated, err := ctrl.Commit(ctx, 11, time.Now())
	assert.NoError(t, err)
	assert.Equal(t, map[string]int{"EntityE1": 2}, updated)
	assert.Equal(t, int32(2), ps.data["EntityE1"]["e1"].Data["propB"])
}

// writeDuringStoreRead returns a getEntityHook that, the first time id is read, writes another
// entity at block from inside the read. With the read under the controller lock this would
// deadlock; the returned channel reports the outcome of that write.
func writeDuringStoreRead(
	t *testing.T,
	ctrl *Controller,
	e *schema.Entity,
	id string,
	block uint64,
	other string,
) (func(*schema.Entity, string) error, chan error) {
	t.Helper()
	written := make(chan error, 1)
	fired := false
	return func(_ *schema.Entity, got string) error {
		if got != id || fired {
			return nil
		}
		fired = true
		done := make(chan error, 1)
		go func() { done <- ctrl.SetEntity(context.Background(), e, addBox(t, e, other, block, 5)) }()
		select {
		case err := <-done:
			written <- err
		case <-time.After(2 * time.Second):
			written <- fmt.Errorf("SetEntity blocked while the store was being read")
		}
		return nil
	}, written
}

func TestController_GetEntityReadsTheStoreOffTheLock(t *testing.T) {
	sch, e := prefetchFixture(t)
	ctx := context.Background()

	t.Run("pending change: its previous version", func(t *testing.T) {
		ps, s := newTestStore(sch, "mainnet")
		seedE1(ps, "e0", 1)
		ctrl, _ := newCtrl(s)
		assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e0", 11, 1)))
		hook, written := writeDuringStoreRead(t, ctrl, e, "e0", 11, "other")
		ps.getEntityHook = hook
		got, err := ctrl.GetEntity(ctx, e, "e0", 11)
		assert.NoError(t, err)
		assert.Equal(t, int32(2), got.Data["propB"])
		assert.NoError(t, <-written)
		assert.Equal(t, int64(1), ps.getEntityCalls.Load())
		// resolved once: a second read is served from the uncommitted change
		_, err = ctrl.GetEntity(ctx, e, "e0", 11)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), ps.getEntityCalls.Load())
		// what a read returned is a copy: a same-block change after it merges into the history
		// entry, not into the box the caller holds
		assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e0", 11, 1000)))
		assert.Equal(t, int32(2), got.Data["propB"])
	})

	t.Run("no change: the store version itself", func(t *testing.T) {
		ps, s := newTestStore(sch, "mainnet")
		seedE1(ps, "e0", 7)
		ctrl, _ := newCtrl(s)
		hook, written := writeDuringStoreRead(t, ctrl, e, "e0", 11, "other")
		ps.getEntityHook = hook
		got, err := ctrl.GetEntity(ctx, e, "e0", 11)
		assert.NoError(t, err)
		assert.Equal(t, int32(7), got.Data["propB"])
		assert.NoError(t, <-written)
		// a read in the block only looks at the changes of that block: no store read at all
		calls := ps.getEntityCalls.Load()
		got, err = ctrl.GetEntityInBlock(ctx, e, "e0", 11)
		assert.NoError(t, err)
		assert.Nil(t, got)
		assert.Equal(t, calls, ps.getEntityCalls.Load())
	})

	t.Run("a change arriving during the store read is the answer", func(t *testing.T) {
		ps, s := newTestStore(sch, "mainnet")
		seedE1(ps, "e0", 7)
		ctrl, _ := newCtrl(s)
		// the read of e0 itself writes e0 at block 11 before returning
		hook, written := writeDuringStoreRead(t, ctrl, e, "e0", 11, "e0")
		ps.getEntityHook = hook
		got, err := ctrl.GetEntity(ctx, e, "e0", 11)
		assert.NoError(t, err)
		assert.NoError(t, <-written)
		assert.Equal(t, int32(12), got.Data["propB"], "the write that landed during the read is included")
		assert.Equal(t, int64(1), ps.getEntityCalls.Load(), "the store version read once serves the resolution too")
	})
}

func TestController_ListEntityReadsTheStoreOffTheLock(t *testing.T) {
	sch, e := prefetchFixture(t)
	ctx := context.Background()
	ps, s := newTestStore(sch, "mainnet")
	for i := range 4 {
		seedE1(ps, fmt.Sprintf("e%d", i), int32(i))
	}
	ctrl, _ := newCtrl(s)
	// e0, e1 pending; e2 untouched; e3 deleted; new created by an update
	assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e0", 11, 100)))
	assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e1", 11, 100)))
	assert.NoError(t, ctrl.SetEntity(ctx, e, UncommittedEntityBox{EntityBox: EntityBox{ID: "e3", GenBlockNumber: 11}}))
	assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "new", 11, 9)))

	// the store is read for the pending entities and then listed, both without the lock
	hook, writtenDuringGet := writeDuringStoreRead(t, ctrl, e, "e0", 12, "other-get")
	ps.getEntityHook = hook
	writtenDuringList := make(chan error, 1)
	ps.listEntitiesHook = func(*schema.Entity) {
		done := make(chan error, 1)
		go func() {
			// a new entity, and a same-block change to e0 that merges into the history entry the
			// list has already taken
			err := ctrl.SetEntity(ctx, e, addBox(t, e, "other-list", 12, 5))
			if err == nil {
				err = ctrl.SetEntity(ctx, e, addBox(t, e, "e0", 11, 1000))
			}
			done <- err
		}()
		select {
		case err := <-done:
			writtenDuringList <- err
		case <-time.After(2 * time.Second):
			writtenDuringList <- fmt.Errorf("SetEntity blocked while the store was being listed")
		}
	}
	boxes, next, err := ctrl.ListEntity(ctx, e, nil, "", 100, 11)
	assert.NoError(t, err)
	assert.NoError(t, <-writtenDuringGet)
	assert.NoError(t, <-writtenDuringList)
	assert.Nil(t, next)
	got := map[string]int32{}
	for _, b := range boxes {
		got[b.ID] = b.Data["propB"].(int32)
	}
	assert.Equal(t, map[string]int32{"e0": 100, "e1": 101, "e2": 2, "new": 9}, got,
		"the list is a snapshot: the change to e0 that landed during the store query is not in it")
	assert.Equal(t, int64(3), ps.getEntityCalls.Load(), "e0, e1 and new: one store read each, e3 is deleted")
	// the merged change is visible to a later read
	e0, err := ctrl.GetEntity(ctx, e, "e0", 11)
	assert.NoError(t, err)
	assert.Equal(t, int32(1100), e0.Data["propB"])
}

func TestController_CommitPicksUpAWriteMadeDuringThePrefetch(t *testing.T) {
	sch, e := prefetchFixture(t)
	ctx := context.Background()
	ps, s := newTestStore(sch, "mainnet")
	seedE1(ps, "e0", 1)
	ctrl, _ := newCtrl(s)
	assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e0", 11, 1)))

	// while e0 is being prefetched, a handler creates "late" at the same block: the commit goes
	// back to the store for it (a second round) instead of reading it under the lock
	hook, written := writeDuringStoreRead(t, ctrl, e, "e0", 11, "late")
	ps.getEntityHook = hook
	_, updated, err := ctrl.Commit(ctx, 11, time.Now())
	assert.NoError(t, err)
	assert.NoError(t, <-written)
	assert.Equal(t, map[string]int{"EntityE1": 2}, updated)
	assert.Equal(t, int64(2), ps.getEntityCalls.Load(), "e0 in the first round, late in the second")
	assert.Equal(t, int32(2), ps.data["EntityE1"]["e0"].Data["propB"])
	assert.Equal(t, int32(5), ps.data["EntityE1"]["late"].Data["propB"])
}

func TestController_ListRelatedReadsTheStoreOffTheLock(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchema)
	assert.NoError(t, err)
	ctx := context.Background()
	ps, s := newTestStore(sch, "mainnet")
	ctrl, _ := newCtrl(s)
	ea, eb := sch.GetEntity("EntityA"), sch.GetEntity("EntityB")

	// a write to the listed entity type from inside its store list must complete: the related
	// list holds the lock for its uncommitted snapshot only
	written := make(chan error, 1)
	ps.listEntitiesHook = func(*schema.Entity) {
		done := make(chan error, 1)
		go func() {
			done <- ctrl.SetEntity(ctx, eb, UncommittedEntityBox{EntityBox: EntityBox{
				ID: "0x0b02", GenBlockNumber: 12, GenBlockHash: "0x1234",
				Data: map[string]any{"id": "0x0b02", "foreignB": "0x0a00", "foreignE": []*string{}, "foreignF": []string{}},
			}})
		}()
		select {
		case err := <-done:
			written <- err
		case <-time.After(2 * time.Second):
			written <- fmt.Errorf("SetEntity blocked while the store was being listed")
		}
	}
	boxes, target, err := ctrl.ListRelated(ctx, ea, "0x0a00", "foreignB", 11)
	assert.NoError(t, err)
	assert.NoError(t, <-written)
	assert.Equal(t, "EntityB", target.GetName())
	ids := make([]string, 0, len(boxes))
	for _, b := range boxes {
		ids = append(ids, b.ID)
	}
	assert.Equal(t, []string{"0x0b00", "0x0b01"}, ids, "the snapshot at block 11 does not include the block-12 write")
}

// holdReadOf returns a getEntityAfterHook that holds the first store read of id (after its answer
// was taken, so the answer is what the store had then) until release is closed.
func holdReadOf(id string) (hook func(*schema.Entity, string, *EntityBox), held chan struct{}, release chan struct{}) {
	held, release = make(chan struct{}), make(chan struct{})
	fired := false
	return func(_ *schema.Entity, got string, _ *EntityBox) {
		if got != id || fired {
			return
		}
		fired = true
		close(held)
		<-release
	}, held, release
}

// A handler read runs concurrently with Commit. Its store version is read without the lock; when a
// Commit finishes meanwhile, it has written the store and pruned the history the version was read
// for, so the version is stale and must be read again.
func TestController_ReadDropsStoreVersionsWhenACommitLands(t *testing.T) {
	sch, e := prefetchFixture(t)
	ctx := context.Background()

	// e0 = 1 in the store, +10 at block 11 and +100 at block 12 pending
	setup := func(t *testing.T) (*mockChainStore, *Controller) {
		ps, s := newTestStore(sch, "mainnet")
		seedE1(ps, "e0", 1)
		ctrl, _ := newCtrl(s)
		assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e0", 11, 10)))
		assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e0", 12, 100)))
		return ps, ctrl
	}
	// commitWhileHeld runs Commit(11) while a read holds the store version it took before
	commitWhileHeld := func(t *testing.T, ps *mockChainStore, ctrl *Controller, held, release chan struct{}) {
		<-held
		ps.getEntityAfterHook = nil // the commit's own reads are not held
		_, _, err := ctrl.Commit(ctx, 11, time.Now())
		assert.NoError(t, err)
		assert.Equal(t, int32(11), ps.data["EntityE1"]["e0"].Data["propB"])
		close(release)
	}

	t.Run("get", func(t *testing.T) {
		ps, ctrl := setup(t)
		hook, held, release := holdReadOf("e0")
		ps.getEntityAfterHook = hook
		type result struct {
			box *EntityBox
			err error
		}
		done := make(chan result, 1)
		go func() {
			box, err := ctrl.GetEntity(ctx, e, "e0", 12)
			done <- result{box, err}
		}()
		commitWhileHeld(t, ps, ctrl, held, release)
		got := <-done
		assert.NoError(t, got.err)
		assert.Equal(t, int32(111), got.box.Data["propB"], "1 + 10 (committed meanwhile) + 100")
		_, _, err := ctrl.Commit(ctx, 12, time.Now())
		assert.NoError(t, err)
		assert.Equal(t, int32(111), ps.data["EntityE1"]["e0"].Data["propB"])
	})

	t.Run("list", func(t *testing.T) {
		ps, ctrl := setup(t)
		hook, held, release := holdReadOf("e0")
		ps.getEntityAfterHook = hook
		type result struct {
			boxes []*EntityBox
			err   error
		}
		done := make(chan result, 1)
		go func() {
			boxes, _, err := ctrl.ListEntity(ctx, e, nil, "", 10, 12)
			done <- result{boxes, err}
		}()
		commitWhileHeld(t, ps, ctrl, held, release)
		got := <-done
		assert.NoError(t, got.err)
		if assert.Len(t, got.boxes, 1) {
			assert.Equal(t, int32(111), got.boxes[0].Data["propB"])
		}
		_, _, err := ctrl.Commit(ctx, 12, time.Now())
		assert.NoError(t, err)
		assert.Equal(t, int32(111), ps.data["EntityE1"]["e0"].Data["propB"])
	})

	t.Run("no change: the store answer is read again after a commit", func(t *testing.T) {
		ps, s := newTestStore(sch, "mainnet")
		seedE1(ps, "e0", 1)
		ctrl, _ := newCtrl(s)
		hook, held, release := holdReadOf("e0")
		ps.getEntityAfterHook = hook
		done := make(chan *EntityBox, 1)
		go func() {
			box, err := ctrl.GetEntity(ctx, e, "e0", 12)
			assert.NoError(t, err)
			done <- box
		}()
		<-held
		// a change arrives and is committed while the read holds the old store version
		ps.getEntityAfterHook = nil
		assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e0", 11, 10)))
		_, _, err := ctrl.Commit(ctx, 11, time.Now())
		assert.NoError(t, err)
		close(release)
		assert.Equal(t, int32(11), (<-done).Data["propB"])
	})
}

// When commits keep landing during a read, withStoreVersions gives up re-reading without the lock
// after maxStoreVersionRounds rounds and reads the missing versions under it: a read whose answer
// is the store version itself still gets the latest committed row, never a missing one.
func TestController_ReadFallsBackToALockedStoreReadAfterRepeatedCommits(t *testing.T) {
	sch, e := prefetchFixture(t)
	ctx := context.Background()
	ps, s := newTestStore(sch, "mainnet")
	seedE1(ps, "e0", 1)
	ctrl, _ := newCtrl(s)

	// every store read of e0 without the lock (maxStoreVersionRounds of them) is followed by a
	// commit of another +10, which moves the watermark and makes the read stale; the read under
	// the lock that follows is left alone (a commit from inside it would deadlock on c.mu)
	var commits, inHook atomic.Int64
	ps.getEntityAfterHook = func(_ *schema.Entity, id string, _ *EntityBox) {
		// the commit below reads the store itself: that nested read must not commit again
		if id != "e0" || !inHook.CompareAndSwap(0, 1) {
			return
		}
		defer inHook.Store(0)
		if commits.Load() >= maxStoreVersionRounds {
			return
		}
		block := uint64(10 + commits.Add(1))
		assert.NoError(t, ctrl.SetEntity(ctx, e, addBox(t, e, "e0", block, 10)))
		_, _, err := ctrl.Commit(ctx, block, time.Now())
		assert.NoError(t, err)
	}
	got, err := ctrl.GetEntity(ctx, e, "e0", 100)
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, int32(1+10*maxStoreVersionRounds), got.Data["propB"], "the latest committed row")
	}
	assert.Equal(t, int64(maxStoreVersionRounds), commits.Load())

	// the snapshot tells the reads apart: maxStoreVersionRounds rounds of reads without the lock
	// (each dropped by the commit that followed), then one read under it
	reads := storeReadsFromSnapshot(t, ctrl)
	assert.Equal(t, maxStoreVersionRounds, reads["staleRounds"])
	assert.Equal(t, 1, reads["underLockCalls"])
	assert.GreaterOrEqual(t, reads["offLockCalls"], maxStoreVersionRounds)
}

// storeReadsFromSnapshot sums the storeReads part of every statistics window of the snapshot.
func storeReadsFromSnapshot(t *testing.T, ctrl *Controller) map[string]int {
	t.Helper()
	sum := map[string]int{}
	for _, w := range ctrl.timeStat.Snapshot() {
		reads := w.(map[string]any)["storeReads"].(map[string]any)
		sum["staleRounds"] += reads["staleRounds"].(int)
		sum["offLockCalls"] += reads["offLock"].(map[string]any)["calls"].(int)
		sum["underLockCalls"] += reads["underLock"].(map[string]any)["calls"].(int)
	}
	return sum
}
