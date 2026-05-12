package supernode

import (
	"context"
	"sentioxyz/sentio-core/chain/fuel"
)

type Storage interface {
	QueryTransactions(
		ctx context.Context,
		startBlock uint64,
		endBlock uint64,
		filters []fuel.TransactionFilter,
	) (result []fuel.WrappedTransaction, err error)
	QueryContractCreateTransaction(ctx context.Context, contractID string) (result *fuel.WrappedTransaction, err error)
}
