package concurrency

import (
	"context"
	"github.com/pkg/errors"
	"sync"
	"time"
)

type resourceMgrWaiter struct {
	priority int64
	index    int
	num      int
	ch       chan struct{}
}

func (w resourceMgrWaiter) inFront(a resourceMgrWaiter) bool {
	if w.priority != a.priority {
		return w.priority < a.priority
	}
	return w.index < a.index
}

type ResourceManager struct {
	maxResource int

	mu          sync.Mutex
	curResource int
	count       int
	waiters     []resourceMgrWaiter // binary heap start from 0, right of n is (n+1)*2 and left of n is (n+1)*2-1
}

func NewResourceManager(resourceCount int) *ResourceManager {
	return &ResourceManager{
		maxResource: resourceCount,
		curResource: resourceCount,
	}
}

func (q *ResourceManager) release(num int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.releaseLocked(num)
}

// releaseLocked returns num resources to the pool and grants the queue leader
// if it can now be satisfied. Caller must hold q.mu.
func (q *ResourceManager) releaseLocked(num int) {
	q.curResource += num
	q.grantLocked()
}

// grantLocked wakes the queue leader if the current resources can satisfy it.
// Caller must hold q.mu.
func (q *ResourceManager) grantLocked() {
	if len(q.waiters) == 0 {
		// no one is waiting
		return
	}
	w := q.waiters[0]
	if q.curResource < w.num {
		// current resource cannot meet the requirement of the queue leader
		return
	}
	// the queue leader will got the resource and leave the queue
	// notice the queue leader
	close(w.ch)
	q.curResource -= w.num
	q.removeWaiterAt(0)
}

func (q *ResourceManager) addWaiter(priority int64, num int) chan struct{} {
	ch := make(chan struct{})
	q.waiters = append(q.waiters, resourceMgrWaiter{
		priority: priority,
		num:      num,
		index:    q.count,
		ch:       ch,
	})
	q.count++
	q.siftUp(len(q.waiters) - 1)
	return ch
}

// cancelWaiter removes the waiter identified by ch from the queue after its
// context was canceled. If a concurrent release() already granted the waiter
// (a race between ctx.Done() and the channel close), the granted resources are
// returned to the pool since no one will ever call release for them.
func (q *ResourceManager) cancelWaiter(ch chan struct{}, num int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.waiters {
		if q.waiters[i].ch == ch {
			q.removeWaiterAt(i)
			// removing the (possibly leader) waiter may unblock the next one
			q.grantLocked()
			return
		}
	}
	// not in the queue anymore: it was already granted by a concurrent
	// release(), which deducted the resources on our behalf. Give them back.
	q.releaseLocked(num)
}

// removeWaiterAt removes the waiter at heap index i and restores the heap.
// Caller must hold q.mu.
func (q *ResourceManager) removeWaiterAt(i int) {
	tail := len(q.waiters) - 1
	if i != tail {
		q.waiters[i] = q.waiters[tail]
	}
	q.waiters[tail] = resourceMgrWaiter{} // avoid retaining the channel
	q.waiters = q.waiters[:tail]
	if i < len(q.waiters) {
		// the moved element may need to go either way
		if !q.siftDown(i) {
			q.siftUp(i)
		}
	}
}

// siftUp moves the element at index n towards the root until the heap order is
// restored. Caller must hold q.mu.
func (q *ResourceManager) siftUp(n int) {
	for n > 0 {
		up := (n - 1) / 2
		if q.waiters[n].inFront(q.waiters[up]) {
			q.waiters[n], q.waiters[up] = q.waiters[up], q.waiters[n]
			n = up
		} else {
			break
		}
	}
}

// siftDown moves the element at index n towards the leaves until the heap order
// is restored. It reports whether the element actually moved. Caller must hold
// q.mu.
func (q *ResourceManager) siftDown(n int) bool {
	start := n
	size := len(q.waiters)
	for {
		right := (n + 1) * 2
		left := right - 1
		if left >= size {
			// n is leaf
			break
		}
		next := left
		if right < size && q.waiters[right].inFront(q.waiters[next]) {
			next = right
		}
		if !q.waiters[next].inFront(q.waiters[n]) {
			break
		}
		q.waiters[next], q.waiters[n] = q.waiters[n], q.waiters[next]
		n = next
	}
	return n > start
}

func (q *ResourceManager) Cap() int {
	return q.maxResource
}

func (q *ResourceManager) Snapshot() any {
	q.mu.Lock()
	defer q.mu.Unlock()
	return map[string]any{
		"max":     q.maxResource,
		"current": q.curResource,
		"applied": q.count,
		"waiting": len(q.waiters),
	}
}

func (q *ResourceManager) Apply(
	ctx context.Context,
	priority int64,
	num int,
	noticeInterval time.Duration,
	waitingCallback func(dur time.Duration),
) (func(), error) {
	if num > q.maxResource {
		return nil, errors.Errorf("num %d exceeds max resource %d", num, q.maxResource)
	}

	startAt := time.Now()
	q.mu.Lock()
	if q.curResource >= num {
		q.curResource -= num
		q.count++
		q.mu.Unlock()
		return func() { q.release(num) }, nil
	}
	ch := q.addWaiter(priority, num)
	q.mu.Unlock()

	if noticeInterval == 0 || waitingCallback == nil {
		select {
		case <-ctx.Done():
			q.cancelWaiter(ch, num)
			return nil, ctx.Err()
		case <-ch:
			return func() { q.release(num) }, nil
		}
	}

	ticker := time.NewTicker(noticeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			q.cancelWaiter(ch, num)
			return nil, ctx.Err()
		case <-ch:
			return func() { q.release(num) }, nil
		case <-ticker.C:
			waitingCallback(time.Since(startAt))
		}
	}
}
