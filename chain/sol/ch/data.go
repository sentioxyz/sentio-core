package ch

import (
	"encoding/json"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/common/objectx"
)

// ClickhouseBlock is one row of the blocks table. There is exactly one row per slot, including
// skipped slots, so that the dimension can detect missing slots by a dense count.
type ClickhouseBlock struct {
	Slot              uint64    `clickhouse:"slot" required:"true" number_field:"true"`
	Skipped           bool      `clickhouse:"skipped" required:"true"`
	Blockhash         string    `clickhouse:"blockhash" required:"true"`
	PreviousBlockhash string    `clickhouse:"previous_blockhash" required:"true"`
	ParentSlot        uint64    `clickhouse:"parent_slot" required:"true"`
	BlockHeight       uint64    `clickhouse:"block_height" required:"true"`
	BlockTime         time.Time `clickhouse:"block_time" required:"true"`
}

// ClickhouseTransaction is one row of the transactions table.
type ClickhouseTransaction struct {
	Slot             uint64    `clickhouse:"slot" required:"true" number_field:"true"`
	BlockTime        time.Time `clickhouse:"block_time" required:"true"`
	TransactionIndex uint32    `clickhouse:"transaction_index" required:"true"`
	Signature        string    `clickhouse:"signature"     required:"true" index:"bloom_filter GRANULARITY 1"`
	AccountKeys      []string  `clickhouse:"account_keys"  index:"bloom_filter GRANULARITY 1"`
	Version          int32     `clickhouse:"version"       required:"true"`
	Err              bool      `clickhouse:"err"           required:"true"`
	TransactionJSON  string    `clickhouse:"transaction_json" compression:"CODEC(ZSTD(1))" required:"true"`
}

// blockTimePtr converts a stored block time back to the optional Unix-seconds timestamp; a zero
// time (skipped/unknown) reconstructs to nil.
func blockTimePtr(t time.Time) *solana.UnixTimeSeconds {
	if t.IsZero() {
		return nil
	}
	ut := solana.UnixTimeSeconds(t.Unix())
	return &ut
}

func (cb *ClickhouseBlock) toBlock() (sol.Block, error) {
	if cb.Skipped {
		return sol.Block{Slot: cb.Slot}, nil
	}
	blockhash, err := solana.HashFromBase58(cb.Blockhash)
	if err != nil {
		return sol.Block{}, errors.Wrapf(err, "parse blockhash of slot %d failed", cb.Slot)
	}
	previousBlockhash, err := solana.HashFromBase58(cb.PreviousBlockhash)
	if err != nil {
		return sol.Block{}, errors.Wrapf(err, "parse previous blockhash of slot %d failed", cb.Slot)
	}
	blockHeight := cb.BlockHeight
	return sol.Block{
		Slot: cb.Slot,
		GetBlockResult: &rpc.GetBlockResult{
			Blockhash:         blockhash,
			PreviousBlockhash: previousBlockhash,
			ParentSlot:        cb.ParentSlot,
			BlockTime:         blockTimePtr(cb.BlockTime),
			BlockHeight:       &blockHeight,
		},
	}, nil
}

func (ct *ClickhouseTransaction) toParsedTransaction() (sol.ParsedTransactionWithMeta, error) {
	var tx sol.ParsedTransactionWithMeta
	if err := json.Unmarshal([]byte(ct.TransactionJSON), &tx); err != nil {
		return tx, errors.Wrapf(err, "unmarshal transaction %d/%s failed", ct.Slot, ct.Signature)
	}
	return tx, nil
}

func (ct *ClickhouseTransaction) toTransactionSignature() (*rpc.TransactionSignature, error) {
	sig, err := solana.SignatureFromBase58(ct.Signature)
	if err != nil {
		return nil, errors.Wrapf(err, "parse signature %d/%s failed", ct.Slot, ct.Signature)
	}
	var errVal any
	if ct.Err {
		errVal = "error"
	}
	return &rpc.TransactionSignature{
		Signature: sig,
		Slot:      ct.Slot,
		BlockTime: blockTimePtr(ct.BlockTime),
		Err:       errVal,
	}, nil
}

func (ct *ClickhouseTransaction) toGetParsedTransactionResult() (*rpc.GetParsedTransactionResult, error) {
	tx, err := ct.toParsedTransaction()
	if err != nil {
		return nil, err
	}
	return &rpc.GetParsedTransactionResult{
		Slot:        ct.Slot,
		BlockTime:   blockTimePtr(ct.BlockTime),
		Transaction: tx.Transaction,
		Meta:        tx.Meta,
		Version:     rpc.TransactionVersion(ct.Version),
	}, nil
}

func blockValues(block ClickhouseBlock) []any {
	return objectx.CollectFieldValues(&block, objectx.HasTag("clickhouse"))
}

func transactionValues(tx ClickhouseTransaction) []any {
	return objectx.CollectFieldValues(&tx, objectx.HasTag("clickhouse"))
}
