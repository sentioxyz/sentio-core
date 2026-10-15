package supernode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/common/jsonrpc"
)

func Test_standardMiddleware_proxyOnly(t *testing.T) {
	h := jsonrpc.NewHandler("test", false, false, nil, nil, "")
	h.RegisterMiddleware(
		NewStandardMiddleware(137, newFakeSlotCache(100, 105), nil, nil),
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

	// a standard Ethereum method is handed to the next middleware only for "Proxy-Only: 1"
	assert.Equal(t, "0x89", call("eth_chainId", "[]", ""))
	assert.Equal(t, "0x89", call("eth_chainId", "[]", "0"))
	assert.Equal(t, "proxied", call("eth_chainId", "[]", "1"))

	// a Sentio-specific method has no chain node equivalent, so it is served here regardless
	latest := call("eth_getLatestBlockNumber", "[0]", "1")
	require.IsType(t, map[string]any{}, latest)
	assert.EqualValues(t, 105, latest.(map[string]any)["latest_block_number"])
}
