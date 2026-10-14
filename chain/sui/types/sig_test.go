package types

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignatureFlagHelpers(t *testing.T) {
	// scheme flag is the first byte: 0x03 multisig, 0x05 zklogin, 0x00 ed25519.
	assert.True(t, IsMultiSigBytes([]byte{0x03, 0xaa}))
	assert.False(t, IsMultiSigBytes([]byte{0x00, 0xaa}))
	assert.False(t, IsMultiSigBytes(nil))

	assert.True(t, IsZkLoginSigBytes([]byte{0x05, 0xaa}))
	assert.False(t, IsZkLoginSigBytes([]byte{0x03, 0xaa}))
	assert.False(t, IsZkLoginSigBytes(nil))

	// base64 of a 0x03-prefixed payload starts with 'A' then a char encoding 11xxxx
	assert.True(t, IsMultiSigBase64("Aw=="))
	assert.False(t, IsMultiSigBase64("AA==")) // 0x00 prefix
	assert.False(t, IsMultiSigBase64("A"))
}

func TestDecodeMultiSig(t *testing.T) {
	s := `AwIAvlJnUP0iJFZL+QTxkKC9FHZGwCa5I4TITHS/QDQ12q1sYW6SMt2Yp3PSNzsAay0Fp2MPVohqyyA02UtdQ2RNAQGH0eLk4ifl9h1I8Uc+4QlRYfJC21dUbP8aFaaRqiM/f32TKKg/4PSsGf9lFTGwKsHJYIMkDoqKwI8Xqr+3apQzAwADAFriILSy9l6XfBLt5hV5/1FwtsIsAGFow3tefGGvAYCDAQECHRUjB8a3Kw7QQYsOcM2A5/UpW42G9XItP1IT+9I5TzYCADtqJ7zOtqQtYqOo0CpvDXNlMhV3HeJDpjrASKGLWdopAwMA`

	sig, err := DecodeMultiSig(s)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(sig.Sigs))
	assert.Equal(t, 3, len(sig.PublicKey.PkMap))
	fmt.Println(sig.DebugString())

	s = `AwkAmdgYPD5NxEk1ysG+PJYKGnKPq67qcxMmLeYU6AA/8Te8qZkyYkaFFCX5BMDgxCx2IptmUe3e9+T10CqDw1niCACbkrC1vCR9sQbX7wGLG8hi6c0UuNmRN2IXoz3iSfcobNlb2VYjQ6kgV+b+Cwfjv0T0eO5qapywM574Qpbp1r4GAOEFjW+oqJR+wHockpQBcbDwBfhTMVETsq6z7nIIlSHny8ETvpI2eZ0AGDCeojCQgNzUh0jnSX0WFFZKWTfDcgsAt1BlPbI2c46A34azqCHg9TfGA/8W86Jn8TF1UV3t2mY5w8lAie2R4VXcZXIjdefZTTxapKpz3PH0nT1MT77XCQBQPOEI7ZnmBQkm6QdunGvk8twynEUs1LuGQUfKA//m8jUNFI08C9Zwyhrj8Y6zeqlDu9O3TE40yiq6Q5BBCjAFAKLJUm6OatjPmLOBE9Bet6/q8cb//PzBJ2qyq/UBMMH5Qh06solYoz4c3q1v4vXOxtsRpFfE4kIF6rk3T4evHA0AR+Lm7es62VKZFZL6UP06IFstkoWgx1eQ8y8PWTrEzc0zrbUAYWvvgarafWakzV0fvJwmrs6mWHVaWPpxfR5UAwCUGJo0Ejnrm+aTsWWTElRABcWbngrB74bF+JGlkuXc3zmi9TBolmW/mc84u1YDAa4nMsMNYSM0AYa/GISNdL8GABytmGrBdOyWy/lccagO0itwlJxMDSzL0DK2xsfXDtyKdat46gqmqng3vaUG+OsmVN4SQeIALBuJkzPphus/Egz/AQoAZ4Aer2KARjhqewIPjWagCs10oCmqfc23o63oIonUwAIBAI0+wTCqcqYF5WE1KQp+VUolZ4qZ5iPHBlFAC/+melmxAQBGL5lmI8v9u2lhNdccF2MjzvEbptVCLHMzE80w8Jg6ZwEAHqYnrOvIUDEFCHZTT6awiXK7xx59cIMF5ZhAEJy3cGUBAOLzHGux38QDb6wRMIZG6XPqZMTKZPA2OdtIhEhhJIpKAQDX+ze6+kW2Tx9yI+psNRnBl7nCIKNgpg1od7MaeyV0zQEA39L7fKsYIT662KMBlCQ1PLsoeap0eNKtggmOVP8h1CwBAFfhHd1PUD32EiacpT7ZZDdApkqfRI18IYLf9T2GlBqlAQD83lfOxft/LoWJAZobBm6n1jBgbiyjAxdb3YMlUvFP8AEAbWF2ZW4wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABCQA=`
	sig, err = DecodeMultiSig(s)
	assert.NoError(t, err)
	found := false
	for _, pk := range sig.PublicKey.PkMap {
		k := pk.PubKey.ED25519
		if bytes.HasPrefix(k[:], []byte("maven0")) {
			fmt.Println("found maven0", hex.EncodeToString(k[:]))
			found = true
		}
	}
	assert.True(t, found)
}

// multiSigWithNewSchemes hand-builds a multisig whose members use the schemes added after the
// classic three, laid out from the sui-types staged snapshot (format__sui.yaml.snap):
// CompressedSignature/PublicKey variants 3 ZkLogin (length-prefixed bytes), 4 Passkey (sig is
// length-prefixed bytes, pk a raw 33-byte array) and 5 MLDSA65 (raw 3309-byte sig, 1952-byte pk).
func multiSigWithNewSchemes(lastPkVariant byte) []byte {
	b := []byte{0x03}                                    // multisig signature flag
	b = append(b, 0x03)                                  // sigs: 3 entries
	b = append(b, 0x05)                                  // sig 0: MLDSA65
	b = append(b, bytes.Repeat([]byte{0xab}, 3309)...)   //   raw [u8; 3309], no length prefix
	b = append(b, 0x03, 0x04, 0xde, 0xad, 0xbe, 0xef)    // sig 1: ZkLogin, len 4 + bytes
	b = append(b, 0x04, 0x02, 0x11, 0x22)                // sig 2: Passkey, len 2 + bytes
	b = append(b, 0x07, 0x00)                            // bitmap u16 LE = 7
	b = append(b, 0x03)                                  // pk_map: 3 entries
	b = append(b, 0x05)                                  // pk 0: MLDSA65
	b = append(b, bytes.Repeat([]byte{0xcd}, 1952)...)   //   raw [u8; 1952]
	b = append(b, 0x01)                                  //   weight
	b = append(b, 0x04, 0x02)                            // pk 1: Passkey, raw [u8; 33]: 0x02 then 32 * 0x33
	b = append(b, bytes.Repeat([]byte{0x33}, 32)...)     //
	b = append(b, 0x01)                                  //   weight
	b = append(b, lastPkVariant, 0x03, 0x0a, 0x0b, 0x0c) // pk 2: ZkLogin (3), len 3 + bytes
	b = append(b, 0x01)                                  //   weight
	b = append(b, 0x02, 0x00)                            // threshold u16 LE = 2
	return b
}

func TestDecodeMultiSig_NewSchemes(t *testing.T) {
	sig, err := DecodeMultiSigBytes(multiSigWithNewSchemes(0x03))
	assert.NoError(t, err)

	assert.Len(t, sig.Sigs, 3)
	assert.Equal(t, bytes.Repeat([]byte{0xab}, 3309), sig.Sigs[0].MLDSA65[:])
	assert.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, sig.Sigs[1].ZkLogin)
	assert.Equal(t, []byte{0x11, 0x22}, sig.Sigs[2].Passkey)
	assert.Equal(t, uint16(7), sig.Bitmap)

	pks := sig.PublicKey.PkMap
	assert.Len(t, pks, 3)
	assert.Equal(t, bytes.Repeat([]byte{0xcd}, 1952), pks[0].PubKey.Bytes())
	assert.Equal(t, append([]byte{0x02}, bytes.Repeat([]byte{0x33}, 32)...), pks[1].PubKey.Bytes())
	assert.Equal(t, []byte{0x0a, 0x0b, 0x0c}, pks[2].PubKey.Bytes())
	for _, pk := range pks {
		assert.Equal(t, uint8(1), pk.Weight)
	}
	assert.Equal(t, uint16(2), sig.PublicKey.Threshold)
	assert.Contains(t, sig.DebugString(), "MLDSA65(0xabab")
}

func TestDecodeMultiSig_UnknownSchemeErrors(t *testing.T) {
	_, err := DecodeMultiSigBytes(multiSigWithNewSchemes(0x06))
	assert.Error(t, err)
}

func TestPublicKeyBytes(t *testing.T) {
	assert.Nil(t, PublicKey{}.Bytes())
	pk := PublicKey{Secp256k1: [33]byte{0x02, 0x01}}
	assert.Equal(t, pk.Secp256k1[:], pk.Bytes())
}
