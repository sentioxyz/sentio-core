package sol

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
)

var testSig = solana.MustSignatureFromBase58(
	"4kAc2ytFEn5m45c9tzNzpP6uY4NEoAmExoEs5FzUV4yygVL2LofQog8AdSjFJ3wNHb4Gg2oQJNxjPhy9Zkbwo6kB")

func buildParsedTx(sig solana.Signature, program, signer, inner solana.PublicKey) ParsedTransactionWithMeta {
	return ParsedTransactionWithMeta{
		Transaction: &rpc.ParsedTransaction{
			Signatures: []solana.Signature{sig},
			Message: rpc.ParsedMessage{
				AccountKeys:  []rpc.ParsedMessageAccount{{PublicKey: signer, Signer: true}},
				Instructions: []*rpc.ParsedInstruction{{ProgramId: program, Accounts: []solana.PublicKey{signer}}},
			},
		},
		Meta: &rpc.ParsedTransactionMeta{
			InnerInstructions: []rpc.ParsedInnerInstruction{
				{Instructions: []*rpc.ParsedInstruction{{ProgramId: inner}}},
			},
		},
	}
}

func TestCollectAccountKeysAndInvolves(t *testing.T) {
	program := solana.NewWallet().PublicKey()
	signer := solana.NewWallet().PublicKey()
	inner := solana.NewWallet().PublicKey()
	stranger := solana.NewWallet().PublicKey()

	tx := buildParsedTx(testSig, program, signer, inner)
	keys := CollectAccountKeys(tx.Transaction, tx.Meta)

	assert.Contains(t, keys, program.String())
	assert.Contains(t, keys, signer.String())
	assert.Contains(t, keys, inner.String(), "inner instruction program must be indexed")
	assert.NotContains(t, keys, stranger.String())

	assert.True(t, InvolvesAddress(tx.Transaction, tx.Meta, program))
	assert.True(t, InvolvesAddress(tx.Transaction, tx.Meta, inner))
	assert.False(t, InvolvesAddress(tx.Transaction, tx.Meta, stranger))
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
			buildParsedTx(testSig, solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey(), solana.NewWallet().PublicKey()),
		},
	}

	block := slot.ToBlock()
	assert.Equal(t, uint64(100), block.Slot)
	assert.False(t, block.GetBlockResult == nil, "non-skipped slot must have a block result")
	assert.Equal(t, uint64(99), block.ParentSlot)
	assert.Equal(t, []solana.Signature{testSig}, block.Signatures)

	skipped := &Slot{SlotNumber: 101, Skipped: true}
	skippedBlock := skipped.ToBlock()
	assert.Equal(t, uint64(101), skippedBlock.Slot)
	assert.True(t, skippedBlock.GetBlockResult == nil, "skipped slot must have a nil block result")
	assert.Empty(t, skipped.GetHash())
}
