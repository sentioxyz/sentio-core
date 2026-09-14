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

// mockStorageJSONRPC implements supernode.StorageJSONRPC with empty responses.
type mockStorageJSONRPC struct{}

func (m *mockStorageJSONRPC) QueryCheckpointTime(_ context.Context, _ uint64) (sui.CheckpointTime, error) {
	return sui.CheckpointTime{}, errors.Errorf("not found")
}
func (m *mockStorageJSONRPC) QuerySimpleCheckpoint(_ context.Context, _ uint64) (sui.SimpleCheckpoint, error) {
	return sui.SimpleCheckpoint{}, errors.Errorf("not found")
}
func (m *mockStorageJSONRPC) QueryTransactions(_ context.Context, _ *sui.TransactionQuery) ([]suitypes.TransactionResponseV1, error) {
	return nil, nil
}
func (m *mockStorageJSONRPC) QueryTransactionsV2(
	_ context.Context, _, _ uint64,
	_ sui.TransactionFilter, _ sui.TransactionFetchConfig, _ int,
) ([]suitypes.TransactionResponseV1, error) {
	return nil, nil
}
func (m *mockStorageJSONRPC) QueryObjectChanges(_ context.Context, _ *sui.ObjectChangeQuery) ([]suitypes.ObjectChangeExtend, error) {
	return nil, nil
}
func (m *mockStorageJSONRPC) QueryObjectChangesV2(
	_ context.Context, _, _ uint64, _ sui.ObjectChangeFilter, _ int,
) ([]suitypes.ObjectChangeExtend, error) {
	return nil, nil
}
func (m *mockStorageJSONRPC) QueryObjectsStat(_ context.Context, _, _ uint64, _ []string) (map[string]sui.ObjectStat, error) {
	return nil, nil
}
func (m *mockStorageJSONRPC) QueryLastObjectChange(_ context.Context, _ string, _, _ uint64) (*sui.ObjectChangeRecord, error) {
	return nil, nil
}
func (m *mockStorageJSONRPC) Snapshot() any { return nil }

// mockStorageGRPC implements supernode.StorageGRPC with empty responses.
type mockStorageGRPC struct {
	committedRanges []rg.Range
	rangeErr        error
	rangeCalls      int
	lastChange      *sui.ObjectChangeRecord
	lastChangeErr   error
	queryCalls      int
	queryObjectID   string
	queryCheckpoint uint64
}

func (m *mockStorageGRPC) CommittedObjectHistoryRange(_ context.Context) (rg.Range, error) {
	m.rangeCalls++
	if m.rangeErr != nil {
		return rg.EmptyRange, m.rangeErr
	}
	if len(m.committedRanges) == 0 {
		return rg.EmptyRange, nil
	}
	return m.committedRanges[min(m.rangeCalls-1, len(m.committedRanges)-1)], nil
}

func (m *mockStorageGRPC) QuerySimpleCheckpoint(_ context.Context, _ uint64) (sui.SimpleCheckpoint, error) {
	return sui.SimpleCheckpoint{}, errors.Errorf("not found")
}
func (m *mockStorageGRPC) QueryTransactions(
	_ context.Context, _, _ uint64,
	_ sui.TransactionFilter, _ sui.TransactionFetchConfig, _ int,
) ([]*sui.ExtendedGrpcTransaction, error) {
	return nil, nil
}
func (m *mockStorageGRPC) QueryObjectChanges(
	_ context.Context, _, _ uint64, _ sui.ObjectChangeFilter, _ int,
) ([]*sui.ExtendedGrpcChangedObject, error) {
	return nil, nil
}
func (m *mockStorageGRPC) QueryObjectsStat(_ context.Context, _, _ uint64, _ []string) (map[string]sui.ObjectStat, error) {
	return nil, nil
}
func (m *mockStorageGRPC) QueryLastObjectChange(_ context.Context, id string, _ uint64, checkpoint uint64) (*sui.ObjectChangeRecord, error) {
	m.queryCalls++
	m.queryObjectID, m.queryCheckpoint = id, checkpoint
	return m.lastChange, m.lastChangeErr
}
func (m *mockStorageGRPC) Snapshot() any { return nil }

func TestGrpcObjectChangeAtCheckpoint(t *testing.T) {
	const checkpoint = uint64(100)
	const objectID = "0x123"
	live := &sui.ObjectChangeRecord{Checkpoint: 12, ObjectVersion: 14, Type: "created", TxDigest: "tx"}
	for _, test := range []struct {
		name       string
		ranges     []rg.Range
		record     *sui.ObjectChangeRecord
		rangeErr   error
		queryErr   error
		wantErr    bool
		queryCalls int
	}{
		{name: "complete absence", ranges: []rg.Range{rg.NewRange(0, checkpoint)}, queryCalls: 1},
		{name: "complete idle object", ranges: []rg.Range{rg.NewRange(0, checkpoint)}, record: live, queryCalls: 1},
		{name: "truncated missing creation", ranges: []rg.Range{rg.NewRange(50, checkpoint)}, wantErr: true},
		{name: "truncated surviving row", ranges: []rg.Range{rg.NewRange(50, checkpoint)}, record: live, wantErr: true},
		{name: "lagging archive with old version", ranges: []rg.Range{rg.NewRange(0, 99)}, record: live, wantErr: true},
		{name: "empty archive", wantErr: true},
		{name: "unbounded metadata", ranges: []rg.Range{{Start: 0}}, wantErr: true},
		{name: "coverage read failure", rangeErr: errors.New("range unavailable"), wantErr: true},
		{name: "object read failure", ranges: []rg.Range{rg.NewRange(0, checkpoint)}, queryErr: errors.New("object unavailable"), wantErr: true, queryCalls: 1},
		{name: "retention during absent lookup", ranges: []rg.Range{rg.NewRange(0, checkpoint), rg.NewRange(50, checkpoint)}, wantErr: true, queryCalls: 1},
		{name: "retention during live lookup", ranges: []rg.Range{rg.NewRange(0, checkpoint), rg.NewRange(50, checkpoint)}, record: live, wantErr: true, queryCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			storage := &mockStorageGRPC{committedRanges: test.ranges, rangeErr: test.rangeErr, lastChange: test.record, lastChangeErr: test.queryErr}
			// No slot cache: consulting the live head or stitching in its tail would panic.
			service := &SuperService{storageGRPC: storage}
			result, err := service.GetGrpcObjectChangeAtCheckpoint(context.Background(), objectID, checkpoint)
			assert.Equal(t, test.wantErr, err != nil)
			assert.Equal(t, test.queryCalls, storage.queryCalls)
			if test.wantErr {
				assert.Nil(t, result)
			} else if assert.NotNil(t, result) {
				assert.Equal(t, objectID, result.ObjectID)
				assert.Equal(t, checkpoint, result.Checkpoint)
				assert.Zero(t, result.HistoryStartCheckpoint)
				assert.Equal(t, test.record, result.Change)
				assert.Equal(t, 2, storage.rangeCalls)
				assert.Equal(t, objectID, storage.queryObjectID)
				assert.Equal(t, checkpoint, storage.queryCheckpoint)
			}
		})
	}
}

func TestGrpcObjectChangeAtCheckpointRPC(t *testing.T) {
	service := &SuperService{storageGRPC: &mockStorageGRPC{committedRanges: []rg.Range{rg.NewRange(0, 100)}}}
	handler := NewSuperNode(service, &sui.ClientPool{})[0](func(context.Context, string, json.RawMessage) (any, error) {
		return nil, errors.New("strict history method was not registered")
	})
	result, err := handler(context.Background(), "sui_getGrpcObjectChangeAtCheckpoint", json.RawMessage(`["0x123",100]`))
	assert.NoError(t, err)
	raw, err := json.Marshal(result)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"object_id":"0x123","checkpoint":100,"history_start_checkpoint":0,"change":null}`, string(raw))
	_, err = (&SuperService{}).GetGrpcObjectChangeAtCheckpoint(context.Background(), "0x123", 100)
	assert.Error(t, err)
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
						JSONRPCConfig: clientpool.JSONRPCConfig{Endpoint: "https://fullnode.mainnet.sui.io"},
					},
				},
			},
		}
		cli.Start(gctx, ch)
		return nil
	})

	ext := sui.NewExtServerDimension(
		cli,
		suitypes.VariationSUI,
		true,  // enableJSONRPC
		true,  // skipValidate
		false, // enableGrpc
		0,     // loadObjectsBatchSize
		0,     // loadObjectsConcurrency
		10,    // loadConcurrency
		3,     // loadRetry
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
		cli,
		sc,
		&mockKVStore[sui.SimpleCheckpoint]{},
		&mockKVStore[sui.CheckpointTime]{},
		&mockKVStore[sui.ObjectCreation]{},
		&mockStorageJSONRPC{},
		&mockStorageGRPC{},
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
