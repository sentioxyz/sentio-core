package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/tracker"
	"sentioxyz/sentio-core/common/utils"
	"sync"
	"time"
)

type WebsocketService struct {
	handler      *Handler
	keepalive    time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration

	mu          sync.Mutex
	connections map[uint64]*WebsocketConnection
}

func newWebsocketService(
	handler *Handler,
	keepalive time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
) *WebsocketService {
	return &WebsocketService{
		handler:      handler,
		keepalive:    keepalive,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
		connections:  make(map[uint64]*WebsocketConnection),
	}
}

type WebsocketConnection struct {
	svr       *WebsocketService
	conn      *websocket.Conn
	writeLock sync.Mutex // send message using conn should not be concurrent

	// immutable properties
	id      uint64
	startAt time.Time
	source  RequestSource

	// variable properties
	mu             sync.Mutex
	sessionCounter uint64
	sessions       map[uint64]*WebsocketSession
}

type WebsocketSession struct {
	conn   *WebsocketConnection
	ctx    context.Context
	cancel context.CancelFunc

	ID      uint64
	StartAt time.Time
	Request JsonrpcMessage

	mu      sync.Mutex
	summary tracker.TrackedObject
}

func (s *WebsocketSession) Snapshot() any {
	sn := map[string]any{
		"id":      s.ID,
		"startAt": s.StartAt.String(),
		"request": s.Request,
		"summary": nil,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.summary != nil {
		sn["summary"] = s.summary.Snapshot()
	}
	return sn
}

func (s *WebsocketSession) SetSummary(summary tracker.TrackedObject) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summary = summary
}

func (s *WebsocketSession) Abort(reason error) error {
	if errors.Is(reason, context.Canceled) {
		reason = nil
	}
	return &SessionAbortedError{reason: reason}
}

func (s *WebsocketSession) WriteJSON(value any) error {
	return s.conn.writeJSON(value)
}

func (s *WebsocketSession) AbortAnotherSession(sid uint64) bool {
	if s.ID == sid {
		return false
	}
	return s.conn.abortSession(sid)
}

type SessionAbortedError struct {
	reason error
}

func (e *SessionAbortedError) Error() string {
	return fmt.Sprintf("session aborted: %v", e.reason)
}

func (s *WebsocketService) Snapshot() any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{
		"connections": utils.MapMapNoError(s.connections, (*WebsocketConnection).Snapshot),
	}
}

func (s *WebsocketService) handleConnection(ctx context.Context, id uint64, src RequestSource, conn *websocket.Conn) {
	c := &WebsocketConnection{
		svr:      s,
		conn:     conn,
		id:       id,
		startAt:  time.Now(),
		source:   src,
		sessions: make(map[uint64]*WebsocketSession),
	}

	s.mu.Lock()
	s.connections[id] = c
	s.mu.Unlock()

	c.main(ctx)

	s.mu.Lock()
	delete(s.connections, id)
	s.mu.Unlock()
}

func (c *WebsocketConnection) writeJSON(value any) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	if setErr := c.conn.SetWriteDeadline(time.Now().Add(c.svr.writeTimeout)); setErr != nil {
		return errors.Wrapf(setErr, "set write deadline failed")
	}
	if writeErr := c.conn.WriteJSON(value); writeErr != nil {
		return errors.Wrapf(writeErr, "write response failed")
	}
	return nil
}

func (c *WebsocketConnection) writeControl(msgType int, msg string) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	return c.conn.WriteControl(msgType, []byte(msg), time.Now().Add(c.svr.writeTimeout))
}

func (c *WebsocketConnection) abortSession(sid uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	session, has := c.sessions[sid]
	if has {
		session.cancel()
		delete(c.sessions, sid)
	}
	return has
}

func (c *WebsocketConnection) newSession(ctx context.Context, rawReq json.RawMessage) error {
	var req JsonrpcMessage
	if parseErr := json.Unmarshal(rawReq, &req); parseErr != nil {
		return c.writeJSON(JSONErrorResponse(
			&JsonrpcMessage{},
			nil,
			errors.Wrapf(parseErr, "parse subscribe request failed"),
		))
	}
	c.mu.Lock()
	c.sessionCounter++
	session := &WebsocketSession{
		conn:    c,
		ID:      c.sessionCounter,
		StartAt: time.Now(),
		Request: req,
	}
	session.ctx, session.cancel = context.WithCancel(ctx)
	session.ctx, _ = log.FromContext(session.ctx, "sid", session.ID, "method", req.Method)
	c.sessions[session.ID] = session
	c.mu.Unlock()

	// calling middleware
	ctxData := &CtxData{
		ReqID:            c.id,
		RawReq:           nil,
		RawReqBody:       rawReq,
		WebsocketSession: session,
		ReqSrc:           c.source,
		Method:           req.Method,
		Params:           req.Params,
	}
	result, err := c.svr.handler.callMethod(session.ctx, ctxData, req.Method, req.Params, JsonEncoder{})

	c.mu.Lock()
	delete(c.sessions, session.ID)
	c.mu.Unlock()

	var aborted *SessionAbortedError
	if errors.As(err, &aborted) {
		return aborted.reason
	} else if err != nil {
		return c.writeJSON(JSONErrorResponse(&req, result, err))
	} else {
		return c.writeJSON(JSONResponse(&req, result))
	}
}

func (c *WebsocketConnection) main(ctx context.Context) {
	_, logger := log.FromContext(ctx)
	g, gctx := errgroup.WithContext(ctx)
	c.conn.SetPongHandler(func(msg string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.svr.readTimeout))
	})
	c.conn.SetPingHandler(func(msg string) error {
		return c.writeControl(websocket.PongMessage, msg)
	})
	// connection keepalive
	g.Go(func() (err error) {
		logger.Debug("websocket connection keepalive started")
		defer func() {
			logger.Debuge(err, "websocket connection keepalive finished")
		}()
		ticker := time.NewTicker(c.svr.keepalive)
		defer ticker.Stop()
		for {
			select {
			case <-gctx.Done():
				return gctx.Err()
			case x := <-ticker.C:
				pingMsg := fmt.Sprintf("%d@%s", c.id, x.Format(time.RFC3339Nano))
				if sendErr := c.writeControl(websocket.PingMessage, pingMsg); sendErr != nil {
					logger.Warnfe(sendErr, "send ping message failed")
				}
			}
		}
	})
	// main loop
	g.Go(func() (err error) {
		logger.Debug("websocket connection main loop started")
		defer func() {
			logger.Debuge(err, "websocket connection main loop finished")
		}()
		for {
			_ = c.conn.SetReadDeadline(time.Now().Add(c.svr.readTimeout))
			msgType, msg, readErr := c.conn.ReadMessage()
			if readErr != nil {
				// will always got an error here when conn.conn closed
				return errors.Wrapf(readErr, "read message failed")
			}
			switch msgType {
			case websocket.TextMessage:
				g.Go(func() error {
					return c.newSession(gctx, msg)
				})
			case websocket.CloseMessage:
				return errors.Errorf("receive close message: %s", string(msg))
			default:
				// ping and pong message will not appear here
				logger.Warnf("unexpectted message (type: %d), will be ignored: %s", msgType, string(msg))
			}
		}
	})
	err := g.Wait()
	if err != nil && !errors.Is(err, context.Canceled) && !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		logger.Warnfe(err, "websocket connection closed")
	} else {
		logger.Debug("websocket connection closed")
	}
}

func (c *WebsocketConnection) Snapshot() any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return map[string]any{
		"id":       c.id,
		"source":   c.source,
		"startAt":  c.startAt.String(),
		"sessions": utils.MapMapNoError(c.sessions, (*WebsocketSession).Snapshot),
	}
}
