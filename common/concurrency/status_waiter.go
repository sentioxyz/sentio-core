package concurrency

import (
	"context"
	"sync"
)

type StatusChecker[STATUS any] func(STATUS) bool

type statusWaiter[STATUS any] struct {
	checker    StatusChecker[STATUS]
	microphone chan STATUS
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
			sendIgnoreClosed(wa.microphone, s)
		}
	}
}

func (w *StatusWaiter[STATUS]) Current() STATUS {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.current
}

func (w *StatusWaiter[STATUS]) Wait(ctx context.Context, checker StatusChecker[STATUS]) (STATUS, error) {
	// new waiter
	w.mu.Lock()
	if checker(w.current) {
		w.mu.Unlock()
		return w.current, nil
	}
	id := w.waiterID
	me := &statusWaiter[STATUS]{
		checker:    checker,
		microphone: make(chan STATUS, 0),
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

	select {
	case s := <-me.microphone:
		return s, nil
	case <-ctx.Done():
		var zero STATUS
		return zero, ctx.Err()
	}
}
