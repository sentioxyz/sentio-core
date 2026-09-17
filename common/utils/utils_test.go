package utils

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

func Test_Fetch(t *testing.T) {
	var a *string
	assert.Equal(t, "aa", Fetch(a, "aa"))
	var v = "bb"
	a = &v
	assert.Equal(t, "bb", Fetch(a, "aa"))
}

func Test_AddSecretMosaic(t *testing.T) {
	testcases := [][]string{
		{"", ""},
		{"s3cr3t", "******"},
		{"0123456789", "0********9"},
		{"0123456789012345678", "0*****************8"},
		{"01234567890123456789", "01****************89"},
		{"密码密码密码密码密码", "密********码"},
	}
	for i, testcase := range testcases {
		assert.Equal(t, testcase[1], AddSecretMosaic(testcase[0]), fmt.Sprintf("testcase #%d %#v", i, testcase))
	}
}

func Test_AddURLMosaic(t *testing.T) {
	testcases := [][]string{
		{"", ""},
		{"eth-mainnet.lb.1", "eth-mainnet.lb.1"},
		{"clickhouse://sentio:s3cr3t@ch-0.sentio.xyz:9000/default",
			"clickhouse://sentio:******@ch-0.sentio.xyz:9000/default"},
		// A long enough password keeps its ends, so a wrong one can be told apart from the intended one.
		{"clickhouse://sentio:abcdefghijklmnopqrst@ch-0:9000/default",
			"clickhouse://sentio:ab****************st@ch-0:9000/default"},
		// The hint describes the configured secret, not its percent-encoded form.
		{"clickhouse://sentio:abcde%40ghij@ch-0:9000/default", "clickhouse://sentio:a********j@ch-0:9000/default"},
		{"clickhouse://sentio:s3cr3t@ch-0:9000,ch-1:9000/default", "clickhouse://sentio:******@ch-0:9000,ch-1:9000/default"},
		{"postgres://sentio:s3cr3t@pg-0:5432/sentio?sslmode=disable",
			"postgres://sentio:******@pg-0:5432/sentio?sslmode=disable"},
		// ClickHouse also reads the credentials from the query, and the other parameters stay as configured.
		{"tcp://ch-0:9000/default?username=sentio&password=s3cr3t&secure=true",
			"tcp://ch-0:9000/default?username=sentio&password=******&secure=true"},
		{"https://sentio:s3cr3t@ch-0:8443/default?password=other", "https://sentio:******@ch-0:8443/default?password=*****"},
		// A parsed query decodes its keys, so this names the password parameter too.
		{"clickhouse://sentio@ch-0:9000/default?pass%77ord=s3cr3t",
			"clickhouse://sentio@ch-0:9000/default?pass%77ord=******"},
		// The driver reads the credentials of a scheme-less authority too.
		{"//sentio:s3cr3t@ch-0:9000/default", "//sentio:******@ch-0:9000/default"},
		{"clickhouse://sentio@ch-0:9000/default?secure=true", "clickhouse://sentio@ch-0:9000/default?secure=true"},
		// A "//" that is not the start of an authority is left alone.
		{"clickhouse://ch-0:9000/a//b", "clickhouse://ch-0:9000/a//b"},
		// A URL that cannot be parsed may still hold a password, so it is never echoed back.
		{"clickhouse://sentio:s3cr3t@ch-0:9000/%zz", "<unparsable url>"},
	}
	for i, testcase := range testcases {
		assert.Equal(t, testcase[1], AddURLMosaic(testcase[0]), fmt.Sprintf("testcase #%d %#v", i, testcase))
	}
}

func Test_AddOwnerNameMosaic(t *testing.T) {
	testcases := [][]string{
		{"", ""},
		{"a", "a"},
		{"ab", "ab"},
		{"abc", "a*c"},
		{"abcd", "a**d"},
		{"abcde", "a***e"},
		{"abcdef", "ab**ef"},
		{"abcdefg", "ab***fg"},
		{"abcdefgh", "ab****gh"},
		{"abcdefghi", "abc***ghi"},
		{"abcdefghij", "abc****hij"},
		{"01234567890123456789", "012**************789"},
	}
	for i, testcase := range testcases {
		assert.Equal(t, testcase[1], AddOwnerNameMosaic(testcase[0]), fmt.Sprintf("testcase #%d %#v", i, testcase))
	}
}

func Test_WrapPointerForArray(t *testing.T) {
	type object struct {
		A string
	}
	arr := []object{{A: "abc"}, {A: "def"}, {A: "xyz"}}
	parr := WrapPointerForArray(arr)
	parr[0].A = "123"
	parr[2].A = "456"
	assert.Equal(t, []object{{A: "123"}, {A: "def"}, {A: "456"}}, arr)
}

func Test_ZeroOrUInt64(t *testing.T) {
	assert.Equal(t, uint64(0), ZeroOrUInt64(nil))
	assert.Equal(t, uint64(1), ZeroOrUInt64(big.NewInt(1)))
}

// fakeRPCError mimics an unexported concrete error type returned by a third-party library
// (e.g. go-ethereum's *rpc.jsonError) to simulate the typed-nil scenario.
type fakeRPCError struct{ Code int }

func (e *fakeRPCError) Error() string { return fmt.Sprintf("rpc error %d", e.Code) }

func Test_IsTypedNil(t *testing.T) {
	// typed nil: *fakeRPCError stored in error interface — err != nil but data pointer is nil
	var typedNilErr error = (*fakeRPCError)(nil)
	assert.True(t, IsTypedNil(typedNilErr), "typed nil pointer in interface should be detected")

	// regular error: non-nil pointer
	assert.False(t, IsTypedNil(errors.New("oops")), "regular error should not be detected as typed nil")

	// untyped nil interface: reflect.ValueOf returns zero Value with Kind==Invalid
	assert.False(t, IsTypedNil[error](nil), "untyped nil should not be detected as typed nil")

	// plain pointer types (not via interface)
	assert.True(t, IsTypedNil((*fakeRPCError)(nil)), "nil pointer should be detected")
	assert.False(t, IsTypedNil(&fakeRPCError{Code: 1}), "non-nil pointer should not be detected")
	assert.True(t, IsTypedNil[error]((*fakeRPCError)(nil)), "nil pointer should be detected")
	assert.False(t, IsTypedNil[error](&fakeRPCError{Code: 1}), "non-nil pointer should not be detected")
}
