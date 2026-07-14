package chain

import (
	"strings"

	"github.com/pkg/errors"
)

// tooManyResultsMarker tags the errors produced by NewTooManyResultsError so they stay recognizable
// after crossing the JSON-RPC boundary, where only the message text survives.
const tooManyResultsMarker = "too many results"

// NewTooManyResultsError reports that a range query matched more records than a server-side limit
// allows, signaling the caller to retry with a smaller range. It is deliberately vague: the limits
// are the server's own resource guards, not caller-chosen, so the message only tells the caller
// what to do about it. It also deliberately contains the phrase "exceeds the limit": some existing
// clients detect over-limit range queries by that phrase and halve their query range on it, so
// keeping it makes them shrink and retry instead of failing outright.
func NewTooManyResultsError() error {
	return errors.New(tooManyResultsMarker + ": the result count exceeds the limit, please narrow the query block range")
}

// IsTooManyResultsError reports whether err was produced by NewTooManyResultsError. The error may
// have crossed the JSON-RPC boundary, so it is matched by message text rather than by identity.
func IsTooManyResultsError(err error) bool {
	return err != nil && strings.Contains(err.Error(), tooManyResultsMarker)
}

// CheckQuerySpan rejects a range query whose block span (to - from) exceeds maxSpan, so a query
// over a huge range cannot force a full-range store scan regardless of how few records it matches.
// The check is on the requested range itself — a caller-visible contract, independent of how the
// request is split internally between the latest-slot cache and the store. The message contains
// "exceeds the limit" for the same reason as NewTooManyResultsError: clients that halve their
// query range on that phrase converge below the span cap.
func CheckQuerySpan(from, to, maxSpan uint64) error {
	if to < from {
		return errors.Errorf("toBlock %d cannot be less than fromBlock %d", to, from)
	}
	if to-from > maxSpan {
		return errors.Errorf("block span %d of range [%d, %d] exceeds the limit %d", to-from, from, to, maxSpan)
	}
	return nil
}

// RangeQueryLimit returns the record cap for a range query: limit for a multi-block range,
// 0 (unlimited) for a single-block one, which cannot be shrunk further and so must return all
// matching records.
func RangeQueryLimit(from, to uint64, limit int) int {
	if from == to {
		return 0
	}
	return limit
}

// StoreQueryLimit converts a response record cap into the scan limit to pass to a storage query:
// one extra record, so a query matching exactly the cap still succeeds while anything beyond it
// trips the storage's scan limit. A disabled cap (limit <= 0) stays unlimited.
func StoreQueryLimit(limit int) int {
	if limit <= 0 {
		return 0
	}
	return limit + 1
}

// CheckTooManyResults finalizes a range query outcome: the TOTAL response may hold at most limit
// records, regardless of how the request was split internally between the latest-slot cache and
// the store. It complements the storage-level scan limit — that one bounds the resource use of a
// single SQL, this one bounds the merged response the caller receives.
func CheckTooManyResults[T any](result []T, err error, limit int) ([]T, error) {
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(result) > limit {
		return nil, NewTooManyResultsError()
	}
	return result, nil
}
