package supernode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/common/jsonrpc"
	rg "sentioxyz/sentio-core/common/range"
)

// logsSpyStorage counts the log queries that reach the storage.
type logsSpyStorage struct {
	Storage
	calls *int
}

func (s logsSpyStorage) QueryLogs(ctx context.Context, where string, limit int, args ...any) ([]types.Log, error) {
	*s.calls++
	return nil, nil
}

func (s logsSpyStorage) QueryLogsBlockSQL(where string) string { return where }

func Test_standardMiddleware_proxyOnly(t *testing.T) {
	var storageCalls int
	h := jsonrpc.NewHandler("test", false, false, nil, nil, "")
	h.RegisterMiddleware(
		NewStandardMiddleware(
			137,
			newFakeSlotCache(100, 105),
			fakeRangeStore{r: rg.NewRange(0, 99)},
			logsSpyStorage{calls: &storageCalls},
		),
		// stands in for the proxy middleware behind the standard one
		func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
				return "proxied", nil
			}
		},
	)
	call := func(method, params, header string) any {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
			`{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":`+params+`}`))
		req.Header.Set("Content-Type", "application/json")
		if header != "" {
			req.Header.Set(proxyOnlyHeader, header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var resp struct{ Result any }
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		return resp.Result
	}

	// a storage-backed standard method: below the latest slot cache it reads the storage, unless the
	// request carries "Proxy-Only: 1", which hands it to the next middleware instead
	const getLogsParams = `[{"fromBlock":"0x62","toBlock":"0x63"}]`
	assert.NotEqual(t, "proxied", call("eth_getLogs", getLogsParams, ""))
	assert.NotEqual(t, "proxied", call("eth_getLogs", getLogsParams, "0"))
	assert.Equal(t, 2, storageCalls)
	assert.Equal(t, "proxied", call("eth_getLogs", getLogsParams, "1"))
	assert.Equal(t, 2, storageCalls)

	// methods that need no storage, or have no chain node equivalent, are served here regardless
	assert.Equal(t, "0x89", call("eth_chainId", "[]", "1"))
	assert.Equal(t, "0x69", call("eth_blockNumber", "[]", "1"))
	latest := call("eth_getLatestBlockNumber", "[0]", "1")
	require.IsType(t, map[string]any{}, latest)
	assert.EqualValues(t, 105, latest.(map[string]any)["latest_block_number"])
}
