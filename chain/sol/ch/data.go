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
// skipped slots, so missing slots can be detected by a dense count and the first non-skipped block
// of an interval window can be found directly.
type ClickhouseBlock struct {
	Slot              uint64    `clickhouse:"slot" required:"true" number_field:"true"`
	Skipped           bool      `clickhouse:"skipped" required:"true"`
	Blockhash         string    `clickhouse:"blockhash" required:"true"`
	PreviousBlockhash string    `clickhouse:"previous_blockhash" required:"true"`
	ParentSlot        uint64    `clickhouse:"parent_slot" required:"true"`
	BlockHeight       uint64    `clickhouse:"block_height" required:"true"`
	BlockTime         time.Time `clickhouse:"block_time" required:"true"`
}

// ClickhouseTransaction is one row of the transactions table. program_ids indexes the programs the
// transaction invokes (top-level and inner instructions) for instruction lookups. The parsed
// transaction and meta are stored separately so each can be read without the other.
type ClickhouseTransaction struct {
	Slot             uint64    `clickhouse:"slot" required:"true" number_field:"true"`
	BlockTime        time.Time `clickhouse:"block_time" required:"true"`
	TransactionIndex uint32    `clickhouse:"transaction_index" required:"true"`
	Signature        string    `clickhouse:"signature" required:"true"`
	ProgramIDs       []string  `clickhouse:"program_ids" index:"bloom_filter GRANULARITY 1"`
	Version          int32     `clickhouse:"version" required:"true"`
	TransactionJSON  string    `clickhouse:"transaction_json" compression:"CODEC(ZSTD(1))" required:"true"`
	MetaJSON         string    `clickhouse:"meta_json" compression:"CODEC(ZSTD(1))" required:"true"`
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

func (cb *ClickhouseBlock) toBlock(signatures []solana.Signature) (sol.Block, error) {
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
			Signatures:        signatures,
		},
	}, nil
}

func (ct *ClickhouseTransaction) toWrappedTransaction() (sol.WrappedTransaction, error) {
	sig, err := solana.SignatureFromBase58(ct.Signature)
	if err != nil {
		return sol.WrappedTransaction{}, errors.Wrapf(err, "parse signature %d/%s failed", ct.Slot, ct.Signature)
	}
	var transaction *rpc.ParsedTransaction
	if err = json.Unmarshal([]byte(ct.TransactionJSON), &transaction); err != nil {
		return sol.WrappedTransaction{}, errors.Wrapf(err, "unmarshal transaction %d/%s failed", ct.Slot, ct.Signature)
	}
	var meta *rpc.ParsedTransactionMeta
	if err = json.Unmarshal([]byte(ct.MetaJSON), &meta); err != nil {
		return sol.WrappedTransaction{}, errors.Wrapf(err, "unmarshal meta %d/%s failed", ct.Slot, ct.Signature)
	}
	return sol.WrappedTransaction{
		TransactionIndex: ct.TransactionIndex,
		Signature:        sig,
		Version:          rpc.TransactionVersion(ct.Version),
		Transaction:      transaction,
		Meta:             meta,
	}, nil
}

func blockValues(block ClickhouseBlock) []any {
	return objectx.CollectFieldValues(&block, objectx.HasTag("clickhouse"))
}

func transactionValues(tx ClickhouseTransaction) []any {
	return objectx.CollectFieldValues(&tx, objectx.HasTag("clickhouse"))
}
