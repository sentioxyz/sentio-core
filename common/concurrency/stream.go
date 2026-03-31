package concurrency

import (
	"context"

	"sentioxyz/sentio-core/common/errgroup"
)

func InCheck[T any](g *errgroup.Group, ctx context.Context, up <-chan T, checkers ...func(int, T) error) <-chan T {
	if len(checkers) == 0 {
		return up
	}
	down := make(chan T)
	g.Go(func() error {
		defer close(down)
		return ForEach(ctx, up, func(ctx context.Context, index int, item T) error {
			for _, checker := range checkers {
				if err := checker(index, item); err != nil {
					return err
				}
			}
			select {
			case down <- item:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	})
	return down
}

func ReadAll[T any](ctx context.Context, ch <-chan T) ([]T, error) {
	var result []T
	for {
		select {
		case item, has := <-ch:
			if !has {
				return result, nil
			}
			result = append(result, item)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func WriteAll[T any](ctx context.Context, src []T, ch chan<- T) (int, error) {
	var count int
	for _, item := range src {
		select {
		case ch <- item:
			count++
		case <-ctx.Done():
			return count, ctx.Err()
		}
	}
	return count, nil
}

func ForEach[T any](ctx context.Context, ch <-chan T, fn func(ctx context.Context, index int, v T) error) error {
	for index := 0; ; index++ {
		select {
		case v, has := <-ch:
			if !has {
				return nil
			}
			if err := fn(ctx, index, v); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
