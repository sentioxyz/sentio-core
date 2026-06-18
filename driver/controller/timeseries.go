package controller

import (
	"context"
	"time"

	"sentioxyz/sentio-core/driver/timeseries"
)

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
