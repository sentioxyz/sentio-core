package grpc

import (
	"context"
	"time"

	"sentioxyz/sentio-core/driver/controller"
	suidata "sentioxyz/sentio-core/driver/controller/data/sui"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

func BuildObjectChangeFetcher(
	name string,
	req suidata.ObjectChangeRequirement,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client suidata.Client,
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
		10000,
		time.Minute,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			changes, err := client.GetGrpcObjectChanges(ctx, start, end, req.Filter)
			if err != nil {
				return nil, err
			}
			result := make(map[uint64]BlockMainData)
			for bn, cs := range changes {
				if len(cs) > 0 {
					// each grpc object change already carries its checkpoint header — no extra RPC needed
					result[bn] = BlockMainData{
						ObjectChanges: cs,
						SimpleBlock:   new(suidata.SimpleBlock(cs[0].GetSimpleCheckpoint())),
					}
				}
			}
			return result, nil
		},
	)
}
