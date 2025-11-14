package rpc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"sentioxyz/sentio-core/common/log"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/handlers"
	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/soheilhy/cmux"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

func recoverFunHandler(ctx context.Context, p interface{}) (err error) {
	_, logger := log.FromContextWithTrace(ctx)
	logger.Errorf("panic recovered: %v", p)
	return status.Error(codes.Internal, "panic recovered")
}

func NewServer(opt ...grpc.ServerOption) *grpc.Server {
	interceptor := grpc.
		ChainUnaryInterceptor(
			grpcrecovery.UnaryServerInterceptor(grpcrecovery.WithRecoveryHandlerContext(recoverFunHandler)))

	statsHandler := grpc.StatsHandler(otelgrpc.NewServerHandler())

	tlsCredentials := insecure.NewCredentials()

	rpcCredentials := grpc.Creds(tlsCredentials)
	maxRevSize := grpc.MaxRecvMsgSize(MaxRevSize)

	return grpc.NewServer(append(opt, statsHandler, interceptor, maxRevSize, rpcCredentials)...)
}

func NewServeMux(opts ...runtime.ServeMuxOption) *runtime.ServeMux {
	// This marshaler override the grpc-web marshaler, which make google.api.HttpBody not work.
	return runtime.NewServeMux(append(opts,
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
				AllowPartial:   true,
			},
			MarshalOptions: protojson.MarshalOptions{
				EmitUnpopulated: true,
			},
		}))...)

}

func BindAndServeWithHTTP(mux http.Handler, grpcServer *grpc.Server, port int, beforeShutdown func()) {
	// Creating a normal HTTP Service
	server := http.Server{
		Handler: handlers.CompressHandler(WithLogger(mux)),
	}
	// creating a listener for server
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatale(err)
	}
	m := cmux.New(l)
	// a different listener for HTTP1
	httpL := m.Match(cmux.HTTP1Fast())
	// a different listener for HTTP2 since gRPC uses HTTP2
	grpcL := m.Match(cmux.TLS(), cmux.HTTP2())
	// start server
	go func() {
		// log.Info("http shutdown: ", server.Serve(httpL))
		err := server.Serve(httpL)
		if err != nil {
			log.Info("http server shutdown: ", err)
		}
	}()
	go func() {
		err := grpcServer.Serve(grpcL)
		if err != nil {
			log.Info("grpc server shutdown: ", err)
		}
	}()
	// actual listener
	log.Infof("listening on %d", port)
	go func() {
		log.Info("mux shut down: ", m.Serve())
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	// select {
	s := <-sigCh
	log.Infof("got signal %q, attempting graceful shutdown", s)
	if beforeShutdown != nil {
		log.Infof("running before shutdown hook")
		beforeShutdown()
	}
	// shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// defer cancel()

	log.Infof("shutting down grpc server")
	grpcServer.GracefulStop()

	// log.Infof("shutting down http server")
	// err = server.Shutdown(shutdownCtx)
	// if err != nil {
	//	log.Error("graceful shut down http server failed", err)
	// }

	m.Close()
	log.Infof("server shut down gracefully")
}

func WithLogger(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, "/healthz") {
			m := httpsnoop.CaptureMetrics(handler, writer, request)
			_, logger := log.FromContextWithTrace(request.Context())
			logger.Debugf(
				"[%s][%d][%s] %s %s %s",
				request.RemoteAddr,
				m.Code,
				m.Duration,
				request.Method,
				request.URL.Path,
				request.URL.RawQuery,
			)
		}
	})
}
