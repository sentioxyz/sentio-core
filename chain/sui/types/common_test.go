package types

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/sui/types/serde"
)

func TestNumberJSON(t *testing.T) {
	// marshals to a quoted decimal string
	n := StringToNumber("1999555")
	b, err := json.Marshal(n)
	require.NoError(t, err)
	assert.Equal(t, `"1999555"`, string(b))

	// unmarshals from BOTH a quoted string and a bare number
	for _, in := range []string{`"1999555"`, `1999555`} {
		var got Number
		require.NoError(t, json.Unmarshal([]byte(in), &got))
		assert.Equal(t, uint64(1999555), got.Uint64())
	}

	var bad Number
	assert.Error(t, json.Unmarshal([]byte(`"not-a-number"`), &bad))
}

func TestNumberBCSIsLittleEndianU64(t *testing.T) {
	n := StringToNumber("1126")
	b, err := serde.Marshal(n)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x66, 0x04, 0, 0, 0, 0, 0, 0}, b)

	var got Number
	require.NoError(t, serde.Unmarshal(b, &got))
	assert.Equal(t, uint64(1126), got.Uint64())
}

func TestBase64DataJSONAndBCS(t *testing.T) {
	d, err := NewBase64Data("aGVsbG8=") // "hello"
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), d.Data())

	// JSON is the base64 string
	b, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Equal(t, `"aGVsbG8="`, string(b))
	var fromJSON Base64Data
	require.NoError(t, json.Unmarshal(b, &fromJSON))
	assert.Equal(t, d, fromJSON)

	// BCS is a length-prefixed byte slice (ULEB128 len + bytes)
	enc, err := serde.Marshal(&d)
	require.NoError(t, err)
	assert.Equal(t, append([]byte{0x05}, []byte("hello")...), enc)
	var fromBCS Base64Data
	require.NoError(t, serde.Unmarshal(enc, &fromBCS))
	assert.Equal(t, d, fromBCS)
}

func TestBase58DataJSON(t *testing.T) {
	const s = "JxF12TrwUP45BMd"
	d, err := NewBase58Data(s)
	require.NoError(t, err)

	b, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Equal(t, `"`+s+`"`, string(b))

	var got Base58Data
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, d, got)
}

func TestUint8SliceJSONIsNumberArray(t *testing.T) {
	s := Uint8Slice{0, 4, 255}
	b, err := json.Marshal(s)
	require.NoError(t, err)
	assert.Equal(t, `[0,4,255]`, string(b))

	var got Uint8Slice
	require.NoError(t, json.Unmarshal([]byte(`[0,4,255]`), &got))
	assert.Equal(t, s, got)

	// out-of-range element is rejected
	var bad Uint8Slice
	assert.Error(t, json.Unmarshal([]byte(`[256]`), &bad))
}

func TestUint8SliceBCSIsByteVec(t *testing.T) {
	s := Uint8Slice{1, 2, 3}
	enc, err := serde.Marshal(&s)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x03, 1, 2, 3}, enc) // ULEB len + raw bytes

	var got Uint8Slice
	require.NoError(t, serde.Unmarshal(enc, &got))
	assert.Equal(t, s, got)
}
