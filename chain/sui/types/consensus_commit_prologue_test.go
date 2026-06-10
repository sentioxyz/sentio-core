package types

import (
	"bytes"
	"os"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsensusCommitPrologueRoundTrip validates that ConsensusCommitPrologue
// transactions decode from and re-encode byte-for-byte to BCS, on both Sui and
// IOTA, using real testnet samples captured via json-rpc showRawInput.
//
// It exercises both the raw BCS round-trip (DeriveAux's decode + re-encode) and
// the getSlot/TxSanityCheck path (json.Unmarshal of the reply -> DeriveAux ->
// EncodeSenderSignedData), which is what gates removal from uncompletedKinds.
func TestConsensusCommitPrologueRoundTrip(t *testing.T) {
	cases := []struct {
		name      string
		file      string
		variation Variation
		wantKind  string
	}{
		{"sui_v4", "testdata/ccp-sui-v4.json", VariationSUI, "ConsensusCommitPrologueV4"},
		{"iota_v1", "testdata/ccp-iota-v1.json", VariationIOTA, "ConsensusCommitPrologueV1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.file)
			require.NoError(t, err)

			var tx TransactionResponseV1
			require.NoError(t, json.Unmarshal(data, &tx))
			require.NotNil(t, tx.Transaction)
			require.NotNil(t, tx.Transaction.Data)
			require.NotNil(t, tx.Transaction.Data.V1)
			require.NotNil(t, tx.Transaction.Data.V1.Kind)
			assert.Equal(t, tc.wantKind, tx.Transaction.Data.V1.Kind.Kind())

			raw := tx.RawTransaction.Data()
			require.NotEmpty(t, raw)

			// 1. pure BCS round-trip: decode the raw bytes then re-encode them.
			decoded, err := DecodeSenderSignedData(raw, tc.variation)
			require.NoError(t, err)
			reencoded, err := EncodeSenderSignedData(decoded, tc.variation)
			require.NoError(t, err)
			assert.True(t, bytes.Equal(reencoded, raw), "raw BCS round-trip mismatch")

			// 2. getSlot mirror: derive aux info onto the json-parsed transaction
			// then encode it (== TxSanityCheck).
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
