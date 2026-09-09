package concurrency

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
)

type testTask struct {
	Data int64
}

func TestRunWithTaskChan(t *testing.T) {
	logger := log.NewZap()
	g, ctx := errgroup.WithContext(context.Background())
	taskChan := make(chan []testTask)

	RunWithTaskChan[[]testTask](g, ctx, 3, taskChan,
		func(ctx context.Context, task []testTask) error {
			s := make([]string, 0, len(task))
			for _, t := range task {
				s = append(s, strconv.FormatInt(t.Data, 10))
			}
			logger.Info("got",
				zap.Any("consumer", GetConsumerID(ctx)),
				zap.String("task", strings.Join(s, ",")))
			return nil
		})

	go func() {
		for i := 10; i < 20; i++ {
			taskChan <- []testTask{{Data: int64(i)}}
		}
		close(taskChan)
	}()

	assert.Equal(t, nil, g.Wait())
}

func TestRunWithTaskArray(t *testing.T) {
	t.Run("every task once, in order, with bounded consumers", func(t *testing.T) {
		tasks := make([]int, 100)
		for i := range tasks {
			tasks[i] = i
		}
		var mu sync.Mutex
		var seen []int
		var inFlight, maxInFlight atomic.Int64
		g, ctx := errgroup.WithContext(context.Background())
		RunWithTaskArray(g, ctx, 4, tasks, func(ctx context.Context, task int) error {
			n := inFlight.Add(1)
			defer inFlight.Add(-1)
			for {
				seenMax := maxInFlight.Load()
				if n <= seenMax || maxInFlight.CompareAndSwap(seenMax, n) {
					break
				}
			}
			assert.Less(t, GetConsumerID(ctx), 4)
			time.Sleep(time.Millisecond)
			mu.Lock()
			seen = append(seen, task)
			mu.Unlock()
			return nil
		})
		assert.NoError(t, g.Wait())
		assert.Len(t, seen, 100)
		sort.Ints(seen)
		assert.Equal(t, tasks, seen)
		assert.LessOrEqual(t, maxInFlight.Load(), int64(4))
		assert.Greater(t, maxInFlight.Load(), int64(1))
	})

	t.Run("no more consumers than tasks", func(t *testing.T) {
		var consumers atomic.Int64
		g, ctx := errgroup.WithContext(context.Background())
		RunWithTaskArray(g, ctx, 8, []int{1, 2}, func(ctx context.Context, _ int) error {
			consumers.Add(1)
			return nil
		})
		assert.NoError(t, g.Wait())
		assert.Equal(t, int64(2), consumers.Load())

		g, ctx = errgroup.WithContext(context.Background())
		RunWithTaskArray(g, ctx, 8, nil, func(ctx context.Context, _ int) error {
			t.Fatal("no task, no consumer call")
			return nil
		})
		assert.NoError(t, g.Wait())
	})

	t.Run("an error stops the rest", func(t *testing.T) {
		tasks := make([]int, 50)
		for i := range tasks {
			tasks[i] = i
		}
		var done atomic.Int64
		g, ctx := errgroup.WithContext(context.Background())
		RunWithTaskArray(g, ctx, 2, tasks, func(ctx context.Context, task int) error {
			if task == 3 {
				return errors.New("task 3 failed")
			}
			done.Add(1)
			time.Sleep(time.Millisecond)
			return nil
		})
		assert.EqualError(t, g.Wait(), "task 3 failed")
		assert.Less(t, done.Load(), int64(49))
	})
}

func TestRunWithProducer(t *testing.T) {
	logger := log.NewZap()
	g, ctx := errgroup.WithContext(context.Background())

	RunWithProducer[[]testTask](g, ctx, 3,
		func(ctx context.Context, taskChan chan<- []testTask) error {
			for i := 10; i < 20; i++ {
				taskChan <- []testTask{{Data: int64(i)}}
			}
			return nil
		},
		func(ctx context.Context, task []testTask) error {
			s := make([]string, 0, len(task))
			for _, t := range task {
				s = append(s, strconv.FormatInt(t.Data, 10))
			}
			logger.Info("got",
				zap.Any("consumer", GetConsumerID(ctx)),
				zap.String("task", strings.Join(s, ",")))
			return nil
		})

	assert.Equal(t, nil, g.Wait())
}
