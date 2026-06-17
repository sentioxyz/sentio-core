package drivererrors

import (
	"errors"
	"os"
	"sentioxyz/sentio-core/common/log"
)

var (
	ErrConfigUpdate      = errors.New("config update error")
	ErrCleanUp           = errors.New("clean up error")
	ErrNeedCheckLatest   = errors.New("need check latest")
	ErrProcessor         = errors.New("user processor error")
	ErrProcessorBadUsage = errors.New("user processor bad usage error")
	ErrOverQuota         = errors.New("over quota error of units")
	ErrNeedRestart       = errors.New("error need restart")
)

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

func halt(err error, exitCode ExitCode) {
	log.Errore(err)
	os.Exit(int(exitCode))
}

func Halt(err error) {
	switch {
	case errors.Is(err, ErrProcessor):
		halt(err, LimitedRetry)
	case errors.Is(err, ErrProcessorBadUsage):
		halt(err, NeverRetry)
	case errors.Is(err, ErrOverQuota):
		halt(err, OverQuota)
	default:
		halt(err, AlwaysRetry)
	}
}

func IsProcessorError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrProcessor) ||
		errors.Is(err, ErrProcessorBadUsage) ||
		errors.Is(err, ErrOverQuota) ||
		errors.Is(err, ErrNeedRestart)
}
