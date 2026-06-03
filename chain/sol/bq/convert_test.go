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
	changes := []balanceChangeRow{{Account: "B", Before: 20, After: 25}, {Account: "A", Before: 10, After: 15}}
	pre, post := toBalances(accounts, changes)
	assert.Equal(t, []uint64{10, 20, 0}, pre)
	assert.Equal(t, []uint64{15, 25, 0}, post)
}

// Sanity: meta status carries a JSON-marshalable Ok shape for success.
func TestStatusMarshal(t *testing.T) {
	_, status := toErrAndStatus("Success", bigquery.NullString{})
	b, err := json.Marshal(rpc.DeprecatedTransactionMetaStatus(status))
	require.NoError(t, err)
	assert.JSONEq(t, `{"Ok":null}`, string(b))
}
