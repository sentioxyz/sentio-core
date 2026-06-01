package ch

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/sol"
)

var testSig = solana.MustSignatureFromBase58(
	"4kAc2ytFEn5m45c9tzNzpP6uY4NEoAmExoEs5FzUV4yygVL2LofQog8AdSjFJ3wNHb4Gg2oQJNxjPhy9Zkbwo6kB")

func buildTxRow(t *testing.T, slot uint64, failed bool) ClickhouseTransaction {
	t.Helper()
	twm := sol.ParsedTransactionWithMeta{
		Transaction: &rpc.ParsedTransaction{
			Signatures: []solana.Signature{testSig},
			Message:    rpc.ParsedMessage{},
		},
		Meta: &rpc.ParsedTransactionMeta{},
	}
	if failed {
		twm.Meta.Err = "InstructionError"
	}
	raw, err := json.Marshal(twm)
	require.NoError(t, err)
	return ClickhouseTransaction{
		Slot:             slot,
		BlockTime:        time.Unix(1700000000, 0),
		TransactionIndex: 0,
		Signature:        testSig.String(),
		Version:          int32(rpc.LegacyTransactionVersion),
		Err:              failed,
		TransactionJSON:  string(raw),
	}
}

func TestTransactionRoundTrip(t *testing.T) {
	row := buildTxRow(t, 100, true)

	twm, err := row.toParsedTransaction()
	require.NoError(t, err)
	assert.Equal(t, []solana.Signature{testSig}, twm.Transaction.Signatures)

	sig, err := row.toTransactionSignature()
	require.NoError(t, err)
	assert.Equal(t, testSig, sig.Signature)
	assert.Equal(t, uint64(100), sig.Slot)
	assert.NotNil(t, sig.Err, "failed tx must carry a non-nil Err")
	require.NotNil(t, sig.BlockTime)

	res, err := row.toGetParsedTransactionResult()
	require.NoError(t, err)
	assert.Equal(t, uint64(100), res.Slot)
	assert.Equal(t, []solana.Signature{testSig}, res.Transaction.Signatures)
	assert.Equal(t, rpc.LegacyTransactionVersion, res.Version)
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
	block, err := cb.toBlock()
	require.NoError(t, err)
	require.NotNil(t, block.GetBlockResult)
	assert.Equal(t, uint64(99), block.ParentSlot)
	require.NotNil(t, block.BlockHeight)
	assert.Equal(t, uint64(123), *block.BlockHeight)

	skipped := ClickhouseBlock{Slot: 101, Skipped: true}
	skippedBlock, err := skipped.toBlock()
	require.NoError(t, err)
	assert.Nil(t, skippedBlock.GetBlockResult, "skipped slot must reconstruct to a nil block result")
	assert.Equal(t, uint64(101), skippedBlock.Slot)
}
