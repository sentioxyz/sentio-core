package jsonrpc

import (
	"fmt"
	"io"
	"net/http"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timewin"
	"strings"
	"sync/atomic"
	"time"
)

type SimpleHTTPHandler struct {
	name  string
	debug bool

	// used to build rid
	requestCounter atomic.Uint64

	stat *timewin.TimeWindowsManager[*statWindow]

	middleware MiddlewareChain
}

func NewSimpleHTTPHandler(name string, debug bool) *SimpleHTTPHandler {
	return &SimpleHTTPHandler{
		name:  name,
		debug: debug,
		stat:  timewin.NewTimeWindowsManager[*statWindow](time.Minute),
	}
}

func (s *SimpleHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	remoteHost, _, _ := strings.Cut(r.RemoteAddr, ":")
	ctx, logger := log.FromContextWithTrace(r.Context(),
		"svr", s.name,
		"rid", s.requestCounter.Add(1),
		"remote", remoteHost)
	// get raw request body
	rawBody, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestContentLength))
	if err != nil {
		err = fmt.Errorf("read request body failed: %w", err)
		logger.Debugfe(err, "request failed")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if s.debug {
		logger.Debugw("[ACCESS] http request", "url", r.URL.String(), "reqBody", string(rawBody))
		for hk, hv := range r.Header {
			logger.Debugf("Header[%s]: %v", hk, hv)
		}
	}
	startTime := time.Now()

	ctxData := &CtxData{
		RawReq:     r,
		RawReqBody: rawBody,
		RespWriter: w,
	}

	_, err = s.middleware.CallMethod(SetCtxData(ctx, ctxData), HTTPRequestMethod, nil)

	used := time.Since(startTime)
	logger = logger.With("used", used.String())
	if err != nil {
		logger.Warnfe(err, "calling method %s failed", HTTPRequestMethod)
	} else {
		logger.Debugf("calling method %s succeed", HTTPRequestMethod)
	}

	s.stat.Append(newStatWindow(HTTPRequestMethod, RequestSource{RemoteHost: remoteHost}, used))
}

func (s *SimpleHTTPHandler) RegisterMiddleware(m ...Middleware) {
	s.middleware = append(s.middleware, m...)
}

func (s *SimpleHTTPHandler) Snapshot() any {
	return map[string]any{
		"name":           s.name,
		"debug":          s.debug,
		"requestCounter": s.requestCounter.Load(),
		"statistics":     s.stat.Snapshot(),
	}
}
