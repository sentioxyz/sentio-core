package types

import (
	"bytes"
	"os"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTransactionKindRoundTrip validates, against real testnet/mainnet samples,
// that every supported TransactionKind decodes from and re-encodes byte-for-byte
// to BCS on its chain. Samples live in testdata/{sui,iota}/<kind>.json and were
// captured via json-rpc showRawInput (see testdata/README.md for provenance).
//
// Each case exercises two paths:
//  1. raw BCS round-trip: DecodeSenderSignedData(raw) -> EncodeSenderSignedData == raw
//  2. the getSlot path: json.Unmarshal(reply) -> DeriveAux -> EncodeSenderSignedData == raw
//
// bcsValidated=false marks kinds whose Go type is not yet exact (Genesis has an
// unmodeled GenesisTransaction payload); for those we only assert the json reply
// decodes and reports the expected Kind(). They remain in uncompletedKinds.
func TestTransactionKindRoundTrip(t *testing.T) {
	cases := []struct {
		file         string
		variation    Variation
		wantKind     string
		bcsValidated bool
	}{
		{"testdata/sui/programmable.json", VariationSUI, "ProgrammableTransaction", true},
		{"testdata/sui/change-epoch.json", VariationSUI, "ChangeEpoch", true},
		{"testdata/sui/consensus-commit-prologue.json", VariationSUI, "ConsensusCommitPrologue", true},
		{"testdata/sui/consensus-commit-prologue-v2.json", VariationSUI, "ConsensusCommitPrologueV2", true},
		{"testdata/sui/consensus-commit-prologue-v3.json", VariationSUI, "ConsensusCommitPrologueV3", true},
		{"testdata/sui/consensus-commit-prologue-v4.json", VariationSUI, "ConsensusCommitPrologueV4", true},
		{"testdata/sui/randomness-state-update.json", VariationSUI, "RandomnessStateUpdate", true},
		{"testdata/sui/authenticator-state-update.json", VariationSUI, "AuthenticatorStateUpdate", true},
		{"testdata/sui/end-of-epoch.json", VariationSUI, "EndOfEpochTransaction", true},
		{"testdata/sui/genesis.json", VariationSUI, "Genesis", false},
		{"testdata/iota/programmable.json", VariationIOTA, "ProgrammableTransaction", true},
		{"testdata/iota/consensus-commit-prologue-v1.json", VariationIOTA, "ConsensusCommitPrologueV1", true},
		{"testdata/iota/randomness-state-update.json", VariationIOTA, "RandomnessStateUpdate", true},
		{"testdata/iota/end-of-epoch.json", VariationIOTA, "EndOfEpochTransaction", true},
		{"testdata/iota/genesis.json", VariationIOTA, "Genesis", false},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile(tc.file)
			require.NoError(t, err)

			var tx TransactionResponseV1
			require.NoError(t, json.Unmarshal(data, &tx))
			require.NotNil(t, tx.Transaction)
			require.NotNil(t, tx.Transaction.Data)
			require.NotNil(t, tx.Transaction.Data.V1)
			require.NotNil(t, tx.Transaction.Data.V1.Kind)
			assert.Equal(t, tc.wantKind, tx.Transaction.Data.V1.Kind.Kind())

			if !tc.bcsValidated {
				return
			}

			raw := tx.RawTransaction.Data()
			require.NotEmpty(t, raw)

			// 1. raw BCS round-trip (DeriveAux's decode + re-encode).
			decoded, err := DecodeSenderSignedData(raw, tc.variation)
			require.NoError(t, err)
			reencoded, err := EncodeSenderSignedData(decoded, tc.variation)
			require.NoError(t, err)
			assert.True(t, bytes.Equal(reencoded, raw), "raw BCS round-trip mismatch")

			// 2. getSlot mirror: derive aux onto the json-parsed transaction then
			// encode it (== TxSanityCheck).
			require.NoError(t, DeriveAuxInformationFromBCSV1(tx.Transaction.Data.V1, raw, tc.variation))
			tx.Transaction.Intent = &EmptyIntentMessage
			encoded, err := EncodeSenderSignedData(&SenderSignedData{
				Transactions: []SenderSignedTransaction{*tx.Transaction},
			}, tc.variation)
			require.NoError(t, err)
			assert.True(t, bytes.Equal(encoded, raw), "sanity-check round-trip mismatch")
		})
	}
}
