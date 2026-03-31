package concurrency

import (
	"context"
	"sync"

	"sentioxyz/sentio-core/common/errgroup"
)

func process[DST any](
	g *errgroup.Group,
	ctx context.Context,
	concurrency uint,
	out chan<- DST,
) (taskOutChan chan chan DST, errChan chan error) {
	if concurrency == 0 {
		panic("concurrency should greater than zero")
	}

	taskOutChan = make(chan chan DST, concurrency-1)
	errChan = make(chan error)

	g.Go(func() error {
		// mapper error watcher
		// once a mapper has error, then stop the whole group
		for err := range errChan {
			if err != nil {
				return err
			}
		}
		return nil
	})

	g.Go(func() error {
		// DST collector
		for taskOut := range taskOutChan {
			for prod := range taskOut {
				select {
				case out <- prod:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		return nil
	})

	return
}

// MapO2M map one-to-many
func MapO2M[SRC any, DST any](
	g *errgroup.Group,
	ctx context.Context,
	concurrency uint,
	in <-chan SRC,
	out chan<- DST,
	mapper func(ctx context.Context, index int, task SRC, taskOut chan<- DST) error,
) {
	if concurrency == 1 {
		// can be simple
		g.Go(func() error {
			return ForEach(ctx, in, func(ctx context.Context, index int, task SRC) error {
				return mapper(ctx, index, task, out)
			})
		})
		return
	}

	// typical time section for concurrency = 3:
	// ---------------------------------------------------------------------------
	// dispatcher(in)              mapper(task, taskOut)   collector(taskOutChan)
	//  ...                         ...                     ...
	//  DONE                        DONE                    DONE
	//  DONE                        DONE                    HANG:out<-taskOut
	//  DONE                        HANG:taskOut<-DST
	//  DONE                        HANG:taskOut<-DST
	//  DONE                        HANG:taskOut<-DST
	//  HANG:taskOutChan<-taskOut
	//
	taskOutChan, errChan := process(g, ctx, concurrency, out)

	g.Go(func() error {
		// SRC dispatcher
		defer close(taskOutChan)
		defer close(errChan)

		// wait all mapper done
		var mapperWaiter sync.WaitGroup
		defer mapperWaiter.Wait()

		for index := 0; ; index++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case task, has := <-in:
				if !has {
					return nil
				}
				// prepare next mapper
				taskOut := make(chan DST)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case taskOutChan <- taskOut:
					// registered to collector
					// then start a mapper
					mapperWaiter.Add(1)
					go func(index int, task SRC, taskOut chan<- DST) {
						defer mapperWaiter.Done()
						defer close(taskOut) // make sure close taskOut after mapper ended
						mapErr := mapper(ctx, index, task, taskOut)
						select {
						case <-ctx.Done():
						case errChan <- mapErr:
						}
					}(index, task, taskOut)
				}
			}
		}
	})
}

func MapO2MWithProducer[SRC any, DST any](
	g *errgroup.Group,
	ctx context.Context,
	concurrency uint,
	producer func(ctx context.Context, ch chan<- SRC) error,
	out chan<- DST,
	mapper func(ctx context.Context, index int, task SRC, taskOut chan<- DST) error,
) {
	in := make(chan SRC)
	g.Go(func() error {
		defer close(in)
		return producer(ctx, in)
	})
	MapO2M(g, ctx, concurrency, in, out, mapper)
}

func MapO2MWithProducerAndConsumer[SRC any, DST any](
	ctx context.Context,
	concurrency uint,
	producer func(ctx context.Context, ch chan<- SRC) error,
	consumer func(ctx context.Context, ch <-chan DST) error,
	mapper func(ctx context.Context, index int, task SRC, taskOut chan<- DST) error,
) error {
	gc, cctx := errgroup.WithContext(ctx)
	gp, pctx := errgroup.WithContext(cctx)
	in := make(chan SRC)
	out := make(chan DST)
	gp.Go(func() error {
		defer close(in)
		return producer(pctx, in)
	})
	MapO2M(gp, pctx, concurrency, in, out, mapper)
	gc.Go(func() error {
		defer close(out)
		return gp.Wait()
	})
	gc.Go(func() error {
		return consumer(cctx, out)
	})
	return gc.Wait()
}
