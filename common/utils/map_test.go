package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_MergeMapIfNotExist(t *testing.T) {
	// adds keys that are missing in dst
	dst := map[string]int{"a": 1}
	n := MergeMapIfNotExist(dst, map[string]int{"b": 2, "c": 3})
	assert.Equal(t, 2, n)
	assert.Equal(t, map[string]int{"a": 1, "b": 2, "c": 3}, dst)

	// existing keys in dst are not overwritten
	dst = map[string]int{"a": 1, "b": 99}
	n = MergeMapIfNotExist(dst, map[string]int{"b": 2, "c": 3})
	assert.Equal(t, 1, n)
	assert.Equal(t, map[string]int{"a": 1, "b": 99, "c": 3}, dst)

	// empty src -> no change, count=0
	dst = map[string]int{"a": 1}
	n = MergeMapIfNotExist(dst, map[string]int{})
	assert.Equal(t, 0, n)
	assert.Equal(t, map[string]int{"a": 1}, dst)

	// nil src -> no change, count=0
	dst = map[string]int{"a": 1}
	n = MergeMapIfNotExist(dst, nil)
	assert.Equal(t, 0, n)
	assert.Equal(t, map[string]int{"a": 1}, dst)
}
