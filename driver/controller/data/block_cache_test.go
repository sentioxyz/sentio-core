package data

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestBlockCacheGetOrFetch(t *testing.T) {
	c, err := NewBlockCache[uint64](1024)
	require.NoError(t, err)

	var fetches atomic.Int64
	fetch := func() (uint64, error) {
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
	_, err = c.GetOrFetch(context.Background(), 1, func() (uint64, error) { return 0, boom })
	require.ErrorIs(t, err, boom)

	// Errors are not cached: a subsequent successful fetch for the same block still runs and caches.
	v, err := c.GetOrFetch(context.Background(), 1, func() (uint64, error) { return 42, nil })
	require.NoError(t, err)
	require.Equal(t, uint64(42), v)

	got, ok := c.Get(1)
	require.True(t, ok)
	require.Equal(t, uint64(42), got)
}

// A flight that dies with its starter's cancellation says nothing about the liveness of the other
// callers sharing it: a caller whose own context is still alive must take over and retry instead
// of failing with someone else's context.Canceled.
func TestBlockCacheGetOrFetchTakesOverAfterStarterCancel(t *testing.T) {
	c, err := NewBlockCache[uint64](16)
	require.NoError(t, err)

	var fetches atomic.Int64
	v, err := c.GetOrFetch(context.Background(), 5, func() (uint64, error) {
		if fetches.Add(1) == 1 {
			// What a shared flight yields when its starter bails mid-way.
			return 0, errors.Wrap(context.Canceled, "starter of the shared flight bailed")
		}
		return 99, nil
	})
	require.NoError(t, err)
	require.Equal(t, uint64(99), v)
	require.EqualValues(t, 2, fetches.Load(), "the takeover retry must run a fresh fetch")

	got, ok := c.Get(5)
	require.True(t, ok)
	require.Equal(t, uint64(99), got)
}

// The takeover retry is for surviving callers only: when our own context is the canceled one we
// fail with it, without retrying.
func TestBlockCacheGetOrFetchOwnCancelIsNotRetried(t *testing.T) {
	c, err := NewBlockCache[uint64](16)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	var fetches atomic.Int64
	_, err = c.GetOrFetch(ctx, 6, func() (uint64, error) {
		fetches.Add(1)
		cancel() // the flight dies together with us, its starter
		return 0, ctx.Err()
	})
	require.ErrorIs(t, err, context.Canceled)
	require.EqualValues(t, 1, fetches.Load(), "a caller canceled itself must not retry")
}

// The takeover loop is bounded: a fetch that keeps returning context.Canceled while our context
// stays alive surfaces the error instead of looping forever.
func TestBlockCacheGetOrFetchTakeoverIsBounded(t *testing.T) {
	c, err := NewBlockCache[uint64](16)
	require.NoError(t, err)

	var fetches atomic.Int64
	_, err = c.GetOrFetch(context.Background(), 7, func() (uint64, error) {
		fetches.Add(1)
		return 0, errors.Wrap(context.Canceled, "always canceled")
	})
	require.ErrorIs(t, err, context.Canceled)
	require.EqualValues(t, maxTakeoverAttempts, fetches.Load())
}

// End-to-end takeover: a starter and a waiter share one flight, the starter is canceled mid-fetch
// and the flight dies with it, and the waiter — whose own context is fine — retries as the new
// starter and gets the value.
func TestBlockCacheGetOrFetchWaiterSurvivesStarterCancel(t *testing.T) {
	c, err := NewBlockCache[uint64](16)
	require.NoError(t, err)

	starterCtx, starterCancel := context.WithCancel(context.Background())
	fetchStarted := make(chan struct{})
	starterErr := make(chan error, 1)
	go func() {
		_, err := c.GetOrFetch(starterCtx, 8, func() (uint64, error) {
			close(fetchStarted)
			<-starterCtx.Done() // the RPC dies together with the starter's context
			return 0, errors.Wrap(starterCtx.Err(), "rpc aborted")
		})
		starterErr <- err
	}()
	<-fetchStarted

	type res struct {
		v   uint64
		err error
	}
	waiterRes := make(chan res, 1)
	go func() {
		v, err := c.GetOrFetch(context.Background(), 8, func() (uint64, error) {
			return 123, nil // runs when the waiter takes over as the new starter
		})
		waiterRes <- res{v, err}
	}()
	// Give the waiter a moment to join the in-flight fetch before killing the starter. Timing only
	// affects which path the waiter takes (shared-flight takeover vs starting fresh); both must
	// yield the value.
	time.Sleep(50 * time.Millisecond)
	starterCancel()

	require.ErrorIs(t, <-starterErr, context.Canceled)
	waiter := <-waiterRes
	require.NoError(t, waiter.err)
	require.Equal(t, uint64(123), waiter.v)

	got, ok := c.Get(8)
	require.True(t, ok)
	require.Equal(t, uint64(123), got)
}

// A waiter whose own context is canceled is released right away with its context error, without
// waiting for the shared flight to finish.
func TestBlockCacheGetOrFetchWaiterCancelReturnsEarly(t *testing.T) {
	c, err := NewBlockCache[uint64](16)
	require.NoError(t, err)

	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	go func() {
		_, _ = c.GetOrFetch(context.Background(), 9, func() (uint64, error) {
			close(fetchStarted)
			<-releaseFetch
			return 11, nil
		})
	}()
	<-fetchStarted

	waiterCtx, waiterCancel := context.WithCancel(context.Background())
	waiterCancel()
	start := time.Now()
	_, err = c.GetOrFetch(waiterCtx, 9, func() (uint64, error) { return 0, nil })
	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(start), time.Second, "a canceled waiter must not block on the flight")

	close(releaseFetch)
}
