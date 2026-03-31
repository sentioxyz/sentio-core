package concurrency

import (
	"context"
	"sentioxyz/sentio-core/common/log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_resourceWaiter1(t *testing.T) {
	testFunc := func() {
		w := NewResourceWaiter[int]()
		w.NewResource(1)

		var wg sync.WaitGroup
		var q []string

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.Wait(context.Background(), func(i int) bool {
				return i < 10
			})
			q = append(q, "done")
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			q = append(q, "+2")
			w.NewResource(2)
			q = append(q, "-1")
			w.ResourceReady(1)
			q = append(q, "+3")
			w.NewResource(3)
			q = append(q, "-2")
			w.ResourceReady(2)
			q = append(q, "-3")
			w.ResourceReady(3)
		}()

		wg.Wait()

		assert.Equal(t, []string{"+2", "-1", "+3", "-2", "-3", "done"}, q)
	}
	for i := 0; i < 1000; i++ {
		testFunc()
	}

}

func Test_resourceWaiter2(t *testing.T) {
	testFunc := func() {
		w := NewResourceWaiter[int]()
		w.NewResource(1)

		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		var q []string

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.Wait(ctx, func(i int) bool {
				return i < 10
			})
			q = append(q, "done")
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			q = append(q, "+2")
			w.NewResource(2)
			q = append(q, "-1")
			w.ResourceReady(1)
			q = append(q, "+3")
			w.NewResource(3)
			q = append(q, "-2")
			w.ResourceReady(2)
			q = append(q, "cancel")
			cancel()
		}()

		wg.Wait()

		assert.Equal(t, []string{"+2", "-1", "+3", "-2", "cancel", "done"}, q)
	}
	for i := 0; i < 1000; i++ {
		testFunc()
	}
}

func Test_resourceWaiter_cancel(t *testing.T) {
	testFunc := func() {
		w := NewResourceWaiter[int]()
		w.NewResource(1)

		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup

		for j := 0; j < 2; j++ { // consumers
			wg.Add(1)
			go func(cid int) {
				defer wg.Done()
				log.Infof("consumer #%d start", cid)
				_ = w.Wait(ctx, func(r int) bool {
					time.Sleep(time.Millisecond * 100)
					log.Infof("consumer #%d selector", cid)
					return r == 1
				})
				log.Infof("consumer #%d done", cid)
			}(j)
		}
		time.Sleep(time.Millisecond * 300) // make sure all consumers waiting
		log.Infof("all consumers waiting")

		wg.Add(1)
		go func() { // producer
			defer wg.Done()
			w.ResourceReady(1)
		}()

		time.Sleep(time.Millisecond * 100) // wait producer started to send ready signal
		cancel()
		log.Infof("ctx canceled")

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second * 5):
			t.Fatalf("dead lock")
		}
	}
	for i := 0; i < 10; i++ {
		log.Infof("round #%d startd", i)
		testFunc()
	}
}
