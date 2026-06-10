package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVariationString(t *testing.T) {
	assert.Equal(t, "sui", VariationSUI.String())
	assert.Equal(t, "iota", VariationIOTA.String())
	assert.Equal(t, "sui", string(VariationSUI))
	assert.Equal(t, "iota", string(VariationIOTA))
}
