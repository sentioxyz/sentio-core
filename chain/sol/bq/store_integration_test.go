//go:build bqintegration

// This file is an OPT-IN integration test that hits the REAL public BigQuery dataset
// bigquery-public-data.crypto_solana_mainnet_us. It is excluded from normal builds and CI by the
// `bqintegration` build tag because it requires live infrastructure and incurs cost:
//
//   - Google Cloud credentials with bigquery.jobs.create + read on the public dataset, supplied via
//     GOOGLE_APPLICATION_CREDENTIALS (Application Default Credentials).
//   - Network access and a non-trivial number of bytes scanned (measured against the sample day):
//       * TestBQIntegration_QueryBlock      ~10-50 GB  (Blocks has no time filter on a point lookup)
//       * TestBQIntegration_FindTransactions ~1.34 TB  (queryInstructions scans a full DAY partition
//         of Instructions — it is clustered by program_id, not by signature/slot, so fetching one
//         transaction's instruction set cannot be pruned). The heavy test is gated behind
//         BQ_RUN_HEAVY=1. Acceptable for a one-off check, NOT for CI.
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
//	GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json \
//	  go test -tags bqintegration -run TestBQIntegration ./chain/sol/bq/ -v
//
// The billing project defaults below and can be overridden with BQ_BILLING_PROJECT. Queries are
// billed to that project even though the dataset is public.
package bq

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/sol"
)

const (
	itSlot            = uint64(422822279)
	itSig             = "3WeJDhD1wXfY1qmHfh7yJotHV2dH7XnxXb7oY4xmK2yu3PNQJzm4oH6SHYiNTHQ48CJx3xhmaeEo45Jq8WsGyywv"
	itBlockhash       = "8a2rWas3z1EQN1yWnkTZxMsGytWfSUatXKcV84vdcTj7"
	itPrevBlockhash   = "5t3qB6gVEf9ejNneXnNGarTBQYrRD7f1dVyRESysEBiA"
	itBlockTimeUnix   = int64(1780011734)
	itBlockHeight     = uint64(400908723)
	itPAMMProgram     = "pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA" // the sample tx invokes this program
	itComputeBudgetID = "ComputeBudget111111111111111111111111111111"
	itATAProgramID    = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"
)

func newITStore(t *testing.T) *Store {
	t.Helper()
	project := os.Getenv("BQ_BILLING_PROJECT")
	if project == "" {
		project = "sentio-352722"
	}
	store, err := NewStore(context.Background(), Config{
		ProjectID: project,
		Dataset:   "bigquery-public-data.crypto_solana_mainnet_us",
		// A single known slot: pad 0 so the partition filter stays on the one DAY partition.
		PartitionPaddingDays: 0,
		// Generous one-off cap. NOTE: reconstructing a transaction scans a whole DAY partition of
		// the Instructions table (it is clustered by program_id, not by signature/slot, so fetching
		// one transaction's instructions cannot be pruned). For the busy sample day this was measured
		// at ~1.34 TB for the queryInstructions step alone. The cap is set above that so the heavy
		// test can run; tune MaxBytesBilled down hard for production.
		MaxBytesBilled: 2 << 40, // 2 TiB
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
	// EXPENSIVE: reconstructing the transaction scans a full DAY partition of the Instructions table
	// (~1.34 TB measured on the sample day ≈ ~$8 at on-demand pricing). Opt in explicitly.
	if os.Getenv("BQ_RUN_HEAVY") == "" {
		t.Skip("expensive (~1.3 TB scan); set BQ_RUN_HEAVY=1 to run")
	}
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

	// Top-level instructions: 7, ordered by index.
	require.Len(t, tx.Transaction.Message.Instructions, 7)
	// [0] ComputeBudget is unparsed (data + no parsed envelope), stackHeight 1.
	assert.Empty(t, tx.Transaction.Message.Instructions[0].Program)
	assert.Nil(t, tx.Transaction.Message.Instructions[0].Parsed)
	assert.Equal(t, itComputeBudgetID, tx.Transaction.Message.Instructions[0].ProgramId.String())
	assert.Equal(t, int64(1), tx.Transaction.Message.Instructions[0].StackHeight)

	// [2] createIdempotent is a parsed instruction; compare the full reconstructed JSON.
	gotInstr, err := json.Marshal(tx.Transaction.Message.Instructions[2])
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"parsed": {
			"info": {
				"account": "FXMZw41raezasEk2Q8JMDxadzWMyCrRnEUyu8YQji8RN",
				"mint": "So11111111111111111111111111111111111111112",
				"source": "6h3xQ9sBEwoNgCZSwHinBNShpikQeeKL67mYgXqeFJe2",
				"systemProgram": "11111111111111111111111111111111",
				"tokenProgram": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
				"wallet": "6h3xQ9sBEwoNgCZSwHinBNShpikQeeKL67mYgXqeFJe2"
			},
			"type": "createIdempotent"
		},
		"program": "spl-associated-token-account",
		"programId": "`+itATAProgramID+`",
		"stackHeight": 1
	}`, string(gotInstr))

	// Inner instructions: groups for parent index 2 (4 instructions) and 3 (7 instructions),
	// ordered by parent index; all inner stackHeight = 2 (§5.6).
	require.Len(t, tx.Meta.InnerInstructions, 2)
	assert.Equal(t, uint64(2), tx.Meta.InnerInstructions[0].Index)
	assert.Len(t, tx.Meta.InnerInstructions[0].Instructions, 4)
	assert.Equal(t, uint64(3), tx.Meta.InnerInstructions[1].Index)
	assert.Len(t, tx.Meta.InnerInstructions[1].Instructions, 7)
	for _, in := range tx.Meta.InnerInstructions[1].Instructions {
		assert.Equal(t, int64(2), in.StackHeight)
	}

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
