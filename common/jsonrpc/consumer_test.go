package jsonrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_consumerManager(t *testing.T) {
	m := newConsumerManager()

	sn := m.Snapshot().(map[string]any)
	assert.Equal(t, 0, sn["currentCount"])
	assert.Equal(t, uint64(0), sn["total"])

	id1 := m.come(&CtxData{ReqID: 1, Method: "eth_call", ReqSrc: RequestSource{Name: "a"}})
	id2 := m.come(&CtxData{ReqID: 2, Method: "eth_getLogs", ReqSrc: RequestSource{Name: "b"}})

	sn = m.Snapshot().(map[string]any)
	assert.Equal(t, 2, sn["currentCount"])
	assert.Equal(t, uint64(2), sn["total"])

	m.leave(id1)
	sn = m.Snapshot().(map[string]any)
	assert.Equal(t, 1, sn["currentCount"])
	assert.Equal(t, uint64(2), sn["total"], "total must never decrease")

	m.leave(id2)
	sn = m.Snapshot().(map[string]any)
	assert.Equal(t, 0, sn["currentCount"])
	assert.Equal(t, uint64(2), sn["total"])
}

func consumersSnapshot(h *Handler) map[string]any {
	return h.Snapshot().(map[string]any)["consumers"].(map[string]any)
}

// Test_Handler_consumersInflight verifies that a request being processed shows
// up in the handler's consumer snapshot while in flight and is removed once it
// finishes.
func Test_Handler_consumersInflight(t *testing.T) {
	h := NewHandler("test", false, false, nil, nil, "")

	entered := make(chan struct{})
	release := make(chan struct{})
	h.RegisterMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			if method == "block" {
				close(entered)
				<-release
				return "ok", nil
			}
			return next(ctx, method, params)
		}
	})

	// nothing in flight initially
	cs := consumersSnapshot(h)
	assert.Equal(t, 0, cs["currentCount"])

	rawReq := httptest.NewRequest(http.MethodPost, "/", nil)
	rawReq.Header.Set("X-Debug-Header", "debug-value")

	done := make(chan struct{})
	var callErr error
	go func() {
		defer close(done)
		_, callErr = h.callMethod(context.Background(), &CtxData{
			ReqID:      42,
			Method:     "block",
			ReqSrc:     RequestSource{Name: "tester"},
			RawReq:     rawReq,
			RawReqBody: []byte(`{"method":"block","params":[1,2,3]}`),
		}, nil)
	}()

	// once the request is being handled, it must appear in the snapshot
	<-entered
	cs = consumersSnapshot(h)
	assert.Equal(t, 1, cs["currentCount"])
	assert.Equal(t, uint64(1), cs["total"])
	current := cs["current"].(map[uint64]any)
	assert.Len(t, current, 1)
	for _, v := range current {
		m := v.(map[string]any)
		assert.Equal(t, "block", m["method"])
		assert.Equal(t, "tester", m["source"])
		assert.Equal(t, uint64(42), m["requestId"])
		assert.Equal(t, `{"method":"block","params":[1,2,3]}`, m["requestBody"])
		headers := m["requestHeaders"].(http.Header)
		assert.Equal(t, "debug-value", headers.Get("X-Debug-Header"))
	}

	// after completion the consumer must be removed, total preserved
	close(release)
	<-done
	assert.NoError(t, callErr)
	cs = consumersSnapshot(h)
	assert.Equal(t, 0, cs["currentCount"])
	assert.Equal(t, uint64(1), cs["total"])
}
