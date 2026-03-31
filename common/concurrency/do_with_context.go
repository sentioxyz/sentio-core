package concurrency

import "context"

func DoWithCtx(ctx context.Context, fn func() error) error {
	var doErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		doErr = fn()
	}()
	select {
	case <-done:
		return doErr
	case <-ctx.Done():
		// fn will be left
		return ctx.Err()
	}
}
