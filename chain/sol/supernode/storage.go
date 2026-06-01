package supernode

import (
	"context"

	"github.com/gagliardetto/solana-go"

	"sentioxyz/sentio-core/chain/sol"
)

// Storage is the ClickHouse read interface backing the super node, implemented by sol/ch.Store.
type Storage interface {
	// QueryBlock returns the header (without signatures) of a slot.
	QueryBlock(ctx context.Context, slot uint64) (*sol.Block, error)
	// QueryBlocksByInterval returns the first non-skipped block (with signatures) of each window
	// within [from, to].
	QueryBlocksByInterval(
		ctx context.Context,
		from uint64,
		to uint64,
		window sol.IntervalWindow,
		limit int,
	) ([]sol.Block, error)
	// QueryPreviousUnskipped returns the nearest non-skipped block (slot and time) with slot < before.
	QueryPreviousUnskipped(
		ctx context.Context,
		before uint64,
	) (slot uint64, blockTime *solana.UnixTimeSeconds, found bool, err error)
	// FindTransactions returns, grouped by block, the transactions in [from, to] invoking any program.
	FindTransactions(
		ctx context.Context,
		from uint64,
		to uint64,
		programIDs []solana.PublicKey,
		limit int,
	) ([]sol.BlockTransactions, error)
	// GetContractStartBlock returns the first slot in [start, latest] that invokes address.
	GetContractStartBlock(
		ctx context.Context,
		address solana.PublicKey,
		start uint64,
		latest uint64,
	) (uint64, bool, error)
}
