package bq

import (
	"encoding/json"
	"testing"

	"cloud.google.com/go/bigquery"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ns(s string) bigquery.NullString { return bigquery.NullString{StringVal: s, Valid: true} }
func ni(n int64) bigquery.NullInt64   { return bigquery.NullInt64{Int64: n, Valid: true} }

func TestBuildInstruction_Parsed(t *testing.T) {
	// From instructions.json: a system "transfer" top-level instruction.
	in := instructionRow{
		ProgramID:       "11111111111111111111111111111111",
		Program:         ns("system"),
		InstructionType: ns("transfer"),
		Accounts:        []string{""},
		Params: []paramRow{
			{Key: ns("destination"), Value: ns(`"57314eWva7RsP4bJANfVfWdSgdLMDGVcbGHCsVB7MRvf"`)},
			{Key: ns("lamports"), Value: ns("5000000")},
			{Key: ns("source"), Value: ns(`"6h3xQ9sBEwoNgCZSwHinBNShpikQeeKL67mYgXqeFJe2"`)},
		},
	}
	pi, err := buildInstruction(in, stackHeightTop)
	require.NoError(t, err)
	got, err := json.Marshal(pi)
	require.NoError(t, err)

	want := `{
		"parsed": {
			"info": {
				"destination": "57314eWva7RsP4bJANfVfWdSgdLMDGVcbGHCsVB7MRvf",
				"lamports": 5000000,
				"source": "6h3xQ9sBEwoNgCZSwHinBNShpikQeeKL67mYgXqeFJe2"
			},
			"type": "transfer"
		},
		"program": "system",
		"programId": "11111111111111111111111111111111",
		"stackHeight": 1
	}`
	assert.JSONEq(t, want, string(got))
}

func TestBuildInstruction_Unparsed(t *testing.T) {
	// From instructions.json: a ComputeBudget instruction (program/parsed NULL).
	in := instructionRow{
		ProgramID: "ComputeBudget111111111111111111111111111111",
		Data:      ns("Eorn15"),
		Accounts:  []string{"jitodontfront111111111116111111111111165521"},
		Params:    []paramRow{{}}, // null key/value placeholder
	}
	pi, err := buildInstruction(in, stackHeightTop)
	require.NoError(t, err)
	assert.Empty(t, pi.Program)
	assert.Nil(t, pi.Parsed)
	got, err := json.Marshal(pi)
	require.NoError(t, err)
	want := `{
		"accounts": ["jitodontfront111111111116111111111111165521"],
		"data": "Eorn15",
		"programId": "ComputeBudget111111111111111111111111111111",
		"stackHeight": 1
	}`
	assert.JSONEq(t, want, string(got))
}

func TestSplitInstructionsAndInner(t *testing.T) {
	rows := []instructionRow{
		{Index: 2, ProgramID: "11111111111111111111111111111111"},
		{Index: 0, ProgramID: "11111111111111111111111111111111"},
		{Index: 1, ParentIndex: bigquery.NullInt64{Int64: 3, Valid: true}, ProgramID: "11111111111111111111111111111111"},
		{Index: 0, ParentIndex: bigquery.NullInt64{Int64: 3, Valid: true}, ProgramID: "11111111111111111111111111111111"},
		{Index: 0, ParentIndex: bigquery.NullInt64{Int64: 2, Valid: true}, ProgramID: "11111111111111111111111111111111"},
	}
	top, inner := splitInstructions(rows)
	require.Len(t, top, 2)
	assert.Equal(t, int64(0), top[0].Index)
	assert.Equal(t, int64(2), top[1].Index)

	innerInstrs, err := buildInnerInstructions(inner)
	require.NoError(t, err)
	require.Len(t, innerInstrs, 2)
	// Ordered by parent index ascending: parent 2 then parent 3.
	assert.Equal(t, uint64(2), innerInstrs[0].Index)
	assert.Equal(t, uint64(3), innerInstrs[1].Index)
	// Parent 3's two inner instructions sorted by index (0, 1); all inner stackHeight=2.
	require.Len(t, innerInstrs[1].Instructions, 2)
	assert.Equal(t, int64(stackHeightInner), innerInstrs[1].Instructions[0].StackHeight)
}

func TestUITokenAmount(t *testing.T) {
	cases := []struct {
		amount   string
		decimals uint8
		wantStr  string
		wantNil  bool
	}{
		{"640216463", 9, "0.640216463", false},
		{"2748249552003", 6, "2748249.552003", false},
		{"5000000", 9, "0.005", false},
		{"0", 6, "0", true},
		{"274105645712834", 6, "274105645.712834", false},
	}
	for _, c := range cases {
		ui := uiTokenAmount(c.amount, c.decimals)
		assert.Equal(t, c.amount, ui.Amount, c.amount)
		assert.Equal(t, c.wantStr, ui.UiAmountString, c.amount)
		if c.wantNil {
			assert.Nil(t, ui.UiAmount, c.amount)
		} else {
			require.NotNil(t, ui.UiAmount, c.amount)
		}
	}
}

func TestErrAndStatus(t *testing.T) {
	// Success.
	errVal, status := toErrAndStatus("Success", bigquery.NullString{})
	assert.Nil(t, errVal)
	assert.Contains(t, status, "Ok")

	// Failure, plain string error => kept as string.
	errVal, status = toErrAndStatus("Fail", ns("Error processing Instruction 1: custom program error: 0x14"))
	assert.Equal(t, "Error processing Instruction 1: custom program error: 0x14", errVal)
	assert.Equal(t, errVal, status["Err"])

	// Failure, JSON error => kept as raw JSON.
	errVal, _ = toErrAndStatus("Fail", ns(`{"InstructionError":[1,{"Custom":20}]}`))
	raw, ok := errVal.(json.RawMessage)
	require.True(t, ok)
	assert.JSONEq(t, `{"InstructionError":[1,{"Custom":20}]}`, string(raw))
}

func TestToBalancesProjection(t *testing.T) {
	accounts := []accountRow{{Pubkey: "A"}, {Pubkey: "B"}, {Pubkey: "C"}}
	// Out-of-order changes; C has none.
	changes := []balanceChangeRow{{Account: "B", Before: ni(20), After: ni(25)}, {Account: "A", Before: ni(10), After: ni(15)}}
	pre, post := toBalances(accounts, changes)
	assert.Equal(t, []uint64{10, 20, 0}, pre)
	assert.Equal(t, []uint64{15, 25, 0}, post)

	// NULL before/after (the public dataset leaves some scalars NULL) default to 0, not a scan error.
	preN, postN := toBalances([]accountRow{{Pubkey: "A"}}, []balanceChangeRow{{Account: "A"}})
	assert.Equal(t, []uint64{0}, preN)
	assert.Equal(t, []uint64{0}, postN)
}

// NULL scalars from the public dataset must default cleanly instead of failing the scan/convert.
func TestToTokenBalancesNullable(t *testing.T) {
	mint := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	rows := []tokenBalanceRow{
		// account_index / decimals / amount all NULL.
		{Mint: ns(mint)},
		// fully populated.
		{AccountIndex: ni(3), Mint: ns(mint), Amount: ns("1000000"), Decimals: ni(6)},
		// no mint => dropped.
		{AccountIndex: ni(1)},
	}
	out, err := toTokenBalances(rows)
	require.NoError(t, err)
	require.Len(t, out, 2) // the mint-less row is skipped

	assert.Equal(t, uint16(0), out[0].AccountIndex)
	assert.Equal(t, "0", out[0].UiTokenAmount.Amount)
	assert.Equal(t, uint8(0), out[0].UiTokenAmount.Decimals)

	assert.Equal(t, uint16(3), out[1].AccountIndex)
	assert.Equal(t, "1000000", out[1].UiTokenAmount.Amount)
	assert.Equal(t, "1", out[1].UiTokenAmount.UiAmountString)
}

// NULL value columns on the transaction (status, fee, signer/writable) default cleanly instead of
// failing the scan/convert. Identity columns (signature, pubkey) stay required.
func TestToWrappedTransactionNullValueColumns(t *testing.T) {
	tx := txRow{
		Signature: "3WeJDhD1wXfY1qmHfh7yJotHV2dH7XnxXb7oY4xmK2yu3PNQJzm4oH6SHYiNTHQ48CJx3xhmaeEo45Jq8WsGyywv",
		Accounts:  []accountRow{{Pubkey: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"}}, // signer/writable NULL
		// Status and Fee left NULL.
	}
	wt, err := toWrappedTransaction(tx, nil)
	require.NoError(t, err)
	require.NotNil(t, wt.Meta)
	assert.Equal(t, uint64(0), wt.Meta.Fee)  // NULL fee → 0
	assert.Nil(t, wt.Meta.Err)               // NULL status → success
	assert.Contains(t, wt.Meta.Status, "Ok") // {"Ok": null}
	require.Len(t, wt.Transaction.Message.AccountKeys, 1)
	assert.False(t, wt.Transaction.Message.AccountKeys[0].Signer)   // NULL → false
	assert.False(t, wt.Transaction.Message.AccountKeys[0].Writable) // NULL → false
}

// A NULL block height maps to a nil BlockHeight (Solana's blockHeight is legitimately nullable),
// not a scan error.
func TestToBlockNullHeight(t *testing.T) {
	row := blockRow{
		Slot:              100,
		BlockHash:         "8a2rWas3z1EQN1yWnkTZxMsGytWfSUatXKcV84vdcTj7",
		PreviousBlockHash: "5t3qB6gVEf9ejNneXnNGarTBQYrRD7f1dVyRESysEBiA",
		// Height left NULL
	}
	blk, err := row.toBlock()
	require.NoError(t, err)
	require.NotNil(t, blk.GetBlockResult)
	assert.Nil(t, blk.BlockHeight)
}

// Sanity: meta status carries a JSON-marshalable Ok shape for success.
func TestStatusMarshal(t *testing.T) {
	_, status := toErrAndStatus("Success", bigquery.NullString{})
	b, err := json.Marshal(rpc.DeprecatedTransactionMetaStatus(status))
	require.NoError(t, err)
	assert.JSONEq(t, `{"Ok":null}`, string(b))
}
