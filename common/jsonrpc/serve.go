package jsonrpc

import (
	"context"
	"net"
	"net/http"

	"sentioxyz/sentio-core/common/log"
)

func ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {
	svr := http.Server{
		Addr:    addr,
		Handler: handler,
		BaseContext: func(listener net.Listener) context.Context {
			return ctx
		},
	}
	_, logger := log.FromContext(ctx)
	logger.Infof("server start %q", addr)
	go func() {
		<-ctx.Done()
		_ = svr.Close()
	}()
	return svr.ListenAndServe()
}
