package sui

import (
	"context"
	"time"

	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

type ObjectChangeRequirement struct {
	controller.BlockRange

	Filter sui.ObjectChangeFilter
}

func (r ObjectChangeRequirement) Snapshot() any {
	return map[string]any{
		"filter": r.Filter,
		"range":  r.BlockRange.String(),
	}
}

func MergeObjectChangeRequirements(current uint64, reqs []ObjectChangeRequirement) (result []ObjectChangeRequirement) {
	rs := controller.CutRangeSet(
		current,
		utils.MapSliceNoError(reqs, func(r ObjectChangeRequirement) controller.BlockRange {
			return r.BlockRange
		}),
	)
	for _, r := range rs {
		var filters []sui.ObjectChangeFilter
		for _, req := range reqs {
			if req.BlockRange.Include(r) {
				filters = append(filters, req.Filter)
			}
		}
		if len(filters) == 0 {
			continue
		}
		result = append(result, ObjectChangeRequirement{
			Filter:     utils.Reduce(filters, sui.ObjectChangeFilter.Merge),
			BlockRange: r,
		})
	}
	return result
}

func BuildObjectChangeFetcher(
	name string,
	req ObjectChangeRequirement,
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
		1000,
		10000,
		100000,
		10000, // the target is that each query got no more than 1000 object change records
		time.Minute,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			changes, err := client.GetObjectChanges(ctx, start, end, req.Filter)
			if err != nil {
				return nil, err
			}
			result := make(map[uint64]BlockMainData)
			for bn, cs := range changes {
				result[bn] = BlockMainData{ObjectChanges: cs}
			}
			if err := attachSimpleBlocks(ctx, client, result); err != nil {
				return nil, err
			}
			return result, nil
		},
	)
}
