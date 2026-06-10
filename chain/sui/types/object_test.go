package types

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectIDStringForms(t *testing.T) {
	o := StrToObjectIDMust("0x5")
	assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000005", o.String())
}

func TestObjectIDFromFullHex(t *testing.T) {
	const s = "0xc16ecefaeeeba3d9d1ccce47751e266e0e362ee418796d2f494bf843c7855e92"
	id, err := StrToObjectID(s)
	require.NoError(t, err)
	assert.Equal(t, s, id.String())
}

// TestObjectOwnerJSON covers every ObjectOwner variant: json round-trips
// byte-for-byte and GetTypeAndID reports the right (type, id, version).
func TestObjectOwnerJSON(t *testing.T) {
	const addr = "0xc16ecefaeeeba3d9d1ccce47751e266e0e362ee418796d2f494bf843c7855e92"
	cases := []struct {
		name        string
		json        string
		wantType    string
		wantID      string
		wantVersion uint64
	}{
		{"Immutable", `"Immutable"`, OwnerTypeSpecial, "Immutable", 0},
		{"AddressOwner", `{"AddressOwner":"` + addr + `"}`, OwnerTypeAddress, addr, 0},
		{"ObjectOwner", `{"ObjectOwner":"` + addr + `"}`, OwnerTypeObject, addr, 0},
		{"SingleOwner", `{"SingleOwner":"` + addr + `"}`, OwnerTypeSingle, addr, 0},
		{"Shared", `{"Shared":{"initial_shared_version":7}}`, OwnerTypeShared, "", 7},
		{"ConsensusAddressOwner", `{"ConsensusAddressOwner":{"start_version":9,"owner":"` + addr + `"}}`, OwnerTypeConsensusAddress, addr, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var o ObjectOwner
			require.NoError(t, json.Unmarshal([]byte(tc.json), &o))

			gotType, gotID, gotVer := o.GetTypeAndID()
			assert.Equal(t, tc.wantType, gotType)
			assert.Equal(t, tc.wantID, gotID)
			assert.Equal(t, tc.wantVersion, gotVer)

			out, err := json.Marshal(o)
			require.NoError(t, err)
			assert.JSONEq(t, tc.json, string(out))
		})
	}
}

// TestBuildObjectOwnerRoundTrip checks BuildObjectOwner produces an owner whose
// GetTypeAndID reports the right (type, id). Note GetTypeAndID only surfaces the
// version for Shared owners; for ConsensusAddressOwner it reports 0 even though
// BuildObjectOwner stores the start version (asserted separately below).
func TestBuildObjectOwnerRoundTrip(t *testing.T) {
	const addr = "0x0000000000000000000000000000000000000000000000000000000000000005"
	cases := []struct {
		ownerType   string
		ownerID     string
		buildVer    uint64
		wantVersion uint64
	}{
		{OwnerTypeAddress, addr, 0, 0},
		{OwnerTypeObject, addr, 0, 0},
		{OwnerTypeSingle, addr, 0, 0},
		{OwnerTypeShared, "", 7, 7},
		{OwnerTypeConsensusAddress, addr, 9, 0},
		{OwnerTypeSpecial, "Immutable", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.ownerType, func(t *testing.T) {
			o := BuildObjectOwner(tc.ownerID, tc.ownerType, tc.buildVer)
			require.NotNil(t, o)
			gotType, gotID, gotVer := o.GetTypeAndID()
			assert.Equal(t, tc.ownerType, gotType)
			assert.Equal(t, tc.ownerID, gotID)
			assert.Equal(t, tc.wantVersion, gotVer)
		})
	}

	// ConsensusAddressOwner stores the start version even though GetTypeAndID
	// does not report it.
	o := BuildObjectOwner(addr, OwnerTypeConsensusAddress, 9)
	require.NotNil(t, o.ConsensusAddressOwner)
	assert.Equal(t, uint64(9), o.ConsensusAddressOwner.StartVersion)
}
