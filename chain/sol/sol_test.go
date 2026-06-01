package sol

import (
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
)

var testSig = solana.MustSignatureFromBase58(
	"4kAc2ytFEn5m45c9tzNzpP6uY4NEoAmExoEs5FzUV4yygVL2LofQog8AdSjFJ3wNHb4Gg2oQJNxjPhy9Zkbwo6kB")

func buildParsedTx(sig solana.Signature, topProgram, innerProgram solana.PublicKey) ParsedTransactionWithMeta {
	return ParsedTransactionWithMeta{
		Transaction: &rpc.ParsedTransaction{
			Signatures: []solana.Signature{sig},
			Message: rpc.ParsedMessage{
				Instructions: []*rpc.ParsedInstruction{{ProgramId: topProgram}},
			},
		},
		Meta: &rpc.ParsedTransactionMeta{
			InnerInstructions: []rpc.ParsedInnerInstruction{
				{Instructions: []*rpc.ParsedInstruction{{ProgramId: innerProgram}}},
			},
		},
		Version: rpc.LegacyTransactionVersion,
	}
}

func TestCollectProgramIDs(t *testing.T) {
	top := solana.NewWallet().PublicKey()
	inner := solana.NewWallet().PublicKey()
	stranger := solana.NewWallet().PublicKey()

	tx := buildParsedTx(testSig, top, inner)
	programs := CollectProgramIDs(tx.Transaction, tx.Meta)

	assert.Contains(t, programs, top.String())
	assert.Contains(t, programs, inner.String(), "inner instruction program must be indexed")
	assert.NotContains(t, programs, stranger.String())

	assert.True(t, txInvokesAnyProgram(tx, map[string]struct{}{top.String(): {}}))
	assert.True(t, txInvokesAnyProgram(tx, map[string]struct{}{inner.String(): {}}))
	assert.False(t, txInvokesAnyProgram(tx, map[string]struct{}{stranger.String(): {}}))
}

func TestMatchingTransactions(t *testing.T) {
	top := solana.NewWallet().PublicKey()
	inner := solana.NewWallet().PublicKey()
	slot := &Slot{
		SlotNumber: 100,
		Transactions: []ParsedTransactionWithMeta{
			buildParsedTx(testSig, top, inner),
			buildParsedTx(solana.Signature{}, solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey()),
		},
	}
	matching := slot.MatchingTransactions(map[string]struct{}{top.String(): {}})
	assert.Len(t, matching, 1)
	assert.Equal(t, uint32(0), matching[0].TransactionIndex)
	assert.Equal(t, testSig, matching[0].Signature)
	assert.True(t, slot.InvokesAnyProgram(map[string]struct{}{inner.String(): {}}))
}

func TestSlotToBlock(t *testing.T) {
	blockTime := solana.UnixTimeSeconds(1700000000)
	height := uint64(123)
	slot := &Slot{
		SlotNumber:        100,
		Blockhash:         solana.MustHashFromBase58("11111111111111111111111111111112"),
		PreviousBlockhash: solana.MustHashFromBase58("11111111111111111111111111111111"),
		ParentSlot:        99,
		BlockHeight:       &height,
		BlockTime:         &blockTime,
		Transactions: []ParsedTransactionWithMeta{
			buildParsedTx(testSig, solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey()),
		},
	}

	withSigs := slot.ToBlock(true)
	assert.Equal(t, uint64(100), withSigs.Slot)
	assert.NotNil(t, withSigs.GetBlockResult)
	assert.Equal(t, []solana.Signature{testSig}, withSigs.Signatures)

	headerOnly := slot.ToBlock(false)
	assert.NotNil(t, headerOnly.GetBlockResult)
	assert.Empty(t, headerOnly.Signatures, "header-only block must omit signatures")

	skipped := (&Slot{SlotNumber: 101, Skipped: true}).ToBlock(true)
	assert.Nil(t, skipped.GetBlockResult, "skipped slot must have a nil block result")
}

func TestIntervalWindowKey(t *testing.T) {
	blockWin := IntervalWindow{BlockWindow: 1000}
	assert.Equal(t, uint64(0), blockWin.Key(500, nil))
	assert.Equal(t, uint64(1), blockWin.Key(1000, nil))
	assert.Equal(t, uint64(1), blockWin.Key(1999, nil))

	timeWin := IntervalWindow{TimeWindow: time.Hour}
	t0 := solana.UnixTimeSeconds(3600)
	t1 := solana.UnixTimeSeconds(7199)
	t2 := solana.UnixTimeSeconds(7200)
	assert.Equal(t, timeWin.Key(10, &t0), timeWin.Key(11, &t1), "same hour bucket")
	assert.NotEqual(t, timeWin.Key(11, &t1), timeWin.Key(12, &t2), "next hour bucket")
}
