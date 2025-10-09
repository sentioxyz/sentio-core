package errgroup

// There is a problem about golang.org/x/sync/errgroup after go1.20 that after a work function returns an error,
// the package will use context.WithCancelCause to cancel the ctx, which will cause the error returned by rpc.Client
// in other work functions to become the error returned by the first failed work function, which is sometimes not as
// expected

import (
	"context"
	"sync"
)

type Group struct {
	cancel context.CancelFunc

	wg      sync.WaitGroup
	errOnce sync.Once
	err     error
}

func WithContext(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &Group{cancel: cancel}, ctx
}

func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}

func (g *Group) Go(f func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := f(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				if g.cancel != nil {
					g.cancel()
				}
			})
		}
	}()
}
