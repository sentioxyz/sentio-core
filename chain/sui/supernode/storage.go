package supernode

import (
	"context"
	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
)

type Storage interface {
	// QueryCheckpointTime will return error if checkpoint not found
	QueryCheckpointTime(ctx context.Context, checkpoint uint64) (sui.CheckpointTime, error)
	// QuerySimpleCheckpoint will return error if checkpoint not found
	QuerySimpleCheckpoint(ctx context.Context, checkpoint uint64) (sui.SimpleCheckpoint, error)
	QueryTransactions(ctx context.Context, query *sui.TransactionQuery) ([]types.TransactionResponseV1, error)
	QueryTransactionsV2(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.TransactionFilter,
		fetchConfig sui.TransactionFetchConfig,
	) ([]types.TransactionResponseV1, error)
	QueryObjectChanges(ctx context.Context, query *sui.ObjectChangeQuery) ([]types.ObjectChangeExtend, error)
	QueryObjectChangesV2(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.ObjectChangeFilter,
	) ([]types.ObjectChangeExtend, error)
	QueryObjectsStat(ctx context.Context, fromBlock, toBlock uint64, objectIDList []string) (map[string]sui.ObjectStat, error)

	Snapshot() any
}
