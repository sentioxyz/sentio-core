package wasm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ByteArray(t *testing.T) {
	b, err := BuildByteArrayFromHex("0x0102030405060708090a0b0c0d0E0F1011121314")
	assert.NoError(t, err)
	assert.Equal(t, "0x0102030405060708090a0b0c0d0e0f1011121314", b.ToHex())
}
