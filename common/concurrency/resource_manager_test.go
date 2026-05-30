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
				result = append(result, p)
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
				errList = append(errList, err)
			} else {
				t.Logf("#%d got", p)
				result = append(result, p)
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
