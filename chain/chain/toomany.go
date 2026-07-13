package chain

import (
	"strings"

	"github.com/pkg/errors"
)

// tooManyResultsMarker tags the errors produced by NewTooManyResultsError so they stay recognizable
// after crossing the JSON-RPC boundary, where only the message text survives.
const tooManyResultsMarker = "too many results"

// NewTooManyResultsError reports that a range query matched more than limit records of some kind
// (e.g. "object changes") in [from, to], signaling the caller to retry with a smaller range. Super
// node range queries return it when their result cap is exceeded; IsTooManyResultsError detects it
// on the client side. The message deliberately contains the phrase "exceeds the limit": some
// existing clients detect over-limit range queries by that phrase and halve their query range on
// it, so keeping it makes them shrink and retry instead of failing outright.
func NewTooManyResultsError(kind string, limit int, from, to uint64) error {
	return errors.Errorf("%s: %s count in range [%d, %d] exceeds the limit %d",
		tooManyResultsMarker, kind, from, to, limit)
}

// IsTooManyResultsError reports whether err was produced by NewTooManyResultsError. The error may
// have crossed the JSON-RPC boundary, so it is matched by message text rather than by identity.
func IsTooManyResultsError(err error) bool {
	return err != nil && strings.Contains(err.Error(), tooManyResultsMarker)
}

// CheckQuerySpan rejects a range query whose block span (to - from) exceeds maxSpan, so a query
// over a huge range cannot force a full-range store scan regardless of how few records it matches.
func CheckQuerySpan(from, to, maxSpan uint64) error {
	if to < from {
		return errors.Errorf("toBlock %d cannot be less than fromBlock %d", to, from)
	}
	if to-from > maxSpan {
		return errors.Errorf("block span %d (> %d) is too large in range [%d, %d]", to-from, maxSpan, from, to)
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

// CheckTooManyResults re-checks the record cap on a fully merged result (e.g. latest-slot cache +
// store): the storage layer enforces the cap on its own part, but records collected from the cache
// also count.
func CheckTooManyResults[T any](result []T, kind string, limit int, from, to uint64) ([]T, error) {
	if limit > 0 && len(result) > limit {
		return nil, NewTooManyResultsError(kind, limit, from, to)
	}
	return result, nil
}
