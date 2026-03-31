package concurrency

import (
	"context"
	"os"
	"os/signal"
	"sentioxyz/sentio-core/common/log"
)

func NewSignalContext(parent context.Context, signals ...os.Signal) context.Context {
	ctx, cancel := context.WithCancel(parent)
	c := make(chan os.Signal, 1)
	if len(signals) == 0 {
		signals = []os.Signal{os.Interrupt}
	}
	signal.Notify(c, signals...)
	go func() {
		defer cancel()
		sig := <-c
		log.Warnf("catch signal %s", sig)
	}()
	return ctx
}
