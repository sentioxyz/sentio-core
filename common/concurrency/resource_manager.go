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
	q.curResource += num
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
	// adjust the queue
	tail := len(q.waiters) - 1
	q.waiters[0] = q.waiters[tail]
	q.waiters = q.waiters[:tail]
	n := 0
	for {
		right := (n + 1) * 2
		left := right - 1
		if left >= tail {
			// n is leaf
			break
		}
		next := left
		if right < tail && q.waiters[right].inFront(q.waiters[next]) {
			next = right
		}
		if q.waiters[next].inFront(q.waiters[n]) {
			q.waiters[next], q.waiters[n] = q.waiters[n], q.waiters[next]
			n = next
		} else {
			break
		}
	}
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
	n := len(q.waiters) - 1
	for n > 0 {
		up := (n - 1) / 2
		if q.waiters[n].inFront(q.waiters[up]) {
			q.waiters[n], q.waiters[up] = q.waiters[up], q.waiters[n]
			n = up
		} else {
			break
		}
	}
	return ch
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
			return nil, ctx.Err()
		case <-ch:
			return func() { q.release(num) }, nil
		case <-ticker.C:
			waitingCallback(time.Since(startAt))
		}
	}
}
