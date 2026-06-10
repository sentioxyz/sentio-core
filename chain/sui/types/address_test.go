package types

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressStringForms(t *testing.T) {
	a := StrToAddressMust("0x5")
	assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000005", a.String())
	assert.Equal(t, "0x5", a.ShortString())
}

func TestAddressJSONRoundTrip(t *testing.T) {
	const full = "0xc16ecefaeeeba3d9d1ccce47751e266e0e362ee418796d2f494bf843c7855e92"
	a := StrToAddressMust(full)

	b, err := json.Marshal(a)
	require.NoError(t, err)
	assert.Equal(t, `"`+full+`"`, string(b))

	var got Address
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, a, got)
}

func TestStrToAddressErrors(t *testing.T) {
	_, err := StrToAddress("not-hex")
	assert.Error(t, err)
}
