package gonanoid

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIdPattern(t *testing.T) {

	result := CheckIDMatchPattern("abc", false, true)
	assert.Equal(t, true, result)

	result = CheckIDMatchPattern("1abc", false, true)
	assert.Equal(t, false, result)

	result = CheckIDMatchPattern("1abc", true, true)
	assert.Equal(t, true, result)

	result = CheckIDMatchPattern("1Abc", true, true)
	assert.Equal(t, false, result)
}
