package jsonrpc

import (
	"net/http"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/utils"
)

// consumerBodyMaxLen bounds how much of the raw request body is kept in a
// consumer snapshot, to avoid bloating the snapshot with large payloads.
const consumerBodyMaxLen = 5 * 1024

// consumer represents an in-flight request being handled by the Handler, from
// the moment it enters callMethod until it returns. It mirrors the consumer
// tracking in chain/clientpool so that Snapshot can show which requests are
// currently being processed and for how long (useful for spotting stuck or
// long-running requests). It also keeps the raw request body and headers to
// aid debugging.
type consumer struct {
	reqID      uint64
	reqSubID   uint64
	method     string
	source     RequestSource
	enterAt    time.Time
	reqBody    []byte
	reqHeaders http.Header
}

func (c consumer) Snapshot() any {
	sn := map[string]any{
		"requestId":     c.reqID,
		"requestSubId":  c.reqSubID,
		"method":        c.method,
		"source":        c.source.Summary(),
		"enterAt":       c.enterAt.String(),
		"enterDuration": time.Since(c.enterAt).String(),
		"requestBody":   utils.StringSummaryV1(string(c.reqBody), consumerBodyMaxLen),
	}
	if c.reqHeaders != nil {
		sn["requestHeaders"] = c.reqHeaders
	}
	return sn
}

// consumerManager tracks the set of in-flight consumers. It is safe for
// concurrent use.
type consumerManager struct {
	mu      sync.Mutex
	current map[uint64]consumer
	counter uint64 // total consumers ever seen; also the source of consumer ids
}

func newConsumerManager() *consumerManager {
	return &consumerManager{
		current: make(map[uint64]consumer),
	}
}

// come registers a new in-flight consumer and returns its id, which must be
// passed to leave when the request finishes.
func (m *consumerManager) come(ctxData *CtxData) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.counter
	m.counter++
	c := consumer{
		reqID:    ctxData.ReqID,
		reqSubID: ctxData.ReqSubID,
		method:   ctxData.Method,
		source:   ctxData.ReqSrc,
		enterAt:  time.Now(),
		reqBody:  ctxData.RawReqBody,
	}
	if ctxData.RawReq != nil {
		c.reqHeaders = ctxData.RawReq.Header
	}
	m.current[id] = c
	return id
}

func (m *consumerManager) leave(id uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.current, id)
}

func (m *consumerManager) Snapshot() any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{
		"total":        m.counter,
		"currentCount": len(m.current),
		"current":      utils.MapMapNoError(m.current, consumer.Snapshot),
	}
}
