package evm

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"sentioxyz/sentio-core/chain/clientpool"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStateNode is a minimal JSON-RPC node whose state window can be changed at runtime:
// blocks in [stateFrom, latest] have state data.
type fakeStateNode struct {
	mu        sync.Mutex
	latest    uint64
	stateFrom uint64 // 0 means archive
	chainID   uint64 // 0 means eth_chainId is not expected to be called
	minProbed uint64 // lowest block eth_getBalance/eth_getCode was asked for
	hangBelow uint64 // state calls below this block hang past the method timeout (0 disables)
}

func (n *fakeStateNode) setStateFrom(from uint64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.stateFrom = from
}

func (n *fakeStateNode) handle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	n.mu.Lock()
	latest, stateFrom := n.latest, n.stateFrom
	n.mu.Unlock()
	reply := func(result string) {
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, result)
	}
	switch req.Method {
	case "eth_chainId":
		reply(fmt.Sprintf(`"0x%x"`, n.chainID))
	case "eth_getBlockByNumber":
		reply(fmt.Sprintf(`{"number":"0x%x","hash":"0x%064x","stateRoot":"0x%064x","timestamp":"0x%x"}`,
			latest, latest, 1, time.Now().Unix()))
	case "eth_getBalance", "eth_getCode":
		var bn hexutil.Uint64
		if len(req.Params) < 2 || json.Unmarshal(req.Params[1], &bn) != nil {
			reply(`"0x0"`) // omitted, or a block tag like "latest", or a block hash
			return
		}
		n.mu.Lock()
		if uint64(bn) < n.minProbed {
			n.minProbed = uint64(bn)
		}
		hangBelow := n.hangBelow
		n.mu.Unlock()
		if uint64(bn) < hangBelow {
			time.Sleep(time.Second)
		}
		if uint64(bn) >= stateFrom && uint64(bn) <= latest {
			reply(`"0x0"`)
			return
		}
		_, _ = fmt.Fprintf(w,
			`{"jsonrpc":"2.0","id":%s,"error":{"code":-32000,"message":"missing trie node at block %d"}}`,
			req.ID, uint64(bn))
	default:
		http.Error(w, "unexpected method "+req.Method, http.StatusBadRequest)
	}
}

func newStateProbeClient(t *testing.T, latest uint64, stateFrom uint64) (*Client, *fakeStateNode) {
	t.Helper()
	return newChainStateProbeClient(t, 0, latest, stateFrom)
}

func newChainStateProbeClient(t *testing.T, chainID, latest, stateFrom uint64) (*Client, *fakeStateNode) {
	t.Helper()
	return newHangingStateProbeClient(t, chainID, latest, stateFrom, 0)
}

func newHangingStateProbeClient(
	t *testing.T, chainID, latest, stateFrom, hangBelow uint64,
) (*Client, *fakeStateNode) {
	t.Helper()
	node := &fakeStateNode{
		latest: latest, stateFrom: stateFrom, chainID: chainID, minProbed: math.MaxUint64, hangBelow: hangBelow,
	}
	server := httptest.NewServer(http.HandlerFunc(node.handle))
	t.Cleanup(server.Close)
	cli, err := NewClient(ClientConfig{
		JSONRPCConfig: clientpool.JSONRPCConfig{
			Endpoint:      server.URL,
			MethodTimeout: map[string]time.Duration{"eth_getBalance": 200 * time.Millisecond},
		},
		ChainID: chainID,
	}, func(string, time.Duration, bool) {})
	require.NoError(t, err)
	_, err = cli.Init(context.Background())
	require.NoError(t, err)
	return cli, node
}

func Test_NewClient_invalidEndpoint_failsAtBuild(t *testing.T) {
	// Static config validation happens at build time, not in Init.
	_, err := NewClient(ClientConfig{
		JSONRPCConfig: clientpool.JSONRPCConfig{Endpoint: "://not-a-url"},
	}, func(string, time.Duration, bool) {})
	require.ErrorIs(t, err, clientpool.ErrInvalidConfig)
}

func Test_Init_arbitrumProbesClassicRangeOnce(t *testing.T) {
	const arb = 42161
	latest := arbitrumNitroGenesis + 200000

	// A node serving the whole history: the classic range is probed with block 1 only.
	full, node := newChainStateProbeClient(t, arb, latest, 1)
	assert.Equal(t, arbitrumClassicProbeBlock, full.hasStateDataFrom.Load())
	assert.Equal(t, arbitrumClassicProbeBlock, node.minProbed)

	// A nitro-only node misses the classic range, its state starts at the nitro genesis.
	nitro, node := newChainStateProbeClient(t, arb, latest, arbitrumNitroGenesis)
	assert.Equal(t, arbitrumNitroGenesis, nitro.hasStateDataFrom.Load())
	assert.Equal(t, arbitrumClassicProbeBlock, node.minProbed)

	// A pruned nitro node is bisected above the classic range, which is not probed at all.
	pruned, node := newChainStateProbeClient(t, arb, latest, arbitrumNitroGenesis+50000)
	assert.Equal(t, arbitrumNitroGenesis+50000, pruned.hasStateDataFrom.Load())
	assert.GreaterOrEqual(t, node.minProbed, arbitrumNitroGenesis)

	// The classic probe hanging (a busy classic node behind the nitro node) is resolved as served
	// after a couple of attempts instead of keeping the client out of the pool.
	hanging, node := newHangingStateProbeClient(t, arb, latest, 1, arbitrumNitroGenesis)
	assert.Equal(t, arbitrumClassicProbeBlock, hanging.hasStateDataFrom.Load())
	assert.Equal(t, arbitrumClassicProbeBlock, node.minProbed)

	// Other chains keep probing all the way down to block 0.
	_, node = newStateProbeClient(t, 100000, 0)
	assert.Equal(t, uint64(0), node.minProbed)
}

func Test_Init_detectsArchiveAndBoundary(t *testing.T) {
	archive, _ := newStateProbeClient(t, 100000, 0)
	assert.Equal(t, uint64(0), archive.hasStateDataFrom.Load())

	full, _ := newStateProbeClient(t, 100000, 80001) // window of 20000 blocks (>= noStateLimit)
	assert.Equal(t, uint64(80001), full.hasStateDataFrom.Load())

	sliding, _ := newStateProbeClient(t, 100000, 99001) // window of 1000 blocks (< noStateLimit)
	assert.Equal(t, uint64(math.MaxUint64), sliding.hasStateDataFrom.Load())
}

func Test_reInit_redetectsStateBoundary(t *testing.T) {
	// The client pool periodically re-runs Init on a live client. Init must be re-entrant and
	// must refresh hasStateDataFrom in both directions.
	cli, node := newStateProbeClient(t, 100000, 0)
	require.Equal(t, uint64(0), cli.hasStateDataFrom.Load())

	// e.g. Zircuit mainnet: the backend behind the endpoint was replaced and history got pruned
	node.setStateFrom(80001)
	_, err := cli.Init(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(80001), cli.hasStateDataFrom.Load())

	// and the other direction: a full node record recovers once the backend is an archive again
	node.setStateFrom(0)
	_, err = cli.Init(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(0), cli.hasStateDataFrom.Load())
}

func Test_Init_customChainID_reentrant(t *testing.T) {
	// A numeric chain ID that is not in the chains registry must resolve to a nil info exactly
	// once (in NewClient); a re-entrant Init must not keep writing the field while concurrent
	// calls read it.
	node := &fakeStateNode{latest: 100000, stateFrom: 0, chainID: 999999}
	server := httptest.NewServer(http.HandlerFunc(node.handle))
	t.Cleanup(server.Close)
	cli, err := NewClient(ClientConfig{
		JSONRPCConfig: clientpool.JSONRPCConfig{Endpoint: server.URL},
		ChainID:       999999,
	}, func(string, time.Duration, bool) {})
	require.NoError(t, err)
	assert.Nil(t, cli.info)
	assert.False(t, cli.isTronChain())

	for i := 0; i < 2; i++ {
		_, err := cli.Init(context.Background())
		require.NoError(t, err)
	}
	assert.Nil(t, cli.info)
	assert.Equal(t, uint64(0), cli.hasStateDataFrom.Load())
}

func Test_reInit_keepsServingConcurrentCalls(t *testing.T) {
	// Init runs while the client keeps serving: concurrent state reads must never observe a
	// half-updated client (torn hasStateDataFrom, nil rpcClient, ...). Run with -race.
	cli, node := newStateProbeClient(t, 100000, 0)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = cli.CallContext(context.Background(), nil, "test", "eth_getBalance",
				"0x26fCb50eEC367ddAB060ccf5E7394Cecd95F7Db2", hexutil.Uint64(90000))
		}
	}()
	for i := 0; i < 3; i++ {
		if i%2 == 1 {
			node.setStateFrom(80001)
		} else {
			node.setStateFrom(0)
		}
		_, err := cli.Init(context.Background())
		require.NoError(t, err)
	}
	close(stop)
	wg.Wait()
}
