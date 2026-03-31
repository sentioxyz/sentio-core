package concurrency

import (
	"context"
	"sync"
	"testing"
	"time"
)

func Test_statusWaiter1(t *testing.T) {
	const num = 8
	testFunc := func(round int) bool {
		w := NewStatusWaiter[int](0)

		var wg sync.WaitGroup

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for i := 0; i <= num; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, _ = w.Wait(ctx, func(s int) bool {
					//t.Logf("[#%d] %d check %d", round, i, s)
					return s >= i
				})
				//t.Logf("[#%d] %d done", round, i)
			}(i)
		}
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		var q []int
		for i := 1; i < num; i++ {
			if round&(1<<(i-1)) > 0 {
				q = append(q, i)
			}
		}
		q = append(q, num)
		time.Sleep(time.Millisecond * 10) // wait waiter ready
		for _, x := range q {
			//t.Logf("[#%d] %d ok", round, x)
			w.NewStatus(x)
		}
		select {
		case <-done:
			t.Logf("[#%d] ok, queue: %v", round, q)
			return true
		case <-time.After(time.Second):
			t.Fatalf("[#%d] timeout, queue: %v", round, q)
			return false
		}
	}
	for i := 0; i < 1<<(num-1); i++ {
		if !testFunc(i) {
			return
		}
	}
}
