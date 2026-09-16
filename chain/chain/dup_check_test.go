package chain

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rg "sentioxyz/sentio-core/common/range"
)

type testDuplicateChecker struct {
	mu        sync.Mutex
	intervals []rg.Range
	err       error
}

func (c *testDuplicateChecker) CheckDuplicates(_ context.Context, interval rg.Range) ([]DuplicateReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.intervals = append(c.intervals, interval)
	return nil, c.err
}

func (c *testDuplicateChecker) checked() []rg.Range {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]rg.Range(nil), c.intervals...)
}

func newTestDuplicateCheck(checker DuplicateChecker, interval time.Duration, lookback uint64) *duplicateCheck {
	return &duplicateCheck{checker: checker, interval: interval, lookback: lookback}
}

func waitIdle(t *testing.T, c *duplicateCheck) {
	t.Helper()
	require.Eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return !c.running
	}, time.Second, time.Millisecond)
}

func TestDuplicateCheck_windowsTileTheSyncedRange(t *testing.T) {
	checker := &testDuplicateChecker{}
	c := newTestDuplicateCheck(checker, time.Millisecond, 0)
	ctx := context.Background()

	// lookback 0: the first run trusts everything synced before startup and only records the watermark
	c.maybeStart(ctx, rg.NewRange(0, 999))
	waitIdle(t, c)
	assert.Empty(t, checker.checked())

	// following runs cover exactly the slots synced since the previous watermark
	time.Sleep(2 * time.Millisecond)
	c.maybeStart(ctx, rg.NewRange(0, 1299))
	waitIdle(t, c)
	time.Sleep(2 * time.Millisecond)
	c.maybeStart(ctx, rg.NewRange(500, 1400)) // a head cut moved the range start, the window is unaffected
	waitIdle(t, c)
	assert.Equal(t, []rg.Range{rg.NewRange(1000, 1299), rg.NewRange(1300, 1400)}, checker.checked())

	// nothing new synced: no run
	time.Sleep(2 * time.Millisecond)
	c.maybeStart(ctx, rg.NewRange(500, 1400))
	waitIdle(t, c)
	assert.Len(t, checker.checked(), 2)
}

func TestDuplicateCheck_firstRunLooksBack(t *testing.T) {
	checker := &testDuplicateChecker{}
	ctx := context.Background()

	c := newTestDuplicateCheck(checker, time.Millisecond, 300)
	c.maybeStart(ctx, rg.NewRange(0, 999))
	waitIdle(t, c)
	assert.Equal(t, []rg.Range{rg.NewRange(700, 999)}, checker.checked())

	// a lookback larger than the destination range is clamped to it
	checker = &testDuplicateChecker{}
	c = newTestDuplicateCheck(checker, time.Millisecond, 5000)
	c.maybeStart(ctx, rg.NewRange(100, 999))
	waitIdle(t, c)
	assert.Equal(t, []rg.Range{rg.NewRange(100, 999)}, checker.checked())
}

func TestDuplicateCheck_intervalAndFailures(t *testing.T) {
	checker := &testDuplicateChecker{}
	c := newTestDuplicateCheck(checker, time.Hour, 10)
	ctx := context.Background()

	c.maybeStart(ctx, rg.NewRange(0, 999))
	waitIdle(t, c)
	// the interval has not elapsed: no second run however much was synced
	c.maybeStart(ctx, rg.NewRange(0, 1999))
	waitIdle(t, c)
	assert.Equal(t, []rg.Range{rg.NewRange(990, 999)}, checker.checked())

	// a failed run keeps its window so the next run covers it again
	checker = &testDuplicateChecker{err: errors.New("boom")}
	c = newTestDuplicateCheck(checker, time.Millisecond, 10)
	c.maybeStart(ctx, rg.NewRange(0, 999))
	waitIdle(t, c)
	time.Sleep(2 * time.Millisecond)
	checker.mu.Lock()
	checker.err = nil
	checker.mu.Unlock()
	c.maybeStart(ctx, rg.NewRange(0, 1099))
	waitIdle(t, c)
	assert.Equal(t, []rg.Range{rg.NewRange(990, 999), rg.NewRange(990, 1099)}, checker.checked())

	// an unsupported destination disables the check for good
	checker = &testDuplicateChecker{err: ErrDuplicateCheckUnsupported}
	c = newTestDuplicateCheck(checker, time.Millisecond, 10)
	c.maybeStart(ctx, rg.NewRange(0, 999))
	waitIdle(t, c)
	time.Sleep(2 * time.Millisecond)
	c.maybeStart(ctx, rg.NewRange(0, 1999))
	waitIdle(t, c)
	assert.Len(t, checker.checked(), 1)
	assert.True(t, c.disabled)
}

func TestNewDuplicateCheck(t *testing.T) {
	dim, _, _ := newTestDimension()
	assert.Nil(t, newDuplicateCheck(dim, SyncConfig{}))
	c := newDuplicateCheck(dim, SyncConfig{DupCheckInterval: time.Hour})
	require.NotNil(t, c)
	// the test slot store does not implement DuplicateChecker: the dimension reports it as unsupported
	_, err := c.checker.CheckDuplicates(context.Background(), rg.NewRange(0, 1))
	assert.ErrorIs(t, err, ErrDuplicateCheckUnsupported)
}
