package startup

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/timeseries"
)

type timeSeriesController struct {
	chainID string
	store   timeseries.Store

	mu        sync.Mutex
	cached    map[uint64]map[uint64][]timeseries.Dataset // map[<blockNumber>][<taskIndex>]
	committed *uint64
}

func newTimeSeriesController(chainID string, store timeseries.Store) *timeSeriesController {
	return &timeSeriesController{
		chainID: chainID,
		store:   store,
		cached:  make(map[uint64]map[uint64][]timeseries.Dataset),
	}
}

func (c *timeSeriesController) Reset(ctx context.Context, checkpoint *controller.Checkpoint) *controller.ExternalError {
	c.mu.Lock()
	defer c.mu.Unlock()
	if checkpoint == nil {
		c.cached = make(map[uint64]map[uint64][]timeseries.Dataset)
	} else {
		utils.MapDelete(c.cached, func(bn uint64) bool {
			return bn > checkpoint.BlockNumber
		})
	}
	var blockNumber int64 = -1
	if checkpoint != nil {
		blockNumber = int64(checkpoint.BlockNumber)
	}
	if err := c.store.DeleteData(ctx, c.chainID, blockNumber); err != nil {
		return controller.NewExternalError(controller.ErrCodeCleanTimeSeriesDataFailed, err)
	}
	return nil
}

var maxUncommitedTimeSeries = envconf.LoadUInt64("SENTIO_MAX_UNCOMMITED_TIME_SERIES_DATA", 1000000,
	envconf.WithMin(10000), envconf.WithMax(1000000))

func (c *timeSeriesController) getCachedSize(blockNumber uint64) (total uint64) {
	for bn, blockData := range c.cached {
		if bn > blockNumber {
			continue
		}
		for _, dss := range blockData {
			for _, ds := range dss {
				total += uint64(len(ds.Rows))
			}
		}
	}
	return total
}

func (c *timeSeriesController) CachedTooMuch(blockNumber uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getCachedSize(blockNumber) > maxUncommitedTimeSeries
}

func (c *timeSeriesController) Commit(
	ctx context.Context,
	blockNumber uint64,
	blockTime time.Time,
) (stat map[timeseries.MetaType]map[string]int, extErr *controller.ExternalError) {
	// collect data to commit
	var data []timeseries.Dataset
	c.mu.Lock()
	for _, bn := range utils.GetOrderedMapKeys(c.cached) {
		if bn <= blockNumber {
			data = append(data, utils.MergeArr(utils.GetMapValuesOrderByKey(c.cached[bn])...)...)
		}
	}
	c.mu.Unlock()

	// actually save dataset
	if err := c.store.AppendData(ctx, data, c.chainID, blockTime); err != nil {
		if errors.Is(err, timeseries.ErrInvalidMetaDiff) {
			return nil, controller.NewExternalError(controller.ErrCodeTimeSeriesDataSchemaChanged, err)
		}
		if errors.Is(err, timeseries.ErrInvalidMeta) {
			return nil, controller.NewExternalError(controller.ErrCodeInvalidTimeSeriesData, err)
		}
		return nil, controller.NewExternalError(controller.ErrCodeSaveTimeSeriesDataFailed,
			errors.Wrapf(err, "failed to commit timeseries data: %s", timeseries.GetDatasetsSummary(data)))
	}
	stat = make(map[timeseries.MetaType]map[string]int)
	for _, ds := range data {
		utils.IncrK2Map(stat, ds.Type, ds.Name, len(ds.Rows))
	}

	// save succeed, clean c.cached
	c.mu.Lock()
	defer c.mu.Unlock()
	utils.MapDelete(c.cached, func(bn uint64) bool {
		return bn <= blockNumber
	})
	c.committed = &blockNumber
	return
}

func (c *timeSeriesController) Insert(blockNumber uint64, taskIndex controller.TaskIndex, data []timeseries.Dataset) {
	c.mu.Lock()
	defer c.mu.Unlock()
	org, _ := utils.GetFromK2Map(c.cached, blockNumber, taskIndex.Global)
	utils.PutIntoK2Map(c.cached, blockNumber, taskIndex.Global, append(org, data...))
}

func (c *timeSeriesController) Snapshot() any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return map[string]any{
		"committed":       c.committed,
		"uncommitedTotal": c.getCachedSize(math.MaxUint64),
		"uncommited": cacheSnapshot(c.cached, func(dss []timeseries.Dataset) (s int) {
			for _, ds := range dss {
				s += len(ds.Rows)
			}
			return s
		}),
	}
}
