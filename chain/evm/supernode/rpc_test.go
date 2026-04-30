package supernode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
)

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

func callRPC[T any](addr, method string, params []any) (T, error) {
	var zero T
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	if err != nil {
		return zero, err
	}
	resp, err := http.Post("http://"+addr, "application/json", bytes.NewReader(body))
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, err
	}
	var envelope struct {
		Result T    `json:"result"`
		Error  *any `json:"error"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return zero, fmt.Errorf("unmarshal failed: %w, body: %s", err, raw)
	}
	if envelope.Error != nil {
		return zero, fmt.Errorf("rpc error: %v", *envelope.Error)
	}
	return envelope.Result, nil
}

func Test_rpc(t *testing.T) {
	log.ManuallySetLevel(zapcore.DebugLevel)
	log.BindFlag()

	ctx, cancel := context.WithCancel(context.Background())
	g, gctx := errgroup.WithContext(ctx)

	// prepare client pool targeting ETH mainnet (Cloudflare public endpoint)
	cli := evm.NewClientPool("client")
	g.Go(func() error {
		ch := make(chan clientpool.PoolConfig[evm.ClientConfig], 1)
		ch <- clientpool.PoolConfig[evm.ClientConfig]{
			ClientConfigs: []clientpool.ClientConfig[evm.ClientConfig]{
				{
					Config: evm.ClientConfig{
						Endpoint:             "https://eth.drpc.org",
						ChainID:              1,
						IgnoreStateFromCheck: true,
					},
				},
			},
		}
		cli.Start(gctx, ch)
		return nil
	})

	sc := chain.NewStdLatestSlotCache[*evm.Slot](
		"ext",
		"eth_mainnet",
		time.Second*12, // ~12s ETH block time
		cli,
		evm.NewExtServerDimension(cli, 1, 3, rg.Range{}, "1", evm.NetworkOptions{DisableTrace: true}, 0),
		nil,
		0,
		nil,
		nil,
	)
	g.Go(func() error {
		return sc.KeepGrowth(gctx)
	})

	addr := "127.0.0.1:18891"
	h := jsonrpc.NewHandler("test", true, false, nil, nil, "")
	h.RegisterMiddleware(NewSimpleProxyService(cli, sc, nil)...)

	g.Go(func() error {
		return jsonrpc.ListenAndServe(gctx, ":18891", h)
	})

	// wait for the slot cache to populate at least one block
	_, _ = sc.Wait(gctx, 0)

	t.Run("eth_blockNumber", func(t *testing.T) {
		result, err := callRPC[hexutil.Uint64](addr, "eth_blockNumber", nil)
		assert.NoError(t, err)
		assert.Greater(t, uint64(result), uint64(0))
	})

	t.Run("eth_getLatestBlockNumber", func(t *testing.T) {
		resp, err := callRPC[evm.GetLatestBlockNumberResponse](addr, "eth_getLatestBlockNumber", []any{uint64(0)})
		assert.NoError(t, err)
		assert.Equal(t, evm.APIVersion, resp.APIVersion)
		assert.Greater(t, resp.LatestBlockNumber, uint64(0))
	})

	t.Run("proxy.eth_chainId", func(t *testing.T) {
		chainID, err := callRPC[string](addr, "eth_chainId", nil)
		assert.NoError(t, err)
		assert.Equal(t, "0x1", chainID)
	})

	t.Run("eth_getBlockByNumber", func(t *testing.T) {
		result, err := callRPC[map[string]any](addr, "eth_getBlockByNumber", []any{"latest", false})
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result["number"])
	})

	t.Run("proxy.eth_getBlockByNumber", func(t *testing.T) {
		result, err := callRPC[map[string]any](addr, "eth_getBlockByNumber", []any{"0x1", false})
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "0x1", result["number"])
	})

	b, _ := json.MarshalIndent(cli.Snapshot(), "", "\t")
	log.Infof("client: %s", string(b))

	cancel()
	_ = g.Wait()
}
