package fetcher

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/controller"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type testBlockHeader struct {
	BlockNumber uint64
	BlockTime   time.Time
}

func (b testBlockHeader) GetBlockNumber() uint64 {
	return b.BlockNumber
}

func (b testBlockHeader) GetBlockParentHash() string {
	return ""
}

func (b testBlockHeader) GetBlockHash() string {
	return ""
}

func (b testBlockHeader) GetBlockTime() time.Time {
	return b.BlockTime
}

func newTestBlockHeader(blockNumber uint64) testBlockHeader {
	zeroTime, _ := time.Parse(time.DateTime, "2025-07-01 00:00:00")
	return testBlockHeader{
		BlockNumber: blockNumber,
		BlockTime:   zeroTime.Add(time.Second * time.Duration(blockNumber)),
	}
}

type testData []string

func (t testData) Size() int { return len(t) }

func buildTestData(bn uint64) (r testData) {
	for j := uint64(0); j < bn%3; j++ {
		r = append(r, fmt.Sprintf("%d-%d", bn, j))
	}
	return
}

func Test_Fetcher(t *testing.T) {
	log.ManuallySetLevel(zap.DebugLevel)
	log.BindFlag()

	fr := NewFetcher[testData](
		"testFetcher",
		nil,
		controller.BlockRange{StartBlock: 10},
		newTestBlockHeader(38),
		3,
		10,
		20,
		0, // maxReadyBlockCount: unlimited
		20,
		time.Second,
		3,
		time.Second,
		1.2,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]testData, error) {
			r := make(map[uint64]testData)
			for i := start; i <= end; i++ {
				br := buildTestData(i)
				if len(br) > 0 {
					r[i] = br
				}
			}
			return r, nil
		},
	)
	f := fr.(*fetcher[testData])

	var g sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		g.Wait()
	}()

	// ==============================================================================================================
	// ### staging 0, init state
	// --------------------------------------------------------------------------------------------------------------
	//    10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//     1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	// --------------------------------------------------------------------------------------------------------------
	//     S                                                                                   L
	// --------------------------------------------------------------------------------------------------------------
	//    FS    FE
	// --------------------------------------------------------------------------------------------------------------
	// count = 0
	// ==============================================================================================================
	// ### staging 1, after growth 6 times
	// --------------------------------------------------------------------------------------------------------------
	//    10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//     1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	// --------------------------------------------------------------------------------------------------------------
	//     S                                                                                   L
	// --------------------------------------------------------------------------------------------------------------
	// R0 FS    FE
	// R1          FS       FE
	// R2                      FS          FE
	// R3                                     FS             FE
	// R4                                                       FS                   FE
	// R5                                                                              *FS   *FE
	// --------------------------------------------------------------------------------------------------------------
	// count = 27
	// ==============================================================================================================
	// ### staging 2, after pop 7 times
	// --------------------------------------------------------------------------------------------------------------
	//    10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//     1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	// --------------------------------------------------------------------------------------------------------------
	//                         *S                                                              L
	// --------------------------------------------------------------------------------------------------------------
	//                                                                                  FS    FE
	// --------------------------------------------------------------------------------------------------------------
	// count = 20
	// ==============================================================================================================
	// ### staging 3, after pop 4 time and growth 1 time
	// --------------------------------------------------------------------------------------------------------------
	//    10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//     1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	// --------------------------------------------------------------------------------------------------------------
	//                                     *S                                                  L
	// --------------------------------------------------------------------------------------------------------------
	// R0                                                                               FS    FE
	// R1                                                                                    *FE*FS
	// --------------------------------------------------------------------------------------------------------------
	// count = 18
	// ==============================================================================================================
	// ### staging 4, after update latest
	// --------------------------------------------------------------------------------------------------------------
	//    10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//     1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	// --------------------------------------------------------------------------------------------------------------
	//                                      S                                                       *L
	// --------------------------------------------------------------------------------------------------------------
	// R0                                                                                     FE FS
	// R1                                                                                          *FE*FS
	// --------------------------------------------------------------------------------------------------------------
	// count = 19
	// ==============================================================================================================
	// ### staging 5, after pop 20 times
	// --------------------------------------------------------------------------------------------------------------
	//    10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//     1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	// --------------------------------------------------------------------------------------------------------------
	//                                                                                               L *S
	// --------------------------------------------------------------------------------------------------------------
	//                                                                                              FE FS
	// --------------------------------------------------------------------------------------------------------------
	// count = 0
	// ==============================================================================================================
	// ### staging 6, after update latest and growth 1 time and pop 1 time
	// --------------------------------------------------------------------------------------------------------------
	//    10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//     1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	// --------------------------------------------------------------------------------------------------------------
	//                                                                                                 *L *S
	// --------------------------------------------------------------------------------------------------------------
	// R0                                                                                           FE FS
	// R1                                                                                             *FE*FS
	// --------------------------------------------------------------------------------------------------------------
	// count = 0

	// ### staging 0
	assert.Equal(t, uint64(10), f.full.StartBlock)
	assert.Equal(t, uint64(10), f.fetchingStart)
	assert.Equal(t, uint64(12), f.fetchingEnd)
	assert.Equal(t, 0, f.totalSize)

	g.Add(1)
	go func() {
		defer g.Done()
		f.KeepFetch(ctx)
	}()

	// ### staging 1
	time.Sleep(time.Second)
	assert.Equal(t, uint64(10), f.full.StartBlock)
	assert.Equal(t, uint64(36), f.fetchingStart)
	assert.Equal(t, uint64(38), f.fetchingEnd)
	assert.Equal(t, 27, f.totalSize)

	// ### staging 2
	for n := uint64(10); n <= 16; n++ {
		r, _, _, err := f.Get(ctx, n)
		assert.Equal(t, buildTestData(n), r)
		assert.Nil(t, err)
	}
	f.MoveStart(17)
	time.Sleep(time.Second)
	assert.Equal(t, uint64(17), f.full.StartBlock)
	assert.Equal(t, uint64(36), f.fetchingStart)
	assert.Equal(t, uint64(38), f.fetchingEnd)
	assert.Equal(t, 20, f.totalSize)

	// ### staging 3
	for n := uint64(17); n <= 20; n++ {
		r, _, _, err := f.Get(ctx, n)
		assert.Equal(t, buildTestData(n), r)
		assert.Nil(t, err)
	}
	f.MoveStart(21)
	time.Sleep(time.Second)
	assert.Equal(t, uint64(21), f.full.StartBlock)
	assert.Equal(t, uint64(39), f.fetchingStart)
	assert.Equal(t, uint64(38), f.fetchingEnd)
	assert.Equal(t, 18, f.totalSize)

	// ### staging 4
	f.UpdateLatest(newTestBlockHeader(40))
	time.Sleep(time.Second)
	assert.Equal(t, uint64(21), f.full.StartBlock)
	assert.Equal(t, uint64(41), f.fetchingStart)
	assert.Equal(t, uint64(40), f.fetchingEnd)
	assert.Equal(t, 19, f.totalSize)

	// ### staging 5
	for n := uint64(21); n <= 40; n++ {
		r, _, _, err := f.Get(ctx, n)
		assert.Equal(t, buildTestData(n), r)
		assert.Nil(t, err)
	}
	f.MoveStart(42) // will be automatically reduced to 41 because fetchingStart is still at 41
	time.Sleep(time.Second)
	assert.Equal(t, uint64(41), f.full.StartBlock)
	assert.Equal(t, uint64(41), f.fetchingStart)
	assert.Equal(t, uint64(40), f.fetchingEnd)
	assert.Equal(t, 0, f.totalSize)

	// ### staging 6
	go func() {
		time.Sleep(time.Second)
		f.UpdateLatest(newTestBlockHeader(41))
	}()
	r, _, _, err := f.Get(ctx, 41)
	assert.Equal(t, buildTestData(41), r)
	assert.Nil(t, err)
	f.MoveStart(42)
	time.Sleep(time.Second)
	assert.Equal(t, uint64(42), f.full.StartBlock)
	assert.Equal(t, uint64(42), f.fetchingStart)
	assert.Equal(t, uint64(41), f.fetchingEnd)
	assert.Equal(t, 0, f.totalSize)

}

// A context.Canceled that leaks out of queryFunc while the fetcher's own context is alive (e.g. a
// shared singleflight fetch whose starter aborted) must be retried like any other failure. Exiting
// on it would end KeepFetch with fetchingFailed unset and fetchingDone never closed, so every
// Get() would block forever.
func Test_Fetcher_LeakedCancelIsRetried(t *testing.T) {
	log.ManuallySetLevel(zap.DebugLevel)
	log.BindFlag()

	var calls atomic.Int64
	fr := NewFetcher[testData](
		"leakedCancelFetcher",
		nil,
		controller.BlockRange{StartBlock: 10},
		newTestBlockHeader(12),
		3,
		10,
		20,
		0, // maxReadyBlockCount: unlimited
		20,
		time.Second,
		3,
		time.Millisecond*10,
		1.2,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]testData, error) {
			if calls.Add(1) == 1 {
				// Not produced by ctx (which is still alive): simulates a cancellation leaked
				// from an unrelated caller sharing the underlying request.
				return nil, errors.Wrap(context.Canceled, "shared flight starter aborted")
			}
			r := make(map[uint64]testData)
			for i := start; i <= end; i++ {
				if br := buildTestData(i); len(br) > 0 {
					r[i] = br
				}
			}
			return r, nil
		},
	)
	f := fr.(*fetcher[testData])

	var g sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	g.Add(1)
	go func() {
		defer g.Done()
		f.KeepFetch(ctx)
	}()

	getCtx, getCancel := context.WithTimeout(ctx, time.Second*5)
	defer getCancel()
	r, has, _, err := f.Get(getCtx, 11)
	assert.Nil(t, err)
	assert.True(t, has)
	assert.Equal(t, buildTestData(11), r)
	assert.GreaterOrEqual(t, calls.Load(), int64(2), "the leaked Canceled must have been retried")

	// A real cancellation (our own context) still stops the fetch loop.
	cancel()
	g.Wait()
}

// A too-many-results error is the server asking for a narrower range, and the fetcher answers it
// by halving. Where the filter matches densely that is the steady state, not an anomaly: the range
// grows by querySizeMultiplier until it trips the cap, every time. Backing off queryRetryInterval
// on each trip would cost far more than the queries themselves, so the interval must be skipped
// whenever the range actually shrank. The interval here is far larger than the test's budget, so
// the fetch can only finish if it was skipped.
func Test_Fetcher_TooManyResultsShrinksWithoutBackoff(t *testing.T) {
	log.ManuallySetLevel(zap.DebugLevel)
	log.BindFlag()

	const retryInterval = time.Second * 30
	var trips atomic.Int64
	fr := NewFetcher[testData](
		"tooManyResultsFetcher",
		nil,
		controller.BlockRange{StartBlock: 10},
		newTestBlockHeader(20),
		1,    // minQuerySize: the fetcher can shrink to the cap-exempt single block
		32,   // maxQuerySize
		1000, // targetKeepDataSize: never full, so the fetch never pauses
		0,    // maxReadyBlockCount: unlimited
		1000, // targetQueryDataSize: never reached, so the range keeps growing into the cap
		time.Second,
		3,
		retryInterval,
		1.2,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]testData, error) {
			if end > start {
				trips.Add(1)
				return nil, chain.NewTooManyResultsError()
			}
			r := make(map[uint64]testData)
			if br := buildTestData(start); len(br) > 0 {
				r[start] = br
			}
			return r, nil
		},
	)
	f := fr.(*fetcher[testData])

	var g sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		g.Wait()
	}()
	g.Add(1)
	go func() {
		defer g.Done()
		f.KeepFetch(ctx)
	}()

	getCtx, getCancel := context.WithTimeout(ctx, time.Second*5)
	defer getCancel()
	r, has, _, err := f.Get(getCtx, 20)
	assert.Nil(t, err)
	assert.True(t, has)
	assert.Equal(t, buildTestData(20), r)
	assert.Greater(t, trips.Load(), int64(1), "the growing range must have tripped the cap repeatedly")
}

// An ordinary failure carries no instruction about the range, so it keeps backing off: the server
// may simply be unwell, and retrying immediately would hammer it.
func Test_Fetcher_OrdinaryErrorStillBacksOff(t *testing.T) {
	log.ManuallySetLevel(zap.DebugLevel)
	log.BindFlag()

	const retryInterval = time.Millisecond * 500
	var calls atomic.Int64
	fr := NewFetcher[testData](
		"ordinaryErrorFetcher",
		nil,
		controller.BlockRange{StartBlock: 10},
		newTestBlockHeader(12),
		3,
		10,
		20,
		0, // maxReadyBlockCount: unlimited
		20,
		time.Second,
		3,
		retryInterval,
		1.2,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]testData, error) {
			if calls.Add(1) == 1 {
				return nil, errors.New("upstream is unwell")
			}
			r := make(map[uint64]testData)
			for i := start; i <= end; i++ {
				if br := buildTestData(i); len(br) > 0 {
					r[i] = br
				}
			}
			return r, nil
		},
	)
	f := fr.(*fetcher[testData])

	var g sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		g.Wait()
	}()
	startAt := time.Now()
	g.Add(1)
	go func() {
		defer g.Done()
		f.KeepFetch(ctx)
	}()

	getCtx, getCancel := context.WithTimeout(ctx, time.Second*5)
	defer getCancel()
	r, has, _, err := f.Get(getCtx, 11)
	elapsed := time.Since(startAt)
	assert.Nil(t, err)
	assert.True(t, has)
	assert.Equal(t, buildTestData(11), r)
	assert.GreaterOrEqual(t, elapsed, retryInterval, "an ordinary failure must still wait queryRetryInterval")
}

// Once the range is down to minQuerySize the retry repeats the identical query, so even a
// too-many-results error has to back off — skipping the interval there would spin through the
// whole retry budget instantly and turn a recoverable stall into an immediate fetch failure.
func Test_Fetcher_TooManyResultsAtMinQuerySizeStillBacksOff(t *testing.T) {
	log.ManuallySetLevel(zap.DebugLevel)
	log.BindFlag()

	const retryInterval = time.Millisecond * 200
	const maxRetry = 2
	var calls atomic.Int64
	fr := NewFetcher[testData](
		"tooManyResultsFloorFetcher",
		nil,
		controller.BlockRange{StartBlock: 10},
		newTestBlockHeader(12),
		3, // minQuerySize equals the first range, so the fetcher cannot shrink at all
		10,
		20,
		0, // maxReadyBlockCount: unlimited
		20,
		time.Second,
		maxRetry,
		retryInterval,
		1.2,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]testData, error) {
			calls.Add(1)
			return nil, chain.NewTooManyResultsError()
		},
	)
	f := fr.(*fetcher[testData])

	var g sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		g.Wait()
	}()
	startAt := time.Now()
	g.Add(1)
	go func() {
		defer g.Done()
		f.KeepFetch(ctx)
	}()

	getCtx, getCancel := context.WithTimeout(ctx, time.Second*5)
	defer getCancel()
	_, _, _, err := f.Get(getCtx, 11)
	elapsed := time.Since(startAt)
	assert.ErrorContains(t, err, "too many results")
	assert.EqualValues(t, maxRetry+1, calls.Load(), "every attempt repeats the same unshrinkable query")
	assert.GreaterOrEqual(t, elapsed, maxRetry*retryInterval, "each repeat must wait queryRetryInterval")
}
