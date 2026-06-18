package evm

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/core/types"

	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

type DataRequirement struct {
	Log      []LogRequirement
	Trace    []TraceRequirement
	Interval []data.IntervalRequirement
	Exact    []uint64
}

type BlockMainData struct {
	Logs      []types.Log
	Traces    []Trace
	Intervals []data.IntervalConfig
	Exact     bool
}

func (d BlockMainData) IsEmpty() bool {
	return d.Size() == 0 && !d.Exact
}

func (d BlockMainData) Size() int {
	return len(d.Logs) + len(d.Traces) + len(d.Intervals)
}

func BuildIntervalFetcher(
	name string,
	req data.IntervalRequirement,
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client Client,
) controller.Fetcher[BlockMainData] {
	timeGetter := func(ctx context.Context, blockNumber uint64) (time.Time, error) {
		getCtx, cancel := context.WithTimeout(ctx, time.Second*3)
		defer cancel()
		h, err := client.GetHeader(getCtx, blockNumber)
		if err != nil {
			return time.Time{}, err
		}
		return h.GetBlockTime(), nil
	}
	return fetcher.NewFetcher[BlockMainData](
		name,
		req,
		controller.BlockRange{
			StartBlock: max(currentBlockNumber, req.StartBlock),
			EndBlock:   req.EndBlock,
		},
		latest,
		10000,
		10000,
		100,
		1000,
		time.Minute,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			bns, err := data.QueryInterval(ctx, start, end, firstBlockNumber, latest, req, timeGetter)
			if err != nil {
				return nil, err
			}
			result := make(map[uint64]BlockMainData)
			for _, bn := range bns {
				result[bn] = BlockMainData{
					Intervals: []data.IntervalConfig{req.IntervalConfig},
				}
			}
			return result, nil
		},
	)
}

func BuildBlockMainDataFetcher(
	namePrefix string,
	req DataRequirement,
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client Client,
) controller.Fetcher[BlockMainData] {
	req.Log = MergeLogRequirements(currentBlockNumber, req.Log)
	req.Trace = MergeTraceRequirement(currentBlockNumber, req.Trace)
	req.Interval = data.MergeIntervalRequirements(req.Interval)
	exact := set.New(req.Exact...)
	var fetchers []controller.Fetcher[BlockMainData]
	for i, r := range req.Log {
		fetchers = append(fetchers, BuildLogFetcher(
			namePrefix+fmt.Sprintf("LogFetcher#%d", i), r, currentBlockNumber, latest, client))
	}
	for i, r := range req.Trace {
		fetchers = append(fetchers, BuildTraceFetcher(
			namePrefix+fmt.Sprintf("TraceFetcher#%d", i), r, currentBlockNumber, latest, client))
	}
	for i, r := range req.Interval {
		fetchers = append(fetchers, BuildIntervalFetcher(
			namePrefix+fmt.Sprintf("IntervalFetcher#%d", i), r, firstBlockNumber, currentBlockNumber, latest, client))
	}
	return fetcher.MergeIsomorphicFetchers(
		namePrefix+"MainDataFetcher",
		req,
		fetchers,
		func(bn uint64, from []BlockMainData) (data BlockMainData, has bool, _ error) {
			data.Exact = exact.Contains(bn)
			has = len(from) > 0 || data.Exact
			// Logs and traces will never be repeated, because a range will only have one fetcher with data.
			for _, box := range from {
				data.Logs = append(data.Logs, box.Logs...)
				data.Traces = append(data.Traces, box.Traces...)
				data.Intervals = append(data.Intervals, box.Intervals...)
			}
			return
		})
}
