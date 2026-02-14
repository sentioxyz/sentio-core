package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/queue"
	"sentioxyz/sentio-core/common/timewin"
	"sentioxyz/sentio-core/common/utils"
)

type SourceGetter interface {
	GetByIP(ip string) (string, map[string]any)
}

type Handler struct {
	name string

	debug        bool
	websocketSvr *WebsocketService

	// used to get driver name from pod ip
	sourceGetter SourceGetter

	// used to build rid
	requestCounter atomic.Uint64
	// used to do statistics
	statUsed metric.Int64Histogram

	stat          *timewin.TimeWindowsManager[*statWindow]
	slowQueries   queue.Circular[slowRequest]
	bigQueries    queue.Circular[bigRequest]
	failedQueries queue.Circular[failedRequest]

	middleware MiddlewareChain
}

func NewHandler(
	name string,
	printAccess bool,
	acceptWebsocket bool,
	sourceGetter SourceGetter,
	statUsed metric.Int64Histogram,
) *Handler {
	h := &Handler{
		name:          name,
		debug:         printAccess,
		sourceGetter:  sourceGetter,
		statUsed:      statUsed,
		stat:          timewin.NewTimeWindowsManager[*statWindow](time.Minute),
		slowQueries:   queue.NewSafeCircular[slowRequest](100),
		bigQueries:    queue.NewSafeCircular[bigRequest](100),
		failedQueries: queue.NewSafeCircular[failedRequest](100),
	}
	if acceptWebsocket {
		h.websocketSvr = newWebsocketService(h, time.Second*20, time.Minute, time.Second*20)
	}
	return h
}

type RequestSource struct {
	RemoteHost string
	Name       string
	Labels     map[string]any
}

func (s RequestSource) Summary() string {
	return s.Name
}

type ctxDataKeyType struct{}

var ctxDataKey ctxDataKeyType

type CtxData struct {
	ReqID            uint64
	ReqSubID         uint64
	RawReq           *http.Request
	RawReqBody       []byte
	WebsocketSession *WebsocketSession
	ReqSrc           RequestSource
	Method           string // maybe empty, mean this is not a jsonrpc request, need to use RespWriter to write the response
	Params           json.RawMessage

	RespHeaders    http.Header
	RespWriter     http.ResponseWriter
	StatLabels     []attribute.KeyValue
	NotSlowRequest bool
}

func (c *CtxData) sign() string {
	m := utils.CopyMap(c.ReqSrc.Labels)
	m["method"] = c.Method
	res, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(res)
}

func GetCtxData(ctx context.Context) *CtxData {
	raw := ctx.Value(ctxDataKey)
	if data, is := raw.(*CtxData); is {
		return data
	}
	return nil
}

func setCtxData(ctx context.Context, data *CtxData) context.Context {
	settings := clickhouse.Settings{
		"log_comment": data.sign(),
	}
	ctx = clickhouse.Context(
		context.WithValue(ctx, ctxDataKey, data),
		clickhouse.WithSettings(settings),
	)
	return ckhmanager.ContextMergeSettings(ctx, settings)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
		// do nothing when upgrade failed
	},
}

func (s *Handler) newEncoder(r *http.Request) Encoder {
	accept := r.Header.Get("Accept")
	if accept != "" && strings.Contains(accept, "application/msgpack") {
		return MsgpackEncoder{}
	} else {
		return JsonEncoder{}
	}
}

func (s *Handler) callMethod(
	ctx context.Context,
	ctxData *CtxData,
	method string,
	params json.RawMessage,
	encoder Encoder,
) (any, error) {
	startAt := time.Now()
	result, err := s.middleware.CallMethod(setCtxData(ctx, ctxData), method, params)
	var ret any
	var retLen int
	var encodeErr error
	var encodeUsed time.Duration
	if encoder != nil {
		encodeStartAt := time.Now()
		ret, retLen, encodeErr = encoder.Marshal(result)
		encodeUsed = time.Since(encodeStartAt)
	} else {
		ret = result
	}
	used := time.Since(startAt)
	if encodeErr != nil && err == nil {
		err = encodeErr
	}

	attributes := []attribute.KeyValue{
		attribute.String("name", s.name),
		attribute.String("method", method),
		attribute.Bool("succeed", err == nil),
		attribute.Bool("proxy", false),
		attribute.Bool("cached", false),
		attribute.String("endpoint", ""),
	}
	opt := metric.WithAttributeSet(attribute.NewSet(utils.MergeArr(attributes, ctxData.StatLabels)...))
	if s.statUsed != nil {
		s.statUsed.Record(context.Background(), used.Milliseconds(), opt)
	}
	s.stat.Append(newStatWindow(method, ctxData.ReqSrc, used))

	rs := requestSample{
		RequestID:    ctxData.ReqID,
		RequestSubID: ctxData.ReqSubID,
		Source:       ctxData.ReqSrc,
		RequestTime:  startAt,
		RequestBody:  ctxData.RawReqBody,
		Used:         used,
	}
	if used > slowQueryUsed && !ctxData.NotSlowRequest {
		s.slowQueries.Push(slowRequest{requestSample: rs})
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		s.failedQueries.Push(failedRequest{
			requestSample: rs,
			Error:         err.Error(),
		})
	}
	if retLen > bigQueryResponseSize || encodeUsed > bigQueryResponseEncodeUsed {
		s.bigQueries.Push(bigRequest{
			requestSample:      rs,
			ResponseSize:       retLen,
			ResponseEncodeUsed: encodeUsed,
		})
	}
	return ret, err
}

const HTTPRequestMethod = "methodHTTP"

func (s *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	remoteHost, _, _ := strings.Cut(r.RemoteAddr, ":")
	src := RequestSource{RemoteHost: remoteHost}
	if s.sourceGetter != nil {
		src.Name, src.Labels = s.sourceGetter.GetByIP(remoteHost)
	}
	rid := s.requestCounter.Add(1)
	ctx, logger := log.FromContextWithTrace(r.Context(), "svr", s.name, "rid", rid, "src", src)

	// check body length
	if r.ContentLength > MaxRequestContentLength {
		err := fmt.Errorf("content length too large (%d>%d)", r.ContentLength, MaxRequestContentLength)
		logger.Debugfe(err, "request failed")
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}

	// print error log if panic
	defer func() {
		if panicErr := recover(); panicErr != nil {
			logger.Errorf("caught panic: %v", panicErr)
			panic(panicErr)
		}
	}()

	// get raw request body
	rawBody, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestContentLength))
	if err != nil {
		err = fmt.Errorf("read request body failed: %w", err)
		logger.Debugfe(err, "request failed")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// determine whether this is a jsonrpc request
	isJSONRPCRequest := true
	var rawMsg json.RawMessage
	var messages []*JsonrpcMessage
	var isBatch bool
	if _, err = ValidateRPCRequest(r); err == nil {
		err = json.Unmarshal(rawBody, &rawMsg)
	}
	if err != nil {
		isJSONRPCRequest = false
		if s.websocketSvr != nil {
			if conn, upgradeErr := upgrader.Upgrade(w, r, nil); upgradeErr == nil {
				if s.debug {
					logger.Debugw("[ACCESS] websocket connection")
					for hk, hv := range r.Header {
						logger.Debugf("Header[%s]: %v", hk, hv)
					}
				}
				logger.Debugf("accept websocket connection")
				defer func() {
					_ = conn.Close()
				}()
				s.websocketSvr.handleConnection(ctx, rid, src, conn)
				return
			}
		}
	}

	if isJSONRPCRequest {
		messages, isBatch = ParseRPCMessage(ctx, rawMsg)
		for _, msg := range messages {
			if msg.Method == "" {
				logger.Debugf("message miss method")
				isJSONRPCRequest = false
				break
			}
		}
	}

	if !isJSONRPCRequest {
		// not a jsonrpc request, and not a websocket connection, try to call the wildcard method in NonJSONRPCNamespace
		if s.debug {
			logger.Debugw("[ACCESS] non-jsonrpc request", "url", r.URL.String(), "reqBody", string(rawBody))
			for hk, hv := range r.Header {
				logger.Debugf("Header[%s]: %v", hk, hv)
			}
		}
		startTime := time.Now()
		ctxData := &CtxData{
			ReqID:      rid,
			RawReq:     r,
			RawReqBody: rawBody,
			RespWriter: w,
			ReqSrc:     src,
		}
		// method for non-jsonrpc request should use ctxData.RespWriter to write response
		_, err = s.callMethod(ctx, ctxData, HTTPRequestMethod, nil, nil)
		used := time.Since(startTime)
		logger = logger.With("used", used.String())
		if err != nil {
			logger.Warnfe(err, "calling non-jsonrpc method failed")
		} else {
			logger.Debug("calling non-jsonrpc method succeed")
		}

		return
	}

	// this is a jsonrpc request
	if s.debug {
		logger.Debugw("[ACCESS] jsonrpc request", "body", utils.MustJSONMarshal(rawMsg))
		for hk, hv := range r.Header {
			logger.Debugf("Header[%s]: %v", hk, hv)
		}
	}
	logger.Debugf("Received %d requests", len(messages))
	// call methods and got the response
	encoder := s.newEncoder(r)
	responses := make([]*JsonrpcMessage, len(messages))
	respHeaders := make([]http.Header, len(messages))
	var wg sync.WaitGroup
	wg.Add(len(messages))
	for i, msg := range messages {
		go func(i int, msg *JsonrpcMessage) {
			defer wg.Done()

			if msg == nil {
				// Message is JSON 'null'. Replace with zero value so it
				// will be treated like any other invalid message.
				msg = new(JsonrpcMessage)
			}

			callCtx, msgLogger := log.FromContext(ctx, "srid", i, "method", msg.Method, "params", string(msg.Params))
			msgLogger.Debugf("calling jsonrpc method")

			span := trace.SpanFromContext(ctx)
			span.SetName(msg.Method)

			// call method
			var res any
			startTime := time.Now()
			ctxData := &CtxData{
				ReqID:      rid,
				ReqSubID:   uint64(i),
				RawReq:     r,
				RawReqBody: rawBody,
				ReqSrc:     src,
				Method:     msg.Method,
				Params:     msg.Params,
			}
			res, err = s.callMethod(callCtx, ctxData, msg.Method, msg.Params, encoder)
			// print log
			used := time.Since(startTime)
			if err != nil {
				msgLogger.With("used", used.String(), "result", res).Warne(err, "calling jsonrpc method failed")
			} else {
				msgLogger.Debugw("calling jsonrpc method succeed", "used", used.String(), "result", res)
			}

			// record response
			respHeaders[i] = ctxData.RespHeaders
			if err != nil {
				responses[i] = JSONErrorResponse(messages[i], res, err)
			} else {
				responses[i] = JSONResponse(messages[i], res)
			}
		}(i, msg)
	}
	wg.Wait()
	// write response header
	for _, headers := range respHeaders {
		for key, values := range headers {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	// write response
	if len(messages) > 0 {
		w.Header().Set("Content-Type", encoder.ContentType())
		if isBatch {
			err = encoder.Encode(w, responses)
		} else {
			err = encoder.Encode(w, responses[0])
		}
		if err != nil {
			logger.Errore(err)
		}
	}

	if s.debug {
		logger.Debugw("[ACCESS] jsonrpc response", "body", utils.MustJSONMarshal(responses))
	}
}

func (s *Handler) RegisterMiddleware(m ...Middleware) {
	s.middleware = append(s.middleware, m...)
}

func (s *Handler) Snapshot() any {
	sn := map[string]any{
		"name":           s.name,
		"debug":          s.debug,
		"requestCounter": s.requestCounter.Load(),
		"statistics":     s.stat.Snapshot(),
		"bigQueries": map[string]any{
			"query": utils.MapSliceNoError(s.bigQueries.Dump(true), bigRequest.Snapshot),
			"total": s.bigQueries.Total(),
			"config": map[string]any{
				"responseSize": bigQueryResponseSize,
				"encodeUsed":   bigQueryResponseEncodeUsed.String(),
			},
		},
		"slowQueries": map[string]any{
			"query": utils.MapSliceNoError(s.slowQueries.Dump(true), slowRequest.Snapshot),
			"total": s.slowQueries.Total(),
			"config": map[string]any{
				"slowQueryUsed": slowQueryUsed.String(),
			},
		},
		"failedQueries": map[string]any{
			"query": utils.MapSliceNoError(s.failedQueries.Dump(true), failedRequest.Snapshot),
			"total": s.failedQueries.Total(),
		},
	}
	if s.websocketSvr != nil {
		sn["websocket"] = s.websocketSvr.Snapshot()
	}
	return sn
}
