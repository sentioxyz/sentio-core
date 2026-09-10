package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// ChainStore never holds its cache lock across a store round trip. These tests drive the real
// ChainStore over a fake store (fakeStore), and check that reads overlap, that
// the lock stays free while a read, a list, a cache load or a reorg is in flight, and that the
// guards keep a row read before a write or a purge out of the caches.

const chainStoreTestSchema = `
type Position @entity {
  id: ID!
  balance: Int!
}
`

// fakeStore is a chainStoreBackend for tests: every query goes through a func field, so a test
// controls what the store returns and when. Methods without a func fail the test if called.
type fakeStore struct {
	chainStoreBackend // nil: any method a test does not expect panics

	t             *testing.T
	getEntity_    func(ctx context.Context, entityType *schema.Entity, id string) (*entityRow, error)
	listEntities_ func(
		ctx context.Context,
		entityType *schema.Entity,
		filters []persistent.EntityFilter,
		excludeDeleted bool,
		limit int,
	) ([]*entityRow, error)
	countEntity_ func(ctx context.Context, entityType *schema.Entity, excludeDeleted bool) (uint64, error)
	getAllID_    func(ctx context.Context, entityType *schema.Entity) (set.Set[string], error)
	reorg_       func(ctx context.Context, blockNumber int64) error
}

func (f *fakeStore) useVersionedCollapsingTable(schema.EntityOrInterface) bool { return false }

func (f *fakeStore) getEntity(
	ctx context.Context, entityType *schema.Entity, _ string, id string,
) (*entityRow, error) {
	if f.getEntity_ == nil {
		f.t.Fatal("getEntity not expected")
	}
	return f.getEntity_(ctx, entityType, id)
}

func (f *fakeStore) listEntities(
	ctx context.Context,
	entityType *schema.Entity,
	_ string,
	filters []persistent.EntityFilter,
	excludeDeleted bool,
	limit int,
) ([]*entityRow, error) {
	if f.listEntities_ == nil {
		f.t.Fatal("listEntities not expected")
	}
	return f.listEntities_(ctx, entityType, filters, excludeDeleted, limit)
}

func (f *fakeStore) countEntity(
	ctx context.Context, entityType *schema.Entity, _ string, excludeDeleted bool,
) (uint64, error) {
	if f.countEntity_ == nil {
		f.t.Fatal("countEntity not expected")
	}
	return f.countEntity_(ctx, entityType, excludeDeleted)
}

func (f *fakeStore) getAllID(ctx context.Context, entityType *schema.Entity, _ string) (set.Set[string], error) {
	if f.getAllID_ == nil {
		f.t.Fatal("getAllID not expected")
	}
	return f.getAllID_(ctx, entityType)
}

func (f *fakeStore) reorg(ctx context.Context, blockNumber int64, _ string) error {
	if f.reorg_ == nil {
		f.t.Fatal("reorg not expected")
	}
	return f.reorg_(ctx, blockNumber)
}

func newTestChainStore(t *testing.T) (*ChainStore, *fakeStore, *schema.Entity) {
	t.Helper()
	sch, err := schema.ParseAndVerifySchema(chainStoreTestSchema)
	require.NoError(t, err)
	e := sch.GetEntity("Position")
	fs := &fakeStore{t: t}
	cs := NewChainStore(fs, "chain", 1000, 1<<20, 1000)
	// both caches refused: the LRU + store path is under test unless a test loads them
	cs.fullCacheRefused[e.Name] = true
	cs.fullIDCacheRefused[e.Name] = true
	return cs, fs, e
}

// lruHit asserts that id is served from the LRU without a store read, and returns how long it took.
func lruHit(t *testing.T, cs *ChainStore, e *schema.Entity, id string) time.Duration {
	t.Helper()
	start := time.Now()
	box, fromCache, err := cs.GetEntity(context.Background(), e, id)
	require.NoError(t, err)
	require.True(t, fromCache)
	require.NotNil(t, box)
	return time.Since(start)
}

// blockingIO parks a store operation until release is closed and reports on started that it is
// waiting; while it waits, the test checks that the cache lock is free.
type blockingIO struct {
	started chan struct{}
	release chan struct{}
}

func newBlockingIO() blockingIO {
	return blockingIO{started: make(chan struct{}, 1), release: make(chan struct{})}
}

func (b blockingIO) wait() {
	b.started <- struct{}{}
	<-b.release
}

func positionRow(id string, balance int32) *entityRow {
	return &entityRow{EntityBox: persistent.EntityBox{
		Entity: "Position", ID: id, Data: map[string]any{"id": id, "balance": balance},
	}}
}

func TestChainStore_GetEntity_ReadsOverlapOffTheLock(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	var reads, inFlight, maxInFlight atomic.Int64
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		reads.Add(1)
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			seen := maxInFlight.Load()
			if n <= seen || maxInFlight.CompareAndSwap(seen, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		return positionRow(id, 1), nil
	}

	const n = 16
	var wg sync.WaitGroup
	for i := range n {
		id := fmt.Sprintf("p%d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			box, fromCache, err := cs.GetEntity(context.Background(), e, id)
			assert.NoError(t, err)
			assert.False(t, fromCache)
			assert.Equal(t, int32(1), box.Data["balance"])
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(n), reads.Load())
	assert.Greater(t, maxInFlight.Load(), int64(1), "store reads must overlap instead of queueing on mu")

	// every row is now in the LRU: no further store read
	for i := range n {
		box, _, err := cs.GetEntity(context.Background(), e, fmt.Sprintf("p%d", i))
		assert.NoError(t, err)
		assert.Equal(t, int32(1), box.Data["balance"])
	}
	assert.Equal(t, int64(n), reads.Load())
}

// blockingRead returns a getEntity that parks every read until release is closed and reports
// on started when a read is waiting.
func blockingRead(row *entityRow) (
	read func(context.Context, *schema.Entity, string) (*entityRow, error),
	started chan struct{},
	release chan struct{},
) {
	started = make(chan struct{}, 1)
	release = make(chan struct{})
	read = func(_ context.Context, _ *schema.Entity, _ string) (*entityRow, error) {
		started <- struct{}{}
		<-release
		return row, nil
	}
	return
}

func TestChainStore_GetEntity_DoesNotCacheARowOlderThanAWrite(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	read, started, release := blockingRead(positionRow("p", 1))
	fs.getEntity_ = read

	type result struct {
		box *persistent.EntityBox
		err error
	}
	done := make(chan result, 1)
	go func() {
		box, _, err := cs.GetEntity(context.Background(), e, "p")
		done <- result{box, err}
	}()
	<-started

	// a write of the same entity lands while the read is in flight; the cache lock is free
	written := persistent.EntityBox{Entity: "Position", ID: "p", Data: map[string]any{"id": "p", "balance": int32(2)}}
	_, logger := log.FromContext(context.Background())
	cs.mu.Lock()
	cs.applyWriteToCaches(logger, e, []persistent.EntityBox{written})
	cs.mu.Unlock()

	close(release)
	r := <-done
	assert.NoError(t, r.err)
	assert.Equal(t, int32(1), r.box.Data["balance"], "the read returns what the store had")

	// the LRU keeps the written version, not the older row the read brought back
	fs.getEntity_ = func(context.Context, *schema.Entity, string) (*entityRow, error) {
		return nil, errors.New("must be served from the LRU")
	}
	box, _, err := cs.GetEntity(context.Background(), e, "p")
	assert.NoError(t, err)
	assert.Equal(t, int32(2), box.Data["balance"])
}

func TestChainStore_GetEntity_DoesNotCacheARowOlderThanAPurge(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	read, started, release := blockingRead(positionRow("p", 1))
	fs.getEntity_ = read

	done := make(chan error, 1)
	go func() {
		_, _, err := cs.GetEntity(context.Background(), e, "p")
		done <- err
	}()
	<-started
	cs.mu.Lock()
	cs.purgeCache()
	cs.fullCacheRefused[e.Name] = true
	cs.fullIDCacheRefused[e.Name] = true
	cs.mu.Unlock()
	close(release)
	assert.NoError(t, <-done)

	// nothing was cached: the next read goes to the store again
	var reads atomic.Int64
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		reads.Add(1)
		return positionRow(id, 3), nil
	}
	box, _, err := cs.GetEntity(context.Background(), e, "p")
	assert.NoError(t, err)
	assert.Equal(t, int32(3), box.Data["balance"])
	assert.Equal(t, int64(1), reads.Load())
}

func TestChainStore_GetEntity_StoreErrorAndMissing(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		if id == "bad" {
			return nil, errors.New("store is down")
		}
		return nil, nil
	}
	_, _, err := cs.GetEntity(context.Background(), e, "bad")
	assert.EqualError(t, err, "store is down")
	box, fromCache, err := cs.GetEntity(context.Background(), e, "missing")
	assert.NoError(t, err)
	assert.False(t, fromCache)
	assert.Nil(t, box)
	// a miss is not cached either: the LRU only holds existing rows
	assert.Equal(t, 0, cs.lruCache.Len())
}

// seedLRU puts id into the LRU through a store read so later reads are cache hits.
func seedLRU(t *testing.T, cs *ChainStore, fs *fakeStore, e *schema.Entity, id string) {
	t.Helper()
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		return positionRow(id, 1), nil
	}
	_, _, err := cs.GetEntity(context.Background(), e, id)
	require.NoError(t, err)
}

func TestChainStore_ListEntities_QueriesOffTheLock(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	seedLRU(t, cs, fs, e, "hot")
	b := newBlockingIO()
	fs.listEntities_ = func(
		context.Context, *schema.Entity, []persistent.EntityFilter, bool, int,
	) ([]*entityRow, error) {
		b.wait()
		return []*entityRow{positionRow("p1", 1), positionRow("p2", 2)}, nil
	}
	done := make(chan int, 1)
	go func() {
		boxes, _, err := cs.ListEntities(context.Background(), e, nil, 10)
		assert.NoError(t, err)
		done <- len(boxes)
	}()
	<-b.started
	// the list query is in flight: a cache read must not wait for it
	assert.Less(t, lruHit(t, cs, e, "hot"), time.Second)
	close(b.release)
	assert.Equal(t, 2, <-done)
}

func TestChainStore_ListEntities_DoesNotLoadTheIDCache(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	cs.fullIDCacheRefused[e.Name] = false // undecided, but a list cannot use it
	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) {
		return 0, errors.New("a list must not count the entity")
	}
	fs.listEntities_ = func(
		context.Context, *schema.Entity, []persistent.EntityFilter, bool, int,
	) ([]*entityRow, error) {
		return []*entityRow{positionRow("p1", 1)}, nil
	}
	boxes, _, err := cs.ListEntities(context.Background(), e, nil, 10)
	assert.NoError(t, err)
	assert.Len(t, boxes, 1)
	cs.mu.Lock()
	assert.False(t, cs.fullIDCacheLoaded[e.Name])
	cs.mu.Unlock()
	// a point read still loads it
	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 1, nil }
	fs.getAllID_ = func(context.Context, *schema.Entity) (set.Set[string], error) { return set.New("p1"), nil }
	_, _, err = cs.GetEntity(context.Background(), e, "missing")
	assert.NoError(t, err)
	cs.mu.Lock()
	assert.True(t, cs.fullIDCacheLoaded[e.Name])
	cs.mu.Unlock()
}

func TestChainStore_EnsureCaches_LoadsOffTheLock(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	seedLRU(t, cs, fs, e, "warm") // while both caches are refused
	cs.mu.Lock()
	cs.fullIDCacheRefused[e.Name] = false // from now on the full-ID cache may be loaded
	cs.mu.Unlock()

	b := newBlockingIO()
	var direct atomic.Int64
	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 3, nil }
	fs.getAllID_ = func(context.Context, *schema.Entity) (set.Set[string], error) {
		b.wait()
		return set.New("a", "b", "c"), nil
	}
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		direct.Add(1)
		return positionRow(id, 1), nil
	}

	// the first caller loads the full-ID cache; the load runs without mu
	done := make(chan error, 1)
	go func() {
		_, _, err := cs.GetEntity(context.Background(), e, "a")
		done <- err
	}()
	<-b.started
	assert.Less(t, lruHit(t, cs, e, "warm"), time.Second, "a cache hit must not wait for the load")
	// a second caller of the same entity type does not wait for the load either: it goes direct
	directBefore := direct.Load()
	box, fromCache, err := cs.GetEntity(context.Background(), e, "b")
	assert.NoError(t, err)
	assert.False(t, fromCache)
	assert.Equal(t, int32(1), box.Data["balance"])
	assert.Equal(t, directBefore+1, direct.Load())
	cs.mu.Lock()
	assert.True(t, cs.loading.Contains(e.Name))
	assert.False(t, cs.fullIDCacheLoaded[e.Name])
	cs.mu.Unlock()

	close(b.release)
	assert.NoError(t, <-done)
	cs.mu.Lock()
	assert.False(t, cs.loading.Contains(e.Name))
	assert.True(t, cs.fullIDCacheLoaded[e.Name])
	cs.mu.Unlock()
	// the loaded ID cache answers a missing id without a store read
	directBefore = direct.Load()
	box, _, err = cs.GetEntity(context.Background(), e, "missing")
	assert.NoError(t, err)
	assert.Nil(t, box)
	assert.Equal(t, directBefore, direct.Load())
}

func TestChainStore_EnsureCaches_DiscardsALoadOlderThanAWrite(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	cs.fullIDCacheRefused[e.Name] = false
	b := newBlockingIO()
	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 1, nil }
	fs.getAllID_ = func(context.Context, *schema.Entity) (set.Set[string], error) {
		b.wait()
		return set.New("old"), nil
	}
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		return positionRow(id, 1), nil
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := cs.GetEntity(context.Background(), e, "old")
		done <- err
	}()
	<-b.started
	// a write of a new row lands while the ID set is being loaded
	_, logger := log.FromContext(context.Background())
	cs.mu.Lock()
	cs.applyWriteToCaches(logger, e, []persistent.EntityBox{{
		Entity: "Position", ID: "new", Data: map[string]any{"id": "new", "balance": int32(2)},
	}})
	cs.mu.Unlock()
	close(b.release)
	assert.NoError(t, <-done)

	// the loaded set predates the write and is discarded: "new" is not wrongly reported missing
	cs.mu.Lock()
	assert.False(t, cs.fullIDCacheLoaded[e.Name])
	cs.mu.Unlock()
	fs.getAllID_ = func(context.Context, *schema.Entity) (set.Set[string], error) {
		return set.New("old", "new"), nil
	}
	box, _, err := cs.GetEntity(context.Background(), e, "new")
	assert.NoError(t, err)
	require.NotNil(t, box)
	assert.Equal(t, int32(2), box.Data["balance"])
	cs.mu.Lock()
	assert.True(t, cs.fullIDCacheLoaded[e.Name])
	cs.mu.Unlock()
}

func TestChainStore_EnsureCaches_DiscardsALoadOverlappingAWriteInFlight(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	cs.fullIDCacheRefused[e.Name] = false
	b := newBlockingIO()
	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 1, nil }
	fs.getAllID_ = func(context.Context, *schema.Entity) (set.Set[string], error) {
		b.wait()
		return set.New("old"), nil
	}
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		return positionRow(id, 1), nil
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := cs.GetEntity(context.Background(), e, "old")
		done <- err
	}()
	<-b.started
	// a write starts while the ID set is being loaded and is still in flight when the load ends:
	// SetEntities marks the entity type as writing before its persistent write
	cs.mu.Lock()
	cs.writing.Add(e.Name)
	cs.mu.Unlock()
	close(b.release)
	assert.NoError(t, <-done)
	cs.mu.Lock()
	assert.False(t, cs.fullIDCacheLoaded[e.Name], "a load that may have seen part of the write is not installed")
	cs.mu.Unlock()

	// once the write has landed, the next caller loads the current state
	_, logger := log.FromContext(context.Background())
	cs.mu.Lock()
	cs.writing.Remove(e.Name)
	cs.applyWriteToCaches(logger, e, []persistent.EntityBox{{
		Entity: "Position", ID: "new", Data: map[string]any{"id": "new", "balance": int32(2)},
	}})
	cs.mu.Unlock()
	fs.getAllID_ = func(context.Context, *schema.Entity) (set.Set[string], error) {
		return set.New("old", "new"), nil
	}
	box, _, err := cs.GetEntity(context.Background(), e, "new")
	assert.NoError(t, err)
	require.NotNil(t, box)
	cs.mu.Lock()
	assert.True(t, cs.fullIDCacheLoaded[e.Name])
	assert.True(t, cs.fullIDCache[e.Name].Contains("new"))
	cs.mu.Unlock()
}

func TestChainStore_Reorg_RunsOffTheLock(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	seedLRU(t, cs, fs, e, "hot")
	b := newBlockingIO()
	fs.reorg_ = func(context.Context, int64) error {
		b.wait()
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- cs.Reorg(context.Background(), 10) }()
	<-b.started
	// the caches are already purged, so this is a store read; it must not wait for the reorg
	start := time.Now()
	_, _, err := cs.GetEntity(context.Background(), e, "hot")
	assert.NoError(t, err)
	assert.Less(t, time.Since(start), time.Second)
	close(b.release)
	assert.NoError(t, <-done)
	// what was read during the reorg is not kept
	assert.Equal(t, 0, cs.lruCache.Len())
	cs.mu.Lock()
	assert.False(t, cs.reorging)
	cs.mu.Unlock()
}
