//go:build bqintegration

// This file is an OPT-IN integration test that hits the REAL public BigQuery dataset
// bigquery-public-data.crypto_solana_mainnet_us. It is excluded from normal builds and CI by the
// `bqintegration` build tag because it requires live infrastructure and incurs cost:
//
//   - Google Cloud credentials with bigquery.jobs.create + read on the public dataset, supplied via
//     GOOGLE_APPLICATION_CREDENTIALS (Application Default Credentials).
//   - Network access and bytes scanned (order of magnitude):
//   - TestBQIntegration_QueryBlock       ~10-50 GB  (Blocks has no time filter on a point lookup)
//   - TestBQIntegration_FindTransactions ~tens of GB. It filters instructions by program_id (the
//     cluster key), so the Instructions scan is pruned to the queried program; the Blocks
//     lookups dominate. (Fetching a transaction's FULL instruction set by tx_signature — the
//     non-clustered path the store deliberately avoids — would instead scan a whole DAY
//     partition, ~1.34 TB.) Each test logs the actual bytes billed via Store.Snapshot().
//
// Why this test exists: the unit tests in convert_test.go validate the row→RPC conversion with
// hand-built rows. They cannot exercise the real SQL, the BigQuery row scanning, or whether the
// public dataset's actual rows reconstruct correctly. This test closes that gap by reconstructing a
// known, immutable transaction and asserting against its real RPC getTransaction(jsonParsed) shape.
//
// Fixed sample (legacy transaction):
//
//	slot      422822279
//	signature 3WeJDhD1wXfY1qmHfh7yJotHV2dH7XnxXb7oY4xmK2yu3PNQJzm4oH6SHYiNTHQ48CJx3xhmaeEo45Jq8WsGyywv
//
// How to run (one-off, from the sentio-core module root):
//
//	BQ_BILLING_PROJECT=<your-gcp-project> \
//	  GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json \
//	  go test -tags bqintegration -run TestBQIntegration ./chain/sol/bq/ -v
//
// BQ_BILLING_PROJECT is required (no default — this is a public repo, so no project id is baked in):
// queries are billed to that project even though the dataset is public. The test is skipped when it
// is unset.
package bq

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/common/kvstore"
)

const (
	itSlot          = uint64(422822279)
	itSig           = "3WeJDhD1wXfY1qmHfh7yJotHV2dH7XnxXb7oY4xmK2yu3PNQJzm4oH6SHYiNTHQ48CJx3xhmaeEo45Jq8WsGyywv"
	itBlockhash     = "8a2rWas3z1EQN1yWnkTZxMsGytWfSUatXKcV84vdcTj7"
	itPrevBlockhash = "5t3qB6gVEf9ejNneXnNGarTBQYrRD7f1dVyRESysEBiA"
	itBlockTimeUnix = int64(1780011734)
	itBlockHeight   = uint64(400908723)
	itPAMMProgram   = "pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA" // the sample tx invokes this program
)

func newITStore(t *testing.T) *Store {
	t.Helper()
	project := os.Getenv("BQ_BILLING_PROJECT")
	if project == "" {
		t.Skip("set BQ_BILLING_PROJECT to run the BigQuery integration test")
	}
	// The day-slot and program-start caches are mandatory; in-memory stores suffice for the test.
	dayCache, err := kvstore.NewLRUKVStore[DaySlotIndex](4)
	require.NoError(t, err)
	programStartCache, err := kvstore.NewLRUKVStore[ProgramStart](1024)
	require.NoError(t, err)
	store, err := NewStore(context.Background(), Config{
		ProjectID:         project,
		Dataset:           "bigquery-public-data.crypto_solana_mainnet_us",
		DayCache:          dayCache,
		ProgramStartCache: programStartCache,
		// Start the day index just before the sample day so its one-time GROUP BY build scans only a
		// couple of Blocks month-partitions (~GB), not all of history.
		HistoryStart: time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
		// Generous one-off cap. FindTransactions filters instructions by program_id (cluster key) so
		// the scan is pruned; this cap is just a safety ceiling for the test. Tune down for production.
		MaxBytesBilled: 512 << 30, // 512 GiB
	})
	require.NoError(t, err)
	return store
}

// TestBQIntegration_QueryBlock validates the block-header path against real data (cheap: Blocks only).
func TestBQIntegration_QueryBlock(t *testing.T) {
	store := newITStore(t)
	defer store.Close()

	blk, err := store.QueryBlock(context.Background(), itSlot)
	require.NoError(t, err)
	require.NotNil(t, blk.GetBlockResult, "block must not be skipped")

	assert.Equal(t, itSlot, blk.Slot)
	assert.Equal(t, itBlockhash, blk.Blockhash.String())
	assert.Equal(t, itPrevBlockhash, blk.PreviousBlockhash.String())
	require.NotNil(t, blk.BlockTime)
	assert.Equal(t, itBlockTimeUnix, int64(*blk.BlockTime))
	require.NotNil(t, blk.BlockHeight)
	assert.Equal(t, itBlockHeight, *blk.BlockHeight)
	// parentSlot is intentionally not populated by the BigQuery store (see toBlock).
	assert.Zero(t, blk.ParentSlot)

	t.Logf("bq stats after QueryBlock: %+v", store.Snapshot())
}

// TestBQIntegration_FindTransactions reconstructs the sample transaction end-to-end (real SQL + row
// scanning + conversion) and asserts it matches the known RPC getTransaction(jsonParsed) shape.
func TestBQIntegration_FindTransactions(t *testing.T) {
	store := newITStore(t)
	defer store.Close()

	pid := solana.MustPublicKeyFromBase58(itPAMMProgram)
	blocks, err := store.FindTransactions(context.Background(), itSlot, itSlot, []solana.PublicKey{pid}, 5000)
	require.NoError(t, err)
	t.Logf("bq stats after FindTransactions: %+v", store.Snapshot())

	// Locate the sample transaction among the block's pAMM-invoking transactions.
	var tx sol.WrappedTransaction
	var group sol.BlockTransactions
	var ok bool
	for bi := range blocks {
		require.Equal(t, itSlot, blocks[bi].Slot)
		for ti := range blocks[bi].Transactions {
			if blocks[bi].Transactions[ti].Signature.String() == itSig {
				tx, group, ok = blocks[bi].Transactions[ti], blocks[bi], true
			}
		}
	}
	require.True(t, ok, "sample transaction not found in pAMM results for slot %d", itSlot)

	// Block header carried on the group.
	assert.Equal(t, itBlockhash, group.Blockhash.String())
	assert.Equal(t, itPrevBlockhash, group.PreviousBlockhash.String())

	// Version: BigQuery has no version column; always legacy (§5.1).
	assert.Equal(t, rpc.LegacyTransactionVersion, tx.Version)

	require.NotNil(t, tx.Transaction)
	require.NotNil(t, tx.Meta)

	// Meta scalars.
	assert.Equal(t, uint64(5005000), tx.Meta.Fee)
	require.NotNil(t, tx.Meta.ComputeUnitsConsumed)
	assert.Equal(t, uint64(106766), *tx.Meta.ComputeUnitsConsumed)
	assert.Nil(t, tx.Meta.Err, "successful tx => err nil")

	// Account keys: 28, first is the fee-payer (signer + writable), order preserved.
	require.Len(t, tx.Transaction.Message.AccountKeys, 28)
	assert.Equal(t, "6h3xQ9sBEwoNgCZSwHinBNShpikQeeKL67mYgXqeFJe2", tx.Transaction.Message.AccountKeys[0].PublicKey.String())
	assert.True(t, tx.Transaction.Message.AccountKeys[0].Signer)
	assert.True(t, tx.Transaction.Message.AccountKeys[0].Writable)
	assert.Equal(t, "AZMFeCQwSKNUZbtyTv8PkTXFMxBkkPF7oj3izK2rM5tN", tx.Transaction.Message.RecentBlockHash)

	// Balances projected onto accountKeys order (§5.4): index 0 = fee-payer.
	require.Len(t, tx.Meta.PreBalances, 28)
	require.Len(t, tx.Meta.PostBalances, 28)
	assert.Equal(t, uint64(53943741), tx.Meta.PreBalances[0])
	assert.Equal(t, uint64(677753040), tx.Meta.PostBalances[0])

	// Partial instructions (this store returns ONLY the queried programs' instructions — see the
	// FindTransactions doc). We filtered by pAMM, so every instruction returned — top-level and
	// inner — must be pAMM, and the other programs' instructions are dropped (gaps in the tree).
	require.NotEmpty(t, tx.Transaction.Message.Instructions)
	for _, in := range tx.Transaction.Message.Instructions {
		assert.Equal(t, itPAMMProgram, in.ProgramId.String())
	}
	// The sample has a single pAMM top-level instruction (the swap), which is unparsed: it carries
	// data + accounts and no parsed envelope, at stackHeight 1.
	require.Len(t, tx.Transaction.Message.Instructions, 1)
	pammTop := tx.Transaction.Message.Instructions[0]
	assert.Nil(t, pammTop.Parsed)
	assert.NotEmpty(t, pammTop.Data)
	assert.NotEmpty(t, pammTop.Accounts)
	assert.Equal(t, int64(1), pammTop.StackHeight)

	// pAMM is also invoked once as an inner instruction (CPI) under top-level index 3; the parent
	// index is preserved even though that parent (a different program) was dropped. Inner
	// stackHeight = 2 (§5.6).
	require.Len(t, tx.Meta.InnerInstructions, 1)
	assert.Equal(t, uint64(3), tx.Meta.InnerInstructions[0].Index)
	require.Len(t, tx.Meta.InnerInstructions[0].Instructions, 1)
	pammInner := tx.Meta.InnerInstructions[0].Instructions[0]
	assert.Equal(t, itPAMMProgram, pammInner.ProgramId.String())
	assert.Equal(t, int64(2), pammInner.StackHeight)

	// Parsed-instruction reconstruction (e.g. spl-token transferChecked, ATA createIdempotent) is
	// covered by the unit tests in convert_test.go; it is not re-asserted here because those
	// programs are filtered out by the pAMM-only query.

	// Token balances: 6 each; uiAmount is computed from amount/decimals (§3.4).
	require.Len(t, tx.Meta.PreTokenBalances, 6)
	require.Len(t, tx.Meta.PostTokenBalances, 6)
	for _, tb := range tx.Meta.PreTokenBalances {
		require.NotNil(t, tb.UiTokenAmount)
		assert.NotEmpty(t, tb.UiTokenAmount.Amount)
		// programId is intentionally nil for the BigQuery tier (§5.5).
		assert.Nil(t, tb.ProgramId)
	}
}
