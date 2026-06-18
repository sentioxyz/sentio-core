package data

import (
	"errors"
	"fmt"
)

// NewClientRetryableError marks a failure returned by a chain data client constructor (the NewClient
// functions under data/<chain>, e.g. data/sol.NewClient) that is transient and worth retrying,
// rather than a permanent misconfiguration. For example, a data endpoint that keeps returning
// HTTP/timeout errors while probing its capabilities may simply be temporarily unavailable; failing
// permanently would strand the processor, whereas retrying (by restarting the pod) gives it a fresh
// chance once the endpoint recovers.
//
// This package intentionally knows nothing about how callers act on the error; it only signals that
// the NewClient failure is retryable. The startup controller (which depends on this package) inspects
// construction errors with IsNewClientRetryable / errors.As and chooses to restart the pod instead of
// failing permanently when it matches.
type NewClientRetryableError struct {
	// Reason is a human-readable description of why client construction failed.
	Reason string
	// Err is the underlying error that triggered the retryable failure, if any.
	Err error
}

// NewClientRetryable wraps err into a NewClientRetryableError with the given reason.
func NewClientRetryable(reason string, err error) *NewClientRetryableError {
	return &NewClientRetryableError{Reason: reason, Err: err}
}

func (e *NewClientRetryableError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("retryable NewClient error: %s: %v", e.Reason, e.Err)
	}
	return fmt.Sprintf("retryable NewClient error: %s", e.Reason)
}

func (e *NewClientRetryableError) Unwrap() error {
	return e.Err
}

// IsNewClientRetryable reports whether err is, or wraps, a *NewClientRetryableError.
func IsNewClientRetryable(err error) bool {
	var target *NewClientRetryableError
	return errors.As(err, &target)
}
