package wasm

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_BuildStringFromBytes(t *testing.T) {
	var b []byte

	assert.Equal(t, "", BuildStringFromBytes(b).String())

	b = []byte{}
	assert.Equal(t, "", BuildStringFromBytes(b).String())

	b = []byte{'a'}
	assert.Equal(t, "a", BuildStringFromBytes(b).String())

	b = []byte{'a', 'b', 'c'}
	assert.Equal(t, "abc", BuildStringFromBytes(b).String())

	b = []byte{'a', 'b', 'c', 0}
	assert.Equal(t, "abc", BuildStringFromBytes(b).String())

	b = []byte{'a', 'b', 'c', 0, 0}
	assert.Equal(t, "abc", BuildStringFromBytes(b).String())

	b = []byte{'a', 'b', 'c', 0xe4, 0xb8, 0xad}
	assert.Equal(t, "abc中", BuildStringFromBytes(b).String())

	b = []byte{'a', 'b', 'c', 0xe4, 0xb8, 0xad, 0xe6, 0x96, 0x87}
	assert.Equal(t, "abc中文", BuildStringFromBytes(b).String())

	b = []byte{'a', 'b', 'c', 0xe4, 0xb8, 0xad, 0xe6, 0x96, 0x87, 0}
	assert.Equal(t, "abc中文", BuildStringFromBytes(b).String())

	b = []byte{'a', 'b', 'c', 0xe4, 0xb8, 0xad, 0xe6, 0x96, 0x87, 0, 0, 0}
	assert.Equal(t, "abc中文", BuildStringFromBytes(b).String())
}
