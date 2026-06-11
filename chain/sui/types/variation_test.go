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

func TestVariationFromNetwork(t *testing.T) {
	cases := map[string]Variation{
		"sui-mainnet":  VariationSUI,
		"sui-testnet":  VariationSUI,
		"sui":          VariationSUI,
		"iota-mainnet": VariationIOTA,
		"iota-testnet": VariationIOTA,
		"iota":         VariationIOTA,
		"":             VariationSUI,
	}
	for network, want := range cases {
		assert.Equal(t, want, VariationFromNetwork(network), "network=%q", network)
	}
}
