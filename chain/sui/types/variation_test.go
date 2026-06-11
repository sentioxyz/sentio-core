package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/chains"
)

func TestVariationString(t *testing.T) {
	assert.Equal(t, "sui", VariationSUI.String())
	assert.Equal(t, "iota", VariationIOTA.String())
	assert.Equal(t, "sui", string(VariationSUI))
	assert.Equal(t, "iota", string(VariationIOTA))
}

func TestVariationFromChainID(t *testing.T) {
	cases := map[chains.SuiChainID]Variation{
		chains.SuiMainnetID:  VariationSUI,
		chains.SuiTestnetID:  VariationSUI,
		chains.IotaMainnetID: VariationIOTA,
		chains.IotaTestnetID: VariationIOTA,
		"":                   VariationSUI,
	}
	for chainID, want := range cases {
		assert.Equal(t, want, VariationFromChainID(chainID), "chainID=%q", chainID)
	}
}

func TestVariationSpecialMethodPrefixAndRPCMethod(t *testing.T) {
	assert.Equal(t, "", VariationSUI.SpecialMethodPrefix())
	assert.Equal(t, "iota", VariationIOTA.SpecialMethodPrefix())

	// SUI leaves method names untouched.
	assert.Equal(t, "sui_getCheckpoint", VariationSUI.RPCMethod("sui_getCheckpoint"))
	// IOTA replaces the leading "sui" with "iota" — for both "sui_*" and "suix_*".
	assert.Equal(t, "iota_getCheckpoint", VariationIOTA.RPCMethod("sui_getCheckpoint"))
	assert.Equal(t, "iotax_queryEvents", VariationIOTA.RPCMethod("suix_queryEvents"))
	// Non-"sui" methods are returned as-is for both.
	assert.Equal(t, "rpc_discover", VariationIOTA.RPCMethod("rpc_discover"))
}
