package controller

import (
	"context"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func Test_externalErrorWrap(t *testing.T) {
	err := errors.Errorf("level1")
	extErr1 := NewExternalError(ErrCodeSystem, err)
	extErr2 := extErr1.Wrapf("level2")
	assert.Equal(t, "level1", err.Error())
	assert.Equal(t, "ERR100: level1", extErr1.Error())
	assert.Equal(t, "ERR100: level2: level1", extErr2.Error())

	var extErr *ExternalError
	assert.False(t, errors.As(err, &extErr))
	assert.True(t, errors.As(extErr1, &extErr))
	assert.Equal(t, extErr1, extErr)
	assert.True(t, errors.As(extErr2, &extErr))
	assert.Equal(t, extErr2, extErr)

	log.Errorfe(err, "err")
	log.Errorfe(extErr1, "extErr1")
	log.Errorfe(extErr2, "extErr2")
	log.Errorf("err: %+v", err)
	log.Errorf("extErr1: %+v", extErr1)
	log.Errorf("extErr2: %+v", extErr2)
}

func Test_errgroup(t *testing.T) {
	g, gctx := errgroup.WithContext(context.Background())
	fn := func(ctx context.Context, wait time.Duration, code int) *ExternalError {
		select {
		case <-ctx.Done():
			return NewExternalError(0, ctx.Err())
		case <-time.After(wait):
			if code == 0 {
				return nil
			}
			return NewExternalError(code, errors.Errorf("err"))
		}
	}
	g.Go(func() error {
		if err := fn(gctx, time.Millisecond*100, 0); err != nil { // got nil
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := fn(gctx, time.Millisecond*200, 2); err != nil { // got code:2
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := fn(gctx, time.Millisecond*300, 3); err != nil { // got code:0 and ignored
			return err
		}
		return nil
	})

	err := g.Wait()
	var extErr *ExternalError
	assert.True(t, errors.As(err, &extErr))
	assert.Equal(t, 2, extErr.code)
}

func Test_panic(t *testing.T) {
	fn := func() (err error) {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				err = errors.Errorf("%v", panicErr) // row2
			}
		}()
		panic("panic here") // row1
	}

	log.Infof("err: %+v", fn()) // row3, log include all row1-3
}
