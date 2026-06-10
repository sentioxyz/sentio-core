package types

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/sui/types/serde"
)

func u16p(v uint16) *uint16 { return &v }

// TestArgumentJSONAndBCS covers every Argument variant: json form, and a
// BCS encode/decode round-trip (Argument is a hand-written bcs.Enum).
func TestArgumentJSONAndBCS(t *testing.T) {
	tru := true
	cases := []struct {
		name string
		arg  Argument
		json string
	}{
		{"GasCoin", Argument{GasCoin: &tru}, `"GasCoin"`},
		{"Input", Argument{Input: u16p(1)}, `{"Input":1}`},
		{"Result", Argument{Result: u16p(2)}, `{"Result":2}`},
		{"NestedResult", Argument{NestedResult: []uint16{3, 4}}, `{"NestedResult":[3,4]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// JSON
			b, err := json.Marshal(tc.arg)
			require.NoError(t, err)
			assert.JSONEq(t, tc.json, string(b))
			var fromJSON Argument
			require.NoError(t, json.Unmarshal([]byte(tc.json), &fromJSON))
			assert.Equal(t, tc.arg, fromJSON)

			// BCS round-trip
			enc, err := serde.Marshal(&tc.arg)
			require.NoError(t, err)
			var fromBCS Argument
			require.NoError(t, serde.Unmarshal(enc, &fromBCS))
			assert.Equal(t, tc.arg, fromBCS)
		})
	}
}

func TestArgumentBCSVariantIndices(t *testing.T) {
	// GasCoin=0 (no payload), Input=1, Result=2, NestedResult=3, each followed by
	// little-endian u16 payload(s).
	tru := true
	for _, tc := range []struct {
		arg  Argument
		want []byte
	}{
		{Argument{GasCoin: &tru}, []byte{0x00}},
		{Argument{Input: u16p(1)}, []byte{0x01, 0x01, 0x00}},
		{Argument{Result: u16p(2)}, []byte{0x02, 0x02, 0x00}},
		{Argument{NestedResult: []uint16{3, 4}}, []byte{0x03, 0x03, 0x00, 0x04, 0x00}},
	} {
		enc, err := serde.Marshal(&tc.arg)
		require.NoError(t, err)
		assert.Equal(t, tc.want, enc)
	}
}

func TestDecodeMakeMoveVec(t *testing.T) {
	raw := []byte{0x05, // MakeMoveVec variant
		0x00, // optional field TypeTag, present=false
		0x04, // Argument slice of size 4
		0x01, 0x01, 0x00,
		0x01, 0x02, 0x00,
		0x01, 0x03, 0x00,
		0x01, 0x04, 0x00}

	command := &Command{}
	if err := serde.Unmarshal(raw, command); err != nil {
		t.Fatal(err)
	}

	var v1, v2, v3, v4 uint16
	v1, v2, v3, v4 = 1, 2, 3, 4
	assert.Equal(t, &Command{
		MakeMoveVec: &MakeMoveVec{
			Args: []Argument{
				{Input: &v1},
				{Input: &v2},
				{Input: &v3},
				{Input: &v4},
			},
		},
	}, command)
}
