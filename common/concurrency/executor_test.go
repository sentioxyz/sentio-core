package concurrency

import (
	"context"
	"strconv"
	"strings"
	"testing"

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
