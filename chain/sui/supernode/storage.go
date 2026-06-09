package supernode

import (
	"context"
	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
)

// StorageShared is the format-agnostic subset served by either backing storage.
// Both StorageJSONRPC and StorageGRPC satisfy it, so format-agnostic methods
// (simple checkpoint, object stat) can be served by whichever storage exists.
type StorageShared interface {
	// QuerySimpleCheckpoint will return error if checkpoint not found
	QuerySimpleCheckpoint(ctx context.Context, checkpoint uint64) (sui.SimpleCheckpoint, error)
	QueryObjectsStat(ctx context.Context, fromBlock, toBlock uint64, objectIDList []string) (map[string]sui.ObjectStat, error)
}

type StorageJSONRPC interface {
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

type StorageGRPC interface {
	// QuerySimpleCheckpoint will return error if checkpoint not found
	QuerySimpleCheckpoint(ctx context.Context, checkpoint uint64) (sui.SimpleCheckpoint, error)

	// QueryTransactions kind in filter.FunctionFilters should use TransactionKind_Kind values:
	//  - PROGRAMMABLE_TRANSACTION
	//  - CHANGE_EPOCH
	//  - GENESIS
	//  - CONSENSUS_COMMIT_PROLOGUE_V1
	//  - AUTHENTICATOR_STATE_UPDATE
	//  - END_OF_EPOCH
	//  - RANDOMNESS_STATE_UPDATE
	//  - CONSENSUS_COMMIT_PROLOGUE_V2
	//  - CONSENSUS_COMMIT_PROLOGUE_V3
	//  - CONSENSUS_COMMIT_PROLOGUE_V4
	//  - PROGRAMMABLE_SYSTEM_TRANSACTION
	QueryTransactions(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.TransactionFilter,
		fetchConfig sui.TransactionFetchConfig,
	) ([]*sui.ExtendedGrpcTransaction, error)

	// QueryObjectChanges ownerType in filter should use Owner_OwnerKind values:
	//  - ADDRESS
	//  - OBJECT
	//  - SHARED
	//  - IMMUTABLE
	//  - CONSENSUS_ADDRESS
	QueryObjectChanges(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.ObjectChangeFilter,
	) ([]*sui.ExtendedGrpcChangedObject, error)

	QueryObjectsStat(
		ctx context.Context,
		fromBlock, toBlock uint64,
		objectIDList []string,
	) (map[string]sui.ObjectStat, error)

	Snapshot() any
}
