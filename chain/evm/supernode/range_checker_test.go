package supernode

import (
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
	"testing"
)

func TestChecker(t *testing.T) {

	result := gjson.Get(`["1", "2"]`, "1")
	assert.True(t, result.Exists())

	assert.Equal(t, "2", result.String())
}
