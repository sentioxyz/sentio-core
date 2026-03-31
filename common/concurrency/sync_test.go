package concurrency

import (
	"context"
	"math"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_MultiSyncRunner(t *testing.T) {
	start := time.Now()
	var m MultiSyncRunner
	g, _ := errgroup.WithContext(context.Background())
	g.Go(func() error {
		return m.Go("k1", func() error {
			time.Sleep(time.Second)
			log.Debug("k1")
			return nil
		})
	})
	g.Go(func() error {
		return m.Go("k2", func() error {
			time.Sleep(time.Second)
			log.Debug("k2")
			return nil
		})
	})
	g.Go(func() error {
		return m.Go("k3", func() error {
			time.Sleep(time.Second)
			log.Debug("k3")
			return nil
		})
	})
	g.Go(func() error {
		return m.Go("k4", func() error {
			time.Sleep(time.Second)
			log.Debug("k4")
			return nil
		})
	})
	g.Go(func() error {
		return m.Go("k2", func() error {
			time.Sleep(time.Second)
			log.Debug("k2")
			return nil
		})
	})
	g.Go(func() error {
		return m.Go("k3", func() error {
			time.Sleep(time.Second)
			log.Debug("k3")
			return nil
		})
	})
	g.Go(func() error {
		return m.Go("k3", func() error {
			time.Sleep(time.Second)
			log.Debug("k3")
			return nil
		})
	})
	_ = g.Wait()
	used := time.Since(start)
	assert.Equal(t, float64(3), math.Round(used.Seconds()))
}
