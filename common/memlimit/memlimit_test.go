package memlimit

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMonitor is a stub Monitor for testing.
type fakeMonitor struct {
	usedMemory    uint64
	processMemory uint64
}

func (f *fakeMonitor) GetUsedMemory() uint64    { return f.usedMemory }
func (f *fakeMonitor) GetProcessMemory() uint64 { return f.processMemory }

// countingFakeMonitor decrements processMemory on each call to simulate memory freeing.
type countingFakeMonitor struct {
	calls         int
	freeAfterCall int
	highMemory    uint64
	lowMemory     uint64
}

func (m *countingFakeMonitor) GetUsedMemory() uint64 { return m.lowMemory }
func (m *countingFakeMonitor) GetProcessMemory() uint64 {
	m.calls++
	if m.calls > m.freeAfterCall {
		return m.lowMemory
	}
	return m.highMemory
}

func TestExec_NoThreshold(t *testing.T) {
	limiter := NewLimiterWithMonitor(&fakeMonitor{
		usedMemory:    100,
		processMemory: 999 * 1024 * 1024, // 999 MB, well above any threshold
	}, Config{ThresholdBytes: 0, PollInterval: 10 * time.Millisecond})

	called := false
	err := limiter.Exec(func() error {
		called = true
		return nil
	}, time.Second)

	require.NoError(t, err)
	assert.True(t, called, "f should be called when threshold is 0")
}

func TestExec_NoThreshold_PropagatesError(t *testing.T) {
	limiter := NewLimiterWithMonitor(&fakeMonitor{}, Config{ThresholdBytes: 0})
	sentinel := errors.New("fn error")

	err := limiter.Exec(func() error { return sentinel }, time.Second)
	assert.ErrorIs(t, err, sentinel)
}

func TestExec_UnderLimit(t *testing.T) {
	limiter := NewLimiterWithMonitor(&fakeMonitor{
		processMemory: 50 * 1024 * 1024, // 50 MB
	}, Config{ThresholdBytes: 100 * 1024 * 1024, PollInterval: 10 * time.Millisecond})

	called := false
	err := limiter.Exec(func() error {
		called = true
		return nil
	}, time.Second)

	require.NoError(t, err)
	assert.True(t, called)
}

func TestExec_OverLimit_ClearsBeforeTimeout(t *testing.T) {
	monitor := &countingFakeMonitor{
		freeAfterCall: 2,
		highMemory:    200 * 1024 * 1024, // 200 MB
		lowMemory:     50 * 1024 * 1024,  // 50 MB
	}
	limiter := NewLimiterWithMonitor(monitor, Config{
		ThresholdBytes: 100 * 1024 * 1024,
		PollInterval:   1 * time.Millisecond,
	})

	called := false
	err := limiter.Exec(func() error {
		called = true
		return nil
	}, 500*time.Millisecond)

	require.NoError(t, err)
	assert.True(t, called, "f should be called after memory drops below threshold")
}

func TestExec_OverLimit_TimeoutExceeded(t *testing.T) {
	limiter := NewLimiterWithMonitor(&fakeMonitor{
		processMemory: 500 * 1024 * 1024, // always 500 MB, always over limit
	}, Config{
		ThresholdBytes: 100 * 1024 * 1024,
		PollInterval:   1 * time.Millisecond,
	})

	called := false
	err := limiter.Exec(func() error {
		called = true
		return nil
	}, 10*time.Millisecond)

	assert.ErrorIs(t, err, ErrMemoryLimitExceeded)
	assert.False(t, called, "f should not be called when timeout is exceeded")
}

func TestIsMemoryLimited(t *testing.T) {
	const threshold = 100 * 1024 * 1024 // 100 MB

	underLimiter := NewLimiterWithMonitor(&fakeMonitor{processMemory: 50 * 1024 * 1024}, Config{ThresholdBytes: threshold})
	assert.False(t, underLimiter.IsMemoryLimited())

	atLimiter := NewLimiterWithMonitor(&fakeMonitor{processMemory: threshold}, Config{ThresholdBytes: threshold})
	assert.True(t, atLimiter.IsMemoryLimited())

	overLimiter := NewLimiterWithMonitor(&fakeMonitor{processMemory: 200 * 1024 * 1024}, Config{ThresholdBytes: threshold})
	assert.True(t, overLimiter.IsMemoryLimited())

	noThresholdLimiter := NewLimiterWithMonitor(&fakeMonitor{processMemory: 200 * 1024 * 1024}, Config{ThresholdBytes: 0})
	assert.False(t, noThresholdLimiter.IsMemoryLimited())
}

func TestGetUsedMemory_RealMonitor(t *testing.T) {
	limiter := NewLimiter(Config{})
	used := limiter.GetUsedMemory()
	assert.Greater(t, used, uint64(0), "Go heap should report non-zero allocated bytes")
}

func TestGetProcessMemory_RealMonitor(t *testing.T) {
	limiter := NewLimiter(Config{})
	rss := limiter.GetProcessMemory()
	assert.Greater(t, rss, uint64(0), "process RSS should be non-zero")
}
