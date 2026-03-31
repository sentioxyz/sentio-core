package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"net"
	"net/http"
	"sentioxyz/sentio-core/common/grpcpool"
	"sentioxyz/sentio-core/common/log"
	"testing"
	"time"
)

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

func Test_grpcHandler(t *testing.T) {
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
	pool := grpcpool.New([]*grpc.ClientConn{conn})
	go pool.Start(ctx)

	// Start the proxy server on another random free port (h2c is built into GRPCHandler).
	proxyLis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	proxyAddr := proxyLis.Addr().String()
	_ = proxyLis.Close() // release so listenAndServe can bind it
	go func() {
		_ = listenAndServe(ctx, proxyAddr, NewGRPCHandler(pool))
	}()

	// Allow both servers a moment to begin accepting connections.
	time.Sleep(100 * time.Millisecond)

	// Verify direct call to the backend works.
	getServiceInfo(t, ctx, backendAddr)
	subscribe(t, ctx, backendAddr, 10)

	// Verify the same calls routed through the proxy work.
	getServiceInfo(t, ctx, proxyAddr)
	subscribe(t, ctx, proxyAddr, 10)
}
