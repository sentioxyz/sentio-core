package concurrency

import (
	"context"
	"fmt"
	"sync"
)

type ResourceSelector[R comparable] func(R) bool

type waiter[R comparable] struct {
	selector   ResourceSelector[R]
	microphone chan int
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

func sendIgnoreClosed[V any](c chan<- V, v V) {
	defer func() {
		_ = recover() // ignore closed
	}()
	c <- v
}

func (w *ResourceWaiter[R]) NewResource(r R) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, has := w.notReady[r]; has {
		return // already has, do nothing
	}
	w.notReady[r] = struct{}{}
	for _, wa := range w.waiters {
		if wa.selector(r) {
			sendIgnoreClosed(wa.microphone, 1)
		}
	}
}

func (w *ResourceWaiter[R]) ResourceReady(r R) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, has := w.notReady[r]; !has {
		panic(fmt.Errorf("no resource %v", r))
	}
	delete(w.notReady, r)
	for _, wa := range w.waiters {
		if wa.selector(r) {
			sendIgnoreClosed(wa.microphone, -1)
		}
	}
}

func (w *ResourceWaiter[R]) Wait(ctx context.Context, selector ResourceSelector[R]) error {
	// new waiter
	w.mu.Lock()
	var missing int
	for r := range w.notReady {
		if selector(r) {
			missing++
		}
	}
	if missing == 0 {
		w.mu.Unlock()
		return nil
	}
	id := w.waiterID
	me := &waiter[R]{
		selector:   selector,
		microphone: make(chan int, 0),
	}
	w.waiterID++
	w.waiters[id] = me
	w.mu.Unlock()

	defer func() {
		// Some producers may be using w.mu and sending signal to me.microphone
		// Here close me.microphone first to make sure the producer will not be blocked by me.microphone
		close(me.microphone)
		w.mu.Lock()
		delete(w.waiters, id)
		w.mu.Unlock()
	}()

	for missing != 0 {
		select {
		case delta := <-me.microphone:
			missing += delta
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
