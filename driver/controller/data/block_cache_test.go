package data

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
)

func TestBlockCacheGetOrFetch(t *testing.T) {
	c, err := NewBlockCache[uint64](1024)
	require.NoError(t, err)

	var fetches atomic.Int64
	fetch := func(context.Context) (uint64, error) {
		fetches.Add(1)
		return 70, nil
	}

	// Concurrent callers that all miss the same block must collapse into a single fetch. Run under
	// -race, this also guards that BlockCache itself has no internal data race.
	const concurrent = 50
	results := make([]uint64, concurrent)
	errs := make([]error, concurrent)
	var wg sync.WaitGroup
	for i := 0; i < concurrent; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = c.GetOrFetch(context.Background(), 7, fetch)
		}()
	}
	wg.Wait()

	for i := 0; i < concurrent; i++ {
		require.NoError(t, errs[i])
		require.Equal(t, uint64(70), results[i])
	}
	require.EqualValues(t, 1, fetches.Load(), "concurrent misses for the same block fetch once")

	// A later call hits the cache without fetching again.
	v, err := c.GetOrFetch(context.Background(), 7, fetch)
	require.NoError(t, err)
	require.Equal(t, uint64(70), v)
	require.EqualValues(t, 1, fetches.Load())
}

func TestBlockCacheGetOrFetchError(t *testing.T) {
	c, err := NewBlockCache[uint64](16)
	require.NoError(t, err)

	boom := errors.New("boom")
	_, err = c.GetOrFetch(context.Background(), 1, func(context.Context) (uint64, error) { return 0, boom })
	require.ErrorIs(t, err, boom)

	// Errors are not cached: a subsequent successful fetch for the same block still runs and caches.
	v, err := c.GetOrFetch(context.Background(), 1, func(context.Context) (uint64, error) { return 42, nil })
	require.NoError(t, err)
	require.Equal(t, uint64(42), v)

	got, ok := c.Get(1)
	require.True(t, ok)
	require.Equal(t, uint64(42), got)
}

// A waiter sharing a flight must not be failed by the cancellation of the caller that started it:
// the flight runs detached, so the waiter still receives the fetched value, while the canceled
// starter itself is released immediately with its own context error.
func TestBlockCacheGetOrFetchStarterCancelDoesNotFailWaiters(t *testing.T) {
	c, err := NewBlockCache[uint64](16)
	require.NoError(t, err)

	starterCtx, starterCancel := context.WithCancel(context.Background())
	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	var fetchCtxErr error
	fetch := func(fetchCtx context.Context) (uint64, error) {
		close(fetchStarted)
		<-releaseFetch
		fetchCtxErr = fetchCtx.Err()
		return 99, nil
	}

	starterErr := make(chan error, 1)
	go func() {
		_, err := c.GetOrFetch(starterCtx, 5, fetch)
		starterErr <- err
	}()
	<-fetchStarted

	// A second caller joins the in-flight fetch (or, if it loses the race with the flight's
	// completion, hits the freshly filled cache — the same value either way).
	waiterV := make(chan uint64, 1)
	waiterErr := make(chan error, 1)
	go func() {
		v, err := c.GetOrFetch(context.Background(), 5, func(context.Context) (uint64, error) {
			t.Error("waiter must join the existing flight, not start its own fetch")
			return 0, nil
		})
		waiterV <- v
		waiterErr <- err
	}()

	// Cancel the starter while the flight is still running: only the starter comes back canceled.
	starterCancel()
	require.ErrorIs(t, <-starterErr, context.Canceled)

	close(releaseFetch)
	require.NoError(t, <-waiterErr)
	require.Equal(t, uint64(99), <-waiterV)
	require.NoError(t, fetchCtxErr, "the flight context must survive the starter's cancellation")

	got, ok := c.Get(5)
	require.True(t, ok)
	require.Equal(t, uint64(99), got)
}

// A waiter whose own context is canceled is released right away with its context error, without
// waiting for (or aborting) the shared flight.
func TestBlockCacheGetOrFetchWaiterCancelReturnsEarly(t *testing.T) {
	c, err := NewBlockCache[uint64](16)
	require.NoError(t, err)

	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	go func() {
		_, _ = c.GetOrFetch(context.Background(), 8, func(context.Context) (uint64, error) {
			close(fetchStarted)
			<-releaseFetch
			return 11, nil
		})
	}()
	<-fetchStarted

	waiterCtx, waiterCancel := context.WithCancel(context.Background())
	waiterCancel()
	start := time.Now()
	_, err = c.GetOrFetch(waiterCtx, 8, func(context.Context) (uint64, error) { return 0, nil })
	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(start), time.Second, "a canceled waiter must not block on the flight")

	close(releaseFetch)
}

// InvalidateRange must obsolete work already in flight, not just cached entries: the in-flight
// value may describe the invalidated (reorged-away) fork, is not yet in the LRU, and its flight
// keeps running detached from any canceled caller. The stale flight's result must be neither
// cached nor served, and a post-invalidation caller must not join the pre-invalidation flight.
func TestBlockCacheInvalidateRangeObsoletesInFlight(t *testing.T) {
	c, err := NewBlockCache[uint64](16)
	require.NoError(t, err)

	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	staleErr := make(chan error, 1)
	go func() {
		_, err := c.GetOrFetch(context.Background(), 5, func(context.Context) (uint64, error) {
			close(fetchStarted)
			<-releaseFetch
			return 100, nil // value from the fork that InvalidateRange will throw away
		})
		staleErr <- err
	}()
	<-fetchStarted

	c.InvalidateRange(controller.BlockRange{StartBlock: 5, EndBlock: utils.WrapPointer(uint64(5))})

	// A caller arriving after the invalidation starts a fresh fetch (new singleflight key) and
	// gets the new fork's value even though the old flight is still running.
	type res struct {
		v   uint64
		err error
	}
	freshRes := make(chan res, 1)
	go func() {
		v, err := c.GetOrFetch(context.Background(), 5, func(context.Context) (uint64, error) {
			return 200, nil
		})
		freshRes <- res{v, err}
	}()
	fresh := <-freshRes
	require.NoError(t, fresh.err)
	require.Equal(t, uint64(200), fresh.v)

	// Only now does the old flight complete: its value is discarded and its waiter told to retry.
	close(releaseFetch)
	require.ErrorIs(t, <-staleErr, ErrFlightInvalidated)

	got, ok := c.Get(5)
	require.True(t, ok)
	require.Equal(t, uint64(200), got, "the stale flight must not overwrite the post-invalidation value")
}

// InvalidateRange still removes already-cached entries in the range (the pre-existing behavior of
// the per-key removal it replaced) and leaves entries outside the range alone.
func TestBlockCacheInvalidateRangeRemovesCached(t *testing.T) {
	c, err := NewBlockCache[uint64](16)
	require.NoError(t, err)
	c.Add(1, 10)
	c.Add(2, 20)
	c.Add(9, 90)

	c.InvalidateRange(controller.BlockRange{StartBlock: 1, EndBlock: utils.WrapPointer(uint64(2))})

	_, ok := c.Get(1)
	require.False(t, ok)
	_, ok = c.Get(2)
	require.False(t, ok)
	v, ok := c.Get(9)
	require.True(t, ok)
	require.Equal(t, uint64(90), v)
}
