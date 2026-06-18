package sui

import (
	"context"
	"time"

	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

type TransactionRequirement struct {
	controller.BlockRange

	Filter      sui.TransactionFilter
	FetchConfig sui.TransactionFetchConfig
}

func (r TransactionRequirement) Snapshot() any {
	return map[string]any{
		"filter":      r.Filter,
		"fetchConfig": r.FetchConfig,
		"range":       r.BlockRange.String(),
	}
}

func MergeTxnRequirements(current uint64, reqs []TransactionRequirement) (result []TransactionRequirement) {
	rs := controller.CutRangeSet(
		current,
		utils.MapSliceNoError(reqs, func(r TransactionRequirement) controller.BlockRange {
			return r.BlockRange
		}),
	)
	for _, r := range rs {
		rr := TransactionRequirement{BlockRange: r}
		first := true
		for _, req := range reqs {
			if !req.BlockRange.Include(r) {
				continue
			}
			if first {
				rr.Filter = req.Filter
				rr.FetchConfig = req.FetchConfig
				first = false
			} else {
				rr.Filter = rr.Filter.Merge(req.Filter)
				rr.FetchConfig = rr.FetchConfig.Merge(req.FetchConfig)
			}
		}
		if first {
			continue
		}
		result = append(result, rr)
	}
	return result
}

func BuildTxnFetcher(
	name string,
	req TransactionRequirement,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client Client,
) controller.Fetcher[BlockMainData] {
	return fetcher.NewFetcher(
		name,
		req,
		controller.BlockRange{
			StartBlock: max(currentBlockNumber, req.StartBlock),
			EndBlock:   req.EndBlock,
		},
		latest,
		100,
		10000,
		10000, // size of transaction is 10, so will cache 1000 transactions
		5000,  // the target is that each query got no more than 500 transactions
		time.Second*10,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			txGroups, err := client.GetTransactions(ctx, start, end, req.Filter, req.FetchConfig)
			if err != nil {
				return nil, err
			}
			result := make(map[uint64]BlockMainData)
			for bn, txs := range txGroups {
				result[bn] = BlockMainData{Txs: txs}
			}
			if err := attachSimpleBlocks(ctx, client, result); err != nil {
				return nil, err
			}
			return result, nil
		},
	)
}
