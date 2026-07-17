package sui

import (
	"sentioxyz/sentio-core/chain/clientpool"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/chains"
)

// TestClientConfigTrimChainID checks that ChainID drives the variation, which in
// turn keys the default MethodTimeout entries with the variation's actual method
// names (sui_* vs iota_*) and the derived special method prefix.
func TestClientConfigTrimChainID(t *testing.T) {
	sui := ClientConfig{JSONRPCConfig: clientpool.JSONRPCConfig{Endpoint: "http://sui"}, ChainID: chains.SuiMainnetID}.Trim()
	assert.Equal(t, types.VariationSUI, sui.Variation())
	assert.Equal(t, "", sui.SpecialMethodPrefix())
	assert.Contains(t, sui.MethodTimeout, "sui_getCheckpoint")
	assert.NotContains(t, sui.MethodTimeout, "iota_getCheckpoint")
	assert.Equal(t, 3*time.Second, sui.MethodTimeout["sui_getCheckpoint"])

	iota := ClientConfig{JSONRPCConfig: clientpool.JSONRPCConfig{Endpoint: "http://iota"}, ChainID: chains.IotaMainnetID}.Trim()
	assert.Equal(t, types.VariationIOTA, iota.Variation())
	assert.Equal(t, "iota", iota.SpecialMethodPrefix())
	assert.Contains(t, iota.MethodTimeout, "iota_getCheckpoint")
	assert.NotContains(t, iota.MethodTimeout, "sui_getCheckpoint")
	assert.Equal(t, 30*time.Second, iota.MethodTimeout["iota_multiGetTransactionBlocks"])

	// SetChainID flips the variation.
	assert.Equal(t, types.VariationIOTA, sui.SetChainID(chains.IotaTestnetID).Variation())
}
