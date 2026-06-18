package fetcher

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timewin"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"

	"github.com/pkg/errors"
)

type Requirement interface {
	Snapshot() any
}

type fetcher[T controller.FetchTarget] struct {
	name      string
	queryFunc func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]T, error)

	requirement         Requirement
	minQuerySize        uint64
	maxQuerySize        uint64
	targetKeepDataSize  int
	targetQueryDataSize int
	maxQueryTime        time.Duration
	maxRetry            int
	queryRetryInterval  time.Duration
	querySizeMultiplier float64

	// protect the data below
	mu sync.Mutex

	full           controller.BlockRange // full is the whole data range
	fetchingStart  uint64                // [fetchingStart,fetchingEnd] is the range where data is fetching
	fetchingEnd    uint64                // so [full.StartBlock, fetchingStart) is the range where data is ready
	fetchingDone   chan struct{}         // if the fetching task has a result, this chan will be closed
	fetchingFailed error                 // has error, the fetching process will be aborted

	brokenErr error

	latest controller.BlockHeader

	data      map[uint64]T // the ready data in [full.StartBlock, fetchingStart) will be here，key is blockNumber
	totalSize int          // is the total size in data，every time data is changed, it will be changed automatically

	// when [full.StartBlock,latest] changed, this chan will be closed to trigger growth,
	// and after growth is completed, this chan will be re-build
	winChanged chan struct{}

	stat *timewin.TimeWindowsManager[*processStat]
}

func NewFetcher[T controller.FetchTarget](
	name string,
	requirement Requirement,
	full controller.BlockRange,
	latest controller.BlockHeader,
	minQuerySize uint64,
	maxQuerySize uint64,
	targetKeepDataSize int,
	targetQueryDataSize int,
	maxQueryTime time.Duration,
	maxRetry int,
	queryRetryInterval time.Duration,
	querySizeMultiplier float64,
	queryFunc func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]T, error),
) controller.Fetcher[T] {
	if querySizeMultiplier < 1 {
		panic("querySizeMultiplier cannot less than 1")
	}
	return &fetcher[T]{
		name:                name,
		requirement:         requirement,
		queryFunc:           queryFunc,
		minQuerySize:        minQuerySize,
		maxQuerySize:        maxQuerySize,
		targetKeepDataSize:  targetKeepDataSize,
		targetQueryDataSize: targetQueryDataSize,
		maxQueryTime:        maxQueryTime,
		maxRetry:            maxRetry,
		queryRetryInterval:  queryRetryInterval,
		querySizeMultiplier: querySizeMultiplier,
		full:                full,
		fetchingStart:       full.StartBlock,
		fetchingEnd:         _min(min(full.StartBlock+minQuerySize-1, latest.GetBlockNumber()), full.EndBlock),
		fetchingDone:        make(chan struct{}),
		latest:              latest,
		data:                make(map[uint64]T),
		winChanged:          make(chan struct{}),
		stat:                timewin.NewTimeWindowsManager[*processStat](time.Minute),
	}
}

func (f *fetcher[T]) GetName() string {
	return f.name
}

func (f *fetcher[T]) Snapshot() any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return map[string]any{
		"name": f.name,
		"type": fmt.Sprintf("%T", f),
		"config": map[string]any{
			"requirement":         f.requirement.Snapshot(),
			"minQuerySize":        f.minQuerySize,
			"maxQuerySize":        f.maxQuerySize,
			"targetKeepDataSize":  f.targetKeepDataSize,
			"targetQueryDataSize": f.targetQueryDataSize,
			"maxQueryTime":        f.maxQueryTime.String(),
			"maxRetry":            f.maxRetry,
			"queryRetryInterval":  f.queryRetryInterval.String(),
			"querySizeMultiplier": f.querySizeMultiplier,
		},
		"fullRange": f.full.String(),
		"readyRange": controller.BlockRange{
			StartBlock: f.full.StartBlock,
			EndBlock:   utils.WrapPointer(f.fetchingStart - 1),
		}.String(),
		"fetchingRange": controller.BlockRange{
			StartBlock: f.fetchingStart,
			EndBlock:   utils.WrapPointer(f.fetchingEnd),
		}.String(),
		"fetchingFailed": f.fetchingFailed,
		"brokenErr":      fmt.Sprintf("%+v", f.brokenErr),
		"latest":         controller.GetBlockFullText(f.latest),
		"dataSize":       f.totalSize,
		"dataBlockCount": len(f.data),
		"statistics":     f.stat.Snapshot(),
	}
}

func (f *fetcher[T]) GetFullRange() controller.BlockRange {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.full
}

func (f *fetcher[T]) nextFetchSize(current uint64, got int, used time.Duration) uint64 {
	if f.targetQueryDataSize > 0 && int(float64(got)*f.querySizeMultiplier) >= f.targetQueryDataSize {
		return current // data got is big enough
	}
	if f.maxQueryTime > 0 && time.Duration(float64(used)*f.querySizeMultiplier) > f.maxQueryTime {
		return current // time used is big enough
	}
	// increase the fetch size, use ceil to make sure newFetchSize not always equal to oldFetchSize
	next := uint64(math.Ceil(float64(current) * f.querySizeMultiplier))
	return min(max(next, f.minQuerySize), f.maxQuerySize)
}

func (f *fetcher[T]) growth(ctx context.Context) (pause bool, reject bool, changed chan struct{}) {
	growthStartAt := time.Now()
	_, logger := log.FromContext(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fetchingFailed != nil {
		// already failed
		return true, true, nil
	}
	if f.full.EndBlock != nil && *f.full.EndBlock < f.fetchingStart {
		// no more data need to fetch
		return true, true, nil
	}
	if f.totalSize >= f.targetKeepDataSize {
		// total size of f.data is more than the limit, continue pause to wait consume
		f.winChanged = make(chan struct{})
		return true, false, f.winChanged
	}
	start, end, latest := f.fetchingStart, f.fetchingEnd, f.latest
	if end < start {
		// last growth reached the right side
		if start <= latest.GetBlockNumber() {
			// latest may be increased, so now we can extend fetching range
			end = _min(min(start+f.minQuerySize-1, latest.GetBlockNumber()), f.full.EndBlock)
		} else {
			// latest not increased, just rebuild f.winChanged
			f.winChanged = make(chan struct{})
			return true, false, f.winChanged
		}
	}
	done := f.fetchingDone
	f.mu.Unlock()

	var result map[uint64]T
	var err error
	for retry := f.maxRetry; retry >= 0; {
		if f.queryRetryInterval > 0 && err != nil {
			select {
			case <-ctx.Done():
				f.mu.Lock()
				return true, true, nil
			case <-time.After(f.queryRetryInterval):
			}
		}
		startAt := time.Now()
		if f.maxQueryTime > 0 {
			queryCtx, cancel := context.WithTimeout(ctx, f.maxQueryTime)
			result, err = f.queryFunc(queryCtx, start, end, latest)
			cancel()
		} else {
			result, err = f.queryFunc(ctx, start, end, latest)
		}
		used := time.Since(startAt)
		var size int
		if err == nil {
			size = sumSize(result)
		}
		st := &processStat{startAt: startAt}
		st.fetchComplete(used, err == nil, end-start+1, size)
		f.stat.Append(st)
		tryLogger := logger.With(
			"start", start,
			"end", end,
			"latest", controller.GetBlockSummary(latest),
			"used", used.String(),
			"retry", retry)
		if err != nil {
			var pe *PermanentError
			if errors.As(err, &pe) {
				err = pe.Err
				retry = -1
			}
			if errors.Is(err, context.Canceled) {
				tryLogger.Debug("fetch canceled")
				f.mu.Lock()
				return true, true, nil
			}
			tryLogger.Warnfe(err, "fetch failed")
			if fetchSize := end - start + 1; fetchSize > f.minQuerySize {
				fetchSize = max(f.minQuerySize, fetchSize/2)
				end = start + fetchSize - 1
			} else {
				retry--
			}
		} else {
			// got the data in [start,end]
			tryLogger.Debugw("fetch succeed", "size", size)
			f.mu.Lock()
			f.totalSize += size
			f.data = utils.MergeMap(f.data, result)
			f.fetchingStart = end + 1
			newFetchSize := f.nextFetchSize(end-start+1, size, used)
			f.fetchingEnd = _min(min(f.fetchingStart+newFetchSize-1, f.latest.GetBlockNumber()), f.full.EndBlock)
			f.fetchingDone = make(chan struct{})
			close(done)
			f.winChanged = make(chan struct{})
			logger.Debugw("growth succeed", "used", time.Since(growthStartAt).String(), "current", f.current())
			return f.totalSize >= f.targetKeepDataSize, false, f.winChanged
		}
	}
	// fetch the data in [start,end], keep reducing the range size, and keep retrying,
	// but it still fails and can no longer continue
	f.mu.Lock()
	f.fetchingEnd = end
	f.fetchingFailed = err
	close(done)
	logger.With("current", f.current()).Errore(err, "growth failed")
	return true, true, nil
}

func (f *fetcher[T]) current() string {
	var ready string
	if f.fetchingStart <= f.full.StartBlock {
		ready = "[empty]"
	} else {
		ready = fmt.Sprintf("[%d,%d]", f.full.StartBlock, f.fetchingStart-1)
	}
	return fmt.Sprintf("FULL%s,READY%s,LATEST:%d,DATA:%d/%d",
		f.full.String(), ready, f.latest.GetBlockNumber(), len(f.data), f.totalSize)
}

func (f *fetcher[T]) KeepFetch(ctx context.Context) {
	_, logger := log.FromContext(ctx, "fetcher", f.name, "requirement", f.requirement)
	logger.Info("keep fetch start")
	defer logger.Info("keep fetch end")
	for round := 0; ; round++ {
		roundCtx, _ := log.FromContext(ctx, "fetcher", f.name, "requirement", f.requirement, "fetchRound", round)
		if pause, reject, changed := f.growth(roundCtx); !pause && !reject {
			continue
		} else if reject {
			return
		} else {
			select {
			case <-changed:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (f *fetcher[T]) Get(ctx context.Context, blockNumber uint64) (
	data T,
	has bool,
	latest controller.BlockHeader,
	err error,
) {
	startAt := time.Now()
	defer func() {
		st := &processStat{startAt: time.Now()}
		st.getComplete(time.Since(startAt), has)
		f.stat.Append(st)
	}()
	f.mu.Lock()
	latest = f.latest
	if !f.full.Contains(blockNumber) {
		f.mu.Unlock()
		return
	}
	for {
		latest = f.latest
		if f.brokenErr != nil {
			// broken, just return the broken error
			err = f.brokenErr
			f.mu.Unlock()
			return
		} else if f.fetchingStart <= blockNumber {
			// the required data is not yet ready
			if f.fetchingFailed != nil {
				// the fetch has failed and cannot continue, an error is returned directly
				err = f.fetchingFailed
				f.mu.Unlock()
				return
			}
			done := f.fetchingDone
			f.mu.Unlock()

			// waiting fetch completed
			select {
			case <-done:
			case <-ctx.Done():
				err = ctx.Err()
				return
			}
			f.mu.Lock()
		} else {
			// the needed data is ready，we can return data now
			data, has = f.data[blockNumber]
			f.mu.Unlock()
			return
		}
	}
}

func (f *fetcher[T]) UpdateLatest(latest controller.BlockHeader) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if latest.GetBlockNumber() < f.latest.GetBlockNumber() {
		panic(errors.Errorf("try update latest from %d back to %d", f.latest.GetBlockNumber(), latest.GetBlockNumber()))
	}
	f.latest = latest
	utils.TryCloseChan(f.winChanged)
}

func (f *fetcher[T]) Broken(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.brokenErr = err
	utils.TryCloseChan(f.winChanged)
}

func (f *fetcher[T]) MoveStart(start uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if start < f.full.StartBlock {
		return
	}
	if start > f.fetchingStart {
		start = f.fetchingStart
	}
	for n := f.full.StartBlock; n < start; n++ {
		if item, has := f.data[n]; has {
			f.totalSize -= item.Size()
			delete(f.data, n)
		}
	}
	f.full.StartBlock = start
	utils.TryCloseChan(f.winChanged)
}
