package types

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/sui/types/serde"
)

// TestCallArgFundsWithdrawalJSON checks CallArg.FundsWithdrawal json round-trips
// and that optional sub-fields are omitted when nil.
func TestCallArgFundsWithdrawalJSON(t *testing.T) {
	raw := `
[
    {
        "type": "fundsWithdrawal",
        "reservation": {
            "maxAmountU64": "12345"
        },
        "typeArg": {
            "balance": "0x2::sui::SUI"
        },
        "withdrawFrom": "sender"
    }
]
`
	var inputs []CallArg
	assert.NoError(t, json.Unmarshal([]byte(raw), &inputs))
	require.NotNil(t, inputs[0].FundsWithdrawal)
	fw := inputs[0].FundsWithdrawal
	require.NotNil(t, fw.Reservation)
	assert.Equal(t, uint64(12345), *fw.Reservation.MaxAmountU64)
	require.NotNil(t, fw.TypeArg)
	require.NotNil(t, fw.TypeArg.Balance)
	assert.Equal(t, "0x2::sui::SUI", fw.TypeArg.Balance.String())
	require.NotNil(t, fw.WithdrawFrom)
	assert.NotNil(t, fw.WithdrawFrom.Sender)
	assert.Nil(t, fw.WithdrawFrom.Sponsor)

	b, err := json.Marshal(inputs)
	assert.NoError(t, err)
	assert.Equal(t, `[{"reservation":{"maxAmountU64":"12345"},"type":"fundsWithdrawal","typeArg":{"balance":"0x2::sui::SUI"},"withdrawFrom":"sender"}]`, string(b))

	inputs[0].FundsWithdrawal.WithdrawFrom = nil
	b, err = json.Marshal(inputs)
	assert.NoError(t, err)
	assert.Equal(t, `[{"reservation":{"maxAmountU64":"12345"},"type":"fundsWithdrawal","typeArg":{"balance":"0x2::sui::SUI"}}]`, string(b))

	inputs[0].FundsWithdrawal.TypeArg = nil
	b, err = json.Marshal(inputs)
	assert.NoError(t, err)
	assert.Equal(t, `[{"reservation":{"maxAmountU64":"12345"},"type":"fundsWithdrawal"}]`, string(b))

	inputs[0].FundsWithdrawal.Reservation = nil
	b, err = json.Marshal(inputs)
	assert.NoError(t, err)
	assert.Equal(t, `[{"type":"fundsWithdrawal"}]`, string(b))
}

// Real ValidDuring expiration bytes taken from sui-testnet tx
// EeQQHi8FhWchTqbeY7R464rF6pPKaKAhC8hGXLy3Z9R1 (checkpoint 346619596):
// variant 2; min_epoch=Some(1126); max_epoch=Some(1127); min/max_timestamp=None;
// chain=32-byte digest (length-prefixed); nonce=3091766946.
const validDuringExpHex = "02" +
	"01" + "6604000000000000" + // min_epoch Some(1126)
	"01" + "6704000000000000" + // max_epoch Some(1127)
	"00" + // min_timestamp None
	"00" + // max_timestamp None
	"20" + "4c78adacf2a2f5ad80f27ed7d54aa69d3a78f1ca67fdef9ecf5754f5b8bb77b0" + // chain
	"a29e48b8" // nonce u32 (3091766946)

func Test_TransactionExpiration_ValidDuring_RoundTrip(t *testing.T) {
	raw, err := hex.DecodeString(validDuringExpHex)
	assert.NoError(t, err)

	var exp TransactionExpiration
	_, err = exp.UnmarshalBCS(bytes.NewReader(raw))
	assert.NoError(t, err)

	// decoded fields match ground truth
	assert.Nil(t, exp.None)
	assert.Nil(t, exp.Epoch)
	if assert.NotNil(t, exp.ValidDuring) {
		vd := exp.ValidDuring
		assert.Equal(t, uint64(1126), *vd.MinEpoch)
		assert.Equal(t, uint64(1127), *vd.MaxEpoch)
		assert.Nil(t, vd.MinTimestamp)
		assert.Nil(t, vd.MaxTimestamp)
		assert.Len(t, vd.Chain, 32)
		assert.Equal(t, uint32(3091766946), vd.Nonce)
	}

	// re-encode must reproduce the original bytes exactly (TxSanityCheck relies on this)
	buf := bytes.NewBuffer(nil)
	err = serde.Encode(buf, exp)
	assert.NoError(t, err)
	assert.Equal(t, raw, buf.Bytes())
}

// Real Validity expiration bytes taken from sui-testnet tx
// HHsEeTS9bwH2gwdeYUX5ZabW52DEfvNBW6wkpuqkVngs (checkpoint 384542031):
// variant 3; min_epoch=Some(1225); max_epoch=Some(1226); min/max_timestamp=None;
// chain=32-byte digest (length-prefixed); nonce=1597091592;
// allowed_proposers=Some({epoch: 1225, proposers: [46, 61, 80]}).
const validityExpHex = "03" +
	"01" + "c904000000000000" + // min_epoch Some(1225)
	"01" + "ca04000000000000" + // max_epoch Some(1226)
	"00" + // min_timestamp None
	"00" + // max_timestamp None
	"20" + "4c78adacf2a2f5ad80f27ed7d54aa69d3a78f1ca67fdef9ecf5754f5b8bb77b0" + // chain
	"08af315f" + // nonce u32 (1597091592)
	"01" + // allowed_proposers Some
	"c904000000000000" + // epoch 1225
	"03" + "2e000000" + "3d000000" + "50000000" // proposers [46, 61, 80]

func Test_TransactionExpiration_Validity_RoundTrip(t *testing.T) {
	raw, err := hex.DecodeString(validityExpHex)
	assert.NoError(t, err)

	var exp TransactionExpiration
	_, err = exp.UnmarshalBCS(bytes.NewReader(raw))
	assert.NoError(t, err)

	// decoded fields match ground truth
	assert.Nil(t, exp.None)
	assert.Nil(t, exp.Epoch)
	assert.Nil(t, exp.ValidDuring)
	if assert.NotNil(t, exp.Validity) {
		v := exp.Validity
		assert.Equal(t, uint64(1225), *v.MinEpoch)
		assert.Equal(t, uint64(1226), *v.MaxEpoch)
		assert.Nil(t, v.MinTimestamp)
		assert.Nil(t, v.MaxTimestamp)
		assert.Len(t, v.Chain, 32)
		assert.Equal(t, uint32(1597091592), v.Nonce)
		if assert.NotNil(t, v.AllowedProposers) {
			assert.Equal(t, uint64(1225), v.AllowedProposers.Epoch)
			assert.Equal(t, []uint32{46, 61, 80}, v.AllowedProposers.Proposers)
		}
	}

	// re-encode must reproduce the original bytes exactly (TxSanityCheck relies on this)
	buf := bytes.NewBuffer(nil)
	err = serde.Encode(buf, exp)
	assert.NoError(t, err)
	assert.Equal(t, raw, buf.Bytes())
}

// Validity without allowed_proposers must round-trip too (the Option is None).
func Test_TransactionExpiration_Validity_NoAllowedProposers_RoundTrip(t *testing.T) {
	// same as validityExpHex up to the nonce, then allowed_proposers None
	raw, err := hex.DecodeString("03" +
		"01" + "c904000000000000" +
		"01" + "ca04000000000000" +
		"00" + "00" +
		"20" + "4c78adacf2a2f5ad80f27ed7d54aa69d3a78f1ca67fdef9ecf5754f5b8bb77b0" +
		"08af315f" +
		"00")
	assert.NoError(t, err)

	var exp TransactionExpiration
	_, err = exp.UnmarshalBCS(bytes.NewReader(raw))
	assert.NoError(t, err)
	if assert.NotNil(t, exp.Validity) {
		assert.Nil(t, exp.Validity.AllowedProposers)
	}

	buf := bytes.NewBuffer(nil)
	err = serde.Encode(buf, exp)
	assert.NoError(t, err)
	assert.Equal(t, raw, buf.Bytes())
}

func Test_TransactionExpiration_UnknownVariantErrors(t *testing.T) {
	var exp TransactionExpiration
	// variant 4 is not known -> must error, not silently produce an empty value
	_, err := exp.UnmarshalBCS(bytes.NewReader([]byte{0x04}))
	assert.Error(t, err)
}

// TestConsensusDeterminedVersionAssignmentsJSON covers the json-rpc spelling
// quirk: the wire uses "Cancelled" (double l) even though the Rust/Go type uses
// "Canceled". UnmarshalJSON accepts both spellings; MarshalJSON emits the
// json-rpc "Cancelled" form.
func TestConsensusDeterminedVersionAssignmentsJSON(t *testing.T) {
	// variant 0 (CanceledTransactions), both spellings accepted
	for _, in := range []string{`{"CancelledTransactions":[]}`, `{"CanceledTransactions":[]}`} {
		var c ConsensusDeterminedVersionAssignments
		require.NoError(t, json.Unmarshal([]byte(in), &c))
		assert.NotNil(t, c.CanceledTransactions)
		assert.Nil(t, c.CanceledTransactionsV2)
	}

	// variant 1 (CanceledTransactionsV2)
	var v2 ConsensusDeterminedVersionAssignments
	require.NoError(t, json.Unmarshal([]byte(`{"CancelledTransactionsV2":[]}`), &v2))
	assert.NotNil(t, v2.CanceledTransactionsV2)

	// marshal emits the json-rpc "Cancelled" spelling
	out, err := json.Marshal(ConsensusDeterminedVersionAssignments{
		CanceledTransactions: &CanceledTransactions{},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"CancelledTransactions":null}`, string(out))

	// unknown variant errors
	var bad ConsensusDeterminedVersionAssignments
	assert.Error(t, json.Unmarshal([]byte(`{"Whatever":[]}`), &bad))
}

// TestCancelledTransactionsTupleJSON locks in the positional-tuple json shape the
// json-rpc uses for the version-assignment payloads (objects would fail to
// unmarshal, which previously broke ConsensusCommitPrologue parsing on
// checkpoints with cancelled transactions). Versions are bare json numbers.
func TestCancelledTransactionsTupleJSON(t *testing.T) {
	// V1: {"CancelledTransactions": [ [digest, [[objectId, version], ...]] ]}
	v1 := `{"CancelledTransactions":[["E19GFcJn1GVk7J74xTpj16fACPr15aDWb7rz11qJzC3R",[["0x0000000000000000000000000000000000000000000000000000000000000006",1126]]]]}`
	var c1 ConsensusDeterminedVersionAssignments
	require.NoError(t, json.Unmarshal([]byte(v1), &c1))
	require.NotNil(t, c1.CanceledTransactions)
	require.Len(t, c1.CanceledTransactions.Transactions, 1)
	tx1 := c1.CanceledTransactions.Transactions[0]
	assert.Equal(t, "E19GFcJn1GVk7J74xTpj16fACPr15aDWb7rz11qJzC3R", tx1.TxDigest.String())
	require.Len(t, tx1.VersionAssignments, 1)
	assert.Equal(t, uint64(1126), tx1.VersionAssignments[0].Version.Uint64())
	out1, err := json.Marshal(c1)
	require.NoError(t, err)
	assert.JSONEq(t, v1, string(out1))

	// V2: {"CancelledTransactionsV2": [ [digest, [[[objectId, startVersion], version], ...]] ]}
	v2 := `{"CancelledTransactionsV2":[["E19GFcJn1GVk7J74xTpj16fACPr15aDWb7rz11qJzC3R",[[["0x0000000000000000000000000000000000000000000000000000000000000006",1],9223372036854775808]]]]}`
	var c2 ConsensusDeterminedVersionAssignments
	require.NoError(t, json.Unmarshal([]byte(v2), &c2))
	require.NotNil(t, c2.CanceledTransactionsV2)
	require.Len(t, c2.CanceledTransactionsV2.Transactions, 1)
	va := c2.CanceledTransactionsV2.Transactions[0].VersionAssignments
	require.Len(t, va, 1)
	assert.Equal(t, uint64(1), va[0].StartVersion.Uint64())
	assert.Equal(t, uint64(9223372036854775808), va[0].Version.Uint64())
	out2, err := json.Marshal(c2)
	require.NoError(t, err)
	assert.JSONEq(t, v2, string(out2))
}

// ForwardingAddressRegistryCreate is Sui's EndOfEpochTransactionKind variant 13
// (upstream staged snapshot). That index used to be claimed by IOTA's
// ChangeEpochV2, which would have mis-decoded a Sui end-of-epoch transaction.
func Test_EndOfEpoch_ForwardingAddressRegistryCreate_RoundTrip(t *testing.T) {
	raw := []byte{0x0d}

	var eoe EndOfEpochTransactionSingle
	dec := serde.NewDecoderForSelector(bytes.NewReader(raw), string(VariationSUI))
	require.NoError(t, dec.Decode(&eoe))
	require.NotNil(t, eoe.ForwardingAddressRegistryCreate)
	assert.Nil(t, eoe.ChangeEpochV2)

	buf := bytes.NewBuffer(nil)
	enc := serde.NewEncoderForSelector(buf, string(VariationSUI))
	require.NoError(t, enc.Encode(&eoe))
	assert.Equal(t, raw, buf.Bytes())

	// the json-rpc reply spells the unit variant as a bare string
	b, err := json.Marshal(eoe)
	require.NoError(t, err)
	assert.Equal(t, `"ForwardingAddressRegistryCreate"`, string(b))

	var back EndOfEpochTransactionSingle
	require.NoError(t, json.Unmarshal(b, &back))
	assert.NotNil(t, back.ForwardingAddressRegistryCreate)
}
