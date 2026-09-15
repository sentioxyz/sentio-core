package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIDSet(t *testing.T) {
	s := newIDSet("0xabc", "0xabc#1")
	assert.Equal(t, 2, s.Size())
	assert.True(t, s.Contains("0xabc"))
	assert.True(t, s.Contains("0xabc#1"))
	assert.False(t, s.Contains("0xabc#2"))
	assert.False(t, s.Contains(""))

	s.Add("")
	assert.True(t, s.Contains(""))
	s.Add("0xabc") // already there
	assert.Equal(t, 3, s.Size())

	s.Remove("0xabc")
	assert.False(t, s.Contains("0xabc"))
	assert.True(t, s.Contains("0xabc#1"), "removing one ID leaves the others")
	s.Remove("never added")
	assert.Equal(t, 2, s.Size())

	s.Merge(newIDSet("0xabc#1", "0xdef"))
	assert.Equal(t, 3, s.Size())
	assert.True(t, s.Contains("0xdef"))
}
