package clientpool

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"regexp"
	"strings"
	"time"
)

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
	if err == nil {
		return false
	}
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
	if err == nil {
		return false
	}
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
	if err == nil {
		return false
	}
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
		r.AddTags = []string{MethodNotSupportedTag(method)}
	}
	return r
}
