package evm

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sentioxyz/sentio-core/chain/clientpool"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	receiptAnswerOK    = "ok"    // the complete and correct result
	receiptAnswerNull  = "null"  // result null without any error, like a lagging backend behind a load balancer
	receiptAnswerError = "error" // a JSON-RPC error
)

// fakeReceiptNode is a minimal JSON-RPC node serving the receipts of one block.
type fakeReceiptNode struct {
	blockHash     common.Hash
	txs           []common.Hash
	blockReceipts string // answer of eth_getBlockReceipts
	txReceipt     string // answer of eth_getTransactionReceipt

	blockReceiptsCalls atomic.Int64
	txReceiptCalls     atomic.Int64
}

func (n *fakeReceiptNode) receipt(txHash common.Hash) map[string]any {
	return map[string]any{
		"blockHash":       n.blockHash,
		"blockNumber":     "0x64",
		"transactionHash": txHash,
		"logs":            []any{},
		"status":          "0x1",
	}
}

func (n *fakeReceiptNode) handle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	reply := func(answer string, result any) {
		switch answer {
		case receiptAnswerOK:
			raw, _ := json.Marshal(result)
			_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, raw)
		case receiptAnswerNull:
			_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":null}`, req.ID)
		default:
			_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32000,"message":"boom"}}`, req.ID)
		}
	}
	switch req.Method {
	case "eth_getBlockByNumber":
		reply(receiptAnswerOK, map[string]any{
			"number":    "0x64",
			"hash":      common.BigToHash(big.NewInt(100)),
			"stateRoot": common.BigToHash(big.NewInt(1)),
			"timestamp": fmt.Sprintf("0x%x", time.Now().Unix()),
		})
	case "eth_getBlockReceipts":
		n.blockReceiptsCalls.Add(1)
		receipts := make([]any, 0, len(n.txs))
		for _, tx := range n.txs {
			receipts = append(receipts, n.receipt(tx))
		}
		reply(n.blockReceipts, receipts)
	case "eth_getTransactionReceipt":
		n.txReceiptCalls.Add(1)
		var txHash common.Hash
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params[0], &txHash)
		}
		reply(n.txReceipt, n.receipt(txHash))
	default:
		http.Error(w, "unexpected method "+req.Method, http.StatusBadRequest)
	}
}

// newReceiptTestDimension starts a client pool with the given nodes, all with the same priority, and
// waits until every node is initialized.
func newReceiptTestDimension(t *testing.T, nodes ...*fakeReceiptNode) *ExtServerDimension {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	pool := NewClientPool(t.Name(), nil)
	var cfg clientpool.PoolConfig[ClientConfig]
	for _, node := range nodes {
		server := httptest.NewServer(http.HandlerFunc(node.handle))
		t.Cleanup(server.Close)
		cfg.ClientConfigs = append(cfg.ClientConfigs, clientpool.ClientConfig[ClientConfig]{
			Config: ClientConfig{
				JSONRPCConfig:        clientpool.JSONRPCConfig{Endpoint: server.URL},
				IgnoreStateFromCheck: true,
			},
		})
	}
	ch := make(chan clientpool.PoolConfig[ClientConfig], 1)
	ch <- cfg
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pool.Start(ctx, ch)
	}()
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	// visit every client: an uninitialized client is waited for, so this returns only after all
	// clients are usable
	waitCtx, waitCancel := context.WithTimeout(ctx, 10*time.Second)
	defer waitCancel()
	require.NoError(t, pool.WaitReady(waitCtx))
	for {
		visited := make(map[*Client]bool)
		r := pool.UseClient(waitCtx, "wait", func(ctx context.Context, cli *Client) clientpool.Result {
			visited[cli] = true
			return clientpool.Result{BrokenForTask: true}
		})
		if len(visited) == len(nodes) {
			break
		}
		require.ErrorIs(t, r.Err, clientpool.ErrNoValidClient)
		select {
		case <-waitCtx.Done():
			require.FailNow(t, "not all clients initialized", "visited %d of %d", len(visited), len(nodes))
		case <-time.After(10 * time.Millisecond):
		}
	}

	return &ExtServerDimension{client: pool}
}

func newReceiptTestSlot(blockHash common.Hash, txs []common.Hash) *Slot {
	st := &Slot{Header: &ExtendedHeader{Hash: blockHash}, Block: &RPCBlock{}}
	st.Header.Number = big.NewInt(100)
	for _, tx := range txs {
		st.Block.Transactions = append(st.Block.Transactions, RPCTransaction{Hash: tx})
	}
	return st
}

func newReceiptTestNode(blockHash common.Hash, txs []common.Hash, blockReceipts, txReceipt string) *fakeReceiptNode {
	return &fakeReceiptNode{blockHash: blockHash, txs: txs, blockReceipts: blockReceipts, txReceipt: txReceipt}
}

var (
	receiptTestBlockHash = common.HexToHash("0xb10c")
	receiptTestTxs       = []common.Hash{common.HexToHash("0x01"), common.HexToHash("0x02")}
)

// loadReceiptsUntilBadNodeUsed calls loadReceipts repeatedly, every call must succeed, until the bad
// node has been chosen (the choice between clients with the same priority is random).
func loadReceiptsUntilBadNodeUsed(t *testing.T, d *ExtServerDimension, bad *fakeReceiptNode) {
	t.Helper()
	for i := 0; i < 64 && bad.blockReceiptsCalls.Load()+bad.txReceiptCalls.Load() == 0; i++ {
		st := newReceiptTestSlot(receiptTestBlockHash, receiptTestTxs)
		require.NoError(t, d.loadReceipts(context.Background(), st))
		require.Len(t, st.Receipts, len(receiptTestTxs))
		for j, tx := range receiptTestTxs {
			assert.Equal(t, tx, st.Receipts[j].TxHash)
			assert.Equal(t, receiptTestBlockHash, st.Receipts[j].BlockHash)
		}
	}
	require.NotZero(t, bad.blockReceiptsCalls.Load()+bad.txReceiptCalls.Load(), "the bad node was never chosen")
}

func Test_loadReceipts_nullBlockReceiptsRetriedOnOtherClient(t *testing.T) {
	// eth_getBlockReceipts and eth_getTransactionReceipt both answer null without an error on the
	// bad node, the good node must be used instead
	bad := newReceiptTestNode(receiptTestBlockHash, receiptTestTxs, receiptAnswerNull, receiptAnswerNull)
	good := newReceiptTestNode(receiptTestBlockHash, receiptTestTxs, receiptAnswerOK, receiptAnswerOK)
	d := newReceiptTestDimension(t, bad, good)
	loadReceiptsUntilBadNodeUsed(t, d, bad)
	assert.Zero(t, good.txReceiptCalls.Load(), "the complete block receipts must not fall back to per-tx receipts")
}

func Test_loadReceipts_nullTxReceiptRetriedOnOtherClient(t *testing.T) {
	// no node supports eth_getBlockReceipts, so every receipt is loaded by eth_getTransactionReceipt,
	// where the bad node answers null without an error
	bad := newReceiptTestNode(receiptTestBlockHash, receiptTestTxs, receiptAnswerError, receiptAnswerNull)
	good := newReceiptTestNode(receiptTestBlockHash, receiptTestTxs, receiptAnswerError, receiptAnswerOK)
	d := newReceiptTestDimension(t, bad, good)
	for i := 0; i < 64 && bad.txReceiptCalls.Load() == 0; i++ {
		st := newReceiptTestSlot(receiptTestBlockHash, receiptTestTxs)
		require.NoError(t, d.loadReceipts(context.Background(), st))
		require.Len(t, st.Receipts, len(receiptTestTxs))
		for j, tx := range receiptTestTxs {
			assert.Equal(t, tx, st.Receipts[j].TxHash)
		}
	}
	require.NotZero(t, bad.txReceiptCalls.Load(), "the bad node was never chosen")
}

func Test_loadReceipts_rpcErrorRetriedOnOtherClient(t *testing.T) {
	bad := newReceiptTestNode(receiptTestBlockHash, receiptTestTxs, receiptAnswerError, receiptAnswerError)
	good := newReceiptTestNode(receiptTestBlockHash, receiptTestTxs, receiptAnswerOK, receiptAnswerOK)
	d := newReceiptTestDimension(t, bad, good)
	loadReceiptsUntilBadNodeUsed(t, d, bad)
}

func Test_loadReceipts_allClientsNullFails(t *testing.T) {
	a := newReceiptTestNode(receiptTestBlockHash, receiptTestTxs, receiptAnswerNull, receiptAnswerNull)
	b := newReceiptTestNode(receiptTestBlockHash, receiptTestTxs, receiptAnswerNull, receiptAnswerNull)
	d := newReceiptTestDimension(t, a, b)
	st := newReceiptTestSlot(receiptTestBlockHash, receiptTestTxs)
	err := d.loadReceipts(context.Background(), st)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "eth_getTransactionReceipt")
	assert.Nil(t, st.Receipts, "no receipt may be filled for a failed block")
	// both nodes were tried for both methods
	assert.NotZero(t, a.blockReceiptsCalls.Load())
	assert.NotZero(t, b.blockReceiptsCalls.Load())
	assert.NotZero(t, a.txReceiptCalls.Load())
	assert.NotZero(t, b.txReceiptCalls.Load())
}

func Test_loadReceipts_emptyBlock(t *testing.T) {
	// a block without transactions legally gets null or an empty list from eth_getBlockReceipts
	for _, answer := range []string{receiptAnswerNull, receiptAnswerOK} {
		t.Run(answer, func(t *testing.T) {
			node := newReceiptTestNode(receiptTestBlockHash, nil, answer, receiptAnswerError)
			d := newReceiptTestDimension(t, node)
			st := newReceiptTestSlot(receiptTestBlockHash, nil)
			require.NoError(t, d.loadReceipts(context.Background(), st))
			assert.Empty(t, st.Receipts)
			assert.Empty(t, st.Logs)
			assert.Equal(t, int64(1), node.blockReceiptsCalls.Load())
			assert.Zero(t, node.txReceiptCalls.Load())
		})
	}
}

func Test_checkBlockReceiptsComplete(t *testing.T) {
	txs := []RPCTransaction{{Hash: receiptTestTxs[0]}, {Hash: receiptTestTxs[1]}}
	assert.NoError(t, checkBlockReceiptsComplete(nil, nil))
	assert.NoError(t, checkBlockReceiptsComplete(
		[]ExtendedReceipt{{TxHash: receiptTestTxs[0]}, {TxHash: receiptTestTxs[1]}}, txs))
	assert.Error(t, checkBlockReceiptsComplete(nil, txs))
	assert.Error(t, checkBlockReceiptsComplete([]ExtendedReceipt{{TxHash: receiptTestTxs[0]}}, txs))
	assert.Error(t, checkBlockReceiptsComplete(
		[]ExtendedReceipt{{TxHash: receiptTestTxs[0]}, {TxHash: receiptTestTxs[0]}}, txs))
}
