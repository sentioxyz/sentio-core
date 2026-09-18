package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/common/log"
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

type Sparse @entity(sparse: true) {
  id: ID!
  balance: Int!
}

type SparseBlob @entity(sparse: true) {
  id: ID!
  payload: String!
}
`

// fakeStore is a chainStoreBackend for tests: every query goes through a func field, so a test
// controls what the store returns and when. Methods without a func fail the test if called.
type fakeStore struct {
	chainStoreBackend // nil: any method a test does not expect panics

	t *testing.T
	// versionedCollapsing switches the fake to the entity shape whose deletes leave a box behind
	// in the full-data cache to keep its version.
	versionedCollapsing bool
	getEntity_          func(ctx context.Context, entityType *schema.Entity, id string) (*entityRow, error)
	listEntities_       func(
		ctx context.Context,
		entityType *schema.Entity,
		filters []persistent.EntityFilter,
		excludeDeleted bool,
		limit int,
	) ([]*entityRow, error)
	countEntity_ func(ctx context.Context, entityType *schema.Entity, excludeDeleted bool) (uint64, error)
	getAllID_    func(ctx context.Context, entityType *schema.Entity) (*idSet, error)
	reorg_       func(ctx context.Context, blockNumber int64) (bool, error)
	// setEntities_ is unset in most tests: a write that only needs to reach the caches is served
	// by the default below, which reports every box as written and stores nothing.
	setEntities_ func(ctx context.Context, entityType *schema.Entity, entities []persistent.EntityBox) (int, error)
	// avgRowBytes_ is unlike the others: every full-cache load measures the row size, so leaving
	// it unset is normal and reports one byte per row, which keeps a test's own counts the thing
	// that decides whether the cache fits.
	avgRowBytes_ func(ctx context.Context, entityType *schema.Entity) (uint64, error)
}

func (f *fakeStore) useVersionedCollapsingTable(schema.EntityOrInterface) bool {
	return f.versionedCollapsing
}

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

func (f *fakeStore) avgRowBytes(ctx context.Context, entityType *schema.Entity, _ string) (uint64, error) {
	if f.avgRowBytes_ == nil {
		return 1, nil
	}
	return f.avgRowBytes_(ctx, entityType)
}

func (f *fakeStore) setEntities(
	ctx context.Context,
	entityType *schema.Entity,
	_ string,
	entities []persistent.EntityBox,
	_ func(string) bool,
	_ func(string) (*cachedEntityBox, bool),
) (int, error) {
	if f.setEntities_ == nil {
		return len(entities), nil
	}
	return f.setEntities_(ctx, entityType, entities)
}

func (f *fakeStore) getAllID(ctx context.Context, entityType *schema.Entity, _ string) (*idSet, error) {
	if f.getAllID_ == nil {
		f.t.Fatal("getAllID not expected")
	}
	return f.getAllID_(ctx, entityType)
}

func (f *fakeStore) reorg(ctx context.Context, blockNumber int64, _ string) (bool, error) {
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
	cacheSize, fullCacheMaxBytes, fullIDCacheMaxCount :=
		defaultEntityStoreCacheSize, defaultEntityStoreFullCacheMaxBytes, defaultEntityStoreFullIDCacheMaxCount
	defaultEntityStoreCacheSize, defaultEntityStoreFullCacheMaxBytes, defaultEntityStoreFullIDCacheMaxCount =
		1000, 1<<20, 1000
	t.Cleanup(func() {
		defaultEntityStoreCacheSize, defaultEntityStoreFullCacheMaxBytes, defaultEntityStoreFullIDCacheMaxCount =
			cacheSize, fullCacheMaxBytes, fullIDCacheMaxCount
	})
	cs := NewChainStore(fs, "chain")
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
	fs.getAllID_ = func(context.Context, *schema.Entity) (*idSet, error) { return newIDSet("p1"), nil }
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
	fs.getAllID_ = func(context.Context, *schema.Entity) (*idSet, error) {
		b.wait()
		return newIDSet("a", "b", "c"), nil
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

// sparseEntity returns the test schema's sparse entity type, the only one the full-data cache is
// used for. It re-parses the schema because fakeStore answers no type lookups of its own.
func sparseEntity(t *testing.T) *schema.Entity {
	t.Helper()
	sch, err := schema.ParseAndVerifySchema(chainStoreTestSchema)
	require.NoError(t, err)
	return sch.GetEntity("Sparse")
}

// The full-data cache is admitted on what the rows actually hold. Both cases below have the same
// two fields per row, so the field-count sizing they replace judged them the same; measured, one
// is 6MB of payload behind 100 ids and the other 64KB behind 1000.
func TestChainStore_EnsureCaches_AdmitsTheFullCacheByStoredBytes(t *testing.T) {
	for _, tc := range []struct {
		name        string
		ids         uint64
		avgRowBytes uint64
		wantLoaded  bool
	}{{
		// Loading rows this large is what exhausted the ClickHouse query memory limit, and since
		// a cold start cannot get past it the driver never ran at all.
		name: "few huge rows", ids: 100, avgRowBytes: 64 << 10, wantLoaded: false,
	}, {
		// The mirror case, and why the estimate multiplies the id count rather than the row
		// count: a table keeps every update of an id as its own row, so an entity of a few
		// thousand live ids can sit behind tens of millions of rows and still fit easily.
		name: "many small rows", ids: 1000, avgRowBytes: 64, wantLoaded: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			cs, fs, _ := newTestChainStore(t)
			e := sparseEntity(t)
			require.True(t, e.IsSparse(), "the full-data cache is only used for sparse entities")
			cs.mu.Lock()
			cs.fullCacheRefused[e.Name] = false
			cs.fullIDCacheRefused[e.Name] = false
			cs.mu.Unlock()

			fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) {
				return tc.ids, nil
			}
			fs.avgRowBytes_ = func(context.Context, *schema.Entity) (uint64, error) {
				return tc.avgRowBytes, nil
			}
			var listed atomic.Bool
			fs.listEntities_ = func(
				context.Context, *schema.Entity, []persistent.EntityFilter, bool, int,
			) ([]*entityRow, error) {
				listed.Store(true)
				return []*entityRow{positionRow("a", 1)}, nil
			}
			fs.getAllID_ = func(context.Context, *schema.Entity) (*idSet, error) {
				return newIDSet("a"), nil
			}
			fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
				return positionRow(id, 1), nil
			}

			_, _, err := cs.GetEntity(context.Background(), e, "a")
			assert.NoError(t, err)
			cs.mu.Lock()
			defer cs.mu.Unlock()
			assert.Equal(t, tc.wantLoaded, cs.fullCacheLoaded[e.Name])
			assert.Equal(t, !tc.wantLoaded, cs.fullCacheRefused[e.Name])
			// A refused entity must not have been read at all: issuing the load and failing on it
			// is the crash loop this check exists to avoid.
			assert.Equal(t, tc.wantLoaded, listed.Load(), "the store read must follow the decision")
			if tc.wantLoaded {
				assert.Equal(t, cs.fullCacheBytes[e.Name], measuredBytes(cs.fullCache[e.Name]),
					"a loaded cache accounts for exactly the boxes it holds")
			}
		})
	}
}

// measuredBytes adds up what a full-data cache holds, independently of the running total the
// store keeps, so a test can assert the two agree.
func measuredBytes(cache map[string]*cachedEntityBox) uint64 {
	var total uint64
	for _, box := range cache {
		total += box.memSize
	}
	return total
}

// blobEntity is an id and one large field: the shape whose size a field count cannot express.
func blobEntity(t *testing.T) *schema.Entity {
	t.Helper()
	sch, err := schema.ParseAndVerifySchema(chainStoreTestSchema)
	require.NoError(t, err)
	return sch.GetEntity("SparseBlob")
}

func blobBoxes(from, count int, payload string) []persistent.EntityBox {
	boxes := make([]persistent.EntityBox, count)
	for i := range boxes {
		id := fmt.Sprintf("id-%d", from+i)
		boxes[i] = persistent.EntityBox{
			Entity: "SparseBlob", ID: id, Data: map[string]any{"id": id, "payload": payload},
		}
	}
	return boxes
}

// A processor starting on an empty table is the case no projection from the store can size: it
// reports nothing stored, which is true, and says nothing about what is about to be written. The
// cache is admitted (there is nothing to refuse) and must then be sized by what it actually
// holds, or it would grow without limit for the whole life of the run -- the same unbounded
// growth the limit exists to prevent, reached from the other side.
func TestChainStore_SetEntities_SizesAFullCacheLoadedFromAnEmptyTable(t *testing.T) {
	cs, fs, _ := newTestChainStore(t)
	e := blobEntity(t)
	cs.mu.Lock()
	cs.fullCacheRefused[e.Name] = false
	cs.fullIDCacheRefused[e.Name] = false
	cs.mu.Unlock()

	// an empty table: no rows, and no parts to measure a row size from
	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 0, nil }
	fs.avgRowBytes_ = func(context.Context, *schema.Entity) (uint64, error) { return 0, nil }
	fs.listEntities_ = func(
		context.Context, *schema.Entity, []persistent.EntityFilter, bool, int,
	) ([]*entityRow, error) {
		return nil, nil
	}
	fs.getEntity_ = func(context.Context, *schema.Entity, string) (*entityRow, error) { return nil, nil }

	_, _, err := cs.GetEntity(context.Background(), e, "id-0")
	require.NoError(t, err)
	cs.mu.Lock()
	require.True(t, cs.fullCacheLoaded[e.Name], "an empty table has nothing to refuse")
	require.Zero(t, cs.fullCacheBytes[e.Name])
	cs.mu.Unlock()

	// 1MiB of payload against the 1MiB limit of newTestChainStore, written in batches so the
	// check runs repeatedly rather than once on a single oversized write
	payload := strings.Repeat("x", 16<<10)
	var refusedAfter int
	for batch := 0; batch < 16; batch++ {
		_, err = cs.SetEntities(context.Background(), e, blobBoxes(batch*4, 4, payload))
		require.NoError(t, err)
		cs.mu.Lock()
		refused := cs.fullCacheRefused[e.Name]
		if !refused {
			assert.Equal(t, measuredBytes(cs.fullCache[e.Name]), cs.fullCacheBytes[e.Name],
				"the running total must track the boxes actually held")
			assert.LessOrEqual(t, cs.fullCacheBytes[e.Name], cs.fullCacheMaxBytes)
		}
		cs.mu.Unlock()
		if refused {
			refusedAfter = batch + 1
			break
		}
	}
	require.NotZero(t, refusedAfter, "writes past the limit must drop the full cache")
	cs.mu.Lock()
	defer cs.mu.Unlock()
	assert.False(t, cs.fullCacheLoaded[e.Name])
	assert.Empty(t, cs.fullCache[e.Name], "a refused cache is dropped, not kept and ignored")
	assert.Zero(t, cs.fullCacheBytes[e.Name])
}

// The running total follows the boxes, not the id count: replacing an id with a larger value
// grows it, and removing an id shrinks it. Getting either wrong would let it drift away from
// what is held and make the check meaningless, in whichever direction it drifted.
func TestChainStore_SetEntities_TracksFullCacheBytesThroughUpdatesAndDeletes(t *testing.T) {
	cs, fs, _ := newTestChainStore(t)
	e := blobEntity(t)
	cs.mu.Lock()
	cs.fullCacheRefused[e.Name] = false
	cs.fullIDCacheRefused[e.Name] = false
	cs.mu.Unlock()

	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 0, nil }
	fs.avgRowBytes_ = func(context.Context, *schema.Entity) (uint64, error) { return 0, nil }
	fs.listEntities_ = func(
		context.Context, *schema.Entity, []persistent.EntityFilter, bool, int,
	) ([]*entityRow, error) {
		return nil, nil
	}
	fs.getEntity_ = func(context.Context, *schema.Entity, string) (*entityRow, error) { return nil, nil }

	_, _, err := cs.GetEntity(context.Background(), e, "id-0")
	require.NoError(t, err)

	held := func() uint64 {
		cs.mu.Lock()
		defer cs.mu.Unlock()
		require.False(t, cs.fullCacheRefused[e.Name], "this test stays under the limit")
		assert.Equal(t, measuredBytes(cs.fullCache[e.Name]), cs.fullCacheBytes[e.Name])
		return cs.fullCacheBytes[e.Name]
	}

	_, err = cs.SetEntities(context.Background(), e, blobBoxes(0, 4, strings.Repeat("x", 512)))
	require.NoError(t, err)
	small := held()
	assert.NotZero(t, small)

	// same four ids, a larger payload each
	_, err = cs.SetEntities(context.Background(), e, blobBoxes(0, 4, strings.Repeat("x", 4096)))
	require.NoError(t, err)
	grown := held()
	assert.Greater(t, grown, small, "an update to a larger value must not be counted as free")

	// a delete is a box with nil Data
	deletes := blobBoxes(0, 2, "")
	for i := range deletes {
		deletes[i].Data = nil
	}
	_, err = cs.SetEntities(context.Background(), e, deletes)
	require.NoError(t, err)
	assert.Less(t, held(), grown, "a removed id must give its bytes back")
}

// A versioned-collapsing delete leaves a box behind to keep its version, and that box holds no
// data. Sizing the cache by the data alone would make those entries free, so creating and
// deleting ids could grow it without ever moving the total meant to limit it -- unbounded growth
// again, reached by holding nothing.
func TestChainStore_SetEntities_CountsEntriesThatHoldNoData(t *testing.T) {
	cs, fs, _ := newTestChainStore(t)
	fs.versionedCollapsing = true
	e := blobEntity(t)
	cs.mu.Lock()
	cs.fullCacheRefused[e.Name] = false
	cs.fullIDCacheRefused[e.Name] = false
	cs.fullCacheMaxBytes = 64 << 10
	cs.mu.Unlock()

	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 0, nil }
	fs.avgRowBytes_ = func(context.Context, *schema.Entity) (uint64, error) { return 0, nil }
	fs.listEntities_ = func(
		context.Context, *schema.Entity, []persistent.EntityFilter, bool, int,
	) ([]*entityRow, error) {
		return nil, nil
	}
	fs.getEntity_ = func(context.Context, *schema.Entity, string) (*entityRow, error) { return nil, nil }
	// reached once the cache is refused and writes fall through to the id-cache path
	fs.getAllID_ = func(context.Context, *schema.Entity) (*idSet, error) { return newIDSet(), nil }

	_, _, err := cs.GetEntity(context.Background(), e, "id-0")
	require.NoError(t, err)

	// each round writes fresh ids and deletes them again: nothing live accumulates, only the
	// tombstones the versioned path keeps
	var refused bool
	for round := 0; round < 64 && !refused; round++ {
		boxes := blobBoxes(round*8, 8, "x")
		_, err = cs.SetEntities(context.Background(), e, boxes)
		require.NoError(t, err)
		for i := range boxes {
			boxes[i].Data = nil
		}
		_, err = cs.SetEntities(context.Background(), e, boxes)
		require.NoError(t, err)
		cs.mu.Lock()
		refused = cs.fullCacheRefused[e.Name]
		if !refused {
			require.NotEmpty(t, cs.fullCache[e.Name], "the versioned path keeps deleted ids")
			assert.Equal(t, measuredBytes(cs.fullCache[e.Name]), cs.fullCacheBytes[e.Name])
		}
		cs.mu.Unlock()
	}
	assert.True(t, refused, "entries holding no data must still count against the limit")
}

// The estimate before a load is an average over the rows an id keeps, so it can fall short of
// what the load must materialize. When that shortfall shows up as ClickHouse refusing the query,
// the cache is refused and the read carries on: returning the error instead fails the run, and
// since the load is on the way to the first read, it fails the same way on every retry -- the
// restart loop this whole change exists to end.
func TestChainStore_EnsureCaches_RefusesAFullLoadTheQueryLimitRejects(t *testing.T) {
	cs, fs, _ := newTestChainStore(t)
	e := blobEntity(t)
	cs.mu.Lock()
	cs.fullCacheRefused[e.Name] = false
	cs.fullIDCacheRefused[e.Name] = false
	cs.mu.Unlock()

	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 1, nil }
	// an id whose retained rows average small: the estimate passes
	fs.avgRowBytes_ = func(context.Context, *schema.Entity) (uint64, error) { return 16, nil }
	fs.listEntities_ = func(
		context.Context, *schema.Entity, []persistent.EntityFilter, bool, int,
	) ([]*entityRow, error) {
		return nil, errors.New("close clickhouse rows failed: code: 241, message: " +
			"Query memory limit exceeded: would use 12.09 GiB, maximum: 12.00 GiB")
	}
	fs.getAllID_ = func(context.Context, *schema.Entity) (*idSet, error) { return newIDSet("id-0"), nil }
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		return &entityRow{EntityBox: persistent.EntityBox{
			Entity: e.Name, ID: id, Data: map[string]any{"id": id, "payload": "v"},
		}}, nil
	}

	box, _, err := cs.GetEntity(context.Background(), e, "id-0")
	require.NoError(t, err, "a cache too large to load must not fail the read")
	assert.Equal(t, "v", box.Data["payload"])
	cs.mu.Lock()
	defer cs.mu.Unlock()
	assert.True(t, cs.fullCacheRefused[e.Name])
	assert.False(t, cs.fullCacheLoaded[e.Name])
	assert.True(t, cs.fullIDCacheLoaded[e.Name], "the refusal falls through to the id cache")
}

// The same shortfall without the query failing: the load succeeds and measures over the limit.
// What was loaded is measured, so it overrules the estimate that admitted it.
func TestChainStore_EnsureCaches_RefusesAFullLoadThatMeasuresOverTheLimit(t *testing.T) {
	cs, fs, _ := newTestChainStore(t)
	e := blobEntity(t)
	cs.mu.Lock()
	cs.fullCacheRefused[e.Name] = false
	cs.fullIDCacheRefused[e.Name] = false
	cs.fullCacheMaxBytes = 64 << 10
	cs.mu.Unlock()

	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 1, nil }
	fs.avgRowBytes_ = func(context.Context, *schema.Entity) (uint64, error) { return 16, nil }
	payload := strings.Repeat("x", 128<<10)
	fs.listEntities_ = func(
		context.Context, *schema.Entity, []persistent.EntityFilter, bool, int,
	) ([]*entityRow, error) {
		return []*entityRow{{EntityBox: persistent.EntityBox{
			Entity: e.Name, ID: "id-0", Data: map[string]any{"id": "id-0", "payload": payload},
		}}}, nil
	}
	fs.getAllID_ = func(context.Context, *schema.Entity) (*idSet, error) { return newIDSet("id-0"), nil }
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		return &entityRow{EntityBox: persistent.EntityBox{
			Entity: e.Name, ID: id, Data: map[string]any{"id": id, "payload": "v"},
		}}, nil
	}

	_, _, err := cs.GetEntity(context.Background(), e, "id-0")
	require.NoError(t, err)
	cs.mu.Lock()
	defer cs.mu.Unlock()
	assert.True(t, cs.fullCacheRefused[e.Name])
	assert.False(t, cs.fullCacheLoaded[e.Name])
	assert.Empty(t, cs.fullCache[e.Name], "an over-limit load is not installed")
	assert.Zero(t, cs.fullCacheBytes[e.Name])
	assert.True(t, cs.fullIDCacheLoaded[e.Name], "the refusal falls through to the id cache")
}

// An entity type whose size cannot be measured is refused rather than loaded blind: an
// unmeasurable type is exactly the case the limit exists for.
func TestChainStore_EnsureCaches_RefusesTheFullCacheItCannotMeasure(t *testing.T) {
	cs, fs, _ := newTestChainStore(t)
	e := sparseEntity(t)
	cs.mu.Lock()
	cs.fullCacheRefused[e.Name] = false
	cs.fullIDCacheRefused[e.Name] = true
	cs.mu.Unlock()

	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 1, nil }
	fs.avgRowBytes_ = func(context.Context, *schema.Entity) (uint64, error) {
		return 0, errors.New("part metadata unavailable")
	}
	fs.listEntities_ = func(
		context.Context, *schema.Entity, []persistent.EntityFilter, bool, int,
	) ([]*entityRow, error) {
		return nil, errors.New("an unmeasured entity must not be loaded")
	}
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		return positionRow(id, 1), nil
	}

	// the read still succeeds, off the LRU + store path
	box, _, err := cs.GetEntity(context.Background(), e, "a")
	assert.NoError(t, err)
	assert.Equal(t, int32(1), box.Data["balance"])
	cs.mu.Lock()
	defer cs.mu.Unlock()
	assert.True(t, cs.fullCacheRefused[e.Name])
	assert.False(t, cs.fullCacheLoaded[e.Name])
}

func TestChainStore_EnsureCaches_DiscardsALoadOlderThanAWrite(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	cs.fullIDCacheRefused[e.Name] = false
	b := newBlockingIO()
	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 1, nil }
	fs.getAllID_ = func(context.Context, *schema.Entity) (*idSet, error) {
		b.wait()
		return newIDSet("old"), nil
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
	fs.getAllID_ = func(context.Context, *schema.Entity) (*idSet, error) {
		return newIDSet("old", "new"), nil
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
	fs.getAllID_ = func(context.Context, *schema.Entity) (*idSet, error) {
		b.wait()
		return newIDSet("old"), nil
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
	fs.getAllID_ = func(context.Context, *schema.Entity) (*idSet, error) {
		return newIDSet("old", "new"), nil
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
	fs.reorg_ = func(context.Context, int64) (bool, error) {
		b.wait()
		return true, nil
	}
	done := make(chan error, 1)
	go func() { done <- cs.Reorg(context.Background(), 10) }()
	<-b.started
	// reads bypass the caches during the reorg, so this is a store read; it must not wait for it
	var direct atomic.Int64
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		direct.Add(1)
		return positionRow(id, 1), nil
	}
	start := time.Now()
	_, _, err := cs.GetEntity(context.Background(), e, "hot")
	assert.NoError(t, err)
	assert.Less(t, time.Since(start), time.Second)
	assert.Equal(t, int64(1), direct.Load())
	close(b.release)
	assert.NoError(t, <-done)
	// the reorg deleted rows: the caches are purged and what was read during it is not kept
	assert.Equal(t, 0, cs.lruCache.Len())
	cs.mu.Lock()
	assert.False(t, cs.reorging)
	cs.mu.Unlock()
}

func TestChainStore_Reorg_KeepsTheCachesWhenNothingWasDeleted(t *testing.T) {
	cs, fs, e := newTestChainStore(t)
	seedLRU(t, cs, fs, e, "hot")
	cs.mu.Lock()
	cs.fullIDCacheRefused[e.Name] = false
	cs.mu.Unlock()
	fs.countEntity_ = func(context.Context, *schema.Entity, bool) (uint64, error) { return 1, nil }
	fs.getAllID_ = func(context.Context, *schema.Entity) (*idSet, error) { return newIDSet("hot"), nil }
	_, _, err := cs.GetEntity(context.Background(), e, "missing") // loads the full-ID cache
	assert.NoError(t, err)
	cs.mu.Lock()
	assert.True(t, cs.fullIDCacheLoaded[e.Name])
	cs.mu.Unlock()

	b := newBlockingIO()
	fs.reorg_ = func(context.Context, int64) (bool, error) {
		b.wait()
		return false, nil
	}
	done := make(chan error, 1)
	go func() { done <- cs.Reorg(context.Background(), 10) }()
	<-b.started
	// during the reorg a read goes to the store even though the caches are loaded
	var direct atomic.Int64
	fs.getEntity_ = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		direct.Add(1)
		return positionRow(id, 1), nil
	}
	box, fromCache, err := cs.GetEntity(context.Background(), e, "hot")
	assert.NoError(t, err)
	assert.False(t, fromCache)
	assert.Equal(t, int32(1), box.Data["balance"])
	assert.Equal(t, int64(1), direct.Load())
	close(b.release)
	assert.NoError(t, <-done)

	// nothing was deleted: the caches survive the reorg and answer as before
	cs.mu.Lock()
	assert.True(t, cs.fullIDCacheLoaded[e.Name])
	assert.False(t, cs.reorging)
	cs.mu.Unlock()
	assert.Less(t, lruHit(t, cs, e, "hot"), time.Second)
	box, _, err = cs.GetEntity(context.Background(), e, "missing")
	assert.NoError(t, err)
	assert.Nil(t, box)
	assert.Equal(t, int64(1), direct.Load(), "answered by the full-ID cache, not the store")
}
