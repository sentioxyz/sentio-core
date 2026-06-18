package exitcode

// Code is the process exit code the streaming driver uses to tell its
// supervisor how to retry. The concrete ErrXXX sentinel errors and the Halt
// helper that map onto these codes are used by driver v2 and stay in the sentio
// repository.
type Code int

const (
	AlwaysRetry       Code = 1
	LimitedRetry      Code = 10
	NeverRetry        Code = 11
	OverQuota         Code = 12
	RetryAfterOneHour Code = 20
	RetryNextDay      Code = 21
	RetryNextMonth    Code = 22
)
