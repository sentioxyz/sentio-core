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

	"go.uber.org/zap/zapcore"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"

	"github.com/stretchr/testify/assert"
)

type mockStorage struct{}

func (m *mockStorage) QueryTransactions(_ context.Context, _ uint64, _ uint64, _ []fuel.TransactionFilter) ([]fuel.WrappedTransaction, error) {
	return nil, nil
}

func (m *mockStorage) QueryContractCreateTransaction(_ context.Context, _ string) (*fuel.WrappedTransaction, error) {
	return nil, nil
}

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

	// prepare client pool
	cli := fuel.NewClientPool("client")
	g.Go(func() error {
		ch := make(chan clientpool.PoolConfig[fuel.ClientConfig], 1)
		ch <- clientpool.PoolConfig[fuel.ClientConfig]{
			ClientConfigs: []clientpool.ClientConfig[fuel.ClientConfig]{
				{
					Config: fuel.ClientConfig{
						Endpoint: "https://mainnet.fuel.network/v1/graphql",
					},
				},
			},
		}
		cli.Start(gctx, ch)
		return nil
	})

	ext := fuel.NewExtServerDimension(cli, 10, 1, 3, rg.Range{}, 0)

	sc := chain.NewStdLatestSlotCache[*fuel.Slot](
		"ext",
		"fuel_mainnet",
		time.Second,
		cli,
		ext,
		nil,
		0,
		nil,
		nil,
	)
	g.Go(func() error {
		return sc.KeepGrowth(gctx)
	})

	var store Storage = &mockStorage{}

	addr := "127.0.0.1:18889"
	h := jsonrpc.NewHandler("test", true, false, nil, nil, "")
	h.RegisterMiddleware(NewSuperNode(cli, ext, sc, store)...)

	g.Go(func() error {
		return jsonrpc.ListenAndServe(gctx, ":18889", h)
	})

	// wait for the server to start and the slot cache to populate at least one block
	_, _ = sc.Wait(gctx, 0)

	t.Run("getLatestHeight", func(t *testing.T) {
		height, err := callRPC[uint64](addr, "fuel_getLatestHeight", nil)
		assert.NoError(t, err)
		assert.Greater(t, height, uint64(0))
	})

	t.Run("getLatestHeader", func(t *testing.T) {
		resp, err := callRPC[fuel.GetLatestBlockResponse](addr, "fuel_getLatestHeader", []any{uint64(0)})
		assert.NoError(t, err)
		assert.Equal(t, fuel.APIVersion, resp.APIVersion)
		assert.Greater(t, uint64(resp.Header.Height), uint64(0))
	})

	t.Run("getBlockHeader", func(t *testing.T) {
		// query a known early block
		header, err := callRPC[map[string]any](addr, "fuel_getBlockHeader", []any{uint64(1)})
		assert.NoError(t, err)
		assert.NotNil(t, header)
	})

	t.Run("getTransactions", func(t *testing.T) {
		param := fuel.GetTransactionsParam{
			StartHeight: 1,
			EndHeight:   5,
		}
		txns, err := callRPC[[]fuel.WrappedTransaction](addr, "fuel_getTransactions", []any{param})
		assert.NoError(t, err)
		assert.NotNil(t, txns)
	})

	b, _ := json.MarshalIndent(cli.Snapshot(), "", "\t")
	log.Infof("client: %s", string(b))

	cancel()
	_ = g.Wait()
}
