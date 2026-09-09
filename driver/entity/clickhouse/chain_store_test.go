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
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// GetEntity reads the store on an LRU miss without holding the cache lock, so that concurrent
// reads overlap. These tests drive the real ChainStore with the store read replaced by a fake
// (readEntity), and check the overlap and the guards that keep a row read before a write or a
// purge out of the LRU.

const chainStoreTestSchema = `
type Position @entity {
  id: ID!
  balance: Int!
}
`

func newTestChainStore(t *testing.T) (*ChainStore, *schema.Entity) {
	t.Helper()
	sch, err := schema.ParseAndVerifySchema(chainStoreTestSchema)
	require.NoError(t, err)
	e := sch.GetEntity("Position")
	cs := NewChainStore(nil, "chain", 1000, 1<<20, 1000)
	// neither full cache can be loaded without a store; the LRU + store path is under test
	cs.fullCacheRefused[e.Name] = true
	cs.fullIDCacheRefused[e.Name] = true
	return cs, e
}

func positionRow(id string, balance int32) *entityRow {
	return &entityRow{EntityBox: persistent.EntityBox{
		Entity: "Position", ID: id, Data: map[string]any{"id": id, "balance": balance},
	}}
}

func TestChainStore_GetEntity_ReadsOverlapOffTheLock(t *testing.T) {
	cs, e := newTestChainStore(t)
	var reads, inFlight, maxInFlight atomic.Int64
	cs.readEntity = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
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

// blockingRead returns a readEntity that parks every read until release is closed and reports
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
	cs, e := newTestChainStore(t)
	read, started, release := blockingRead(positionRow("p", 1))
	cs.readEntity = read

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
	cs.readEntity = func(context.Context, *schema.Entity, string) (*entityRow, error) {
		return nil, errors.New("must be served from the LRU")
	}
	box, _, err := cs.GetEntity(context.Background(), e, "p")
	assert.NoError(t, err)
	assert.Equal(t, int32(2), box.Data["balance"])
}

func TestChainStore_GetEntity_DoesNotCacheARowOlderThanAPurge(t *testing.T) {
	cs, e := newTestChainStore(t)
	read, started, release := blockingRead(positionRow("p", 1))
	cs.readEntity = read

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
	cs.readEntity = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
		reads.Add(1)
		return positionRow(id, 3), nil
	}
	box, _, err := cs.GetEntity(context.Background(), e, "p")
	assert.NoError(t, err)
	assert.Equal(t, int32(3), box.Data["balance"])
	assert.Equal(t, int64(1), reads.Load())
}

func TestChainStore_GetEntity_StoreErrorAndMissing(t *testing.T) {
	cs, e := newTestChainStore(t)
	cs.readEntity = func(_ context.Context, _ *schema.Entity, id string) (*entityRow, error) {
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
