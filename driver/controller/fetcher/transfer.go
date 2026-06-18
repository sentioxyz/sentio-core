package fetcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timewin"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"

	"github.com/pkg/errors"
)

func TransferFetcher[F controller.FetchTarget, T controller.FetchTarget](
	name string,
	upstream controller.Fetcher[F],
	latest controller.BlockHeader,
	maxConcurrency uint64,
	maxKeepDataSize int,
	maxKeepDataNum int,
	transferTimeout time.Duration,
	maxRetry int,
	retryInterval time.Duration,
	transferFunc func(ctx context.Context, blockNumber uint64, from F) (T, bool, error),
) controller.Fetcher[T] {
	full := upstream.GetFullRange()
	return &transferFetcher[F, T]{
		name:            name,
		transferFunc:    transferFunc,
		maxConcurrency:  maxConcurrency,
		maxKeepDataSize: maxKeepDataSize,
		maxKeepDataNum:  maxKeepDataNum,
		transferTimeout: transferTimeout,
		maxRetry:        maxRetry,
		retryInterval:   retryInterval,
		upstream:        upstream,

		full:          full,
		readyEnd:      full.StartBlock,
		fetchStart:    full.StartBlock,
		fetched:       make(map[uint64]struct{}),
		readyEndMoved: make(chan struct{}),
		latest:        latest,
		data:          make(map[uint64]T),
		tryGrowth:     make(chan struct{}),
		broken:        make(chan struct{}),
		stat:          timewin.NewTimeWindowsManager[*processStat](time.Minute),
	}
}

type transferFetcher[F controller.FetchTarget, T controller.FetchTarget] struct {
	name         string
	transferFunc func(ctx context.Context, blockNumber uint64, from F) (T, bool, error)

	maxConcurrency  uint64
	maxKeepDataSize int
	maxKeepDataNum  int
	transferTimeout time.Duration
	maxRetry        int
	retryInterval   time.Duration

	upstream controller.Fetcher[F]

	// protect the data below
	mu sync.Mutex
	g  sync.WaitGroup

	full          controller.BlockRange // full is the whole data range
	readyEnd      uint64                // [full.StartBlock,readyEnd) is the range where data is ready
	fetchStart    uint64                // [readyEnd,fetchStart) is the range where data is fetching
	fetched       map[uint64]struct{}
	readyEndMoved chan struct{}
	fetchFailed   error
	fetchFailedAt uint64

	latest controller.BlockHeader

	data      map[uint64]T // the ready data in [full.StartBlock, fetchingStart) will be here，key is blockNumber
	totalSize int          // is the total size in data，every time data is changed, it will be changed automatically

	// when [full.StartBlock,readyEnd) changed or latest changed, this chan will be closed to trigger growth,
	// and after growth is completed, this chan will be re-build
	tryGrowth chan struct{}

	// will be closed when Broken called
	broken    chan struct{}
	brokenErr error

	stat *timewin.TimeWindowsManager[*processStat]
}

func (f *transferFetcher[F, T]) GetName() string {
	return f.name
}

func (f *transferFetcher[F, T]) GetFullRange() controller.BlockRange {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.full
}

func (f *transferFetcher[F, T]) Snapshot() any {
	f.mu.Lock()
	defer f.mu.Unlock()

	return map[string]any{
		"name": f.name,
		"type": fmt.Sprintf("%T", f),
		"config": map[string]any{
			"maxConcurrency":     f.maxConcurrency,
			"targetKeepDataSize": f.maxKeepDataSize,
			"targetKeepDataNum":  f.maxKeepDataNum,
			"transferTimeout":    f.transferTimeout.String(),
			"maxRetry":           f.maxRetry,
			"retryInterval":      f.retryInterval.String(),
		},
		"fullRange": f.full.String(),
		"readyRange": controller.BlockRange{
			StartBlock: f.full.StartBlock,
			EndBlock:   utils.WrapPointer(f.readyEnd - 1),
		}.String(),
		"fetchingRange": controller.BlockRange{
			StartBlock: f.readyEnd,
			EndBlock:   utils.WrapPointer(f.fetchStart - 1),
		}.String(),
		"fetchFailed":   f.fetchFailed,
		"fetchFailedAt": f.fetchFailedAt,
		"dataSize":      f.totalSize,
		// dataBlockCount only counts ready blocks that carried data; empty blocks advance readyEnd
		// without entering f.data, so fetchedAhead (the ready-but-unconsumed prefetch depth) can be larger.
		"dataBlockCount": len(f.data),
		// fetchedAhead is the ready prefetch depth: blocks already prepared and waiting to be consumed.
		// (MoveStart clamps full.StartBlock to readyEnd, so this never underflows.)
		"fetchedAhead": f.readyEnd - f.full.StartBlock,
		// fetchedUnreadyCount is the number of out-of-order-completed blocks held back because an
		// earlier block in [readyEnd, fetchStart) is not done yet. Always 0 when maxConcurrency==1.
		"fetchedUnreadyCount": len(f.fetched),
		"upstream":            f.upstream.Snapshot(),
		"statistics":          f.stat.Snapshot(),
	}
}

// current is used as log argument, if not print debug log, current.String will not be called
type current struct {
	full          controller.BlockRange
	readyEnd      uint64
	fetchStart    uint64
	dataBlockNum  int
	dataTotalSize int
}

func (c current) String() string {
	return fmt.Sprintf("FULL%s,READY[%d,%d),FETCHING[%d,%d),DATA:%d/%d",
		c.full.String(), c.full.StartBlock, c.readyEnd, c.readyEnd, c.fetchStart, c.dataBlockNum, c.dataTotalSize)
}

func (f *transferFetcher[F, T]) current() current {
	return current{
		full:          f.full,
		readyEnd:      f.readyEnd,
		fetchStart:    f.fetchStart,
		dataBlockNum:  len(f.data),
		dataTotalSize: f.totalSize,
	}
}

func (f *transferFetcher[F, T]) setFetchFailed(blockNumber uint64, err error) {
	if f.fetchFailed == nil || f.fetchFailedAt > blockNumber {
		f.fetchFailed = err
		f.fetchFailedAt = blockNumber
	}
}

func (f *transferFetcher[F, T]) transfer(ctx context.Context, blockNumber uint64) {
	_, logger := log.FromContext(ctx)
	from, has, _, err := f.upstream.Get(ctx, blockNumber)
	var result T
	var stage string
	if err != nil {
		stage = "fetch from upstream"
	} else if has {
		for retry := f.maxRetry; retry >= 0; {
			if f.retryInterval > 0 && err != nil {
				select {
				case <-ctx.Done():
					return
				case <-time.After(f.retryInterval):
				}
			}
			startAt := time.Now()
			if f.transferTimeout > 0 {
				transferCtx, cancel := context.WithTimeout(ctx, f.transferTimeout)
				result, has, err = f.transferFunc(transferCtx, blockNumber, from)
				cancel()
			} else {
				result, has, err = f.transferFunc(ctx, blockNumber, from)
			}
			used := time.Since(startAt)
			var dataSize int
			if err == nil && has {
				dataSize = result.Size()
			}
			st := &processStat{startAt: time.Now()}
			st.fetchComplete(used, err == nil, 1, dataSize)
			f.stat.Append(st)
			if err == nil {
				break
			}
			var pe *PermanentError
			if errors.As(err, &pe) {
				err = pe.Err
				retry = -1
			}
			tryLogger := logger.With("blockNumber", blockNumber, "used", used.String(), "retry", retry)
			if errors.Is(err, context.Canceled) {
				tryLogger.Debug("fetch canceled")
				return
			}
			tryLogger.Warnfe(err, "fetch failed")
			retry--
		}
		if err != nil {
			stage = "transfer"
		}
	}

	f.mu.Lock()
	f.fetched[blockNumber] = struct{}{}
	if err != nil {
		f.setFetchFailed(blockNumber, err)
		if errors.Is(err, context.Canceled) {
			logger.With("blockNumber", blockNumber).Warnfe(err, "fetch failed because %s canceled", stage)
		} else {
			logger.With("blockNumber", blockNumber).Errorfe(err, "fetch failed because %s failed", stage)
		}
	} else if has {
		f.data[blockNumber] = result
		f.totalSize += result.Size()
		logger.Debugw("fetch succeed", "blockNumber", blockNumber)
	} else {
		logger.Debugw("fetch succeed and no data", "blockNumber", blockNumber)
	}
	// move f.readyEnd
	var readyEnd uint64
	for readyEnd = f.readyEnd; readyEnd < f.fetchStart; readyEnd++ {
		if _, ready := f.fetched[readyEnd]; ready {
			delete(f.fetched, readyEnd)
		} else {
			break
		}
	}
	if f.readyEnd < readyEnd {
		f.readyEnd = readyEnd
		logger.Debugw("growth succeed", "current", f.current())
		utils.TryCloseChan(f.tryGrowth)
		utils.TryCloseChan(f.readyEndMoved)
		f.readyEndMoved = make(chan struct{})
	}
	f.mu.Unlock()
}

func (f *transferFetcher[F, T]) growth(ctx context.Context) (reject bool, changed chan struct{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fetchFailed != nil {
		// already failed
		return true, nil
	}
	if f.full.EndBlock != nil && *f.full.EndBlock < f.fetchStart {
		// no more data need to fetch
		return true, nil
	}
	if f.totalSize < f.maxKeepDataSize && len(f.data) < f.maxKeepDataNum {
		// fetch more
		maxFetchBlock := min(f.readyEnd+f.maxConcurrency-1, f.latest.GetBlockNumber())
		if f.full.EndBlock != nil {
			maxFetchBlock = min(maxFetchBlock, *f.full.EndBlock)
		}
		for ; f.fetchStart <= maxFetchBlock; f.fetchStart++ {
			f.g.Add(1)
			go func(bn uint64) {
				f.transfer(ctx, bn)
				f.g.Done()
			}(f.fetchStart)
		}
	}
	f.tryGrowth = make(chan struct{})
	return false, f.tryGrowth
}

func (f *transferFetcher[F, T]) keepFetch(ctx context.Context) {
	_, logger := log.FromContext(ctx, "fetcher", f.name)
	logger.Info("keep fetch start")
	defer logger.Info("keep fetch end")
	defer func() {
		f.g.Wait()
	}()
	for round := 0; ; round++ {
		roundCtx, _ := log.FromContext(ctx, "fetcher", f.name, "round", round)
		if reject, changed := f.growth(roundCtx); reject {
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

func (f *transferFetcher[F, T]) KeepFetch(ctx context.Context) {
	var g sync.WaitGroup
	g.Add(2)
	go func() {
		defer g.Done()
		f.upstream.KeepFetch(ctx)
	}()
	go func() {
		defer g.Done()
		f.keepFetch(ctx)
	}()
	g.Wait()
}

func (f *transferFetcher[F, T]) Get(ctx context.Context, blockNumber uint64) (
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
	for {
		latest = f.latest
		if !f.full.Contains(blockNumber) {
			// not in full, no data
			f.mu.Unlock()
			return
		}
		if f.brokenErr != nil {
			err = f.brokenErr
			f.mu.Unlock()
			return
		}
		if f.readyEnd <= blockNumber {
			// data not ready, wait readyEnd moved
			readyEndMoved, broken := f.readyEndMoved, f.broken
			f.mu.Unlock()
			select {
			case <-readyEndMoved:
			case <-broken:
			case <-ctx.Done():
				err = ctx.Err()
				return
			}
			f.mu.Lock()
			continue
		}
		// data is ready
		if f.fetchFailed != nil && f.fetchFailedAt <= blockNumber {
			err = f.fetchFailed
		} else {
			data, has = f.data[blockNumber]
		}
		f.mu.Unlock()
		return
	}
}

func (f *transferFetcher[F, T]) UpdateLatest(latest controller.BlockHeader) {
	f.upstream.UpdateLatest(latest)

	f.mu.Lock()
	f.latest = latest
	utils.TryCloseChan(f.tryGrowth)
	f.mu.Unlock()
}

func (f *transferFetcher[F, T]) Broken(err error) {
	f.upstream.Broken(err)

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.brokenErr == nil {
		f.brokenErr = err
		utils.TryCloseChan(f.broken)
	}
}

func (f *transferFetcher[F, T]) MoveStart(start uint64) {
	f.upstream.MoveStart(start)

	f.mu.Lock()
	defer f.mu.Unlock()
	if start < f.full.StartBlock {
		return
	}
	if start > f.readyEnd {
		start = f.readyEnd
	}
	for n := f.full.StartBlock; n < start; n++ {
		if item, has := f.data[n]; has {
			f.totalSize -= item.Size()
			delete(f.data, n)
		}
	}
	f.full.StartBlock = start
	utils.TryCloseChan(f.tryGrowth)
}
