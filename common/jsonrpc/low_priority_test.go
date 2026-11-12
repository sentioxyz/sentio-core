package jsonrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/clientpool"
)

// newLowPriorityProbeServer serves "probe", which returns whether its context is marked low priority.
func newLowPriorityProbeServer(t *testing.T) *httptest.Server {
	t.Helper()
	h := NewHandler("test", false, true, nil, nil, "")
	h.RegisterMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			if method == "probe" {
				return clientpool.IsLowPriority(ctx), nil
			}
			return next(ctx, method, params)
		}
	})
	svr := httptest.NewServer(h)
	t.Cleanup(svr.Close)
	return svr
}

const probeRequest = `{"jsonrpc":"2.0","id":1,"method":"probe","params":[]}`

func Test_Handler_lowPrioritySource_http(t *testing.T) {
	svr := newLowPriorityProbeServer(t)
	for _, c := range []struct {
		name   string
		source string
		want   bool
	}{
		{name: "no header", want: false},
		{name: "normal source", source: `{"Name":"RPCNODE","Labels":{"kind":"rpcnode"}}`, want: false},
		{name: "low priority source", source: `{"Name":"RPCNODE","LowPriority":true}`, want: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, svr.URL, strings.NewReader(probeRequest))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			if c.source != "" {
				req.Header.Set(RequestSourceHeader, c.source)
			}
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			var body struct{ Result bool }
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.Equal(t, c.want, body.Result)
		})
	}
}

func Test_Handler_lowPrioritySource_websocket(t *testing.T) {
	svr := newLowPriorityProbeServer(t)
	wsURL := "ws" + strings.TrimPrefix(svr.URL, "http")
	for _, c := range []struct {
		name   string
		header http.Header
		want   bool
	}{
		{name: "no header", want: false},
		{name: "low priority source", header: http.Header{RequestSourceHeader: {`{"LowPriority":true}`}}, want: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, c.header)
			require.NoError(t, err)
			defer conn.Close()
			// the source of the upgrade request applies to every request of the connection
			for range 2 {
				require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(probeRequest)))
				var body struct{ Result bool }
				require.NoError(t, conn.ReadJSON(&body))
				assert.Equal(t, c.want, body.Result)
			}
		})
	}
}
