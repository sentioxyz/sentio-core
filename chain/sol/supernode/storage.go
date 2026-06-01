package supernode

import (
	"context"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"sentioxyz/sentio-core/chain/sol"
)

// Storage is the ClickHouse read interface backing the super node, implemented by sol/ch.Store.
type Storage interface {
	QueryBlock(ctx context.Context, slot uint64) (*sol.Block, error)
	QueryBlockTransactions(ctx context.Context, slot uint64) (sol.ParsedBlock, error)
	QueryTransaction(ctx context.Context, sig solana.Signature) (*rpc.GetParsedTransactionResult, error)
	FindTransactions(
		ctx context.Context,
		startBlock uint64,
		endBlock uint64,
		address solana.PublicKey,
		limit int,
	) ([]*rpc.TransactionSignature, error)
	GetContractStartBlock(
		ctx context.Context,
		address solana.PublicKey,
		start uint64,
		latest uint64,
	) (uint64, bool, error)
}
