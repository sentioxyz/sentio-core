package drivererrors

// ExitCode is the process exit code the streaming driver uses to tell its
// supervisor how to retry. The concrete ErrXXX sentinel errors and the Halt
// helper that map onto these codes are used by driver v2 and stay in the sentio
// repository.
type ExitCode int

const (
	AlwaysRetry       ExitCode = 1
	LimitedRetry      ExitCode = 10
	NeverRetry        ExitCode = 11
	OverQuota         ExitCode = 12
	RetryAfterOneHour ExitCode = 20
	RetryNextDay      ExitCode = 21
	RetryNextMonth    ExitCode = 22
)
