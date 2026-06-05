package concurrency

import (
	"context"
	"github.com/stretchr/testify/assert"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func Test_resMgr_normal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := NewResourceManager(10)
	var g sync.WaitGroup
	var mu sync.Mutex
	var result []int
	for i := 1; i <= 10; i++ {
		g.Add(1)
		time.Sleep(time.Millisecond * 100)
		go func(p int) {
			defer g.Done()
			release, err := q.Apply(ctx, int64(100-p), p, time.Second, func(dur time.Duration) {
				t.Logf("#%d waited %s", p, dur.String())
			})
			if err != nil {
				t.Errorf("#%d failed: %s", p, err)
			} else {
				t.Logf("#%d got", p)
				mu.Lock()
				result = append(result, p)
				mu.Unlock()
				time.Sleep(time.Second)
				release()
				t.Logf("#%d released", p)
			}
		}(i)
	}
	g.Wait()

	assert.Equal(t, []int{1, 2, 3, 4, 10, 9, 8, 7, 6, 5}, result)
}

func Test_resMgr_normal2(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := NewResourceManager(100)
	var g sync.WaitGroup
	for i := 1; i <= 10000; i++ {
		g.Add(1)
		go func(p int) {
			defer g.Done()
			release, err := q.Apply(ctx, 0, 1, 0, nil)
			if err != nil {
				t.Errorf("#%d failed: %s", p, err)
			} else {
				time.Sleep(time.Millisecond * 10)
				release()
			}
		}(i)
	}
	g.Wait()
}

// Test_resMgr_neverExceedsMax verifies that the total resources held by
// concurrent holders never exceed maxResource, even when holders are granted
// through the queue via release(). This guards against the release path
// failing to deduct curResource for a granted waiter, which would let
// concurrency drift past the configured maximum.
func Test_resMgr_neverExceedsMax(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const maxResource = 5
	q := NewResourceManager(maxResource)

	var inFlight int64
	var peak int64
	var g sync.WaitGroup
	for i := 0; i < 500; i++ {
		g.Add(1)
		// vary the requested amount so grants go through both the fast path
		// and the queue, and so release() wakes waiters needing > 1 unit.
		num := (i % maxResource) + 1
		go func(num int) {
			defer g.Done()
			release, err := q.Apply(ctx, 0, num, 0, nil)
			if err != nil {
				t.Errorf("apply failed: %s", err)
				return
			}
			held := atomic.AddInt64(&inFlight, int64(num))
			for {
				p := atomic.LoadInt64(&peak)
				if held <= p || atomic.CompareAndSwapInt64(&peak, p, held) {
					break
				}
			}
			assert.LessOrEqual(t, held, int64(maxResource),
				"in-flight resources %d exceeded max %d", held, maxResource)
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&inFlight, -int64(num))
			release()
		}(num)
	}
	g.Wait()

	t.Logf("peak in-flight: %d (max %d)", peak, maxResource)
	assert.Equal(t, int64(0), atomic.LoadInt64(&inFlight))

	// all resources must be fully returned to the pool
	q.mu.Lock()
	assert.Equal(t, maxResource, q.curResource)
	assert.Equal(t, 0, len(q.waiters))
	q.mu.Unlock()
}

func Test_resMgr_canceled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	q := NewResourceManager(10)
	var g sync.WaitGroup
	var mu sync.Mutex
	var result []int
	var errList []error
	for i := 1; i <= 10; i++ {
		g.Add(1)
		time.Sleep(time.Millisecond * 100)
		go func(p int) {
			defer g.Done()
			release, err := q.Apply(ctx, int64(100-p), p, 0, nil)
			if err != nil {
				t.Logf("#%d failed: %s", p, err)
				mu.Lock()
				errList = append(errList, err)
				mu.Unlock()
			} else {
				t.Logf("#%d got", p)
				mu.Lock()
				result = append(result, p)
				mu.Unlock()
				time.Sleep(time.Second * 2)
				release()
				t.Logf("#%d released", p)
			}
		}(i)
	}
	g.Wait()

	assert.Equal(t, []int{1, 2, 3, 4}, result)
	assert.Equal(t, 6, len(errList))
}

// Test_resMgr_canceledWaiterReleasesResource verifies that a waiter whose
// context is canceled while it is queued is removed from the queue and does
// not leak resources. Previously the canceled waiter stayed in the heap; a
// later release() would grant it (deducting curResource) but no one would ever
// call release for it, permanently shrinking the available pool.
func Test_resMgr_canceledWaiterReleasesResource(t *testing.T) {
	q := NewResourceManager(1)

	// hold the only resource
	release, err := q.Apply(context.Background(), 0, 1, 0, nil)
	assert.NoError(t, err)

	// a waiter queues up, then its context is canceled while waiting
	wctx, wcancel := context.WithCancel(context.Background())
	var g sync.WaitGroup
	g.Add(1)
	go func() {
		defer g.Done()
		_, werr := q.Apply(wctx, 0, 1, 0, nil)
		assert.ErrorIs(t, werr, context.Canceled)
	}()

	// let the waiter enter the queue, then cancel it
	time.Sleep(100 * time.Millisecond)
	q.mu.Lock()
	assert.Equal(t, 1, len(q.waiters), "waiter should be queued")
	q.mu.Unlock()
	wcancel()
	g.Wait()

	// the canceled waiter must have left the queue
	q.mu.Lock()
	assert.Equal(t, 0, len(q.waiters), "canceled waiter must be removed from queue")
	q.mu.Unlock()

	// releasing the held resource must fully restore the pool, and a fresh
	// Apply must succeed immediately (no resource was leaked)
	release()
	next, err := q.Apply(context.Background(), 0, 1, 0, nil)
	assert.NoError(t, err)
	next()

	q.mu.Lock()
	assert.Equal(t, 1, q.curResource, "no resource should be leaked")
	q.mu.Unlock()
}

// Test_resMgr_cancelHeadUnblocksNext verifies that canceling the queue leader
// promotes and grants a following waiter that the now-removed leader was
// blocking. grant only ever inspects the head, so a big leader can keep a
// satisfiable follower waiting until the leader leaves the queue.
func Test_resMgr_cancelHeadUnblocksNext(t *testing.T) {
	q := NewResourceManager(3)

	// fully occupy the pool: A holds 2 (whole test), B holds 1
	_, err := q.Apply(context.Background(), 0, 2, 0, nil)
	assert.NoError(t, err)
	relB, err := q.Apply(context.Background(), 0, 1, 0, nil)
	assert.NoError(t, err)

	// leader needs 3, higher priority -> queues at the front
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	var lg sync.WaitGroup
	lg.Add(1)
	go func() {
		defer lg.Done()
		_, lerr := q.Apply(leaderCtx, 0, 3, 0, nil)
		assert.ErrorIs(t, lerr, context.Canceled)
	}()
	time.Sleep(50 * time.Millisecond)

	// follower needs 1, lower priority -> queues behind the leader
	granted := make(chan struct{})
	go func() {
		rel, ferr := q.Apply(context.Background(), 1, 1, 0, nil)
		assert.NoError(t, ferr)
		close(granted)
		rel()
	}()
	time.Sleep(50 * time.Millisecond)

	q.mu.Lock()
	assert.Equal(t, 2, len(q.waiters))
	q.mu.Unlock()

	// free 1 unit: not enough for the leader (needs 3), and grant only checks
	// the head, so the follower stays blocked behind the leader
	relB()
	time.Sleep(50 * time.Millisecond)
	select {
	case <-granted:
		t.Fatal("follower should still be blocked behind the bigger leader")
	default:
	}

	// canceling the leader must promote and grant the follower from the free unit
	cancelLeader()
	lg.Wait()

	select {
	case <-granted:
		// follower was unblocked by the leader's cancellation
	case <-time.After(time.Second):
		t.Fatal("follower was not granted after leader cancellation")
	}
}
