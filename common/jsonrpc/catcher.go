package jsonrpc

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"net/http"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sync"
	"time"
)

type catcher struct {
	// catch condition.
	srcLabels map[string]string
	methods   set.Set[string]

	// ending conditions
	timeLen time.Duration
	count   uint64

	// status
	startAt time.Time
	caught  uint64

	session *WebsocketSession
	endChan chan error
}

func (c *catcher) Snapshot() any {
	var remainTime = "INF"
	if c.timeLen > 0 {
		remainTime = (c.timeLen - time.Since(c.startAt)).String()
	}
	var remainCount any = "INF"
	if c.count > 0 {
		remainCount = c.count - c.caught
	}
	return map[string]any{
		"config": map[string]any{
			"srcLabels": c.srcLabels,
			"methods":   c.methods.DumpValues(),
			"timeLen":   c.timeLen.String(),
			"count":     c.count,
		},
		"status": map[string]any{
			"startAt":  c.startAt.String(),
			"duration": time.Since(c.startAt).String(),
			"caught":   c.caught,
		},
		"remain": map[string]any{
			"duration": remainTime,
			"count":    remainCount,
		},
	}
}

func (c *catcher) catch(result catchResult) (bool, error) {
	if len(c.srcLabels) > 0 {
		ok := false
		for k, v := range c.srcLabels {
			if result.ReqSrc.Labels[k] == v {
				ok = true
				break
			}
		}
		if !ok {
			// source not match
			return false, nil
		}
	}
	if result.Method == catchMethod {
		// catch request self cannot be captured
		return false, nil
	}
	if !c.methods.Empty() && !c.methods.Contains(result.Method) {
		// method not match
		return false, nil
	}
	resp := map[string]any{
		"jsonrpc": c.session.Request.Version,
		"method":  catchResultMethod,
		"params": map[string]any{
			"catcher": c.session.ID,
			"index":   c.caught,
			"result":  result,
		},
	}
	return true, c.session.WriteJSON(resp)
}

func (c *catcher) writeReport() error {
	return c.session.WriteJSON(map[string]any{
		"jsonrpc": c.session.Request.Version,
		"method":  catchReportMethod,
		"params": map[string]any{
			"catcher":  c.session.ID,
			"caught":   c.caught,
			"startAt":  c.startAt.String(),
			"duration": time.Since(c.startAt).String(),
		},
	})
}

type catchResult struct {
	ReqID    uint64
	ReqSubID uint64
	ReqSrc   RequestSource

	StartAt time.Time
	Used    time.Duration

	Method    string
	Params    json.RawMessage
	ReqUserID json.RawMessage

	RespHeaders http.Header
	Result      any
	Error       error
}

func (c catchResult) MarshalJSON() ([]byte, error) {
	response := map[string]any{
		"result": c.Result,
	}
	if c.Error != nil {
		response["error"] = c.Error.Error()
	}
	if len(c.RespHeaders) > 0 {
		response["headers"] = c.RespHeaders
	}
	return json.Marshal(map[string]any{
		"reqID":    c.ReqID,
		"reqSubID": c.ReqSubID,
		"reqSrc":   c.ReqSrc,
		"startAt":  c.StartAt.String(),
		"used":     c.Used.String(),
		"request": map[string]any{
			"method": c.Method,
			"params": c.Params,
			"id":     c.ReqUserID,
		},
		"response": response,
	})
}

type catcherManager struct {
	secret string

	mu       sync.Mutex
	counter  int
	catchers map[int]*catcher
}

func newCatcherManager(secret string) *catcherManager {
	return &catcherManager{
		secret:   secret,
		catchers: make(map[int]*catcher),
	}
}

const (
	catchMethod       = "system_catch"
	catchResultMethod = "system_catchResult"
	catchReportMethod = "system_catchReport"
)

type catchRequest struct {
	Secret    string            `json:"secret"`
	SrcLabels map[string]string `json:"srcLabels"`
	Methods   []string          `json:"methods"`
	Seconds   uint64            `json:"seconds"`
	Count     uint64            `json:"count"`
}

func (m *catcherManager) newMiddleware() Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			if method != catchMethod {
				return next(ctx, method, params)
			}
			return CallMethod(m.newCatcher, ctx, params)
		}
	}
}

func (m *catcherManager) newCatcher(ctx context.Context, req *catchRequest) (any, error) {
	ctxData := GetCtxData(ctx)
	ctxData.NotSlowRequest = true
	session := ctxData.WebsocketSession
	if session == nil {
		return nil, errors.Errorf("websocket only")
	}
	if req.Secret != m.secret {
		return nil, errors.Errorf("invalid secret")
	}

	c := &catcher{
		srcLabels: req.SrcLabels,
		methods:   set.New(req.Methods...),
		timeLen:   time.Duration(req.Seconds) * time.Second,
		count:     req.Count,
		startAt:   time.Now(),
		session:   session,
		endChan:   make(chan error, 1),
	}

	if err := session.WriteJSON(JSONResponse(&session.Request, session.ID)); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.counter++
	id := m.counter
	m.catchers[id] = c
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.catchers, id)
		m.mu.Unlock()
	}()

	var timeout <-chan time.Time
	if c.timeLen > 0 {
		timeout = time.After(c.timeLen)
	} else {
		timeout = make(chan time.Time)
	}
	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-c.endChan:
	case <-timeout:
	}
	if err != nil {
		return nil, session.Abort(err)
	}
	return nil, session.Abort(c.writeReport())
}

func (m *catcherManager) catch(result catchResult) {
	m.mu.Lock()
	for id, c := range m.catchers {
		caught, err := c.catch(result)
		if err != nil {
			// write result failed
			c.endChan <- err
			delete(m.catchers, id)
		}
		if !caught {
			continue
		}
		c.caught++
		if c.count > 0 && c.caught >= c.count {
			// number is enough
			c.endChan <- nil
			delete(m.catchers, id)
		}
	}
	m.mu.Unlock()
}

func (m *catcherManager) Snapshot() any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{
		"counter":  m.counter,
		"secret":   m.secret,
		"catchers": utils.MapMapNoError(m.catchers, (*catcher).Snapshot),
	}
}
