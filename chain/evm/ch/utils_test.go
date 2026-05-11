package ch

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_nonceMarshal(t *testing.T) {
	s := "0x000d5366a647224c"
	n := StringToNonce(s)
	s1 := NonceToString(n)
	n1 := StringToNonce(s1)
	assert.Equal(t, s, s1)
	assert.Equal(t, n, n1)
}
