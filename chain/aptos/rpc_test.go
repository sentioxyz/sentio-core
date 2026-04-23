package aptos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
)

type mockStorage struct{}

func (m *mockStorage) Functions(_ context.Context, _ GetFunctionsArgs) ([]*Transaction, error) {
	return nil, nil
}
func (m *mockStorage) FullEvents(_ context.Context, _ GetEventsArgs) ([]*Transaction, error) {
	return nil, nil
}
func (m *mockStorage) ResourceChanges(_ context.Context, _ ResourceChangeArgs) ([]*Transaction, error) {
	return nil, nil
}
func (m *mockStorage) GetTransactionByVersion(_ context.Context, _ uint64) (*Transaction, error) {
	return nil, nil
}
func (m *mockStorage) GetChangeStat(_ context.Context, _ uint64, _ string) (ChangeStat, error) {
	return ChangeStat{}, nil
}
func (m *mockStorage) GetFirstChange(_ context.Context, _ string, _ uint64) (version, blockHeight uint64, has bool, err error) {
	return 0, 0, false, nil
}
func (m *mockStorage) QueryMinimalistTransaction(_ context.Context, _ uint64) (*MinimalistTransaction, error) {
	return nil, nil
}
func (m *mockStorage) QueryTransactions(_ context.Context, _ GetTransactionsRequest) ([]Transaction, error) {
	return nil, nil
}
func (m *mockStorage) QueryResourceChanges(_ context.Context, _ GetResourceChangesRequest) ([]MinimalistTransactionWithChanges, error) {
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
	cli := NewClientPool("client")
	g.Go(func() error {
		ch := make(chan clientpool.PoolConfig[ClientConfig], 1)
		ch <- clientpool.PoolConfig[ClientConfig]{
			ClientConfigs: []clientpool.ClientConfig[ClientConfig]{
				{
					Config: ClientConfig{
						Endpoint: "https://api.mainnet.aptoslabs.com",
					},
				},
			},
		}
		cli.Start(gctx, ch)
		return nil
	})

	sc := chain.NewStdLatestSlotCache[*Slot](
		"ext",
		"aptos_mainnet",
		time.Second,
		cli,
		NewExtServerDimension(cli, 10, 3, rg.Range{}, 0),
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
	time.Sleep(5 * time.Second)

	t.Run("latestHeight", func(t *testing.T) {
		height, err := callRPC[uint64](addr, "aptos_latestHeight", nil)
		assert.NoError(t, err)
		assert.Greater(t, height, uint64(0))
	})

	t.Run("latestNew", func(t *testing.T) {
		tx, err := callRPC[*Transaction](addr, "aptos_latestNew", []any{"aptos_mainnet"})
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("aptosV2_getLatestMinimalistTransaction", func(t *testing.T) {
		resp, err := callRPC[GetLatestMinimalistTransactionResponse](addr, "aptosV2_getLatestMinimalistTransaction", []any{uint64(0)})
		assert.NoError(t, err)
		assert.Equal(t, APIVersion, resp.APIVersion)
		assert.Greater(t, resp.Transaction.Version, uint64(0))
	})

	cancel()
	_ = g.Wait()
}
