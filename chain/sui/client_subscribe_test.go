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

	"sentioxyz/sentio-core/chain/clientpool"
)

// fakeSuiNode serves the two gRPC calls SubscribeLatest can make and counts them.
type fakeSuiNode struct {
	rpcv2.UnimplementedLedgerServiceServer
	rpcv2.UnimplementedSubscriptionServiceServer

	serviceInfoCalls atomic.Int64
	subscribeCalls   atomic.Int64
}

func (n *fakeSuiNode) GetServiceInfo(context.Context, *rpcv2.GetServiceInfoRequest) (*rpcv2.GetServiceInfoResponse, error) {
	height := uint64(100) + uint64(n.serviceInfoCalls.Add(1))
	return &rpcv2.GetServiceInfoResponse{CheckpointHeight: &height}, nil
}

func (n *fakeSuiNode) SubscribeCheckpoints(
	_ *rpcv2.SubscribeCheckpointsRequest,
	stream grpc.ServerStreamingServer[rpcv2.SubscribeCheckpointsResponse],
) error {
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
