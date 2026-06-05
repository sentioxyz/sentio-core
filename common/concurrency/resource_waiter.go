package concurrency

import (
	"context"
	"fmt"
	"sync"
)

type ResourceSelector[R comparable] func(R) bool

type waiter[R comparable] struct {
	selector ResourceSelector[R]
	// notify is a buffered(1) wakeup signal. Producers coalesce wakeups into it with a
	// non-blocking send (see signal); the waiter recomputes its missing count from notReady
	// under the lock after each wakeup. Nothing is delivered through the channel itself, so
	// there is no blocking send under the lock and no close/send race.
	notify chan struct{}
}

type ResourceWaiter[R comparable] struct {
	notReady map[R]struct{}
	waiters  map[int]*waiter[R]
	waiterID int
	mu       sync.Mutex
}

func NewResourceWaiter[R comparable]() *ResourceWaiter[R] {
	return &ResourceWaiter[R]{
		notReady: make(map[R]struct{}),
		waiters:  make(map[int]*waiter[R]),
	}
}

// signal performs a non-blocking, coalescing wakeup on a buffered(1) channel: if a wakeup is
// already pending it is a no-op. It never blocks, so it is safe to call while holding a lock.
func trySignal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (w *ResourceWaiter[R]) wakeMatching(r R) {
	for _, wa := range w.waiters {
		if wa.selector(r) {
			trySignal(wa.notify)
		}
	}
}

func (w *ResourceWaiter[R]) NewResource(r R) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, has := w.notReady[r]; has {
		return // already has, do nothing
	}
	w.notReady[r] = struct{}{}
	w.wakeMatching(r)
}

func (w *ResourceWaiter[R]) ResourceReady(r R) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, has := w.notReady[r]; !has {
		panic(fmt.Errorf("no resource %v", r))
	}
	delete(w.notReady, r)
	w.wakeMatching(r)
}

// countMissing returns how many not-ready resources match selector. Caller must hold w.mu.
func (w *ResourceWaiter[R]) countMissing(selector ResourceSelector[R]) int {
	missing := 0
	for r := range w.notReady {
		if selector(r) {
			missing++
		}
	}
	return missing
}

func (w *ResourceWaiter[R]) Wait(ctx context.Context, selector ResourceSelector[R]) error {
	// Register first, then count under the same lock, so no readiness change is missed.
	w.mu.Lock()
	id := w.waiterID
	me := &waiter[R]{
		selector: selector,
		notify:   make(chan struct{}, 1),
	}
	w.waiterID++
	w.waiters[id] = me
	missing := w.countMissing(selector)
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		delete(w.waiters, id)
		w.mu.Unlock()
	}()

	for missing != 0 {
		select {
		case <-me.notify:
			w.mu.Lock()
			missing = w.countMissing(selector)
			w.mu.Unlock()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
