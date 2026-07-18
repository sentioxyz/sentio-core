package evm

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"sentioxyz/sentio-core/chain/clientpool"
)

// The embedded clientpool.JSONRPCConfig must keep the wire format flat — exactly the shape of
// the existing on-disk endpoint config files.
const endpointYAML = `endpoint: https://ethereum-a.example:8545
priority: 0
method_authority: true
keep_watch: 200ms
strict_data_integrity_check: true
method_timeout:
  eth_chainId: 3s
method_black_list:
  - eth_getLogs
`

func Test_ClientConfig_YAML_flatWireFormat(t *testing.T) {
	var cc clientpool.ClientConfig[ClientConfig]
	require.NoError(t, yaml.Unmarshal([]byte(endpointYAML), &cc))

	assert.Equal(t, uint32(0), cc.Priority)
	assert.Equal(t, "https://ethereum-a.example:8545", cc.Config.Endpoint)
	assert.True(t, cc.Config.MethodAuthority)
	assert.Equal(t, 200*time.Millisecond, cc.Config.KeepWatch)
	assert.True(t, cc.Config.StrictDataIntegrityCheck)
	assert.Equal(t, 3*time.Second, cc.Config.MethodTimeout["eth_chainId"])
	assert.Equal(t, []string{"eth_getLogs"}, cc.Config.MethodBlackList)
}

func Test_ClientConfig_JSON_flatWireFormat(t *testing.T) {
	original := ClientConfig{
		JSONRPCConfig: clientpool.JSONRPCConfig{
			Endpoint:        "https://ethereum-a.example:8545",
			KeepWatch:       time.Second,
			MethodBlackList: []string{"eth_getLogs"},
			MethodAuthority: true,
		},
		ChainID: 1,
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	// embedded fields must appear at the top level, not nested under a struct key
	assert.Equal(t, "https://ethereum-a.example:8545", m["endpoint"])
	assert.Equal(t, true, m["method_authority"])
	_, nested := m["JSONRPCConfig"]
	assert.False(t, nested)

	var decoded ClientConfig
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.True(t, original.Equal(decoded))
}
