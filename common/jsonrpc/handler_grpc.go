package jsonrpc

import (
	"context"
	"encoding/binary"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"net/http"
	"net/textproto"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timewin"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// grpcRawCodec is a passthrough codec used on the backend (client) side of
// the proxy.  It carries raw protobuf bytes without additional serialisation,
// enabling transparent frame forwarding.
//
// Name() returns "proto" so the outgoing Content-Type header remains
// "application/grpc+proto", which every standard gRPC backend accepts.
// The codec is only ever used via grpc.ForceCodec and is never registered in
// the global encoding registry, so it does not affect other gRPC services
// running in the same process.
type grpcRawCodec struct{}

func (grpcRawCodec) Marshal(v any) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, status.Errorf(codes.Internal, "grpcRawCodec: expected []byte, got %T", v)
	}
	return b, nil
}

func (grpcRawCodec) Unmarshal(data []byte, v any) error {
	p, ok := v.(*[]byte)
	if !ok {
		return status.Errorf(codes.Internal, "grpcRawCodec: expected *[]byte, got %T", v)
	}
	*p = append((*p)[:0], data...)
	return nil
}

func (grpcRawCodec) Name() string { return "proto" }

type ConnectionPool interface {
	UseRawConnection(
		ctx context.Context,
		method string,
		fn func(ctx context.Context, conn *grpc.ClientConn) error,
	) (bool, error)
	Snapshot() any
}

// GRPCProbe is an optional callback interface that allows callers to observe
// gRPC proxy events without modifying the core proxy logic.  Useful for
// billing, metering, logging, and access control.
//
// All methods are called synchronously from the proxy goroutine, so
// implementations should be fast and non-blocking.
type GRPCProbe interface {
	// OnRequest is called once when a new gRPC request arrives, before the
	// upstream connection is established.  Returning a non-nil error aborts
	// the request and sends the error back as a gRPC status to the client.
	//
	// The returned context replaces the request context for the rest of the
	// proxy call (passed to ConnectionPool.UseRawConnection and to OnFinish).
	// Implementations can use context.WithValue to stash request-scoped data
	// (e.g. resolved endpoint, billing metadata) that the ConnectionPool or
	// OnFinish can later retrieve.
	OnRequest(ctx context.Context, method string, r *http.Request) (context.Context, error)

	// OnResponseMsg is called for each response message frame forwarded
	// from the backend to the client.  msgIndex is 0-based.
	OnResponseMsg(ctx context.Context, method string, msgIndex int)

	// OnFinish is called once after the stream ends (success or failure).
	// msgCount is the total number of response messages forwarded.
	// err is nil on clean EOF, otherwise the gRPC error.
	OnFinish(ctx context.Context, method string, msgCount int, err error)
}

// GRPCProxyHandler is an http.Handler that transparently proxies all incoming gRPC
// requests to a backend gRPC server selected from a connection pool.
//
// It works at the HTTP/2 framing level: raw length-prefixed message frames are
// forwarded unchanged, so no knowledge of the backend's protobuf schema is
// required.  Incoming request metadata (HTTP headers) and the backend's
// response headers and trailers are forwarded as well.
//
// h2c (HTTP/2 cleartext) is built in: the handler automatically upgrades
// plain-TCP connections to HTTP/2, so callers can use it directly with a
// standard http.Server — no external h2c wrapping is required.
//
// Example:
//
//	pool := grpcpool.New([]*grpc.ClientConn{conn1, conn2})
//	go pool.Start(ctx)
//	h := jsonrpc.NewGRPCProxyHandler(pool)
//	http.ListenAndServe(":8080", h)
type GRPCProxyHandler struct {
	pool  ConnectionPool
	probe GRPCProbe
	name  string
	debug bool

	// h2c-wrapped inner handler
	handler http.Handler

	// used to build rid
	requestCounter atomic.Uint64

	stat *timewin.TimeWindowsManager[*statWindow]
}

// GRPCProxyOption configures optional behaviour of the proxy handler.
type GRPCProxyOption func(*GRPCProxyHandler)

// WithGRPCProbe attaches a probe to the handler.  At most one probe can be
// set; a later call replaces any earlier probe.
func WithGRPCProbe(p GRPCProbe) GRPCProxyOption {
	return func(h *GRPCProxyHandler) { h.probe = p }
}

// NewGRPCProxyHandler returns a GRPCProxyHandler that forwards gRPC calls to connections
// obtained from pool.
func NewGRPCProxyHandler(pool ConnectionPool, name string, debug bool, opts ...GRPCProxyOption) *GRPCProxyHandler {
	h := &GRPCProxyHandler{
		pool:  pool,
		name:  name,
		debug: debug,
		stat:  timewin.NewTimeWindowsManager[*statWindow](time.Minute),
	}
	for _, o := range opts {
		o(h)
	}
	h.handler = h2c.NewHandler(http.HandlerFunc(h.serveGRPC), &http2.Server{})
	return h
}

// ServeHTTP implements http.Handler.
func (h *GRPCProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

// Snapshot returns a point-in-time summary of the handler's state and
// forwarding statistics (mirrors the shape of SimpleHTTPHandler.Snapshot).
func (h *GRPCProxyHandler) Snapshot() any {
	return map[string]any{
		"name":           h.name,
		"debug":          h.debug,
		"requestCounter": h.requestCounter.Load(),
		"pool":           h.pool.Snapshot(),
		"statistics":     h.stat.Snapshot(),
	}
}

// serveGRPC is the actual proxy logic, called after h2c negotiation.
//
// Only requests whose Content-Type starts with "application/grpc" are
// accepted; all others receive 415 Unsupported Media Type.
//
// The full gRPC method is taken from r.URL.Path, which must follow the
// standard gRPC URL format "/{package}.{Service}/{Method}".
func (h *GRPCProxyHandler) serveGRPC(w http.ResponseWriter, r *http.Request) {
	remoteHost, _, _ := strings.Cut(r.RemoteAddr, ":")
	method := r.URL.Path
	ctx, logger := log.FromContextWithTrace(r.Context(),
		"svr", h.name,
		"rid", h.requestCounter.Add(1),
		"method", method,
		"remote", remoteHost)

	startTime := time.Now()
	defer func() {
		h.stat.Append(newStatWindow(method, RequestSource{RemoteHost: remoteHost}, time.Since(startTime)))
	}()

	if h.debug {
		logger.Debugw("access", "header", r.Header)
		defer func() {
			logger.Debugw("leave", "used", time.Since(startTime).String())
		}()
	}

	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
		http.Error(w, "Content-Type must start with application/grpc", http.StatusUnsupportedMediaType)
		return
	}

	// Probe: OnRequest — gives the caller a chance to reject early and to
	// enrich the context with request-scoped data (e.g. resolved endpoint).
	if h.probe != nil {
		var err error
		ctx, err = h.probe.OnRequest(ctx, method, r)
		if err != nil {
			grpcWriteError(w, err)
			return
		}
	}

	done, err := h.pool.UseRawConnection(ctx, method, func(ctx context.Context, conn *grpc.ClientConn) error {
		// Propagate request metadata (HTTP headers) as gRPC outgoing metadata.
		outCtx := metadata.NewOutgoingContext(ctx, grpcMetadataFromHTTPHeaders(r.Header))

		// Open a bidirectional stream to the backend using raw byte passthrough.
		// grpc.ForceCodec bypasses the global codec registry: grpcRawCodec is used
		// only for this call and does not affect other gRPC services.
		cs, err := conn.NewStream(outCtx,
			&grpc.StreamDesc{ServerStreams: true, ClientStreams: true},
			r.URL.Path,
			grpc.ForceCodec(grpcRawCodec{}),
		)
		if err != nil {
			grpcWriteError(w, err)
			return err
		}

		// Forward client → backend frames in a background goroutine.
		// The goroutine exits when the client body closes or the stream context is
		// cancelled (which happens when ServeHTTP returns).
		go func() {
			for {
				frame, err := grpcReadFrame(r.Body)
				if err != nil {
					_ = cs.CloseSend()
					return
				}
				if err := cs.SendMsg(frame); err != nil {
					return
				}
			}
		}()

		// Wait for the backend's initial response headers.  An error here means
		// the RPC failed before the backend produced any response.
		hdr, err := cs.Header()
		if err != nil {
			grpcWriteError(w, err)
			return err
		}

		// Write HTTP response status and headers.
		w.Header().Set("Content-Type", "application/grpc")
		for k, vs := range hdr {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Stream backend → client frames.
		var streamErr error
		msgCount := 0
		for {
			var frame []byte
			if err := cs.RecvMsg(&frame); err != nil {
				if err != io.EOF {
					streamErr = err
				}
				break
			}
			// Probe: OnResponseMsg — called for each forwarded response frame.
			if h.probe != nil {
				h.probe.OnResponseMsg(ctx, method, msgCount)
			}
			msgCount++
			if err := grpcWriteFrame(w, frame); err != nil {
				break
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		// Probe: OnFinish — called once after the stream ends.
		if h.probe != nil {
			h.probe.OnFinish(ctx, method, msgCount, streamErr)
		}

		// Forward backend trailing metadata.
		for k, vs := range cs.Trailer() {
			w.Header()[http.TrailerPrefix+textproto.CanonicalMIMEHeaderKey(k)] = vs
		}
		// Ensure Grpc-Status is always present in the trailers.
		if _, ok := cs.Trailer()["grpc-status"]; !ok {
			st, _ := status.FromError(streamErr)
			w.Header()[http.TrailerPrefix+"Grpc-Status"] = []string{strconv.Itoa(int(st.Code()))}
			if msg := st.Message(); msg != "" {
				w.Header()[http.TrailerPrefix+"Grpc-Message"] = []string{msg}
			}
		}

		return nil
	})

	if !done {
		grpcWriteError(w, status.Errorf(codes.Unavailable, "no healthy backend: %v", err))
		// Probe: OnFinish for pool-miss.
		if h.probe != nil {
			h.probe.OnFinish(ctx, method, 0, err)
		}
	}
}

// grpcMetadataFromHTTPHeaders converts HTTP request headers to gRPC outgoing
// metadata, skipping HTTP-only hop-by-hop headers irrelevant to gRPC.
func grpcMetadataFromHTTPHeaders(h http.Header) metadata.MD {
	skip := map[string]bool{
		"Content-Type":      true,
		"Te":                true,
		"Connection":        true,
		"Keep-Alive":        true,
		"Transfer-Encoding": true,
		"Upgrade":           true,
	}
	md := make(metadata.MD, len(h))
	for k, vs := range h {
		if !skip[textproto.CanonicalMIMEHeaderKey(k)] {
			md[strings.ToLower(k)] = vs
		}
	}
	return md
}

// grpcReadFrame reads one length-prefixed gRPC message frame from r.
//
// Wire format: [1 byte compressed flag][4 bytes big-endian length][<length> bytes data].
// Returns io.EOF if the stream ended cleanly before the frame header.
func grpcReadFrame(r io.Reader) ([]byte, error) {
	hdr := make([]byte, 5)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[1:5])
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

// grpcWriteFrame writes one uncompressed length-prefixed gRPC message frame to w.
func grpcWriteFrame(w io.Writer, data []byte) error {
	hdr := [5]byte{} // hdr[0] = 0: not compressed
	binary.BigEndian.PutUint32(hdr[1:], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// grpcWriteError writes a terminal gRPC error as HTTP/2 response headers
// (no body).  Used when the stream fails before any backend response is
// available.
func grpcWriteError(w http.ResponseWriter, err error) {
	st, _ := status.FromError(err)
	w.Header().Set("Content-Type", "application/grpc")
	w.Header().Set("Grpc-Status", strconv.Itoa(int(st.Code())))
	if msg := st.Message(); msg != "" {
		w.Header().Set("Grpc-Message", msg)
	}
	w.WriteHeader(http.StatusOK)
}
