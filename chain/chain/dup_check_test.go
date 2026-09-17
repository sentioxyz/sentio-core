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
	mu     sync.Mutex
	checks int
	err    error
	block  chan struct{} // when set, a check waits for it before returning
}

func (c *testDuplicateChecker) CheckDuplicates(_ context.Context) (rg.Range, []DuplicateReport, error) {
	c.mu.Lock()
	c.checks++
	err, block := c.err, c.block
	c.mu.Unlock()
	if block != nil {
		<-block
	}
	return rg.NewRange(0, 1), nil, err
}

func (c *testDuplicateChecker) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.checks
}

func waitIdle(t *testing.T, c *duplicateCheck) {
	t.Helper()
	require.Eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return !c.running
	}, time.Second, time.Millisecond)
}

func TestDuplicateCheck_runsOncePerInterval(t *testing.T) {
	checker := &testDuplicateChecker{}
	c := &duplicateCheck{checker: checker, interval: time.Hour}
	ctx := context.Background()

	c.maybeStart(ctx)
	waitIdle(t, c)
	assert.Equal(t, 1, checker.count())

	// the interval has not elapsed, however many rounds go by
	for i := 0; i < 5; i++ {
		c.maybeStart(ctx)
		waitIdle(t, c)
	}
	assert.Equal(t, 1, checker.count())

	c.interval = time.Millisecond
	time.Sleep(2 * time.Millisecond)
	c.maybeStart(ctx)
	waitIdle(t, c)
	assert.Equal(t, 2, checker.count())
}

func TestDuplicateCheck_skipsWhileOneIsInFlight(t *testing.T) {
	block := make(chan struct{})
	checker := &testDuplicateChecker{block: block}
	c := &duplicateCheck{checker: checker, interval: time.Millisecond}
	ctx := context.Background()

	c.maybeStart(ctx)
	require.Eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.running
	}, time.Second, time.Millisecond)

	// a check that outlasts the interval must not have a second one started on top of it
	time.Sleep(3 * time.Millisecond)
	c.maybeStart(ctx)
	c.maybeStart(ctx)
	assert.Equal(t, 1, checker.count())

	close(block)
	waitIdle(t, c)
	assert.Equal(t, 1, checker.count())
}

func TestDuplicateCheck_failureAndUnsupported(t *testing.T) {
	ctx := context.Background()

	// a failed check is simply run again next time
	checker := &testDuplicateChecker{err: errors.New("boom")}
	c := &duplicateCheck{checker: checker, interval: time.Millisecond}
	c.maybeStart(ctx)
	waitIdle(t, c)
	time.Sleep(2 * time.Millisecond)
	c.maybeStart(ctx)
	waitIdle(t, c)
	assert.Equal(t, 2, checker.count())
	assert.False(t, c.disabled)

	// a destination that cannot check is asked once and then left alone
	checker = &testDuplicateChecker{err: ErrDuplicateCheckUnsupported}
	c = &duplicateCheck{checker: checker, interval: time.Millisecond}
	c.maybeStart(ctx)
	waitIdle(t, c)
	time.Sleep(2 * time.Millisecond)
	c.maybeStart(ctx)
	waitIdle(t, c)
	assert.Equal(t, 1, checker.count())
	assert.True(t, c.disabled)
}

func TestNewDuplicateCheck(t *testing.T) {
	dim, rs, store := newTestDimension()
	assert.Nil(t, newDuplicateCheck(dim, SyncConfig{}))

	c := newDuplicateCheck(dim, SyncConfig{DupCheckInterval: time.Hour})
	require.NotNil(t, c)

	// nothing has been recorded yet, so there is no window to scan
	window, _, err := c.checker.CheckDuplicates(context.Background())
	assert.NoError(t, err)
	assert.True(t, window.IsEmpty())
	assert.Empty(t, store.checkedWindows())

	// the window spans from the oldest end the range store still holds to the current one
	_, _ = rs.Update(context.Background(), rg.RangeSetter(rg.NewRange(100, 199)))
	_, _ = rs.Update(context.Background(), rg.RangeSetter(rg.NewRange(100, 299)))
	window, _, err = c.checker.CheckDuplicates(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, rg.NewRange(199, 299), window)
	assert.Equal(t, []rg.Range{rg.NewRange(199, 299)}, store.checkedWindows())

	// a store that cannot scan reports it
	plain := NewSimpleDimension[*testSlot](&testRangeStore{cur: rg.EmptyRange}, &uncheckableSlotStore{})
	_, _, err = plain.CheckDuplicates(context.Background())
	assert.ErrorIs(t, err, ErrDuplicateCheckUnsupported)
}

// uncheckableSlotStore is a slot store that does not implement DuplicateScanner.
type uncheckableSlotStore struct {
	SimpleSlotStore[*testSlot]
}
