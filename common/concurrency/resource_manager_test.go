package concurrency

import (
	"context"
	"github.com/stretchr/testify/assert"
	"sync"
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
