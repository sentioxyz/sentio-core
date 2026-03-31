package concurrency

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
)

func Test_MapO2M(t *testing.T) {
	const count = 10
	const concurrency = 3
	const expand = 3

	startTime := time.Now()

	g, ctx := errgroup.WithContext(context.Background())

	out := make(chan int, count*expand)

	MapO2MWithProducer(
		g,
		ctx,
		concurrency,
		func(ctx context.Context, ch chan<- int) error {
			for i := 0; i < count; i++ {
				select {
				case ch <- i:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		},
		out,
		func(ctx context.Context, index int, task int, taskOut chan<- int) error {
			log.Debugf("index: %d, task: %d, Mapper ENTER", index, task)
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
			log.Debugf("index: %d, task: %d, Mapper READY", index, task)
			for i := 0; i < expand; i++ {
				select {
				case taskOut <- task*expand + i:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			log.Debugf("index: %d, task: %d, Mapper DONE", index, task)
			return nil
		})

	assert.Equal(t, nil, g.Wait())
	close(out)

	// time-consuming depends on concurrency
	used := time.Since(startTime)
	usedExp := time.Second * ((count-1)/concurrency + 1)
	const margin = time.Millisecond * 100
	if used > usedExp+margin {
		t.Fatalf("time used %s, expect is %s, margin is %s", used, usedExp, margin)
	}

	// the result is strictly ordered
	result, _ := ReadAll(context.Background(), out)
	var resultExp []int
	for i := 0; i < count*expand; i++ {
		resultExp = append(resultExp, i)
	}
	assert.Equal(t, resultExp, result)
}
