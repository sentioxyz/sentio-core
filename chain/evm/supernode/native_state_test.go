package supernode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/common/jsonrpc"
)

// fakeBlockNumberStore resolves the hashes of the blocks [0, to] and counts the lookups.
type fakeBlockNumberStore struct {
	to    uint64
	err   error
	calls int
}

func (s *fakeBlockNumberStore) QueryBlockNumberByHash(ctx context.Context, hash common.Hash) (*uint64, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if n := hash.Big(); n.IsUint64() && n.Uint64() <= s.to {
		number := n.Uint64()
		return &number, nil
	}
	return nil, nil
}

func Test_nativeStateOnlyMiddleware(t *testing.T) {
	const nativeFrom = 50
	newCaller := func(middleware jsonrpc.Middleware) func(method, params string, labeled bool) (any, string) {
		h := jsonrpc.NewHandler("test", false, false, nil, nil, "")
		h.RegisterMiddleware(
			middleware,
			// stands in for the proxy middleware
			func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
				return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
					return "proxied", nil
				}
			},
		)
		return func(method, params string, labeled bool) (any, string) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
				`{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":`+params+`}`))
			req.Header.Set("Content-Type", "application/json")
			if labeled {
				req.Header.Set(jsonrpc.RequestSourceHeader, `{"Labels":{"`+jsonrpc.NativeStateOnlyLabel+`":true}}`)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var resp struct {
				Result any
				Error  *struct{ Message string }
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			if resp.Error != nil {
				return resp.Result, resp.Error.Message
			}
			return resp.Result, ""
		}
	}
	callArgs := func(block string) string {
		return `[{"to":"0x0000000000000000000000000000000000000001"},` + block + `]`
	}
	hashArg := func(n uint64) string { return `{"blockHash":"` + hashOf(n).Hex() + `"}` }

	// latest slot cache holds [100, 105], the store holds [0, 99]
	store := &fakeBlockNumberStore{to: 99}
	call := newCaller(NewNativeStateOnlyMiddleware(nativeFrom, newFakeSlotCache(100, 105), store))
	served := func(method, params string, labeled bool) {
		t.Helper()
		result, errMsg := call(method, params, labeled)
		assert.Empty(t, errMsg)
		assert.Equal(t, "proxied", result)
	}
	rejected := func(method, params, reason string) {
		t.Helper()
		_, errMsg := call(method, params, true)
		assert.Contains(t, errMsg, "the method "+method+" is not available at")
		assert.Contains(t, errMsg, reason)
	}

	// without the label nothing is checked
	served("eth_call", callArgs(`"0x1"`), false)
	served("eth_call", callArgs(`"earliest"`), false)
	served("eth_call", callArgs(hashArg(1)), false)
	served("eth_call", callArgs(hashOfUnknown), false)
	assert.Zero(t, store.calls)

	// block numbers and tags
	served("eth_call", callArgs(`"0x32"`), true)
	served("eth_call", callArgs(`"0x1000"`), true)
	served("eth_call", callArgs(`{"blockNumber":"0x32"}`), true)
	served("eth_call", `[{"to":"0x0000000000000000000000000000000000000001"}]`, true)
	for _, tag := range []string{"latest", "pending", "safe", "finalized"} {
		served("eth_call", callArgs(`"`+tag+`"`), true)
	}
	rejected("eth_call", callArgs(`"0x31"`), "block 49: only blocks from 50 on")
	rejected("eth_call", callArgs(`"0x0"`), "block 0")
	rejected("eth_call", callArgs(`{"blockNumber":"0x31"}`), "block 49")
	rejected("eth_call", callArgs(`"earliest"`), "the earliest block")
	// every state method is covered, with its own block parameter position
	rejected("eth_getBalance", `["0x0000000000000000000000000000000000000001","0x31"]`, "block 49")
	rejected("eth_getStorageAt", `["0x0000000000000000000000000000000000000001","0x0","0x31"]`, "block 49")
	served("eth_getStorageAt", `["0x0000000000000000000000000000000000000001","0x0","0x32"]`, true)
	// other methods are not looked at
	served("eth_getBlockByNumber", `["0x1",false]`, true)
	served("eth_getLogs", `[{"blockHash":"`+hashOf(1).Hex()+`"}]`, true)
	assert.Zero(t, store.calls)

	// a block hash found in the latest slot cache needs no store lookup
	served("eth_call", callArgs(hashArg(100)), true)
	served("eth_call", callArgs(`"`+hashOf(105).Hex()+`"`), true)
	assert.Zero(t, store.calls)
	// otherwise the store resolves it, once per hash
	served("eth_call", callArgs(hashArg(50)), true)
	served("eth_call", callArgs(hashArg(50)), true)
	assert.Equal(t, 1, store.calls)
	rejected("eth_call", callArgs(hashArg(49)), "block 49: only blocks from 50 on")
	rejected("eth_getBalance", `["0x0000000000000000000000000000000000000001",`+hashArg(49)+`]`, "block 49")
	assert.Equal(t, 2, store.calls)
	// a hash nobody knows is never proxied, and is looked up again next time
	rejected("eth_call", callArgs(hashOfUnknown), "unknown block")
	rejected("eth_call", callArgs(hashOfUnknown), "unknown block")
	assert.Equal(t, 4, store.calls)
	// neither is one the store failed to resolve
	store.err = errors.New("store is down")
	rejected("eth_call", callArgs(hashArg(60)), "cannot be resolved for now")
	store.err = nil
	served("eth_call", callArgs(hashArg(60)), true)

	// without a store only the hashes in the latest slot cache are known
	call = newCaller(NewNativeStateOnlyMiddleware(nativeFrom, newFakeSlotCache(100, 105), nil))
	served("eth_call", callArgs(hashArg(100)), true)
	rejected("eth_call", callArgs(hashArg(60)), "unknown block")
	// with neither, none is
	call = newCaller(NewNativeStateOnlyMiddleware(nativeFrom, nil, nil))
	served("eth_call", callArgs(`"0x32"`), true)
	rejected("eth_call", callArgs(`"0x31"`), "block 49")
	rejected("eth_call", callArgs(hashArg(100)), "unknown block")

	// a chain without a native range boundary serves everything
	call = newCaller(NewNativeStateOnlyMiddleware(0, nil, nil))
	served("eth_call", callArgs(`"0x0"`), true)
	served("eth_call", callArgs(`"earliest"`), true)
	served("eth_call", callArgs(hashOfUnknown), true)
}

// hashOfUnknown is a block parameter holding a hash no slot cache or store of the tests knows.
const hashOfUnknown = `{"blockHash":"0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}`
