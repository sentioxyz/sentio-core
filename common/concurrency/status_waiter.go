package concurrency

import (
	"context"
	"sync"
)

type StatusChecker[STATUS any] func(STATUS) bool

type statusWaiter[STATUS any] struct {
	checker StatusChecker[STATUS]
	// notify is a buffered(1) wakeup signal: producers coalesce wakeups into it without
	// blocking. result carries the status that satisfied the checker; it is written and read
	// only under StatusWaiter.mu (a coalesced trySignal may skip the send, so the channel alone
	// does not order the latest write — the lock does).
	notify chan struct{}
	result STATUS
}

type StatusWaiter[STATUS any] struct {
	current  STATUS
	waiters  map[int]*statusWaiter[STATUS]
	waiterID int
	mu       sync.Mutex
}

func NewStatusWaiter[STATUS any](initial STATUS) *StatusWaiter[STATUS] {
	return &StatusWaiter[STATUS]{
		current: initial,
		waiters: make(map[int]*statusWaiter[STATUS]),
	}
}

func (w *StatusWaiter[STATUS]) NewStatus(s STATUS) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.current = s
	for _, wa := range w.waiters {
		if wa.checker(s) {
			wa.result = s
			trySignal(wa.notify)
		}
	}
}

func (w *StatusWaiter[STATUS]) Current() STATUS {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.current
}

func (w *StatusWaiter[STATUS]) Waiting() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.waiters)
}

func (w *StatusWaiter[STATUS]) Wait(ctx context.Context, checker StatusChecker[STATUS]) (STATUS, error) {
	w.mu.Lock()
	if checker(w.current) {
		s := w.current
		w.mu.Unlock()
		return s, nil
	}
	id := w.waiterID
	me := &statusWaiter[STATUS]{
		checker: checker,
		notify:  make(chan struct{}, 1),
	}
	w.waiterID++
	w.waiters[id] = me
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		delete(w.waiters, id)
		w.mu.Unlock()
	}()

	// A notify is only sent after a status satisfied this waiter's checker (see NewStatus), so
	// a single wakeup is enough — read the recorded result under the lock and return it.
	select {
	case <-me.notify:
		w.mu.Lock()
		s := me.result
		w.mu.Unlock()
		return s, nil
	case <-ctx.Done():
		var zero STATUS
		return zero, ctx.Err()
	}
}
