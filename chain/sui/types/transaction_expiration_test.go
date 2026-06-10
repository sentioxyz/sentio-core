package types

import (
	"bytes"
	"encoding/hex"
	"testing"

	"sentioxyz/sentio-core/chain/sui/types/serde"

	"github.com/stretchr/testify/assert"
)

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

func Test_TransactionExpiration_UnknownVariantErrors(t *testing.T) {
	var exp TransactionExpiration
	// variant 3 is not known -> must error, not silently produce an empty value
	_, err := exp.UnmarshalBCS(bytes.NewReader([]byte{0x03}))
	assert.Error(t, err)
}
