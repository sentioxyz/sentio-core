package clientpool

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/utils"
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
	regexp.MustCompile("block requested not found"),
	// aggregator endpoints (e.g. dRPC) reporting that none of their upstreams holds the requested
	// data range: "no available upstreams to process a request. Cause - ... Upstream lower height
	// 42810022 of type RECEIPTS is greater than 35662252"
	regexp.MustCompile("no available upstreams"),
}

var brokenMsgErrorMatcher = []*regexp.Regexp{
	regexp.MustCompile("rate limit exceeded"),
}

// executionRevertedPrefix is the standard message prefix of an EVM revert (JSON-RPC error code 3).
const executionRevertedPrefix = "execution reverted"

// isRevertableMethod reports whether an "execution reverted" error is a legitimate deterministic
// result for the method, i.e. the method executes contract code. For any other method such a
// message is not a real revert and keeps going through the regular classification.
func isRevertableMethod(method string) bool {
	return method == "eth_call" || method == "eth_estimateGas"
}

func isOneOf(err string, matchers []*regexp.Regexp) bool {
	for _, r := range matchers {
		if r.FindString(strings.ToLower(err)) != "" {
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
			return isOneOf(err.Error(), invalidEVMMethodErrorMatcher)
		default:
			return false
		}
	}
	return false
}

func isMissDataError(method string, err error) bool {
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
		// Codes in (-32000, 0] are standard application-level JSON-RPC errors — never miss-data.
		// Codes <= -32000 (server error range) go through message matching, and so do positive
		// codes: those are non-standard vendor codes, e.g. dRPC reports "no available upstreams"
		// with code 1 and X Layer legacy-range proxying reports "Temporary internal error" with
		// code 19. The only standard positive code is 3 (execution reverted), excluded below.
		if code := rpcErr.ErrorCode(); code > -32000 && code <= 0 {
			return false
		}
		// "execution reverted: <reason>" carries a contract-controlled revert reason (standard
		// code 3, but some vendors report it under other codes). For methods that execute
		// contract code it is a deterministic result of the call, not missing data, and the
		// reason must never reach the keyword matching below: a contract reverting with e.g.
		// "Unexpected error" would otherwise be retried on every endpoint in the pool.
		if isRevertableMethod(method) && strings.HasPrefix(strings.ToLower(err.Error()), executionRevertedPrefix) {
			return false
		}
		return isOneOf(err.Error(), missDataErrorMatcher)
	}
	return false
}

func isBrokenError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
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
		// 5xx is usually triggered by this particular request rather than the
		// endpoint being globally unhealthy, so it is not a broken endpoint.
		// It is handled as BrokenForTask instead (retry on another endpoint).
		if httpErr.StatusCode >= 500 {
			return false
		}
		// 429 means the endpoint is rejecting us due to rate limiting,
		// independent of the request — back off the whole endpoint.
		if httpErr.StatusCode == 429 {
			return true
		}
		var msg jsonrpcMessage
		if json.Unmarshal(httpErr.Body, &msg) == nil && msg.Error != nil && msg.Error.Code != nil {
			if isOneOf(msg.Error.Message, brokenMsgErrorMatcher) {
				return true // e.g. a rate-limit reported with a non-429 status
			}
			return false // jsonrpc message with error code, no keyword
		}
		return true // http error without error code in jsonrpc message
	}
	return true // It can only be a TCP error.
}

// isServerError reports whether err is an HTTP 5xx response. A 5xx is treated
// as a per-request failure (BrokenForTask): the request is retried on another
// endpoint, but the endpoint is not banned for all requests.
func isServerError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr rpc.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 500
	}
	return false
}

func CallContext(
	client *rpc.Client,
	ctx context.Context,
	result any,
	method string,
	args ...any,
) (r Result) {
	r.Err = client.CallContext(ctx, result, method, args...)
	r.Broken = isBrokenError(r.Err)
	r.BrokenForTask = errors.Is(r.Err, context.DeadlineExceeded) || isMissDataError(method, r.Err) || isServerError(r.Err)
	if isInvalidMethodError(r.Err) {
		r.AddTags = []string{MethodNotSupportedTag(method)}
	}
	return r
}

// WithAuthorityVeto upgrades a runtime method-not-supported rejection into an authority veto:
// when the endpoint is a method authority (JSONRPCConfig.MethodAuthority) and the result
// already carries MethodNotSupportedTag(method), it additionally raises
// MethodNotSupportedByAuthorityTag(method) so method-scoped consumers interrupt the whole pool
// for this method (see InterruptWithTags). Chain clients apply it on their live call path
// only — never on the CheckMethod path, so a config black/white list cannot raise the veto.
func (r Result) WithAuthorityVeto(method string, authority bool) Result {
	// len check first: the success path must not pay the tag-string concatenation
	if !authority || len(r.AddTags) == 0 || utils.IndexOf(r.AddTags, MethodNotSupportedTag(method)) < 0 {
		return r
	}
	r.AddTags = append(r.AddTags, MethodNotSupportedByAuthorityTag(method))
	return r
}

func CheckMethod(method string, blackList, whiteList []string) Result {
	if len(blackList) > 0 && utils.IndexOf(blackList, method) >= 0 {
		return Result{
			Err:           errors.New("method in blacklist"),
			BrokenForTask: true,
			AddTags:       []string{MethodNotSupportedTag(method)},
		}
	}
	if len(whiteList) > 0 && utils.IndexOf(whiteList, method) < 0 {
		return Result{
			Err:           errors.New("method not in whitelist"),
			BrokenForTask: true,
			AddTags:       []string{MethodNotSupportedTag(method)},
		}
	}
	return Result{}
}
