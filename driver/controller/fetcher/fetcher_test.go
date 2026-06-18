package fetcher

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/controller"

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
