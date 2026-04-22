package aptos

import (
	"context"
)

type Storage interface {
	Functions(ctx context.Context, req GetFunctionsArgs) ([]*Transaction, error)
	FullEvents(ctx context.Context, req GetEventsArgs) ([]*Transaction, error)
	ResourceChanges(ctx context.Context, req ResourceChangeArgs) ([]*Transaction, error)
	GetTransactionByVersion(ctx context.Context, txVersion uint64) (*Transaction, error)
	GetChangeStat(ctx context.Context, minTxVersion uint64, address string) (ChangeStat, error)

	GetFirstChange(ctx context.Context, address string, maxTxVersion uint64) (version, blockHeight uint64, has bool, err error)
	QueryMinimalistTransaction(ctx context.Context, txVersion uint64) (*MinimalistTransaction, error)
	QueryTransactions(ctx context.Context, req GetTransactionsRequest) ([]Transaction, error)
	QueryResourceChanges(ctx context.Context, req GetResourceChangesRequest) ([]MinimalistTransactionWithChanges, error)
}
