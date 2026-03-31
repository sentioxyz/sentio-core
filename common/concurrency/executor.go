package concurrency

import (
	"context"
	"fmt"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
)

type key struct {
}

var (
	ctxKeyConsumer = key{}
)

func GetConsumerID(ctx context.Context) int {
	consumerID := ctx.Value(ctxKeyConsumer)
	if consumerID == nil {
		return 0
	}
	return consumerID.(int)
}

func RunWithProducer[T any](
	g *errgroup.Group,
	ctx context.Context,
	concurrency int,
	producer func(ctx context.Context, taskChan chan<- T) error,
	consumer func(ctx context.Context, task T) error,
) {
	taskChan := make(chan T)
	g.Go(func() error {
		defer close(taskChan)
		return producer(ctx, taskChan)
	})
	RunWithTaskChan(g, ctx, concurrency, taskChan, consumer)
}

func RunWithTaskChan[T any](
	g *errgroup.Group,
	ctx context.Context,
	concurrency int,
	taskChan <-chan T,
	consumer func(ctx context.Context, task T) error,
) {
	for i := 0; i < concurrency; i++ {
		consumerID := i
		g.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case task, has := <-taskChan:
					if !has {
						return nil
					}
					if err := consumer(context.WithValue(ctx, ctxKeyConsumer, consumerID), task); err != nil {
						return err
					}
				}
			}
		})
	}
}

type Page struct {
	Num   int
	Start int
	End   int
}

func TraverseByPage[T any, R any](
	ctx context.Context,
	concurrency int,
	pageSize int,
	data []T,
	fn func(ctx context.Context, page Page, data []T) ([]R, error),
) ([]R, error) {
	if len(data) == 0 {
		return nil, nil
	}
	pageTotal := (len(data)-1)/pageSize + 1
	results := make([][]R, pageTotal)
	g, gctx := errgroup.WithContext(ctx)
	RunWithProducer(
		g, gctx, concurrency,
		func(ctx context.Context, taskChan chan<- Page) error {
			for cursor := 0; cursor < len(data); cursor += pageSize {
				select {
				case taskChan <- Page{
					Num:   cursor / pageSize,
					Start: cursor,
					End:   min(cursor+pageSize, len(data)),
				}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		},
		func(ctx context.Context, page Page) (err error) {
			pageTitle := fmt.Sprintf("%d/%d,%d/%d", page.Num, pageTotal, page.End-page.Start, len(data))
			pageCtx, _ := log.FromContext(ctx, "page", pageTitle)
			results[page.Num], err = fn(pageCtx, page, data[page.Start:page.End])
			return
		})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return utils.MergeArr(results...), nil
}
