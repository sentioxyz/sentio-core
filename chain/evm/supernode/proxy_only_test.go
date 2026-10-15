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
		NewStandardMiddleware(137, nil, nil, nil),
		// stands in for the proxy middleware behind the standard one
		func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
				return "proxied", nil
			}
		},
	)
	call := func(header string) any {
		req := httptest.NewRequest(http.MethodPost, "/",
			strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`))
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

	assert.Equal(t, "0x89", call(""), "answered by the standard middleware")
	assert.Equal(t, "0x89", call("0"), "only \"1\" marks a proxy-only request")
	assert.Equal(t, "proxied", call("1"), "handed to the next middleware")
}
