package fetcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/timewin"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
)

func MergeIsomorphicFetchers[F controller.FetchTarget, T controller.FetchTarget](
	name string,
	config any,
	fetchers []controller.Fetcher[F],
	mergeFunc func(blockNumber uint64, from []F) (data T, has bool, err error),
) controller.Fetcher[T] {
	fetchers = utils.FilterArr(fetchers, func(f controller.Fetcher[F]) bool {
		return !f.GetFullRange().IsEmpty()
	})
	return &mergedIsomorphicFetchers[F, T]{
		name:      name,
		config:    config,
		fetchers:  fetchers,
		mergeFunc: mergeFunc,
		stat:      timewin.NewTimeWindowsManager[*processStat](time.Minute),
	}
}

type mergedIsomorphicFetchers[F controller.FetchTarget, T controller.FetchTarget] struct {
	name      string
	config    any
	fetchers  []controller.Fetcher[F]
	mergeFunc func(blockNumber uint64, from []F) (data T, has bool, err error)
	stat      *timewin.TimeWindowsManager[*processStat]
}

func (f *mergedIsomorphicFetchers[F, T]) GetName() string {
	return f.name
}

func (f *mergedIsomorphicFetchers[F, T]) GetFullRange() controller.BlockRange {
	full := controller.EmptyBlockRange
	for _, up := range f.fetchers {
		full = full.Cover(up.GetFullRange())
	}
	return full
}

func (f *mergedIsomorphicFetchers[F, T]) Snapshot() any {
	upstreams := make([]any, len(f.fetchers))
	for i, up := range f.fetchers {
		upstreams[i] = up.Snapshot()
	}
	return map[string]any{
		"name":       f.name,
		"config":     f.config,
		"type":       fmt.Sprintf("%T", f),
		"upstreams":  upstreams,
		"statistics": f.stat.Snapshot(),
	}
}

func (f *mergedIsomorphicFetchers[F, T]) KeepFetch(ctx context.Context) {
	var g sync.WaitGroup
	g.Add(len(f.fetchers))
	for _, up := range f.fetchers {
		go func(up controller.Fetcher[F]) {
			defer g.Done()
			up.KeepFetch(ctx)
		}(up)
	}
	g.Wait()
}

func (f *mergedIsomorphicFetchers[F, T]) Get(ctx context.Context, blockNumber uint64) (
	data T,
	has bool,
	latest controller.BlockHeader,
	err error,
) {
	startAt := time.Now()
	var from []F
	for _, up := range f.fetchers {
		var d F
		d, has, latest, err = up.Get(ctx, blockNumber)
		if err != nil {
			return
		}
		if has {
			from = append(from, d)
		}
	}
	mergeStartAt := time.Now()
	data, has, err = f.mergeFunc(blockNumber, from)
	var dataSize int
	if err == nil && has {
		dataSize = data.Size()
	}
	st := &processStat{startAt: time.Now()}
	st.fetchComplete(time.Since(mergeStartAt), err == nil, 1, dataSize)
	st.getComplete(time.Since(startAt), has)
	f.stat.Append(st)
	return
}

func (f *mergedIsomorphicFetchers[F, T]) Broken(err error) {
	for _, up := range f.fetchers {
		up.Broken(err)
	}
}

func (f *mergedIsomorphicFetchers[F, T]) UpdateLatest(latest controller.BlockHeader) {
	for _, up := range f.fetchers {
		up.UpdateLatest(latest)
	}
}

func (f *mergedIsomorphicFetchers[F, T]) MoveStart(start uint64) {
	for _, up := range f.fetchers {
		up.MoveStart(start)
	}
}
