package supernode

import (
	"context"
	"sentioxyz/sentio-core/chain/aptos"
)

type Storage interface {
	Functions(ctx context.Context, req aptos.GetFunctionsArgs) ([]*aptos.Transaction, error)
	FullEvents(ctx context.Context, req aptos.GetEventsArgs) ([]*aptos.Transaction, error)
	ResourceChanges(ctx context.Context, req aptos.ResourceChangeArgs) ([]*aptos.Transaction, error)
	GetTransactionByVersion(ctx context.Context, txVersion uint64) (*aptos.Transaction, error)
	GetChangeStat(ctx context.Context, minTxVersion uint64, address string) (aptos.ChangeStat, error)

	GetFirstChange(ctx context.Context, address string, maxTxVersion uint64) (version, blockHeight uint64, has bool, err error)
	QueryMinimalistTransaction(ctx context.Context, txVersion uint64) (*aptos.MinimalistTransaction, error)
	QueryTransactions(ctx context.Context, req aptos.GetTransactionsRequest) ([]aptos.Transaction, error)
	// QueryResourceChanges returns chain.NewTooManyResultsError once more than limit matching
	// changes accumulate (limit <= 0 means unlimited), so an over-dense range aborts early instead
	// of materializing an unbounded result.
	QueryResourceChanges(
		ctx context.Context,
		req aptos.GetResourceChangesRequest,
		limit int,
	) ([]aptos.MinimalistTransactionWithChanges, error)
}
