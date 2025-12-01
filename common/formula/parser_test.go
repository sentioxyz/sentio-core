package formula

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	var err error
	_, err = Parse("a")
	assert.NoError(t, err)
	//
	_, err = Parse("2")
	assert.NoError(t, err)
	_, err = Parse("a + b")
	assert.NoError(t, err)
	_, err = Parse("a+2")
	assert.NoError(t, err)
	//
	_, err = Parse("a & b")
	assert.ErrorContains(t, err, "Unknown binary operator")
}

func TestParseComplex(t *testing.T) {
	e, err := Parse("(a+b)+c*2")
	assert.NoError(t, err)
	assert.Equal(t, "(a+b)+c*2.000000", e.ToString())

	e, err = Parse("SUM(a+b)+c*2")
	assert.NoError(t, err)
	assert.Equal(t, "SUM(a+b)+c*2.000000", e.ToString())
}
