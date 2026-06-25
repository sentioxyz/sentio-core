package jsonrpc

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timewin"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
	UseGRPCConnection(
		ctx context.Context,
		method string,
		fn func(ctx context.Context, conn *grpc.ClientConn) clientpool.Result,
	) clientpool.Report
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

	// Resolve the client's wire format (native gRPC vs gRPC-web). The proxy always
	// speaks native gRPC to the upstream; the response is re-encoded to match the
	// client. Only grpc+proto and grpc-web+proto are supported today.
	wire := wireForContentType(r.Header.Get("Content-Type"))
	if wire == nil {
		http.Error(w, "unsupported gRPC Content-Type", http.StatusUnsupportedMediaType)
		return
	}

	// The proxy forwards message payloads verbatim and never decompresses, so a
	// request that declares a non-identity message compression cannot be handled.
	// Reject it up front (per the gRPC spec) with UNIMPLEMENTED, advertising
	// identity as the only accepted encoding, instead of failing later on the
	// first compressed frame. (grpc-accept-encoding needs no such check: identity
	// is always acceptable and responses are always sent uncompressed.)
	if enc := r.Header.Get("Grpc-Encoding"); enc != "" && enc != "identity" {
		w.Header().Set("Grpc-Accept-Encoding", "identity")
		grpcWriteError(w, wire, status.Errorf(codes.Unimplemented, "grpc-encoding %q is not supported", enc))
		return
	}

	// Probe: OnRequest — gives the caller a chance to reject early and to
	// enrich the context with request-scoped data (e.g. resolved endpoint).
	if h.probe != nil {
		probeCtx, err := h.probe.OnRequest(ctx, method, r)
		if err != nil {
			grpcWriteError(w, wire, err)
			return
		}
		if probeCtx != nil {
			ctx = probeCtx
		}
	}

	report := h.pool.UseGRPCConnection(ctx, "grpc.proxy."+method, func(ctx context.Context, conn *grpc.ClientConn) clientpool.Result {
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
			grpcWriteError(w, wire, err)
			return clientpool.Result{Err: err}
		}

		// Forward client → backend frames in a background goroutine.
		// The goroutine exits when the client body closes or the stream context is
		// cancelled (which happens when ServeHTTP returns).
		go func() {
			for {
				frame, err := wire.readMessage(r.Body)
				if err != nil {
					// io.EOF is the normal "client done sending" signal; anything
					// else (e.g. a rejected compressed frame) fails the request, so
					// surface it in the log before closing the send direction.
					if !errors.Is(err, io.EOF) {
						logger.Debugw("read request frame failed", "err", err)
					}
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
			grpcWriteError(w, wire, err)
			return clientpool.Result{Err: err}
		}

		// Write HTTP response status and headers. Use the client's wire-format
		// content-type and skip the upstream's content-type (always native gRPC,
		// which would mislead a gRPC-web client).
		w.Header().Set("Content-Type", wire.responseContentType())
		for k, vs := range hdr {
			if textproto.CanonicalMIMEHeaderKey(k) == "Content-Type" {
				continue
			}
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
			if err := wire.writeMessage(w, frame); err != nil {
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

		// Forward backend trailing metadata + final status in the client's wire
		// format (native: HTTP/2 trailers; gRPC-web: in-body trailer frame).
		st := status.New(codes.OK, "")
		if streamErr != nil {
			st = status.Convert(streamErr)
		}
		if err := wire.writeTrailer(w, w.Header(), cs.Trailer(), st); err != nil {
			logger.Debugw("write trailer failed", "err", err)
		}

		return clientpool.Result{}
	})

	if errors.Is(report.Err, clientpool.ErrNoValidClient) {
		grpcErr := status.Errorf(codes.Unavailable, "no healthy backend")
		grpcWriteError(w, wire, grpcErr)
		// Probe: OnFinish for pool-miss.
		if h.probe != nil {
			h.probe.OnFinish(ctx, method, 0, grpcErr)
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

// errCompressedFrame is returned by grpcReadFrame for a message frame whose
// Compressed-Flag is set (or otherwise non-zero).
var errCompressedFrame = errors.New("compressed gRPC message frames are not supported")

// grpcReadFrame reads one length-prefixed gRPC message frame from r.
//
// Wire format: [1 byte compressed flag][4 bytes big-endian length][<length> bytes data].
// Returns io.EOF if the stream ended cleanly before the frame header.
//
// The proxy negotiates no compression and forwards message payloads verbatim
// (grpc-go's SendMsg never decompresses its input, and grpcWriteFrame always
// emits Compressed-Flag 0). A request frame with a non-zero Compressed-Flag
// therefore cannot be re-framed correctly — it would be sent upstream as if
// uncompressed and break decoding — so it is rejected with errCompressedFrame
// rather than silently corrupted. (Responses are decompressed by grpc-go's
// RecvMsg before reaching us, so only this read path needs the guard.)
//
// A conformant client declares compression via the grpc-encoding header, which
// serveGRPC rejects up front with UNIMPLEMENTED; this per-frame check is
// defense-in-depth for a client that sets the flag without that header. If
// per-message compression is ever required, decompress here first.
func grpcReadFrame(r io.Reader) ([]byte, error) {
	hdr := make([]byte, 5)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, err
	}
	if hdr[0] != 0 {
		return nil, errCompressedFrame
	}
	n := binary.BigEndian.Uint32(hdr[1:5])
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

// grpcWriteFrame writes one uncompressed length-prefixed gRPC message frame to w
// (compressed flag always 0; the proxy never compresses, matching grpcReadFrame).
func grpcWriteFrame(w io.Writer, data []byte) error {
	hdr := [5]byte{} // hdr[0] = 0: not compressed
	binary.BigEndian.PutUint32(hdr[1:], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// grpcWriteError writes a terminal gRPC error (no message frames) to the client
// in its wire format.  Used when the stream fails before any backend response is
// available.
func grpcWriteError(w http.ResponseWriter, wire grpcWire, err error) {
	st, _ := status.FromError(err)
	wire.writeStatusOnly(w, st)
}

// encodeGrpcMessage percent-encodes a status message for the grpc-message
// header/trailer per the gRPC-over-HTTP/2 spec: any byte outside printable
// ASCII (0x20–0x7E), plus '%' itself, is escaped as %XX. grpc-go applies this
// to statuses it emits, but this proxy writes the trailer by hand and so must
// encode the message itself; an unescaped non-ASCII byte would otherwise be an
// illegal header value.
func encodeGrpcMessage(msg string) string {
	for i := 0; i < len(msg); i++ {
		if c := msg[i]; c < 0x20 || c > 0x7E || c == '%' {
			return encodeGrpcMessageUnchecked(msg)
		}
	}
	return msg
}

func encodeGrpcMessageUnchecked(msg string) string {
	var sb strings.Builder
	for i := 0; i < len(msg); i++ {
		if c := msg[i]; c >= 0x20 && c <= 0x7E && c != '%' {
			sb.WriteByte(c)
		} else {
			fmt.Fprintf(&sb, "%%%02X", c)
		}
	}
	return sb.String()
}

// ── Client wire format (native gRPC vs gRPC-web) ────────────────────────────
//
// The proxy always speaks native gRPC to the upstream (via grpc-go); toward the
// client it must speak whatever the client used.  grpcWire abstracts that wire
// format. Native gRPC and gRPC-web share the same length-prefixed message frame
// ([1-byte flag][4-byte big-endian length][payload]) and differ only in:
//   - the response Content-Type, and
//   - how trailing metadata (grpc-status/message) is delivered: native uses
//     HTTP/2 trailers, gRPC-web a trailer frame (flag 0x80) in the response body
//     that fetch-based clients can read.
//
// Only the +proto encodings are handled today; +json / grpc-web-text are
// rejected with 415 (see wireForContentType). New formats only need a new
// grpcWire implementation.
type grpcWire interface {
	// responseContentType is the Content-Type set on the response. It echoes the
	// request's Content-Type (within the supported set) so a client that sent an
	// explicit "+proto" suffix gets it back verbatim, avoiding any mismatch with
	// strict exact-match content-type validators (e.g. @protobuf-ts/grpcweb).
	responseContentType() string
	// readMessage reads one request message frame from the client.
	readMessage(r io.Reader) ([]byte, error)
	// writeMessage writes one response message frame to the client.
	writeMessage(w io.Writer, msg []byte) error
	// writeTrailer flushes trailing metadata + final status after the message
	// stream.  h is the response header (native HTTP/2 trailers); w is the
	// response body (gRPC-web in-body trailer frame).
	writeTrailer(w io.Writer, h http.Header, trailer metadata.MD, st *status.Status) error
	// writeStatusOnly writes a trailers-only response (an error before any
	// message frame), including the Content-Type.
	writeStatusOnly(w http.ResponseWriter, st *status.Status)
}

// wireForContentType resolves the client wire format from the request
// Content-Type, or returns nil for unsupported types (caller responds 415).
func wireForContentType(ct string) grpcWire {
	ct = strings.TrimSpace(ct)
	if i := strings.IndexByte(ct, ';'); i >= 0 { // strip params, e.g. "; charset=..."
		ct = strings.TrimSpace(ct[:i])
	}
	switch ct {
	case "application/grpc", "application/grpc+proto":
		return nativeWire{contentType: ct}
	case "application/grpc-web", "application/grpc-web+proto":
		return webWire{contentType: ct}
	default:
		return nil
	}
}

// nativeWire is the standard HTTP/2 gRPC wire format. contentType is the
// (param-stripped) request Content-Type, echoed back on the response.
type nativeWire struct{ contentType string }

func (n nativeWire) responseContentType() string                { return n.contentType }
func (n nativeWire) readMessage(r io.Reader) ([]byte, error)    { return grpcReadFrame(r) }
func (n nativeWire) writeMessage(w io.Writer, msg []byte) error { return grpcWriteFrame(w, msg) }

func (n nativeWire) writeTrailer(_ io.Writer, h http.Header, trailer metadata.MD, st *status.Status) error {
	for k, vs := range trailer {
		h[http.TrailerPrefix+textproto.CanonicalMIMEHeaderKey(k)] = vs
	}
	if _, ok := trailer["grpc-status"]; !ok {
		h[http.TrailerPrefix+"Grpc-Status"] = []string{strconv.Itoa(int(st.Code()))}
		if msg := st.Message(); msg != "" {
			h[http.TrailerPrefix+"Grpc-Message"] = []string{encodeGrpcMessage(msg)}
		}
	}
	return nil
}

func (n nativeWire) writeStatusOnly(w http.ResponseWriter, st *status.Status) {
	w.Header().Set("Content-Type", n.contentType)
	w.Header().Set("Grpc-Status", strconv.Itoa(int(st.Code())))
	if msg := st.Message(); msg != "" {
		w.Header().Set("Grpc-Message", encodeGrpcMessage(msg))
	}
	w.WriteHeader(http.StatusOK)
}

// webWire is the gRPC-web wire format (binary +proto).  Trailing metadata is
// sent as a trailer frame in the response body instead of HTTP/2 trailers, so
// fetch-based clients (which can't read HTTP/2 trailers) can consume it.
// contentType is the (param-stripped) request Content-Type, echoed back.
type webWire struct{ contentType string }

func (wf webWire) responseContentType() string                { return wf.contentType }
func (wf webWire) readMessage(r io.Reader) ([]byte, error)    { return grpcReadFrame(r) }
func (wf webWire) writeMessage(w io.Writer, msg []byte) error { return grpcWriteFrame(w, msg) }

func (wf webWire) writeTrailer(w io.Writer, _ http.Header, trailer metadata.MD, st *status.Status) error {
	var buf bytes.Buffer
	for k, vs := range trailer {
		for _, v := range vs {
			buf.WriteString(strings.ToLower(k))
			buf.WriteString(": ")
			buf.WriteString(v)
			buf.WriteString("\r\n")
		}
	}
	if _, ok := trailer["grpc-status"]; !ok {
		fmt.Fprintf(&buf, "grpc-status: %d\r\n", st.Code())
		if msg := st.Message(); msg != "" {
			fmt.Fprintf(&buf, "grpc-message: %s\r\n", encodeGrpcMessage(msg))
		}
	}
	// gRPC-web trailer frame: the 0x80 flag bit marks the frame as trailers.
	var hdr [5]byte
	hdr[0] = 0x80
	binary.BigEndian.PutUint32(hdr[1:], uint32(buf.Len()))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

func (wf webWire) writeStatusOnly(w http.ResponseWriter, st *status.Status) {
	w.Header().Set("Content-Type", wf.contentType)
	w.WriteHeader(http.StatusOK)
	_ = wf.writeTrailer(w, w.Header(), metadata.MD{}, st)
}
