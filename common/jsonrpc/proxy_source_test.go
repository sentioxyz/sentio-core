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
)

type knownCallerGetter struct{}

func (knownCallerGetter) GetByIP(string) (string, map[string]any) {
	return "known-caller", map[string]any{"kind": "driver"}
}

// newNeverDowngradeProbeServer serves "probe", which returns whether the proxy would serve the request
// with clientpool.NeverDowngrade.
func newNeverDowngradeProbeServer(t *testing.T, getter SourceGetter) *httptest.Server {
	t.Helper()
	h := NewHandler("test", false, true, getter, nil, "")
	h.RegisterMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			if method == "probe" {
				return len(sourceOptions[struct{}](GetCtxData(ctx))) > 0, nil
			}
			return next(ctx, method, params)
		}
	})
	svr := httptest.NewServer(h)
	t.Cleanup(svr.Close)
	return svr
}

const (
	probeRequest     = `{"jsonrpc":"2.0","id":1,"method":"probe","params":[]}`
	normalSource     = `{"Name":"proxy","Labels":{"kind":"proxy"}}`
	trueLabelSource  = `{"Name":"proxy","Labels":{"kind":"proxy","` + NeverDowngradeLabel + `":true}}`
	falseLabelSource = `{"Name":"proxy","Labels":{"` + NeverDowngradeLabel + `":false}}`
)

func Test_Handler_neverDowngradeLabel_http(t *testing.T) {
	for _, c := range []struct {
		name   string
		getter SourceGetter
		source string
		want   bool
	}{
		{name: "no header", want: false},
		{name: "without the label", source: normalSource, want: false},
		{name: "label set to false", source: falseLabelSource, want: false},
		{name: "label set to true", source: trueLabelSource, want: true},
		{name: "caller recognized by ip", getter: knownCallerGetter{}, source: trueLabelSource, want: false},
	} {
		t.Run(c.name, func(t *testing.T) {
			svr := newNeverDowngradeProbeServer(t, c.getter)
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

func Test_Handler_neverDowngradeLabel_websocket(t *testing.T) {
	for _, c := range []struct {
		name   string
		getter SourceGetter
		source string
		want   bool
	}{
		{name: "no header", want: false},
		{name: "without the label", source: normalSource, want: false},
		{name: "label set to false", source: falseLabelSource, want: false},
		{name: "label set to true", source: trueLabelSource, want: true},
		{name: "caller recognized by ip", getter: knownCallerGetter{}, source: trueLabelSource, want: false},
	} {
		t.Run(c.name, func(t *testing.T) {
			svr := newNeverDowngradeProbeServer(t, c.getter)
			var header http.Header
			if c.source != "" {
				header = http.Header{RequestSourceHeader: {c.source}}
			}
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(svr.URL, "http"), header)
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
