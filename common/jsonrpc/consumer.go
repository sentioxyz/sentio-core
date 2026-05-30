package jsonrpc

import (
	"sync"
	"time"

	"sentioxyz/sentio-core/common/utils"
)

// consumer represents an in-flight request being handled by the Handler, from
// the moment it enters callMethod until it returns. It mirrors the consumer
// tracking in chain/clientpool so that Snapshot can show which requests are
// currently being processed and for how long (useful for spotting stuck or
// long-running requests).
type consumer struct {
	reqID    uint64
	reqSubID uint64
	method   string
	source   RequestSource
	enterAt  time.Time
}

func (c consumer) Snapshot() any {
	return map[string]any{
		"requestId":     c.reqID,
		"requestSubId":  c.reqSubID,
		"method":        c.method,
		"source":        c.source.Summary(),
		"enterAt":       c.enterAt.String(),
		"enterDuration": time.Since(c.enterAt).String(),
	}
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
func (m *consumerManager) come(reqID, reqSubID uint64, method string, src RequestSource) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.counter
	m.counter++
	m.current[id] = consumer{
		reqID:    reqID,
		reqSubID: reqSubID,
		method:   method,
		source:   src,
		enterAt:  time.Now(),
	}
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
