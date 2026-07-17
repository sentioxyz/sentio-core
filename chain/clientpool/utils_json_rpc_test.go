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
	// 4xx with a non-jsonrpc body (e.g. plain text) → broken.
	assert.True(t, isBrokenError(httpErr(403, "Forbidden")))
}

func Test_serverErrors_notBroken_butBrokenForTask(t *testing.T) {
	// 5xx is usually request-specific, not an unhealthy endpoint: it must NOT
	// be a broken endpoint, but it IS a per-request failure (retry elsewhere).
	jsonrpcBody := `{"jsonrpc":"2.0","error":{"code":-32000,"message":"server error"},"id":1}`
	for _, status := range []int{500, 502, 503, 504} {
		assert.False(t, isBrokenError(httpErr(status, jsonrpcBody)), "status %d should not be broken", status)
		assert.False(t, isBrokenError(httpErr(status, "Bad Gateway")), "status %d (plain body) should not be broken", status)
		assert.True(t, isServerError(httpErr(status, jsonrpcBody)), "status %d should be a server error", status)
	}
}

func Test_isServerError_non5xx_false(t *testing.T) {
	assert.False(t, isServerError(httpErr(429, rateLimitBody)))
	assert.False(t, isServerError(httpErr(400, "bad request")))
	assert.False(t, isServerError(nil))
	assert.False(t, isServerError(fakeRPCErr{code: -32000, msg: "execution reverted"}))
}

func Test_isBrokenError_429WithJsonrpcBody_isBroken(t *testing.T) {
	// 429 is broken purely by status code, independent of the body keyword.
	assert.True(t, isBrokenError(httpErr(429, `{"jsonrpc":"2.0","error":{"code":-32000,"message":"slow down"},"id":1}`)))
}

func Test_isBrokenError_non429RateLimitByMessage_isBroken(t *testing.T) {
	// Some providers report rate limiting with a non-429 status; the message
	// matcher still catches it.
	assert.True(t, isBrokenError(httpErr(403, rateLimitBody)))
}

func Test_isBrokenError_4xxNonRateLimit_notBroken(t *testing.T) {
	// A normal 4xx with a valid jsonrpc error (e.g. invalid params) is a valid
	// response, not a broken endpoint.
	body := `{"jsonrpc":"2.0","error":{"code":-32602,"message":"invalid argument"},"id":1}`
	assert.False(t, isBrokenError(httpErr(400, body)))
}

func Test_isBrokenError_rpcError_notBroken(t *testing.T) {
	assert.False(t, isBrokenError(fakeRPCErr{code: -32000, msg: "execution reverted"}))
}

func Test_isBrokenError_nilAndContext_notBroken(t *testing.T) {
	assert.False(t, isBrokenError(nil))
	assert.False(t, isBrokenError(context.Canceled))
	assert.False(t, isBrokenError(context.DeadlineExceeded))
}

// ── isMissDataError: non-standard (positive) vendor error codes ───────────────

// Real errors observed on X Layer mainnet (chain 196): after the chain migrated to OP Stack at
// block 42810021, nodes proxy legacy-range requests upstream, and the proxy target may itself
// lack the legacy data. Both come back as rpc.Error with positive vendor codes, so they used to
// bypass message matching entirely and the pool returned the error without trying other endpoints.
const drpcNoUpstreamMsg = "no available upstreams to process a request. " +
	"Cause - fornex-eu-bcn-09-xlayer-mainnet - Upstream lower height 42810022 of type RECEIPTS is greater than 35662252"
const xlayerLegacyInternalMsg = "Temporary internal error. Please retry, trace-id: db921db51529dc6c2d152d0c9839a437"

func Test_isMissDataError_positiveVendorCodes_missData(t *testing.T) {
	assert.True(t, isMissDataError(fakeRPCErr{code: 1, msg: drpcNoUpstreamMsg}))
	assert.True(t, isMissDataError(fakeRPCErr{code: 19, msg: xlayerLegacyInternalMsg}))
}

func Test_isMissDataError_positiveVendorCodes_notBroken(t *testing.T) {
	// Miss-data must trigger a per-request retry on another endpoint (BrokenForTask),
	// not a ban of the endpoint itself.
	assert.False(t, isBrokenError(fakeRPCErr{code: 1, msg: drpcNoUpstreamMsg}))
	assert.False(t, isBrokenError(fakeRPCErr{code: 19, msg: xlayerLegacyInternalMsg}))
}

func Test_isMissDataError_executionReverted_notMissData(t *testing.T) {
	// Code 3 is the only standard positive code (execution reverted); it now reaches
	// message matching but must never be classified as miss-data.
	assert.False(t, isMissDataError(fakeRPCErr{code: 3, msg: "execution reverted"}))
	assert.False(t, isMissDataError(fakeRPCErr{code: 3, msg: "execution reverted: ERC20: transfer exceeds balance"}))
}

func Test_isMissDataError_standardApplicationErrors_notMissData(t *testing.T) {
	// Standard application-level codes in (-32000, 0] are never miss-data, even if the
	// message contains a matcher keyword.
	assert.False(t, isMissDataError(fakeRPCErr{code: -32602, msg: "invalid argument"}))
	assert.False(t, isMissDataError(fakeRPCErr{code: -32601, msg: "method not found"}))
	assert.False(t, isMissDataError(fakeRPCErr{code: -1, msg: "internal error"}))
}

func Test_isMissDataError_serverErrorRange_stillMatches(t *testing.T) {
	// The pre-existing behavior for codes <= -32000 is unchanged.
	assert.True(t, isMissDataError(fakeRPCErr{code: -32000, msg: "missing trie node deadbeef"}))
	assert.False(t, isMissDataError(fakeRPCErr{code: -32000, msg: "execution timeout"}))
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

// ── Result.WithAuthorityVeto ──────────────────────────────────────────────────

func Test_WithAuthorityVeto_raisesAuthorityTag(t *testing.T) {
	r := Result{
		Err:     fakeRPCErr{code: -32601, msg: "the method foo_bar does not exist"},
		AddTags: []string{MethodNotSupportedTag("foo_bar")},
	}
	r = r.WithAuthorityVeto("foo_bar", true)
	assert.Contains(t, r.AddTags, MethodNotSupportedByAuthorityTag("foo_bar"))
	assert.Contains(t, r.AddTags, MethodNotSupportedTag("foo_bar"))
}

func Test_WithAuthorityVeto_noopWithoutAuthority(t *testing.T) {
	r := Result{
		Err:     fakeRPCErr{code: -32601, msg: "the method foo_bar does not exist"},
		AddTags: []string{MethodNotSupportedTag("foo_bar")},
	}
	r = r.WithAuthorityVeto("foo_bar", false)
	assert.NotContains(t, r.AddTags, MethodNotSupportedByAuthorityTag("foo_bar"))
}

func Test_WithAuthorityVeto_noopWithoutMethodTag(t *testing.T) {
	// Success, unrelated errors, and other tags never raise the authority tag.
	assert.Empty(t, Result{}.WithAuthorityVeto("foo_bar", true).AddTags)
	r := Result{Err: fakeRPCErr{code: -32000, msg: "some other error"}}
	assert.Empty(t, r.WithAuthorityVeto("foo_bar", true).AddTags)
	r = Result{AddTags: []string{MethodNotSupportedTag("other_method")}}
	assert.NotContains(t, r.WithAuthorityVeto("foo_bar", true).AddTags,
		MethodNotSupportedByAuthorityTag("foo_bar"))
}
