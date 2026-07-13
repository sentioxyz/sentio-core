package grpc

import (
	"context"
	"time"

	"sentioxyz/sentio-core/driver/controller"
	suidata "sentioxyz/sentio-core/driver/controller/data/sui"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

func BuildTxnFetcher(
	name string,
	req suidata.TransactionRequirement,
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
		// minQuerySize 1: the super node errors when a multi-block range exceeds its record cap,
		// so the fetcher must be able to shrink to a single block (where the cap no longer applies).
		1,
		10000,
		10000,
		5000,
		time.Second*10,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			txGroups, err := client.GetGrpcTransactions(ctx, start, end, req.Filter, req.FetchConfig)
			if err != nil {
				return nil, err
			}
			result := make(map[uint64]BlockMainData)
			for bn, txs := range txGroups {

				if len(txs) > 0 {
					// each grpc tx already carries its checkpoint header — no extra GetSimpleBlock RPC needed
					result[bn] = BlockMainData{
						Txs:         txs,
						SimpleBlock: new(suidata.SimpleBlock(txs[0].GetSimpleCheckpoint())),
					}
				}
			}
			return result, nil
		},
	)
}
