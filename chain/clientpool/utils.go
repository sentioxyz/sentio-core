package clientpool

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/queue"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
)

func pushLatestQueue(q queue.Queue[Block], latest Block, dur time.Duration) (queue.Queue[Block], time.Duration) {
	if q == nil {
		q = queue.NewQueue[Block]()
	}
	if bc, has := q.Back(); !has || (bc.Number < latest.Number && bc.Timestamp.Before(latest.Timestamp)) {
		q.PushBack(latest)
	}
	// here q will never be empty
	var fr Block
	for {
		fr, _ = q.Front()
		if latest.Timestamp.Sub(fr.Timestamp) <= dur {
			break
		}
		q.PopFront()
	}
	if fr.Number < latest.Number && fr.Timestamp.Before(latest.Timestamp) {
		return q, latest.Timestamp.Sub(fr.Timestamp) / time.Duration(latest.Number-fr.Number)
	}
	return q, 0
}

func BuildPublicName(name string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(name))
	return hex.EncodeToString(h.Sum(nil))
}

func SubscribeUsingGetLatest(
	ctx context.Context,
	start uint64,
	interval time.Duration,
	checkBlockIntervalDur time.Duration,
	ch chan<- Block,
	getLatest func(ctx2 context.Context) (Block, error),
) {
	_, logger := log.FromContext(ctx)
	wait := interval
	var q queue.Queue[Block]
	var blockInterval time.Duration
	for {
		latest, err := getLatest(ctx)
		if err == nil {
			if latest.Number >= start {
				select {
				case ch <- latest:
				case <-ctx.Done():
					return
				}
			}
			q, blockInterval = pushLatestQueue(q, latest, checkBlockIntervalDur)
			wait = max(interval, blockInterval)
		} else {
			logger.Warnfe(err, "get latest failed")
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return
		}
	}
}

func BuildHTTPRequest(
	ctx context.Context,
	method string,
	baseURL string,
	path string,
	params url.Values,
	headers http.Header,
	body []byte,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, baseURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err // baseURL is invalid
	}
	if req.URL.Path == "" {
		req.URL.Path = "/"
	}
	req.URL = req.URL.JoinPath(path)
	if params != nil && len(params) > 0 {
		req.URL.RawQuery = url.Values(utils.MergeMap(req.URL.Query(), params)).Encode()
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	return req, nil
}

var invalidRequestStatusCode = set.New[int](
	http.StatusBadRequest,
	http.StatusNotFound,
	http.StatusMethodNotAllowed,
	http.StatusNotAcceptable,
	http.StatusConflict,
	http.StatusGone,
	http.StatusLengthRequired,
	http.StatusPreconditionFailed,
	http.StatusRequestEntityTooLarge,
	http.StatusRequestURITooLong,
	http.StatusUnsupportedMediaType,
	http.StatusUnprocessableEntity,
	http.StatusUpgradeRequired,
	http.StatusPreconditionRequired,
	http.StatusRequestHeaderFieldsTooLarge,
	http.StatusUnavailableForLegalReasons,
)

func SendHTTP(
	client *http.Client,
	req *http.Request,
	result any,
) (resp *http.Response, body []byte, r Result) {
	resp, r.Err = client.Do(req)
	if r.Err != nil {
		r.Broken = !errors.Is(r.Err, context.Canceled) && !errors.Is(r.Err, context.DeadlineExceeded)
		return resp, nil, r
	}
	body, r.Err = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if r.Err != nil {
		r.Broken = !errors.Is(r.Err, context.Canceled) && !errors.Is(r.Err, context.DeadlineExceeded)
		r.Err = errors.Wrapf(r.Err, "read response body failed")
		return resp, nil, r
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// status code not in [200,300)
		r.Err = errors.Errorf("[StatusCode:%d] %s", resp.StatusCode, string(body))
		r.Broken = !invalidRequestStatusCode.Contains(resp.StatusCode)
		return resp, body, r
	}
	if result != nil {
		if r.Err = json.Unmarshal(body, result); r.Err != nil {
			// result type is invalid
			r.Err = errors.Wrapf(r.Err, "unmarshal response body to %T failed", result)
			return resp, body, r
		}
	}
	return resp, body, r
}

const MethodNotSupportedTagPrefix = "MethodNotSupported/"

var invalidEVMMethodErrorMatcher = []*regexp.Regexp{
	regexp.MustCompile(`unsupported method`),
	regexp.MustCompile(`method.*not available`),
	regexp.MustCompile(`method.*not support`),
	regexp.MustCompile(`method.*not found`),
	regexp.MustCompile(`method.*not allowed`),
	regexp.MustCompile(`resource.*not available`),
	regexp.MustCompile(`invalid method`),
	regexp.MustCompile(`is not whitelisted`),
}

var missDataErrorMatcher = []*regexp.Regexp{
	regexp.MustCompile("old data not available due to pruning"),
	regexp.MustCompile("missing trie node"),
	regexp.MustCompile("invalid block range"),
	regexp.MustCompile("incorrect response body"),
	regexp.MustCompile("historical state is not"),
	regexp.MustCompile("historical state unavailable"),
	regexp.MustCompile("unexpected error"),
	regexp.MustCompile("internal error"),
	regexp.MustCompile("transaction sent to quarantine by sls"), // more: https://ar5iv.labs.arxiv.org/html/2405.01819
}

func isOneOf(err error, matchers []*regexp.Regexp) bool {
	for _, r := range matchers {
		if r.FindString(strings.ToLower(err.Error())) != "" {
			return true
		}
	}
	return false
}

type jsonError struct {
	Code    *int   `json:"code,omitempty"`
	Message string `json:"message"`
}

func (err *jsonError) Error() string {
	return err.Message
}

func (err *jsonError) ErrorCode() int {
	if err.Code == nil {
		return 0
	}
	return *err.Code
}

type jsonrpcMessage struct {
	Error *jsonError `json:"error,omitempty"`
}

func isInvalidMethodError(err error) bool {
	var httpErr rpc.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode < 400 {
			return false
		}
		var msg jsonrpcMessage
		if json.Unmarshal(httpErr.Body, &msg) != nil || msg.Error == nil || msg.Error.Code == nil {
			return false // not jsonrpc message with error
		}
		err = msg.Error
	}
	var rpcErr rpc.Error
	if errors.As(err, &rpcErr) {
		switch rpcErr.ErrorCode() {
		case -32601:
			return true
		case -32000:
			return isOneOf(err, invalidEVMMethodErrorMatcher)
		default:
			return false
		}
	}
	return false
}

func isMissDataError(err error) bool {
	var httpErr rpc.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode < 400 {
			return false
		}
		var msg jsonrpcMessage
		if json.Unmarshal(httpErr.Body, &msg) != nil || msg.Error == nil || msg.Error.Code == nil {
			return false // not jsonrpc message with error
		}
		err = msg.Error
	}
	var rpcErr rpc.Error
	if errors.As(err, &rpcErr) {
		if rpcErr.ErrorCode() > -32000 {
			return false
		}
		return isOneOf(rpcErr, missDataErrorMatcher)
	}
	return false
}

func isBrokenError(err error) bool {
	if errors.Is(err, rpc.ErrNoResult) {
		return false
	}
	var rpcDataErr rpc.DataError
	if errors.As(err, &rpcDataErr) {
		return false
	}
	var rpcErr rpc.Error
	if errors.As(err, &rpcErr) {
		return false
	}
	var httpErr rpc.HTTPError
	if errors.As(err, &httpErr) {
		var msg jsonrpcMessage
		if json.Unmarshal(httpErr.Body, &msg) == nil && msg.Error != nil && msg.Error.Code != nil {
			return false // jsonrpc message with error code
		}
		return true
	}
	return true // It can only be a TCP error.
}

func CallContext(
	client *rpc.Client,
	ctx context.Context,
	timeout time.Duration,
	result any,
	method string,
	args ...any,
) (r Result) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	r.Err = client.CallContext(ctx, result, method, args...)
	r.Broken = isBrokenError(r.Err)
	r.BrokenForTask = isMissDataError(r.Err)
	if isInvalidMethodError(r.Err) {
		r.AddTags = []string{MethodNotSupportedTagPrefix + method}
	}
	return r
}

func OptSupportMethod[CONFIG any](method string) Option[CONFIG] {
	return WithoutTags[CONFIG](MethodNotSupportedTagPrefix + method)
}
