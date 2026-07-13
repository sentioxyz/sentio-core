package chain

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func Test_TooManyResultsError(t *testing.T) {
	err := NewTooManyResultsError("object changes", 20000, 100, 5000)
	assert.True(t, IsTooManyResultsError(err))
	assert.Contains(t, err.Error(), "object changes")
	assert.Contains(t, err.Error(), "[100, 5000]")
	// legacy clients halve their query range when the error message contains this phrase
	assert.Contains(t, err.Error(), "exceeds the limit")
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

func Test_CheckTooManyResults(t *testing.T) {
	within := []int{1, 2, 3}
	result, err := CheckTooManyResults(within, nil, "transactions", 3, 0, 10)
	assert.NoError(t, err)
	assert.Equal(t, within, result)

	_, err = CheckTooManyResults([]int{1, 2, 3, 4}, nil, "transactions", 3, 0, 10)
	assert.True(t, IsTooManyResultsError(err))

	// limit 0 = unlimited (single-block queries)
	result, err = CheckTooManyResults([]int{1, 2, 3, 4}, nil, "transactions", 0, 5, 5)
	assert.NoError(t, err)
	assert.Len(t, result, 4)

	// an inner too-many error (possibly referencing an internal sub-range) is rewritten against
	// the caller's full request range
	inner := errors.Wrapf(NewTooManyResultsError("transactions", 3, 7, 8), "scan result failed")
	_, err = CheckTooManyResults[int](nil, inner, "transactions", 3, 0, 10)
	assert.True(t, IsTooManyResultsError(err))
	assert.Contains(t, err.Error(), "[0, 10]")
	assert.NotContains(t, err.Error(), "[7, 8]")

	// other errors pass through unchanged
	plain := errors.New("boom")
	_, err = CheckTooManyResults[int](nil, plain, "transactions", 3, 0, 10)
	assert.Equal(t, plain, err)
}
