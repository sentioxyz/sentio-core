package jsonrpc

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/log"
	"strings"
	"testing"
	"time"

	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

// singleConnPool is a minimal ConnectionPool for tests that wraps one connection.
type singleConnPool struct {
	conn *grpc.ClientConn
}

func (p *singleConnPool) UseGRPCConnection(
	ctx context.Context,
	method string,
	fn func(ctx context.Context, conn *grpc.ClientConn) clientpool.Result,
) clientpool.Report {
	r := fn(ctx, p.conn)
	return clientpool.Report{Err: r.Err, ClientName: "test", ConfigName: "test"}
}

func (p *singleConnPool) Snapshot() any {
	return map[string]any{"size": 1}
}

// testLedgerServer is a minimal in-process backend for Test_grpcHandler.
type testLedgerServer struct {
	rpcv2.UnimplementedLedgerServiceServer
}

func (s *testLedgerServer) GetServiceInfo(_ context.Context, _ *rpcv2.GetServiceInfoRequest) (*rpcv2.GetServiceInfoResponse, error) {
	chain := "testnet"
	server := "test-server/0.1"
	var epoch uint64 = 42
	return &rpcv2.GetServiceInfoResponse{
		Chain:  &chain,
		Epoch:  &epoch,
		Server: &server,
	}, nil
}

// testSubscriptionServer streams a fixed number of synthetic checkpoints.
type testSubscriptionServer struct {
	rpcv2.UnimplementedSubscriptionServiceServer
}

func (s *testSubscriptionServer) SubscribeCheckpoints(
	_ *rpcv2.SubscribeCheckpointsRequest,
	stream grpc.ServerStreamingServer[rpcv2.SubscribeCheckpointsResponse],
) error {
	for i := uint64(0); i < 20; i++ {
		cursor := i
		digest := fmt.Sprintf("digest-%d", i)
		if err := stream.Send(&rpcv2.SubscribeCheckpointsResponse{
			Cursor: &cursor,
			Checkpoint: &rpcv2.Checkpoint{
				SequenceNumber: &i,
				Digest:         &digest,
			},
		}); err != nil {
			return err
		}
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

// startBackendGRPCServer starts an in-process gRPC server on a random free port
// and returns the address it is listening on.
func startBackendGRPCServer(t *testing.T, ctx context.Context) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if !assert.NoError(t, err) {
		return ""
	}
	s := grpc.NewServer()
	rpcv2.RegisterLedgerServiceServer(s, &testLedgerServer{})
	rpcv2.RegisterSubscriptionServiceServer(s, &testSubscriptionServer{})
	go func() {
		if serveErr := s.Serve(lis); serveErr != nil && ctx.Err() == nil {
			t.Errorf("gRPC backend error: %v", serveErr)
		}
	}()
	go func() {
		<-ctx.Done()
		s.GracefulStop()
	}()
	return lis.Addr().String()
}

func listenAndServe(ctx context.Context, addr string, handler http.Handler) error {
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

func getServiceInfo(t *testing.T, ctx context.Context, ep string) {
	pc, pe := grpc.NewClient(ep,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*10)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, pe)
	defer func() {
		_ = pc.Close()
	}()

	cli := rpcv2.NewLedgerServiceClient(pc)
	resp, pe := cli.GetServiceInfo(ctx, &rpcv2.GetServiceInfoRequest{})
	assert.NoError(t, pe)
	b, pe := json.MarshalIndent(resp, "", "  ")
	assert.NoError(t, pe)
	log.Infof("service info (%s): %s", ep, string(b))
}

func subscribe(t *testing.T, ctx context.Context, ep string, round int) {
	pc, pe := grpc.NewClient(ep,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*10)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, pe)
	defer func() {
		_ = pc.Close()
	}()

	cli := rpcv2.NewSubscriptionServiceClient(pc)
	stream, err := cli.SubscribeCheckpoints(ctx, &rpcv2.SubscribeCheckpointsRequest{})
	if !assert.NoError(t, err) {
		log.Errore(err, "subscribe failed")
		return
	}
	defer func() {
		_ = stream.CloseSend()
	}()
	for i := 0; i < round; i++ {
		var res *rpcv2.SubscribeCheckpointsResponse
		res, err = stream.Recv()
		assert.NoError(t, err)
		var b []byte
		b, err = json.MarshalIndent(res.GetCheckpoint(), "", " ")
		assert.NoError(t, err)
		log.Infof("subscribe (%s) got #%d: %s", ep, i, string(b))
	}
}

// grpcWebGetServiceInfo issues a unary GetServiceInfo call using the raw
// gRPC-web wire format (a fetch-style client over HTTP) and verifies that the
// proxy responds with a gRPC-web response: application/grpc-web+proto
// Content-Type, a message frame, and an in-body trailer frame carrying
// grpc-status: 0.
func grpcWebGetServiceInfo(t *testing.T, ctx context.Context, ep, reqContentType string) {
	reqMsg, err := proto.Marshal(&rpcv2.GetServiceInfoRequest{})
	if !assert.NoError(t, err) {
		return
	}
	var body bytes.Buffer
	var hdr [5]byte
	binary.BigEndian.PutUint32(hdr[1:], uint32(len(reqMsg)))
	body.Write(hdr[:])
	body.Write(reqMsg)

	url := "http://" + ep + "/sui.rpc.v2.LedgerService/GetServiceInfo"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if !assert.NoError(t, err) {
		return
	}
	httpReq.Header.Set("Content-Type", reqContentType)

	resp, err := http.DefaultClient.Do(httpReq)
	if !assert.NoError(t, err) {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	// The response Content-Type echoes the request's (within the gRPC-web set).
	assert.Equal(t, reqContentType, resp.Header.Get("Content-Type"))

	respBody, err := io.ReadAll(resp.Body)
	if !assert.NoError(t, err) {
		return
	}

	var sawMessage, sawTrailer bool
	for off := 0; off+5 <= len(respBody); {
		flag := respBody[off]
		n := int(binary.BigEndian.Uint32(respBody[off+1 : off+5]))
		off += 5
		if off+n > len(respBody) {
			break
		}
		payload := respBody[off : off+n]
		off += n
		if flag&0x80 != 0 {
			sawTrailer = true
			assert.Contains(t, string(payload), "grpc-status: 0")
			log.Infof("grpc-web trailer (%s): %s", ep, strings.TrimSpace(string(payload)))
		} else {
			sawMessage = true
			var info rpcv2.GetServiceInfoResponse
			assert.NoError(t, proto.Unmarshal(payload, &info))
			assert.Equal(t, "testnet", info.GetChain())
		}
	}
	assert.True(t, sawMessage, "expected a gRPC-web message frame")
	assert.True(t, sawTrailer, "expected a gRPC-web trailer frame")
}

func Test_encodeGrpcMessage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"ok", "ok"},
		{"plain ASCII message.", "plain ASCII message."},
		{"100%", "100%25"},
		{"line\nbreak", "line%0Abreak"},
		{"héllo", "h%C3%A9llo"}, // non-ASCII UTF-8 bytes escaped per-byte
	}
	for _, c := range cases {
		assert.Equal(t, c.want, encodeGrpcMessage(c.in), "encodeGrpcMessage(%q)", c.in)
	}
}

// grpcWebExpectCompressionRejected sends a request declaring an unsupported
// grpc-encoding and verifies the proxy rejects it up front with UNIMPLEMENTED
// (code 12) and advertises identity as the accepted encoding.
func grpcWebExpectCompressionRejected(t *testing.T, ctx context.Context, ep string) {
	var body bytes.Buffer
	body.Write([]byte{0, 0, 0, 0, 0}) // an (empty) frame; should never be read
	url := "http://" + ep + "/sui.rpc.v2.LedgerService/GetServiceInfo"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if !assert.NoError(t, err) {
		return
	}
	req.Header.Set("Content-Type", "application/grpc-web+proto")
	req.Header.Set("Grpc-Encoding", "gzip")

	resp, err := http.DefaultClient.Do(req)
	if !assert.NoError(t, err) {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, "identity", resp.Header.Get("Grpc-Accept-Encoding"))
	respBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(respBody), "grpc-status: 12") // codes.Unimplemented
}

func Test_grpcReadFrame_rejectsCompressed(t *testing.T) {
	// Compressed-Flag set (0x01): the proxy can't re-frame a compressed payload,
	// so it must reject rather than forward it as if uncompressed.
	frame := []byte{0x01, 0, 0, 0, 3, 'a', 'b', 'c'}
	_, err := grpcReadFrame(bytes.NewReader(frame))
	assert.ErrorIs(t, err, errCompressedFrame)

	// Uncompressed (0x00) reads back the payload.
	frame[0] = 0x00
	data, err := grpcReadFrame(bytes.NewReader(frame))
	assert.NoError(t, err)
	assert.Equal(t, []byte("abc"), data)
}

func Test_grpcHandler(t *testing.T) {
	log.ManuallySetLevel(zap.DebugLevel)
	log.BindFlag()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start the in-process backend gRPC server on a random free port.
	backendAddr := startBackendGRPCServer(t, ctx)

	// Create a connection pool pointing at the backend.
	conn, err := grpc.NewClient(backendAddr,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*10)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	assert.NoError(t, err)
	defer func() { _ = conn.Close() }()
	pool := &singleConnPool{conn: conn}

	// Start the proxy server on another random free port (h2c is built into GRPCProxyHandler).
	proxyLis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	proxyAddr := proxyLis.Addr().String()
	_ = proxyLis.Close() // release so listenAndServe can bind it
	proxyHandler := NewGRPCProxyHandler(pool, "proxy", true)
	go func() {
		_ = listenAndServe(ctx, proxyAddr, proxyHandler)
	}()

	// Allow both servers a moment to begin accepting connections.
	time.Sleep(100 * time.Millisecond)

	// Verify direct call to the backend works.
	getServiceInfo(t, ctx, backendAddr)
	subscribe(t, ctx, backendAddr, 10)

	// Verify the same calls routed through the proxy work.
	getServiceInfo(t, ctx, proxyAddr)
	subscribe(t, ctx, proxyAddr, 10)

	// Verify a gRPC-web client (e.g. a browser/fetch-based SDK) is served the
	// gRPC-web wire format rather than native application/grpc, and that the
	// response Content-Type echoes the request's (both +proto and bare forms).
	grpcWebGetServiceInfo(t, ctx, proxyAddr, "application/grpc-web+proto")
	grpcWebGetServiceInfo(t, ctx, proxyAddr, "application/grpc-web")

	// A request declaring an unsupported compression is rejected up front.
	grpcWebExpectCompressionRejected(t, ctx, proxyAddr)

	time.Sleep(time.Second)
	b, _ := json.MarshalIndent(proxyHandler.Snapshot(), "", "  ")
	log.Infof("snapshot: %s", string(b))
}
