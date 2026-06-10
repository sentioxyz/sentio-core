package types

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/sui/types/serde"
)

func TestDigestStringRoundTrip(t *testing.T) {
	const s = "13PrnWn4KTma3AyMATzP255eKu8XZkkm4v1nGMGtWV5G"
	d, err := StrToDigest(s)
	require.NoError(t, err)
	assert.Len(t, d, DigestLength)
	// base58 decode then re-encode reproduces the original string
	assert.Equal(t, s, d.String())
}

func TestDigestJSONRoundTrip(t *testing.T) {
	const s = "13PrnWn4KTma3AyMATzP255eKu8XZkkm4v1nGMGtWV5G"
	d := StrToDigestMust(s)

	b, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Equal(t, `"`+s+`"`, string(b))

	var got Digest
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, d, got)
}

// TestDigestBCSIsLengthPrefixed guards the gotcha that a Digest is serialized
// with a ULEB128 length prefix (0x20) rather than 32 raw bytes.
func TestDigestBCSIsLengthPrefixed(t *testing.T) {
	d := StrToDigestMust("13PrnWn4KTma3AyMATzP255eKu8XZkkm4v1nGMGtWV5G")

	enc, err := serde.Marshal(&d)
	require.NoError(t, err)
	require.Len(t, enc, 1+DigestLength)
	assert.Equal(t, byte(0x20), enc[0])
	assert.Equal(t, d[:], enc[1:])

	var got Digest
	require.NoError(t, serde.Unmarshal(enc, &got))
	assert.Equal(t, d, got)
}
