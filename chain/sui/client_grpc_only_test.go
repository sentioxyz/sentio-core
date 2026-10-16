package sui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/chains"
)

func grpcOnlyConfig(endpoint string) ClientConfig {
	return ClientConfig{
		JSONRPCConfig: clientpool.JSONRPCConfig{Endpoint: endpoint},
		GRPCOnly:      true,
		ChainID:       chains.SuiMainnetID,
	}
}

func TestClientConfigTrim_GRPCOnlyUsesEndpointAsGRPC(t *testing.T) {
	c := grpcOnlyConfig(" https://fullnode.testnet.sui.io/ ")
	c.GrpcEndpoint = "https://ignored.example:443"
	c = c.Trim()
	assert.Equal(t, "https://fullnode.testnet.sui.io", c.GrpcEndpoint)
	assert.True(t, c.GRPCOnly)
	assert.True(t, c.SupportGRPC())

	plain := ClientConfig{
		JSONRPCConfig: clientpool.JSONRPCConfig{Endpoint: "http://node:9000"},
		GrpcEndpoint:  "http://node:9000",
	}.Trim()
	assert.Equal(t, "http://node:9000", plain.GrpcEndpoint)
	assert.False(t, plain.GRPCOnly)
}

func TestNewClient_GRPCOnlyValidation(t *testing.T) {
	notify := func(string, time.Duration, bool) {}
	for _, ep := range []string{"https://fullnode.mainnet.sui.io", "https://fullnode.mainnet.sui.io:443", "http://node:9000"} {
		_, err := NewClient(grpcOnlyConfig(ep).Trim(), notify)
		assert.NoError(t, err, ep)
	}

	iota := grpcOnlyConfig("http://node:9000")
	iota.ChainID = chains.IotaMainnetID
	_, err := NewClient(iota.Trim(), notify)
	assert.ErrorIs(t, err, clientpool.ErrInvalidConfig, "IOTA has no gRPC path")

	_, err = NewClient(grpcOnlyConfig("https://rpc.example/sui/key").Trim(), notify)
	assert.ErrorIs(t, err, clientpool.ErrInvalidConfig, "a gRPC target cannot carry a path")

	_, err = NewClient(grpcOnlyConfig("http://node").Trim(), notify)
	assert.ErrorIs(t, err, clientpool.ErrInvalidConfig, "plain http needs an explicit port")
}

func TestClient_GRPCOnlyRefusesJSONRPCAndHTTPLocally(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.NotFound(w, nil)
	}))
	defer srv.Close()

	cfg := grpcOnlyConfig(srv.URL)
	cfg.AdditionalEndpoints = map[string]string{"debug": srv.URL}
	cli, err := NewClient(cfg.Trim(), func(string, time.Duration, bool) {})
	require.NoError(t, err)

	var result any
	r := cli.CallContext(context.Background(), &result, "test", "sui_getCheckpoint", "1")
	assert.Error(t, r.Err)
	assert.True(t, r.BrokenForTask)
	assert.False(t, r.Broken, "a refused call must not ban the client")

	u, _ := url.Parse(srv.URL)
	r = cli.UseHTTPClient(context.Background(), "", "test", u,
		func(context.Context, string, *http.Client) clientpool.Result {
			t.Fatal("plain http must be refused before it is sent")
			return clientpool.Result{}
		})
	assert.True(t, r.BrokenForTask)
	assert.False(t, r.Broken)

	// an additional endpoint is a separate service and stays reachable
	called := false
	r = cli.UseHTTPClient(context.Background(), "debug", "test", u,
		func(context.Context, string, *http.Client) clientpool.Result {
			called = true
			return clientpool.Result{}
		})
	assert.NoError(t, r.Err)
	assert.True(t, called)

	assert.Zero(t, hits.Load(), "nothing may reach the endpoint")
}

func TestClient_GRPCOnlyInitOverGRPC(t *testing.T) {
	node, grpcEndpoint := startFakeSuiNode(t)
	cli, err := NewClient(grpcOnlyConfig(grpcEndpoint).Trim(), func(string, time.Duration, bool) {})
	require.NoError(t, err)

	latest, err := cli.Init(context.Background())
	require.NoError(t, err)
	assert.Greater(t, latest.Number, uint64(100))
	assert.Equal(t, int64(1), node.serviceInfoCalls.Load())
}
