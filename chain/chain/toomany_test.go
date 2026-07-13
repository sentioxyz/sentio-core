package chain

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func Test_TooManyResultsError(t *testing.T) {
	err := NewTooManyResultsError()
	assert.True(t, IsTooManyResultsError(err))
	// legacy clients halve their query range when the error message contains this phrase
	assert.Contains(t, err.Error(), "exceeds the limit")
	// the message tells the caller what to do, without exposing server-side internals
	assert.Contains(t, err.Error(), "narrow the query block range")
	// the marker survives wrapping (e.g. the JSON-RPC transport or errors.Wrapf)
	assert.True(t, IsTooManyResultsError(errors.Wrapf(err, "scan result failed")))

	assert.False(t, IsTooManyResultsError(nil))
	assert.False(t, IsTooManyResultsError(errors.New("some other error")))
}

func Test_CheckQuerySpan(t *testing.T) {
	assert.NoError(t, CheckQuerySpan(100, 100, 1000))  // single block
	assert.NoError(t, CheckQuerySpan(100, 1100, 1000)) // span == maxSpan
	assert.Error(t, CheckQuerySpan(100, 99, 1000))     // reversed range

	err := CheckQuerySpan(100, 1101, 1000) // span > maxSpan
	assert.Error(t, err)
	// clients that halve their query range on this phrase converge below the span cap too
	assert.Contains(t, err.Error(), "exceeds the limit")
}

func Test_RangeQueryLimit(t *testing.T) {
	assert.Equal(t, 0, RangeQueryLimit(42, 42, 5000)) // single block: unlimited
	assert.Equal(t, 5000, RangeQueryLimit(42, 43, 5000))
}

func Test_StoreQueryLimit(t *testing.T) {
	assert.Equal(t, 0, StoreQueryLimit(0)) // disabled cap stays unlimited
	assert.Equal(t, 2001, StoreQueryLimit(2000))
}

func Test_CheckTooManyResults(t *testing.T) {
	within := []int{1, 2, 3}
	result, err := CheckTooManyResults(within, nil, 3)
	assert.NoError(t, err)
	assert.Equal(t, within, result)

	_, err = CheckTooManyResults([]int{1, 2, 3, 4}, nil, 3)
	assert.True(t, IsTooManyResultsError(err))

	// limit 0 = unlimited (single-block queries)
	result, err = CheckTooManyResults([]int{1, 2, 3, 4}, nil, 0)
	assert.NoError(t, err)
	assert.Len(t, result, 4)

	// errors (including the storage's own too-many error) pass through unchanged
	inner := errors.Wrapf(NewTooManyResultsError(), "scan result failed")
	_, err = CheckTooManyResults[int](nil, inner, 3)
	assert.Equal(t, inner, err)
	assert.True(t, IsTooManyResultsError(err))

	plain := errors.New("boom")
	_, err = CheckTooManyResults[int](nil, plain, 3)
	assert.Equal(t, plain, err)
}
