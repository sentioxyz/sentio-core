package sui

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"sentioxyz/sentio-core/chain/clientpool"
)

// fakeSuiNode serves the two gRPC calls SubscribeLatest can make and counts them.
type fakeSuiNode struct {
	rpcv2.UnimplementedLedgerServiceServer
	rpcv2.UnimplementedSubscriptionServiceServer

	serviceInfoCalls atomic.Int64
	subscribeCalls   atomic.Int64

	serviceInfoMD atomic.Pointer[metadata.MD]
	subscribeMD   atomic.Pointer[metadata.MD]
}

func storeIncomingMD(ctx context.Context, p *atomic.Pointer[metadata.MD]) {
	md, _ := metadata.FromIncomingContext(ctx)
	p.Store(&md)
}

func (n *fakeSuiNode) GetServiceInfo(ctx context.Context, _ *rpcv2.GetServiceInfoRequest) (*rpcv2.GetServiceInfoResponse, error) {
	storeIncomingMD(ctx, &n.serviceInfoMD)
	height := uint64(100) + uint64(n.serviceInfoCalls.Add(1))
	return &rpcv2.GetServiceInfoResponse{CheckpointHeight: &height}, nil
}

func (n *fakeSuiNode) SubscribeCheckpoints(
	_ *rpcv2.SubscribeCheckpointsRequest,
	stream grpc.ServerStreamingServer[rpcv2.SubscribeCheckpointsResponse],
) error {
	storeIncomingMD(stream.Context(), &n.subscribeMD)
	n.subscribeCalls.Add(1)
	<-stream.Context().Done()
	return nil
}

func startFakeSuiNode(t *testing.T) (*fakeSuiNode, string) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	node := &fakeSuiNode{}
	srv := grpc.NewServer()
	rpcv2.RegisterLedgerServiceServer(srv, node)
	rpcv2.RegisterSubscriptionServiceServer(srv, node)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return node, "http://" + lis.Addr().String()
}

func runSubscribeLatest(t *testing.T, disableSubscribe bool) (*fakeSuiNode, []clientpool.Block) {
	node, grpcEndpoint := startFakeSuiNode(t)
	cli, err := NewClient(ClientConfig{
		JSONRPCConfig:        clientpool.JSONRPCConfig{Endpoint: "http://127.0.0.1:1", KeepWatch: 50 * time.Millisecond},
		GrpcEndpoint:         grpcEndpoint,
		DisableGRPCSubscribe: disableSubscribe,
	}.Trim(), func(string, time.Duration, bool) {})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	ch := make(chan clientpool.Block)
	done := make(chan struct{})
	go func() {
		defer close(done)
		cli.SubscribeLatest(ctx, ch)
	}()
	var got []clientpool.Block
	for {
		select {
		case b := <-ch:
			got = append(got, b)
		case <-done:
			return node, got
		}
	}
}

func TestSubscribeLatest_DisableGRPCSubscribePollsOverGRPC(t *testing.T) {
	node, got := runSubscribeLatest(t, true)
	assert.Zero(t, node.subscribeCalls.Load(), "no SubscribeCheckpoints stream when disabled")
	assert.GreaterOrEqual(t, node.serviceInfoCalls.Load(), int64(2), "latest still polled over gRPC")
	require.NotEmpty(t, got)
	assert.Greater(t, got[0].Number, uint64(100))
}

func TestSubscribeLatest_GRPCSubscribeByDefault(t *testing.T) {
	node, _ := runSubscribeLatest(t, false)
	assert.Positive(t, node.subscribeCalls.Load(), "SubscribeCheckpoints stream opened by default")
}

func TestClientConfigTrimKeepsDisableGRPCSubscribe(t *testing.T) {
	assert.True(t, ClientConfig{DisableGRPCSubscribe: true}.Trim().DisableGRPCSubscribe)
}

func TestClientConfigTrim_GRPCMetadata(t *testing.T) {
	c := ClientConfig{GRPCMetadata: map[string]string{" X-DRPC-Key ": " key ", "x-drpc-chain": "sui"}}.Trim()
	assert.Equal(t, map[string]string{"x-drpc-key": "key", "x-drpc-chain": "sui"}, c.GRPCMetadata)
	assert.Nil(t, ClientConfig{GRPCMetadata: map[string]string{}}.Trim().GRPCMetadata)
}

func TestClient_GRPCMetadataOnUnaryAndStreamCalls(t *testing.T) {
	node, grpcEndpoint := startFakeSuiNode(t)
	cli, err := NewClient(ClientConfig{
		JSONRPCConfig: clientpool.JSONRPCConfig{Endpoint: "http://127.0.0.1:1"},
		GrpcEndpoint:  grpcEndpoint,
		GRPCMetadata:  map[string]string{"x-drpc-chain": "sui", "x-drpc-key": "key"},
	}.Trim(), func(string, time.Duration, bool) {})
	require.NoError(t, err)

	// a value the caller already carries under the same key is overridden, other keys pass through
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-drpc-key", "forwarded", "x-other", "kept")
	_, err = cli.getLatest(ctx, "test")
	require.NoError(t, err)
	md := *node.serviceInfoMD.Load()
	assert.Equal(t, []string{"sui"}, md.Get("x-drpc-chain"))
	assert.Equal(t, []string{"key"}, md.Get("x-drpc-key"))
	assert.Equal(t, []string{"kept"}, md.Get("x-other"))

	subCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cli.subscribeUsingGRPC(subCtx, make(chan clientpool.Block))
	}()
	require.Eventually(t, func() bool { return node.subscribeMD.Load() != nil }, time.Second, 10*time.Millisecond)
	cancel()
	<-done
	md = *node.subscribeMD.Load()
	assert.Equal(t, []string{"sui"}, md.Get("x-drpc-chain"))
	assert.Equal(t, []string{"key"}, md.Get("x-drpc-key"))
}
