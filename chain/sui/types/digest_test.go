package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDigestStringRoundTrip(t *testing.T) {
	const s = "13PrnWn4KTma3AyMATzP255eKu8XZkkm4v1nGMGtWV5G"
	d, err := StrToDigest(s)
	assert.NoError(t, err)
	assert.Len(t, d, DigestLength)
	// base58 decode then re-encode must reproduce the original string
	assert.Equal(t, s, d.String())
}
