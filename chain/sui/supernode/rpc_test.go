package supernode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/sui"
	suitypes "sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/kvstore"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"

	"github.com/stretchr/testify/assert"
)

// mockKVStore implements kvstore.Store[T] as a no-op in-memory store.
type mockKVStore[T any] struct{}

func (m *mockKVStore[T]) List(_ context.Context, _ chan<- string) error { return nil }
func (m *mockKVStore[T]) Get(_ context.Context, _ ...string) (map[string]T, error) {
	return nil, nil
}
func (m *mockKVStore[T]) Set(_ context.Context, _ map[string]T) error { return nil }
func (m *mockKVStore[T]) Del(_ context.Context, _ ...string) error    { return nil }

var _ kvstore.Store[sui.SimpleCheckpoint] = (*mockKVStore[sui.SimpleCheckpoint])(nil)

// mockStorage implements supernode.Storage with empty responses.
type mockStorage struct{}

func (m *mockStorage) QueryCheckpointTime(_ context.Context, _ uint64) (sui.CheckpointTime, error) {
	return sui.CheckpointTime{}, errors.Errorf("not found")
}
func (m *mockStorage) QuerySimpleCheckpoint(_ context.Context, _ uint64) (sui.SimpleCheckpoint, error) {
	return sui.SimpleCheckpoint{}, errors.Errorf("not found")
}
func (m *mockStorage) QueryTransactions(_ context.Context, _ *sui.TransactionQuery) ([]suitypes.TransactionResponseV1, error) {
	return nil, nil
}
func (m *mockStorage) QueryTransactionsV2(
	_ context.Context, _, _ uint64,
	_ sui.TransactionFilter, _ sui.TransactionFetchConfig,
) ([]suitypes.TransactionResponseV1, error) {
	return nil, nil
}
func (m *mockStorage) QueryObjectChanges(_ context.Context, _ *sui.ObjectChangeQuery) ([]suitypes.ObjectChangeExtend, error) {
	return nil, nil
}
func (m *mockStorage) QueryObjectChangesV2(_ context.Context, _, _ uint64, _ sui.ObjectChangeFilter) ([]suitypes.ObjectChangeExtend, error) {
	return nil, nil
}
func (m *mockStorage) QueryObjectsStat(_ context.Context, _, _ uint64, _ []string) (map[string]sui.ObjectStat, error) {
	return nil, nil
}
func (m *mockStorage) Snapshot() any { return nil }

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

func Test_suiRpc(t *testing.T) {
	log.ManuallySetLevel(zapcore.DebugLevel)
	log.BindFlag()

	ctx, cancel := context.WithCancel(context.Background())
	g, gctx := errgroup.WithContext(ctx)

	// prepare client pool targeting Sui mainnet (official Mysten Labs public endpoint)
	cli := sui.NewClientPool("client", nil)
	g.Go(func() error {
		ch := make(chan clientpool.PoolConfig[sui.ClientConfig], 1)
		ch <- clientpool.PoolConfig[sui.ClientConfig]{
			ClientConfigs: []clientpool.ClientConfig[sui.ClientConfig]{
				{
					Config: sui.ClientConfig{
						Endpoint: "https://fullnode.mainnet.sui.io",
					},
				},
			},
		}
		cli.Start(gctx, ch)
		return nil
	})

	ext := sui.NewExtServerDimension(
		cli,
		true, // skipValidate
		10,   // loadConcurrency
		3,    // loadRetry
		rg.Range{},
		0,
	)

	sc := chain.NewStdLatestSlotCache[*sui.Slot](
		"ext",
		"sui_mainnet",
		time.Millisecond*500, // ~500ms Sui checkpoint time
		time.Millisecond*500, // ~500ms Sui checkpoint time
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

	superSvr := NewSuperService(
		sc,
		&mockKVStore[sui.SimpleCheckpoint]{},
		&mockKVStore[sui.CheckpointTime]{},
		&mockKVStore[sui.ObjectCreation]{},
		&mockStorage{},
	)

	addr := "127.0.0.1:18892"
	h := jsonrpc.NewHandler("test", true, false, nil, nil, "")
	h.RegisterMiddleware(NewSuperNode(superSvr, cli)...)

	g.Go(func() error {
		return jsonrpc.ListenAndServe(gctx, ":18892", h)
	})

	// wait for the slot cache to populate at least one checkpoint
	_, _ = sc.Wait(gctx, 0)

	t.Run("sui_getLatestCheckpointSequenceNumber", func(t *testing.T) {
		// returns types.Number which marshals to a quoted decimal string
		result, err := callRPC[string](addr, "sui_getLatestCheckpointSequenceNumber", nil)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)
		n := suitypes.StringToNumber(result)
		assert.Greater(t, n.Uint64(), uint64(0))
	})

	t.Run("sui_getLatestSimpleCheckpoint", func(t *testing.T) {
		resp, err := callRPC[sui.GetLatestSimpleCheckpointResponse](addr, "sui_getLatestSimpleCheckpoint", []any{uint64(0)})
		assert.NoError(t, err)
		assert.Equal(t, sui.APIVersion, resp.APIVersion)
		assert.Greater(t, resp.Checkpoint.Checkpoint, uint64(0))
		assert.NotEmpty(t, resp.Checkpoint.Digest)
	})

	t.Run("proxy.sui_getCheckpoint", func(t *testing.T) {
		// proxied directly to the Sui node
		result, err := callRPC[map[string]any](addr, "sui_getCheckpoint", []any{"1"})
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result["digest"])
	})

	b, _ := json.MarshalIndent(cli.Snapshot(), "", "\t")
	log.Infof("client: %s", string(b))

	cancel()
	_ = g.Wait()
}
