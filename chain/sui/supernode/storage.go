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
	// QueryTransactionsV2 scans at most limit raw rows (0 = unlimited; a SQL LIMIT bounding the
	// ClickHouse-side resource use of one query) and fails with chain.NewTooManyResultsError when
	// the scan hits it, so a returned result is always complete. The super node passes its record
	// cap + 1 (chain.StoreQueryLimit), so a query matching exactly the cap still succeeds.
	QueryTransactionsV2(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.TransactionFilter,
		fetchConfig sui.TransactionFetchConfig,
		limit int,
	) ([]types.TransactionResponseV1, error)

	QueryObjectChanges(ctx context.Context, query *sui.ObjectChangeQuery) ([]types.ObjectChangeExtend, error)
	// QueryObjectChangesV2 applies limit like QueryTransactionsV2.
	QueryObjectChangesV2(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.ObjectChangeFilter,
		limit int,
	) ([]types.ObjectChangeExtend, error)

	QueryObjectsStat(ctx context.Context, fromBlock, toBlock uint64, objectIDList []string) (map[string]sui.ObjectStat, error)

	// QueryLastObjectChange returns the object's newest recorded change with
	// object_version <= maxVersion (no bound when maxVersion is 0) and
	// checkpoint <= maxCheckpoint, or nil when nothing is recorded. A per-object
	// point lookup whose cost is independent of the checkpoint span of the
	// object's history; the checkpoint bound lets the super node serve the tail
	// from its latest-slot cache and this only for the range below it.
	QueryLastObjectChange(
		ctx context.Context, objectID string, maxVersion uint64, maxCheckpoint uint64,
	) (*sui.ObjectChangeRecord, error)

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
	// It applies limit like StorageJSONRPC.QueryTransactionsV2 (a SQL LIMIT on the raw rows
	// scanned; hitting it fails with chain.NewTooManyResultsError).
	QueryTransactions(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.TransactionFilter,
		fetchConfig sui.TransactionFetchConfig,
		limit int,
	) ([]*sui.ExtendedGrpcTransaction, error)

	// QueryObjectChanges ownerType in filter should use Owner_OwnerKind values:
	//  - ADDRESS
	//  - OBJECT
	//  - SHARED
	//  - IMMUTABLE
	//  - CONSENSUS_ADDRESS
	// It applies limit like QueryTransactions.
	QueryObjectChanges(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.ObjectChangeFilter,
		limit int,
	) ([]*sui.ExtendedGrpcChangedObject, error)

	QueryObjectsStat(
		ctx context.Context,
		fromBlock, toBlock uint64,
		objectIDList []string,
	) (map[string]sui.ObjectStat, error)

	// QueryLastObjectChange applies the same contract as
	// StorageJSONRPC.QueryLastObjectChange; the grpc-derived objects history also
	// records wrapped / unwrapped / deleted rows.
	QueryLastObjectChange(
		ctx context.Context, objectID string, maxVersion uint64, maxCheckpoint uint64,
	) (*sui.ObjectChangeRecord, error)

	Snapshot() any
}
