package supernode

import (
	"context"
	"sentioxyz/sentio-core/chain/aptos"
)

// Storage is the ClickHouse-backed store behind the super node. Every range-query method taking a
// limit returns at most limit (0 = unlimited) matching records, counted after post-filtering so a
// truncated result is always a prefix of the full result. The storage performs no limit CHECK of
// its own: the super node passes its record cap + 1 (chain.StoreQueryLimit) and detects an
// over-cap query from the record count (chain.CheckTooManyResults).
type Storage interface {
	Functions(ctx context.Context, req aptos.GetFunctionsArgs, limit int) ([]*aptos.Transaction, error)
	FullEvents(ctx context.Context, req aptos.GetEventsArgs, limit int) ([]*aptos.Transaction, error)
	ResourceChanges(ctx context.Context, req aptos.ResourceChangeArgs, limit int) ([]*aptos.Transaction, error)
	GetTransactionByVersion(ctx context.Context, txVersion uint64) (*aptos.Transaction, error)
	GetChangeStat(ctx context.Context, minTxVersion uint64, address string) (aptos.ChangeStat, error)

	GetFirstChange(ctx context.Context, address string, maxTxVersion uint64) (version, blockHeight uint64, has bool, err error)
	QueryMinimalistTransaction(ctx context.Context, txVersion uint64) (*aptos.MinimalistTransaction, error)
	QueryTransactions(ctx context.Context, req aptos.GetTransactionsRequest, limit int) ([]aptos.Transaction, error)
	QueryResourceChanges(
		ctx context.Context,
		req aptos.GetResourceChangesRequest,
		limit int,
	) ([]aptos.MinimalistTransactionWithChanges, error)
}
