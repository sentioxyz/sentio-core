package fetcher

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/controller"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func Test_transfer(t *testing.T) {
	log.ManuallySetLevel(zap.DebugLevel)
	log.BindFlag()

	upstream := NewFetcher[testData](
		"testFetcher",
		nil,
		controller.BlockRange{StartBlock: 10},
		newTestBlockHeader(25),
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
	fr := TransferFetcher[testData](
		"transferFetcher",
		upstream,
		newTestBlockHeader(25),
		2,
		5,
		5,
		0,
		10,
		0,
		func(ctx context.Context, blockNumber uint64, from testData) (testData, bool, error) {
			if len(from) == 0 {
				return testData{}, false, nil
			}
			select {
			case <-ctx.Done():
				return testData{}, false, ctx.Err()
			case <-time.After(time.Millisecond * 100):
				//if rand.Int()%2 == 0 {
				//	return testData{}, false, errors.Errorf("transfer failed sadly")
				//}
				return testData{strings.Join(from, ",")}, true, nil
			}
		})
	f := fr.(*transferFetcher[testData, testData])

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

	mergedTestData := func(n uint64) testData {
		src := buildTestData(n)
		if len(src) == 0 {
			return nil
		}
		return testData{strings.Join(src, ",")}
	}

	// 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	// FS                      RE                   LA
	time.Sleep(time.Second)
	assert.Equal(t, uint64(10), f.full.StartBlock)
	assert.Equal(t, uint64(18), f.readyEnd)
	assert.Equal(t, uint64(18), f.fetchStart)
	assert.Equal(t, 6, f.totalSize)
	for n := uint64(10); n < 18; n++ {
		d, has, latest, err := f.Get(ctx, n)
		assert.NoError(t, err)
		assert.Equalf(t, uint64(25), latest.GetBlockNumber(), "n = %d", n)
		assert.Equalf(t, len(buildTestData(n)) > 0, has, "n = %d", n)
		assert.Equalf(t, mergedTestData(n), d, "n = %d", n)
	}

	// 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	//       FS                         RE          LA
	f.MoveStart(12)
	time.Sleep(time.Second)
	assert.Equal(t, uint64(12), f.full.StartBlock)
	assert.Equal(t, uint64(21), f.readyEnd)
	assert.Equal(t, uint64(21), f.fetchStart)
	assert.Equal(t, 6, f.totalSize)
	for n := uint64(12); n < 21; n++ {
		d, has, latest, err := f.Get(ctx, n)
		assert.NoError(t, err)
		assert.Equalf(t, uint64(25), latest.GetBlockNumber(), "n = %d", n)
		assert.Equalf(t, len(buildTestData(n)) > 0, has, "n = %d", n)
		assert.Equalf(t, mergedTestData(n), d, "n = %d", n)
	}

	// 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	//                                  FS          LA RE
	f.MoveStart(21)
	time.Sleep(time.Second)
	assert.Equal(t, uint64(21), f.full.StartBlock)
	assert.Equal(t, uint64(26), f.readyEnd)
	assert.Equal(t, uint64(26), f.fetchStart) // latest is 25, so no block is transferring
	assert.Equal(t, 3, f.totalSize)
	for n := uint64(21); n < 25; n++ {
		d, has, latest, err := f.Get(ctx, n)
		assert.NoError(t, err)
		assert.Equalf(t, uint64(25), latest.GetBlockNumber(), "n = %d", n)
		assert.Equalf(t, len(buildTestData(n)) > 0, has, "n = %d", n)
		assert.Equalf(t, mergedTestData(n), d, "n = %d", n)
	}

	// 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	//                                  FS             LA RE
	f.UpdateLatest(newTestBlockHeader(26))
	time.Sleep(time.Second)
	assert.Equal(t, uint64(21), f.full.StartBlock)
	assert.Equal(t, uint64(27), f.readyEnd)
	assert.Equal(t, uint64(27), f.fetchStart) // latest is 26, so no block is transferring
	assert.Equal(t, 4, f.totalSize)
	for n := uint64(21); n < 26; n++ {
		d, has, latest, err := f.Get(ctx, n)
		assert.NoError(t, err)
		assert.Equalf(t, uint64(26), latest.GetBlockNumber(), "n = %d", n)
		assert.Equalf(t, len(buildTestData(n)) > 0, has, "n = %d", n)
		assert.Equalf(t, mergedTestData(n), d, "n = %d", n)
	}

	// 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45
	//  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0  1  2  0
	//                                                 LA RE
	//                                                    FA
	f.MoveStart(27)
	time.Sleep(time.Second)
	assert.Equal(t, uint64(27), f.full.StartBlock)
	assert.Equal(t, uint64(27), f.readyEnd)
	assert.Equal(t, uint64(27), f.fetchStart)
	assert.Equal(t, 0, f.totalSize)
	brokenErr := errors.New("broken")
	go func() {
		time.Sleep(time.Second)
		f.Broken(brokenErr)
	}()
	{
		d, has, latest, err := f.Get(ctx, 27)
		assert.ErrorIs(t, err, brokenErr)
		assert.Equal(t, uint64(26), latest.GetBlockNumber())
		assert.False(t, has)
		assert.Nil(t, d)
	}

}
