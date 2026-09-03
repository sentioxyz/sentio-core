package controller

import (
	"context"
	"time"

	"sentioxyz/sentio-core/driver/timeseries"
)

// TimeSeriesController buffers time series data per block and commits them to the underlying store.
//
// Implementations must be safe for concurrent use: Insert may be called concurrently with Commit,
// CachedTooMuch, Snapshot and other Inserts without any external locking (checkpointController
// calls Insert without holding its own mutex). Callers guarantee that Insert is only invoked for
// block numbers greater than the block number of any Commit that has started, so implementations
// do not need to handle inserts at or below the committing block number.
type TimeSeriesController interface {
	Reset(ctx context.Context, checkpoint *Checkpoint) *ExternalError
	CachedTooMuch(blockNumber uint64) bool
	Commit(
		ctx context.Context,
		blockNumber uint64,
		blockTime time.Time,
	) (stat map[timeseries.MetaType]map[string]int, err *ExternalError)

	Insert(blockNumber uint64, taskIndex TaskIndex, data []timeseries.Dataset)

	Snapshot() any
}

type EmptyTimeSeriesController struct{}

func (c EmptyTimeSeriesController) Reset(ctx context.Context, checkpoint *Checkpoint) *ExternalError {
	return nil
}

func (c EmptyTimeSeriesController) CachedTooMuch(blockNumber uint64) bool {
	return false
}

func (c EmptyTimeSeriesController) Commit(
	ctx context.Context,
	blockNumber uint64,
	blockTime time.Time,
) (map[timeseries.MetaType]map[string]int, *ExternalError) {
	return nil, nil
}

func (c EmptyTimeSeriesController) Insert(blockNumber uint64, taskIndex TaskIndex, data []timeseries.Dataset) {
	if len(data) > 0 {
		panic("do not support time series data")
	}
}

func (c EmptyTimeSeriesController) Snapshot() any {
	return nil
}
