package ch

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSig = solana.MustSignatureFromBase58(
	"4kAc2ytFEn5m45c9tzNzpP6uY4NEoAmExoEs5FzUV4yygVL2LofQog8AdSjFJ3wNHb4Gg2oQJNxjPhy9Zkbwo6kB")

func buildTxRow(t *testing.T, slot uint64) ClickhouseTransaction {
	t.Helper()
	transaction := &rpc.ParsedTransaction{Signatures: []solana.Signature{testSig}}
	meta := &rpc.ParsedTransactionMeta{Fee: 5000}
	txJSON, err := json.Marshal(transaction)
	require.NoError(t, err)
	metaJSON, err := json.Marshal(meta)
	require.NoError(t, err)
	return ClickhouseTransaction{
		Slot:             slot,
		BlockTime:        time.Unix(1700000000, 0),
		TransactionIndex: 7,
		Signature:        testSig.String(),
		ProgramIDs:       []string{solana.SystemProgramID.String()},
		Version:          int32(rpc.LegacyTransactionVersion),
		TransactionJSON:  string(txJSON),
		MetaJSON:         string(metaJSON),
	}
}

func TestTransactionRoundTrip(t *testing.T) {
	row := buildTxRow(t, 100)

	wt, err := row.toWrappedTransaction()
	require.NoError(t, err)
	assert.Equal(t, uint32(7), wt.TransactionIndex)
	assert.Equal(t, testSig, wt.Signature)
	assert.Equal(t, rpc.LegacyTransactionVersion, wt.Version)
	require.NotNil(t, wt.Transaction)
	assert.Equal(t, []solana.Signature{testSig}, wt.Transaction.Signatures)
	require.NotNil(t, wt.Meta)
	assert.Equal(t, uint64(5000), wt.Meta.Fee)

	blockTime := solana.UnixTimeSeconds(time.Unix(1700000000, 0).Unix())
	res := wt.ToParsedTransactionResult(100, &blockTime)
	assert.Equal(t, uint64(100), res.Slot)
	assert.Equal(t, rpc.LegacyTransactionVersion, res.Version)
	assert.Equal(t, []solana.Signature{testSig}, res.Transaction.Signatures)
}

func TestBlockRoundTrip(t *testing.T) {
	cb := ClickhouseBlock{
		Slot:              100,
		Skipped:           false,
		Blockhash:         "11111111111111111111111111111112",
		PreviousBlockhash: "11111111111111111111111111111111",
		ParentSlot:        99,
		BlockHeight:       123,
		BlockTime:         time.Unix(1700000000, 0),
	}
	block, err := cb.toBlock([]solana.Signature{testSig})
	require.NoError(t, err)
	require.NotNil(t, block.GetBlockResult)
	assert.Equal(t, uint64(99), block.ParentSlot)
	assert.Equal(t, []solana.Signature{testSig}, block.Signatures)
	require.NotNil(t, block.BlockHeight)
	assert.Equal(t, uint64(123), *block.BlockHeight)

	skipped := ClickhouseBlock{Slot: 101, Skipped: true}
	skippedBlock, err := skipped.toBlock(nil)
	require.NoError(t, err)
	assert.Nil(t, skippedBlock.GetBlockResult, "skipped slot must reconstruct to a nil block result")
	assert.Equal(t, uint64(101), skippedBlock.Slot)
}
