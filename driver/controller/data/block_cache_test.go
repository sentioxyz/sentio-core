package data

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

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
			results[i], errs[i] = c.GetOrFetch(7, fetch)
		}()
	}
	wg.Wait()

	for i := 0; i < concurrent; i++ {
		require.NoError(t, errs[i])
		require.Equal(t, uint64(70), results[i])
	}
	require.EqualValues(t, 1, fetches.Load(), "concurrent misses for the same block fetch once")

	// A later call hits the cache without fetching again.
	v, err := c.GetOrFetch(7, fetch)
	require.NoError(t, err)
	require.Equal(t, uint64(70), v)
	require.EqualValues(t, 1, fetches.Load())
}

func TestBlockCacheGetOrFetchError(t *testing.T) {
	c, err := NewBlockCache[uint64](16)
	require.NoError(t, err)

	boom := errors.New("boom")
	_, err = c.GetOrFetch(1, func() (uint64, error) { return 0, boom })
	require.ErrorIs(t, err, boom)

	// Errors are not cached: a subsequent successful fetch for the same block still runs and caches.
	v, err := c.GetOrFetch(1, func() (uint64, error) { return 42, nil })
	require.NoError(t, err)
	require.Equal(t, uint64(42), v)

	got, ok := c.Get(1)
	require.True(t, ok)
	require.Equal(t, uint64(42), got)
}
