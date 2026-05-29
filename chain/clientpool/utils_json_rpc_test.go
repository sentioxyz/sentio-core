package clientpool

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
)

// fakeRPCErr implements rpc.Error (Error() + ErrorCode()).
type fakeRPCErr struct {
	code int
	msg  string
}

func (e fakeRPCErr) Error() string  { return e.msg }
func (e fakeRPCErr) ErrorCode() int { return e.code }

// The exact 429 body observed from the Conduit-backed RPC (rpc.lyra.finance).
const rateLimitBody = `{"jsonrpc":"2.0","error":{"code":-32017,"message":"Rate Limit Exceeded. Please get an api key at https://app.conduit.xyz/nodes to increase your rate limit."},"id":1}`

func httpErr(status int, body string) error {
	return rpc.HTTPError{StatusCode: status, Status: fmt.Sprintf("%d", status), Body: []byte(body)}
}

// ── isBrokenError ─────────────────────────────────────────────────────────────

func Test_isBrokenError_rateLimit429_isBroken(t *testing.T) {
	// Regression: a 429 that carries a valid jsonrpc error body used to be
	// treated as "not broken", so the pool kept hammering a rate-limited endpoint.
	assert.True(t, isBrokenError(httpErr(429, rateLimitBody)))
}

func Test_isBrokenError_rateLimit_wrapped_isBroken(t *testing.T) {
	// errors.As must still find the HTTPError when it is wrapped.
	wrapped := fmt.Errorf("call failed: %w", httpErr(429, rateLimitBody))
	assert.True(t, isBrokenError(wrapped))
}

func Test_isBrokenError_normalJsonrpcError_notBroken(t *testing.T) {
	// A regular jsonrpc error (e.g. invalid params) is a valid response, not a broken endpoint.
	body := `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Log response size exceeded."},"id":1}`
	assert.False(t, isBrokenError(httpErr(400, body)))
}

func Test_isBrokenError_httpErrorWithoutJsonrpcBody_isBroken(t *testing.T) {
	// Non-jsonrpc body (gateway/plain text) → broken.
	assert.True(t, isBrokenError(httpErr(502, "Bad Gateway")))
}

func Test_isBrokenError_rpcError_notBroken(t *testing.T) {
	assert.False(t, isBrokenError(fakeRPCErr{code: -32000, msg: "execution reverted"}))
}

func Test_isBrokenError_nilAndContext_notBroken(t *testing.T) {
	assert.False(t, isBrokenError(nil))
	assert.False(t, isBrokenError(context.Canceled))
	assert.False(t, isBrokenError(context.DeadlineExceeded))
}

// ── rate-limit body must not be misclassified by the other detectors ──────────

func Test_rateLimit_notMissData_notInvalidMethod(t *testing.T) {
	err := httpErr(429, rateLimitBody)
	assert.False(t, isMissDataError(err))
	assert.False(t, isInvalidMethodError(err))
}

// ── isOneOf ───────────────────────────────────────────────────────────────────

func Test_isOneOf_caseInsensitive(t *testing.T) {
	// Matchers are lowercase; input is normalized before matching.
	assert.True(t, isOneOf("Rate Limit Exceeded. Please get an api key", brokenMsgErrorMatcher))
	assert.False(t, isOneOf("some other error", brokenMsgErrorMatcher))
}
