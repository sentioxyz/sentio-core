package fuel

import (
	"context"
	"time"

	"sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

type TransactionRequirement struct {
	controller.BlockRange

	Filters []fuel.TransactionFilter
}

func (r TransactionRequirement) Snapshot() any {
	return map[string]any{
		"filters": r.Filters,
		"range":   r.BlockRange.String(),
	}
}

// MergeTxRequirements it can be guaranteed that all the item ranges of the result must be disjoint,
// and each range has at most one filter
func MergeTxRequirements(current uint64, reqs []TransactionRequirement) (result []TransactionRequirement) {
	rs := controller.CutRangeSet(
		current,
		utils.MapSliceNoError(reqs, func(r TransactionRequirement) controller.BlockRange {
			return r.BlockRange
		}),
	)
	for _, r := range rs {
		var filters []fuel.TransactionFilter
		for _, req := range reqs {
			if req.BlockRange.Include(r) {
				filters = append(filters, req.Filters...)
			}
		}
		if len(filters) == 0 {
			continue
		}
		result = append(result, TransactionRequirement{
			Filters:    filters,
			BlockRange: r,
		})
	}
	return result
}

func BuildTxFetcher(
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
		1,
		100,
		100000,
		1000, // the target is that each query got no more than 1000 transactions
		time.Second*10,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			txs, err := client.GetTransactions(ctx, fuel.GetTransactionsParam{
				StartHeight: start,
				EndHeight:   end,
				Filters:     req.Filters,
			})
			if err != nil {
				return nil, err
			}
			result := make(map[uint64]BlockMainData)
			for _, tx := range txs {
				bd, _ := result[tx.BlockHeight]
				bd.Txs = append(bd.Txs, tx)
				result[tx.BlockHeight] = bd
			}
			return result, nil
		},
	)
}
