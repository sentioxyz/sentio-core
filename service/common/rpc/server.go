package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/handlers"
	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/soheilhy/cmux"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"

	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
)

// var tracer = otel.Tracer("Server")

func GormUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			_, logger := log.FromContextWithTrace(ctx)
			logger = logger.With(zap.String("fullMethod", info.FullMethod))
			if err != nil {
				logger.Errore(err, "call failed")
			} else {
				logger.Debugf("call succeed")
			}
			_ = logger.Sync()
		}()
		// ctx, span := tracer.Start(ctx, "Service Call:"+info.FullMethod)
		// defer span.End()
		h, err := handler(ctx, req)
		// convert ErrRecordNotFound to 404
		// TODO convert more errors
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = status.Error(codes.NotFound, err.Error())
		}
		return h, err
	}
}

// func HttpTraceServerInterceptor() grpc.UnaryServerInterceptor {
// 	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler)
// (interface{}, error) {
//		return handler(ctx, req)
//	}
// }

func loadServerTLSCredentials() (credentials.TransportCredentials, error) {
	if !*enableTLS {
		return insecure.NewCredentials(), nil
	}
	// Load certificate of the CA who signed client's certificate
	pemClientCA, err := os.ReadFile(*caCert)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemClientCA) {
		return nil, fmt.Errorf("failed to add client CA's certificate")
	}

	// Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair(*serverCert, *serverKey)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS13,
	}

	return credentials.NewTLS(config), nil
}

func recoverFunHandler(ctx context.Context, p interface{}) (err error) {
	_, logger := log.FromContextWithTrace(ctx)
	logger.Errorf("panic recovered: %v", p)
	return status.Error(codes.Internal, "panic recovered")
}

func NewServer(tls bool, opt ...grpc.ServerOption) *grpc.Server {
	interceptor := grpc.
		ChainUnaryInterceptor(
			GormUnaryServerInterceptor(),
			MetricUnaryServerInterceptor(),
			grpcrecovery.UnaryServerInterceptor(grpcrecovery.WithRecoveryHandlerContext(recoverFunHandler)))

	statsHandler := grpc.StatsHandler(otelgrpc.NewServerHandler())

	var tlsCredentials credentials.TransportCredentials
	var err error
	if tls {
		tlsCredentials, err = loadServerTLSCredentials()
		if err != nil {
			log.Fatal("cannot load TLS credentials: ", err)
		}
	} else {
		tlsCredentials = insecure.NewCredentials()
	}

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
		}), runtime.WithMetadata(WithAuthAndTraceMetadata))...)

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

func ListenAndServe(ctx context.Context, addr string, handler http.Handler, grpcServer *grpc.Server) error {
	_, logger := log.FromContext(ctx)
	var httpServer *http.Server
	if handler != nil {
		httpServer = &http.Server{
			Handler: handler,
			Addr:    addr,
			BaseContext: func(listener net.Listener) context.Context {
				return ctx
			},
		}
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	var httpL = l
	var grpcL = l
	m := cmux.New(l)
	if httpServer != nil && grpcServer != nil {
		// a different listener for HTTP1
		httpL = m.Match(cmux.HTTP1Fast())
		// a different listener for HTTP2 since gRPC uses HTTP2
		grpcL = m.Match(cmux.TLS(), cmux.HTTP2())
	}
	g, ctx := errgroup.WithContext(ctx)
	go func() {
		// m.Close() will not end m.Serve(), so had to give up using g.Go
		logger.Debugfe(m.Serve(), "mux listen ended")
	}()
	if httpServer != nil {
		g.Go(func() error {
			defer logger.Debugf("http server ended")
			return httpServer.Serve(httpL)
		})
	}
	if grpcServer != nil {
		g.Go(func() error {
			defer logger.Debugf("grpc server ended")
			return grpcServer.Serve(grpcL)
		})
	}
	g.Go(func() error {
		// when ctx canceled, close the rpc server and http server
		<-ctx.Done()
		m.Close()
		if grpcServer != nil {
			grpcServer.GracefulStop()
			logger.Debugf("grpc server graceful stopped")
		}
		if httpServer != nil {
			_ = httpServer.Close()
			logger.Debugf("http server stopped")
		}
		return nil
	})
	logger.Infof("server start %q", addr)
	// wait rpc server and http server end
	return g.Wait()
}

func ExtractSpanContext(req *http.Request) (*trace.SpanContext, error) {
	traceID, err := strconv.ParseInt(req.Header.Get("x-datadog-trace-id"), 10, 0)
	if err != nil {
		return nil, err
	}
	spanID, err := strconv.ParseInt(req.Header.Get("x-datadog-parent-id"), 10, 0)
	if err != nil {
		return nil, err
	}

	sampled := req.Header.Get("x-datadog-sampling-priority") == "1"

	var otelTraceID trace.TraceID
	var otelSpanID trace.SpanID

	for i := len(otelTraceID) - 1; traceID > 0; traceID >>= 8 {
		otelTraceID[i] = byte(traceID & 0xff)
		i--
	}
	for i := len(otelSpanID) - 1; spanID > 0; spanID >>= 8 {
		otelSpanID[i] = byte(spanID & 0xff)
		i--
	}

	var traceFlag trace.TraceFlags

	ctx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    otelTraceID,
		SpanID:     otelSpanID,
		TraceFlags: traceFlag.WithSampled(sampled),
		Remote:     true,
	})
	return &ctx, nil
}

func WithAuthAndTraceMetadata(ctx context.Context, req *http.Request) metadata.MD {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}

	spanCtx, err := ExtractSpanContext(req)
	if err == nil {
		tmpCtx := trace.ContextWithSpanContext(context.Background(), *spanCtx)
		otelgrpc.Inject(tmpCtx, &md)
	}
	// else {
	//	log.Debug("Error injecting datadog trace id to opentelemetry", err)
	// }

	authorization := req.Header.Get("Authorization")
	if authorization != "" {
		if strings.HasPrefix(authorization, "Bearer ") {
			md.Append("auth", strings.TrimPrefix(authorization, "Bearer "))
		}

		if strings.HasPrefix(authorization, "Basic ") {
			md.Append("api-key", strings.TrimPrefix(authorization, "Basic "))
		}
	}
	apiKey := req.Header.Get("Api-Key")
	if apiKey != "" {
		md.Append("api-key", apiKey)
	}
	apiKeyInQuery := req.URL.Query().Get("api-key")
	if apiKeyInQuery != "" {
		md.Append("api-key", apiKeyInQuery)
	}
	if auid, err := req.Cookie("AUID"); err == nil {
		md.Append("auid", auid.Value)
	}
	if adminMode := req.Header.Get("X-Admin-Mode"); adminMode != "" {
		md.Append("admin-mode", adminMode)
	}
	if shareDashboard := req.Header.Get("share-dashboard"); shareDashboard != "" {
		md.Append("share-dashboard", shareDashboard)
	}
	if importProject := req.Header.Get("external-project"); importProject != "" {
		md.Append("external-project", importProject)
	}
	if shareQuery := req.Header.Get("share-query"); shareQuery != "" {
		md.Append("share-query", shareQuery)
	}
	md.Append("from-http", "true")
	return md
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

func WithAuth(handler runtime.HandlerFunc) runtime.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		md := WithAuthAndTraceMetadata(context.TODO(), req)
		req = req.WithContext(metadata.NewIncomingContext(req.Context(), md))
		handler(w, req, pathParams)
	}
}
