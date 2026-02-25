package memlimit

import (
	"errors"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/process"
)

// ErrMemoryLimitExceeded is returned by Exec when maxWait elapses while memory remains above threshold.
var ErrMemoryLimitExceeded = errors.New("memory limit exceeded")

// Monitor provides memory statistics for the current process.
type Monitor interface {
	// GetUsedMemory returns Go heap allocated bytes (runtime.MemStats.HeapAlloc).
	GetUsedMemory() uint64
	// GetProcessMemory returns the process RSS (resident set size) in bytes.
	GetProcessMemory() uint64
}

// Config holds configuration for a Limiter.
type Config struct {
	// ThresholdBytes is the RSS threshold in bytes above which memory is considered limited.
	// A value of 0 disables the limit (Exec always runs immediately).
	ThresholdBytes uint64
	// PollInterval controls how often Exec rechecks memory while waiting.
	// Defaults to 100ms if zero.
	PollInterval time.Duration
}

// DefaultConfig returns a Config with sensible defaults (no threshold set, 100ms poll interval).
func DefaultConfig() Config {
	return Config{
		PollInterval: 100 * time.Millisecond,
	}
}

// runtimeMonitor is the default Monitor backed by runtime.ReadMemStats and gopsutil.
type runtimeMonitor struct {
	proc *process.Process
}

func newRuntimeMonitor() *runtimeMonitor {
	proc, _ := process.NewProcess(int32(os.Getpid()))
	return &runtimeMonitor{proc: proc}
}

func (m *runtimeMonitor) GetUsedMemory() uint64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

func (m *runtimeMonitor) GetProcessMemory() uint64 {
	if m.proc == nil {
		return 0
	}
	info, err := m.proc.MemoryInfo()
	if err != nil || info == nil {
		return 0
	}
	return info.RSS
}

// Limiter wraps a Monitor with threshold-based execution control.
type Limiter struct {
	monitor Monitor
	config  Config
}

// NewLimiter creates a Limiter backed by the default runtime monitor.
func NewLimiter(config Config) *Limiter {
	if config.PollInterval == 0 {
		config.PollInterval = 100 * time.Millisecond
	}
	return &Limiter{
		monitor: newRuntimeMonitor(),
		config:  config,
	}
}

// NewLimiterWithMonitor creates a Limiter with a custom Monitor (useful for testing).
func NewLimiterWithMonitor(monitor Monitor, config Config) *Limiter {
	if config.PollInterval == 0 {
		config.PollInterval = 100 * time.Millisecond
	}
	return &Limiter{
		monitor: monitor,
		config:  config,
	}
}

// GetUsedMemory returns current Go heap allocated bytes.
func (l *Limiter) GetUsedMemory() uint64 {
	return l.monitor.GetUsedMemory()
}

// GetProcessMemory returns current process RSS in bytes.
func (l *Limiter) GetProcessMemory() uint64 {
	return l.monitor.GetProcessMemory()
}

// IsMemoryLimited returns true when process RSS is at or above ThresholdBytes.
// Always returns false when ThresholdBytes is 0.
func (l *Limiter) IsMemoryLimited() bool {
	if l.config.ThresholdBytes == 0 {
		return false
	}
	return l.monitor.GetProcessMemory() >= l.config.ThresholdBytes
}

// Exec waits until memory is no longer limited, then calls f and returns its result.
// Returns ErrMemoryLimitExceeded if maxWait elapses while memory remains above threshold.
// If ThresholdBytes is 0, f is called immediately without any memory check.
func (l *Limiter) Exec(f func() error, maxWait time.Duration) error {
	if l.config.ThresholdBytes == 0 {
		return f()
	}
	deadline := time.Now().Add(maxWait)
	for l.IsMemoryLimited() {
		if time.Now().After(deadline) {
			return ErrMemoryLimitExceeded
		}
		time.Sleep(l.config.PollInterval)
	}
	return f()
}
