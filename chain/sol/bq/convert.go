// Package bq implements the BigQuery-backed sol/supernode.Storage. It serves Solana history that
// predates the ClickHouse range from the public BigQuery dataset
// bigquery-public-data.crypto_solana_mainnet_us, reconstructing the same jsonParsed shapes
// (rpc.ParsedTransaction / rpc.ParsedTransactionMeta / rpc.GetBlockResult) that the ClickHouse
// store produces. See chain/sol/BIGQUERY_DATASOURCE_DESIGN.md for the full field mapping.
package bq

import (
	"encoding/json"
	"math/big"
	"sort"
	"strings"

	"cloud.google.com/go/bigquery"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/sol"
)

// ---------------------------------------------------------------------------
// BigQuery row structs (one per table; numeric columns are CAST in SQL so they
// scan into plain Go types instead of *big.Rat).
// ---------------------------------------------------------------------------

// blockRow is a row of the Blocks table. There is no parent-slot column; the BigQuery store does
// not populate GetBlockResult.ParentSlot (see toBlock).
type blockRow struct {
	Slot              int64                  `bigquery:"slot"`
	BlockHash         string                 `bigquery:"block_hash"`
	BlockTimestamp    bigquery.NullTimestamp `bigquery:"block_timestamp"`
	Height            int64                  `bigquery:"height"`
	PreviousBlockHash string                 `bigquery:"previous_block_hash"`
}

type accountRow struct {
	Pubkey   string `bigquery:"pubkey"`
	Signer   bool   `bigquery:"signer"`
	Writable bool   `bigquery:"writable"`
}

type balanceChangeRow struct {
	Account string `bigquery:"account"`
	Before  int64  `bigquery:"before"`
	After   int64  `bigquery:"after"`
}

type tokenBalanceRow struct {
	AccountIndex int64               `bigquery:"account_index"`
	Mint         string              `bigquery:"mint"`
	Owner        bigquery.NullString `bigquery:"owner"`
	Amount       string              `bigquery:"amount"` // CAST(amount AS STRING) in SQL
	Decimals     int64               `bigquery:"decimals"`
}

// txRow is a row of the Transactions table. fee/compute_units_consumed are CAST to INT64 and token
// amounts to STRING in the projection.
type txRow struct {
	BlockSlot         int64              `bigquery:"block_slot"`
	BlockHash         string             `bigquery:"block_hash"`
	RecentBlockHash   string             `bigquery:"recent_block_hash"`
	Signature         string             `bigquery:"signature"`
	Index             int64              `bigquery:"index"`
	Fee               int64              `bigquery:"fee"`
	Status            string             `bigquery:"status"`
	Err               bigquery.NullString `bigquery:"err"`
	ComputeUnits      bigquery.NullInt64 `bigquery:"compute_units_consumed"`
	Accounts          []accountRow       `bigquery:"accounts"`
	LogMessages       []string           `bigquery:"log_messages"`
	BalanceChanges    []balanceChangeRow `bigquery:"balance_changes"`
	PreTokenBalances  []tokenBalanceRow  `bigquery:"pre_token_balances"`
	PostTokenBalances []tokenBalanceRow  `bigquery:"post_token_balances"`
}

type paramRow struct {
	Key   bigquery.NullString `bigquery:"key"`
	Value bigquery.NullString `bigquery:"value"`
}

// instructionRow is a row of the Instructions table. parent_index is NULL for top-level
// instructions; program/instruction_type/parsed are NULL for unparsed instructions.
type instructionRow struct {
	BlockSlot       int64               `bigquery:"block_slot"`
	TxSignature     string              `bigquery:"tx_signature"`
	Index           int64               `bigquery:"index"`
	ParentIndex     bigquery.NullInt64  `bigquery:"parent_index"`
	Accounts        []string            `bigquery:"accounts"`
	Data            bigquery.NullString `bigquery:"data"`
	Program         bigquery.NullString `bigquery:"program"`
	ProgramID       string              `bigquery:"program_id"`
	InstructionType bigquery.NullString `bigquery:"instruction_type"`
	Params          []paramRow          `bigquery:"params"`
}

// ---------------------------------------------------------------------------
// Wire structs: the only part of the reconstruction that must go through JSON.
// rpc.ParsedInstruction.Parsed is an InstructionInfoEnvelope whose fields are unexported with no
// constructor, so a parsed instruction can only be built by unmarshalling JSON into it.
// ---------------------------------------------------------------------------

type wireParsed struct {
	Type string         `json:"type"`
	Info map[string]any `json:"info"`
}

type wireInstruction struct {
	Program     string      `json:"program,omitempty"`
	ProgramID   string      `json:"programId"`
	Parsed      *wireParsed `json:"parsed,omitempty"`
	Data        string      `json:"data,omitempty"`
	Accounts    []string    `json:"accounts,omitempty"`
	StackHeight int64       `json:"stackHeight"`
}

// stack heights per the design (§5.6): top-level instructions are 1, all inner instructions are 2.
// Deeper CPI nesting is not recoverable from the Instructions table alone.
const (
	stackHeightTop   = 1
	stackHeightInner = 2
)

// ---------------------------------------------------------------------------
// Conversions
// ---------------------------------------------------------------------------

// toBlock builds the block header. signatures are attached when provided (sol_getBlocksByInterval
// needs them; sol_getBlock does not).
//
// NOTE: ParentSlot is intentionally left zero. BigQuery has no parent-slot column, and deriving it
// (MAX(slot) below the block) costs a full-column scan of the Blocks table per block — too expensive
// for a value downstream consumers of the historical (archival) tier do not rely on.
func (b blockRow) toBlock(signatures []solana.Signature) (sol.Block, error) {
	blockhash, err := solana.HashFromBase58(b.BlockHash)
	if err != nil {
		return sol.Block{}, errors.Wrapf(err, "parse blockhash of slot %d", b.Slot)
	}
	previous, err := solana.HashFromBase58(b.PreviousBlockHash)
	if err != nil {
		return sol.Block{}, errors.Wrapf(err, "parse previous blockhash of slot %d", b.Slot)
	}
	height := uint64(b.Height)
	result := &rpc.GetBlockResult{
		Blockhash:         blockhash,
		PreviousBlockhash: previous,
		BlockHeight:       &height,
		Signatures:        signatures,
	}
	if b.BlockTimestamp.Valid {
		bt := solana.UnixTimeSeconds(b.BlockTimestamp.Timestamp.Unix())
		result.BlockTime = &bt
	}
	return sol.Block{Slot: uint64(b.Slot), GetBlockResult: result}, nil
}

// toWrappedTransaction reconstructs a full parsed transaction from its Transactions row and the
// Instructions rows belonging to it.
func toWrappedTransaction(tx txRow, instrs []instructionRow) (sol.WrappedTransaction, error) {
	sig, err := solana.SignatureFromBase58(tx.Signature)
	if err != nil {
		return sol.WrappedTransaction{}, errors.Wrapf(err, "parse signature %s", tx.Signature)
	}

	accountKeys := make([]rpc.ParsedMessageAccount, len(tx.Accounts))
	for i, a := range tx.Accounts {
		pk, err := solana.PublicKeyFromBase58(a.Pubkey)
		if err != nil {
			return sol.WrappedTransaction{}, errors.Wrapf(err, "parse account %s of %s", a.Pubkey, tx.Signature)
		}
		accountKeys[i] = rpc.ParsedMessageAccount{PublicKey: pk, Signer: a.Signer, Writable: a.Writable}
	}

	top, inner := splitInstructions(instrs)
	topInstructions, err := buildInstructions(top, stackHeightTop)
	if err != nil {
		return sol.WrappedTransaction{}, errors.Wrapf(err, "build top instructions of %s", tx.Signature)
	}
	innerInstructions, err := buildInnerInstructions(inner)
	if err != nil {
		return sol.WrappedTransaction{}, errors.Wrapf(err, "build inner instructions of %s", tx.Signature)
	}

	preTB, err := toTokenBalances(tx.PreTokenBalances)
	if err != nil {
		return sol.WrappedTransaction{}, errors.Wrapf(err, "pre token balances of %s", tx.Signature)
	}
	postTB, err := toTokenBalances(tx.PostTokenBalances)
	if err != nil {
		return sol.WrappedTransaction{}, errors.Wrapf(err, "post token balances of %s", tx.Signature)
	}
	pre, post := toBalances(tx.Accounts, tx.BalanceChanges)
	errVal, status := toErrAndStatus(tx.Status, tx.Err)

	var computeUnits *uint64
	if tx.ComputeUnits.Valid {
		c := uint64(tx.ComputeUnits.Int64)
		computeUnits = &c
	}

	transaction := &rpc.ParsedTransaction{
		Signatures: []solana.Signature{sig},
		Message: rpc.ParsedMessage{
			AccountKeys:     accountKeys,
			Instructions:    topInstructions,
			RecentBlockHash: tx.RecentBlockHash,
		},
	}
	meta := &rpc.ParsedTransactionMeta{
		Err:                  errVal,
		Fee:                  uint64(tx.Fee),
		PreBalances:          pre,
		PostBalances:         post,
		InnerInstructions:    innerInstructions,
		PreTokenBalances:     preTB,
		PostTokenBalances:    postTB,
		LogMessages:          tx.LogMessages,
		Status:               status,
		Rewards:              []rpc.BlockReward{},
		ComputeUnitsConsumed: computeUnits,
	}
	return sol.WrappedTransaction{
		TransactionIndex: uint32(tx.Index),
		Signature:        sig,
		// §5.1: BigQuery has no version column; default legacy. Mis-labeling a v0 tx as legacy only
		// affects the version field; instructions/accounts/balances are correct regardless.
		Version:     rpc.LegacyTransactionVersion,
		Transaction: transaction,
		Meta:        meta,
	}, nil
}

func sortInt64(s []int64) {
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
}

func sortByTxIndex(txs []sol.WrappedTransaction) {
	sort.Slice(txs, func(i, j int) bool { return txs[i].TransactionIndex < txs[j].TransactionIndex })
}

// splitInstructions partitions instructions into top-level (parent_index NULL) and inner (grouped
// by parent_index), each sorted ascending by index.
func splitInstructions(instrs []instructionRow) (top []instructionRow, inner map[int64][]instructionRow) {
	inner = make(map[int64][]instructionRow)
	for _, in := range instrs {
		if in.ParentIndex.Valid {
			inner[in.ParentIndex.Int64] = append(inner[in.ParentIndex.Int64], in)
		} else {
			top = append(top, in)
		}
	}
	sort.Slice(top, func(i, j int) bool { return top[i].Index < top[j].Index })
	for k := range inner {
		group := inner[k]
		sort.Slice(group, func(i, j int) bool { return group[i].Index < group[j].Index })
	}
	return top, inner
}

func buildInstructions(rows []instructionRow, stackHeight int64) ([]*rpc.ParsedInstruction, error) {
	out := make([]*rpc.ParsedInstruction, 0, len(rows))
	for _, in := range rows {
		pi, err := buildInstruction(in, stackHeight)
		if err != nil {
			return nil, err
		}
		out = append(out, pi)
	}
	return out, nil
}

// buildInnerInstructions builds the meta.innerInstructions list, ordered by parent index ascending.
func buildInnerInstructions(inner map[int64][]instructionRow) ([]rpc.ParsedInnerInstruction, error) {
	if len(inner) == 0 {
		return nil, nil
	}
	parents := make([]int64, 0, len(inner))
	for p := range inner {
		parents = append(parents, p)
	}
	sort.Slice(parents, func(i, j int) bool { return parents[i] < parents[j] })

	out := make([]rpc.ParsedInnerInstruction, 0, len(parents))
	for _, p := range parents {
		instructions, err := buildInstructions(inner[p], stackHeightInner)
		if err != nil {
			return nil, err
		}
		out = append(out, rpc.ParsedInnerInstruction{Index: uint64(p), Instructions: instructions})
	}
	return out, nil
}

// buildInstruction converts a single instruction row to an rpc.ParsedInstruction by marshalling a
// wire representation and unmarshalling it back (the only way to populate the unexported
// InstructionInfoEnvelope in the parsed case).
func buildInstruction(in instructionRow, stackHeight int64) (*rpc.ParsedInstruction, error) {
	w := wireInstruction{ProgramID: in.ProgramID, StackHeight: stackHeight}
	if in.Program.Valid && in.Program.StringVal != "" {
		// Parsed instruction: program + parsed{type, info}.
		info := make(map[string]any)
		for _, p := range in.Params {
			if !p.Key.Valid || p.Key.StringVal == "" {
				continue
			}
			var v any
			if p.Value.Valid {
				if err := json.Unmarshal([]byte(p.Value.StringVal), &v); err != nil {
					// Not valid JSON: keep the raw string.
					v = p.Value.StringVal
				}
			}
			info[p.Key.StringVal] = v
		}
		w.Program = in.Program.StringVal
		w.Parsed = &wireParsed{Type: in.InstructionType.StringVal, Info: info}
	} else {
		// Unparsed instruction: data + accounts.
		if in.Data.Valid {
			w.Data = in.Data.StringVal
		}
		for _, a := range in.Accounts {
			if a != "" {
				w.Accounts = append(w.Accounts, a)
			}
		}
	}

	raw, err := json.Marshal(w)
	if err != nil {
		return nil, errors.Wrap(err, "marshal wire instruction")
	}
	var pi rpc.ParsedInstruction
	if err := json.Unmarshal(raw, &pi); err != nil {
		return nil, errors.Wrap(err, "unmarshal parsed instruction")
	}
	return &pi, nil
}

// toBalances projects per-account balance changes onto the accountKeys order, producing the
// preBalances/postBalances arrays (§5.4). Accounts without a recorded change default to 0.
func toBalances(accounts []accountRow, changes []balanceChangeRow) (pre, post []uint64) {
	byAccount := make(map[string]balanceChangeRow, len(changes))
	for _, c := range changes {
		byAccount[c.Account] = c
	}
	pre = make([]uint64, len(accounts))
	post = make([]uint64, len(accounts))
	for i, a := range accounts {
		if c, ok := byAccount[a.Pubkey]; ok {
			pre[i] = uint64(c.Before)
			post[i] = uint64(c.After)
		}
	}
	return pre, post
}

func toTokenBalances(rows []tokenBalanceRow) ([]rpc.TokenBalance, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	out := make([]rpc.TokenBalance, 0, len(rows))
	for _, t := range rows {
		mint, err := solana.PublicKeyFromBase58(t.Mint)
		if err != nil {
			return nil, errors.Wrapf(err, "parse token mint %s", t.Mint)
		}
		tb := rpc.TokenBalance{
			AccountIndex:  uint16(t.AccountIndex),
			Mint:          mint,
			UiTokenAmount: uiTokenAmount(t.Amount, uint8(t.Decimals)),
			// ProgramId left nil (§5.5): BigQuery does not record the token program.
		}
		if t.Owner.Valid && t.Owner.StringVal != "" {
			owner, err := solana.PublicKeyFromBase58(t.Owner.StringVal)
			if err != nil {
				return nil, errors.Wrapf(err, "parse token owner %s", t.Owner.StringVal)
			}
			tb.Owner = &owner
		}
		out = append(out, tb)
	}
	return out, nil
}

// uiTokenAmount computes uiAmount/uiAmountString from the raw amount and decimals (§3.4). A zero
// amount yields uiAmount=nil and uiAmountString="0", matching the RPC.
func uiTokenAmount(amount string, decimals uint8) *rpc.UiTokenAmount {
	ui := &rpc.UiTokenAmount{Amount: amount, Decimals: decimals, UiAmountString: "0"}
	amt, ok := new(big.Int).SetString(amount, 10)
	if !ok || amt.Sign() == 0 {
		return ui
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	rat := new(big.Rat).SetFrac(amt, scale)
	f, _ := rat.Float64()
	ui.UiAmount = &f
	ui.UiAmountString = trimDecimal(rat.FloatString(int(decimals)))
	return ui
}

// trimDecimal removes trailing zeros (and a trailing dot) from a fixed-point decimal string.
func trimDecimal(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}

// toErrAndStatus maps the BigQuery status/err columns to the RPC meta.err and meta.status (§5.10).
// Success => err=nil, status={"Ok":null}. Failure => err is the BigQuery string (as json.RawMessage
// when it is itself valid JSON, else the raw string), status={"Err": <that value>}.
func toErrAndStatus(status string, errStr bigquery.NullString) (any, rpc.DeprecatedTransactionMetaStatus) {
	if status == "Success" || !errStr.Valid || errStr.StringVal == "" {
		return nil, rpc.DeprecatedTransactionMetaStatus{"Ok": nil}
	}
	var errVal any
	if s := errStr.StringVal; json.Valid([]byte(s)) {
		errVal = json.RawMessage(s)
	} else {
		errVal = s
	}
	return errVal, rpc.DeprecatedTransactionMetaStatus{"Err": errVal}
}
