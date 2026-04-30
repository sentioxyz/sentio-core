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

	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"

	"github.com/stretchr/testify/assert"
)

type mockStorage struct{}

func (m *mockStorage) Functions(_ context.Context, _ aptos.GetFunctionsArgs) ([]*aptos.Transaction, error) {
	return nil, nil
}
func (m *mockStorage) FullEvents(_ context.Context, _ aptos.GetEventsArgs) ([]*aptos.Transaction, error) {
	return nil, nil
}
func (m *mockStorage) ResourceChanges(_ context.Context, _ aptos.ResourceChangeArgs) ([]*aptos.Transaction, error) {
	return nil, nil
}
func (m *mockStorage) GetTransactionByVersion(_ context.Context, _ uint64) (*aptos.Transaction, error) {
	return nil, nil
}
func (m *mockStorage) GetChangeStat(_ context.Context, _ uint64, _ string) (aptos.ChangeStat, error) {
	return aptos.ChangeStat{}, nil
}
func (m *mockStorage) GetFirstChange(_ context.Context, _ string, _ uint64) (version, blockHeight uint64, has bool, err error) {
	return 0, 0, false, nil
}
func (m *mockStorage) QueryMinimalistTransaction(_ context.Context, _ uint64) (*aptos.MinimalistTransaction, error) {
	return nil, nil
}
func (m *mockStorage) QueryTransactions(_ context.Context, _ aptos.GetTransactionsRequest) ([]aptos.Transaction, error) {
	return nil, nil
}
func (m *mockStorage) QueryResourceChanges(_ context.Context, _ aptos.GetResourceChangesRequest) ([]aptos.MinimalistTransactionWithChanges, error) {
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
	cli := aptos.NewClientPool("client")
	g.Go(func() error {
		ch := make(chan clientpool.PoolConfig[aptos.ClientConfig], 1)
		ch <- clientpool.PoolConfig[aptos.ClientConfig]{
			ClientConfigs: []clientpool.ClientConfig[aptos.ClientConfig]{
				{
					Config: aptos.ClientConfig{
						Endpoint: "https://api.mainnet.aptoslabs.com",
					},
				},
			},
		}
		cli.Start(gctx, ch)
		return nil
	})

	sc := chain.NewStdLatestSlotCache[*aptos.Slot](
		"ext",
		"aptos_mainnet",
		time.Second,
		cli,
		aptos.NewExtServerDimension(cli, 10, 3, rg.Range{}, 0),
		nil,
		0,
		nil,
		nil,
	)
	g.Go(func() error {
		return sc.KeepGrowth(gctx)
	})

	var store Storage = &mockStorage{}

	addr := "127.0.0.1:18888"
	h := jsonrpc.NewHandler("test", true, false, nil, nil, "")
	h.RegisterMiddleware(NewRPCService(sc, cli, store)...)

	g.Go(func() error {
		return jsonrpc.ListenAndServe(gctx, ":18888", h)
	})

	// wait for the server to start and the slot cache to populate at least one block
	_, _ = sc.Wait(gctx, 0)

	t.Run("latestHeight", func(t *testing.T) {
		height, err := callRPC[uint64](addr, "aptos_latestHeight", nil)
		assert.NoError(t, err)
		assert.Greater(t, height, uint64(0))
	})

	t.Run("latestNew", func(t *testing.T) {
		tx, err := callRPC[*aptos.Transaction](addr, "aptos_latestNew", []any{"aptos_mainnet"})
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("aptosV2_getLatestMinimalistTransaction", func(t *testing.T) {
		resp, err := callRPC[aptos.GetLatestMinimalistTransactionResponse](addr, "aptosV2_getLatestMinimalistTransaction", []any{uint64(0)})
		assert.NoError(t, err)
		assert.Equal(t, aptos.APIVersion, resp.APIVersion)
		assert.Greater(t, resp.Transaction.Version, uint64(0))
	})

	t.Run("proxy.getTransactionByVersion", func(t *testing.T) {
		resp, err := http.Get("http://" + addr + "/v1/transactions/by_version/1")
		assert.NoError(t, err)
		for k, vs := range resp.Header {
			log.Infof("getTransactionByVersion got header: %s = %s", k, vs)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NoError(t, err)
		var buf bytes.Buffer
		assert.NoError(t, json.Indent(&buf, raw, "", "\t"))
		log.Infof("getTransactionByVersion got body: %s", buf.String())
	})

	t.Run("proxy.getTransactionByVersion", func(t *testing.T) {
		resp, err := http.Get("http://" + addr + "/v1/transactions/by_versions/1")
		assert.NoError(t, err)
		for k, vs := range resp.Header {
			log.Infof("getTransactionByVersion got header: %s = %s", k, vs)
		}
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	b, _ := json.MarshalIndent(cli.Snapshot(), "", "\t")
	log.Infof("client: %s", string(b))

	cancel()
	_ = g.Wait()
}
