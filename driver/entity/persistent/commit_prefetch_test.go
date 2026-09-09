package persistent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	rsh "sentioxyz/sentio-core/common/richstructhelper"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	entityProtos "sentioxyz/sentio-core/processor/protos"
)

// Commit resolves the pending operators of every changed entity, and the first pending version
// of each needs the store version. These tests check that those reads happen before the commit
// takes the controller lock (so writers are not stalled), in parallel, once per entity.

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
