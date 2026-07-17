package clientpool

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ── ClientConfig JSON ─────────────────────────────────────────────────────────

func Test_ClientConfig_MarshalJSON_inlinesConfigFields(t *testing.T) {
	cc := ClientConfig[testClientConfig]{
		Priority: 2,
		Config:   testClientConfig{Name: "c1", Value: "v1", Version: 3},
	}
	data, err := json.Marshal(cc)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))

	// Priority and Config fields must all appear at the top level.
	assert.Equal(t, float64(2), m["priority"])
	assert.Equal(t, "c1", m["Name"])
	assert.Equal(t, "v1", m["Value"])
	assert.Equal(t, float64(3), m["Version"])

	// The nested "Config" key must NOT exist.
	_, hasConfig := m["Config"]
	assert.False(t, hasConfig)
}

func Test_ClientConfig_UnmarshalJSON_readsInlinedFields(t *testing.T) {
	raw := `{"priority":2,"Name":"c1","Value":"v1","Version":3}`
	var cc ClientConfig[testClientConfig]
	require.NoError(t, json.Unmarshal([]byte(raw), &cc))

	assert.Equal(t, uint32(2), cc.Priority)
	assert.Equal(t, "c1", cc.Config.Name)
	assert.Equal(t, "v1", cc.Config.Value)
	assert.Equal(t, 3, cc.Config.Version)
}

func Test_ClientConfig_JSONRoundTrip(t *testing.T) {
	original := ClientConfig[testClientConfig]{
		Priority: 5,
		Config:   testClientConfig{Name: "roundtrip", Value: "val", Version: 7},
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ClientConfig[testClientConfig]
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original.Priority, decoded.Priority)
	assert.Equal(t, original.Config.Name, decoded.Config.Name)
	assert.Equal(t, original.Config.Value, decoded.Config.Value)
	assert.Equal(t, original.Config.Version, decoded.Config.Version)
}

// ── ClientConfig YAML ─────────────────────────────────────────────────────────

func Test_ClientConfig_MarshalYAML_inlinesConfigFields(t *testing.T) {
	cc := ClientConfig[testClientConfig]{
		Priority: 2,
		Config:   testClientConfig{Name: "c1", Value: "v1", Version: 3},
	}
	data, err := yaml.Marshal(cc)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, yaml.Unmarshal(data, &m))

	// Priority and Config fields must all appear at the top level.
	// yaml.v3 lowercases field names without explicit tags.
	assert.Equal(t, 2, m["priority"])
	assert.Equal(t, "c1", m["name"])
	assert.Equal(t, "v1", m["value"])
	assert.Equal(t, 3, m["version"])

	// The nested "Config" key must NOT exist.
	_, hasConfig := m["Config"]
	assert.False(t, hasConfig)
	_, hasConfigLower := m["config"]
	assert.False(t, hasConfigLower)
}

func Test_ClientConfig_UnmarshalYAML_readsInlinedFields(t *testing.T) {
	raw := "priority: 2\nname: c1\nvalue: v1\nversion: 3\n"
	var cc ClientConfig[testClientConfig]
	require.NoError(t, yaml.Unmarshal([]byte(raw), &cc))

	assert.Equal(t, uint32(2), cc.Priority)
	assert.Equal(t, "c1", cc.Config.Name)
	assert.Equal(t, "v1", cc.Config.Value)
	assert.Equal(t, 3, cc.Config.Version)
}

func Test_ClientConfig_YAMLRoundTrip(t *testing.T) {
	original := ClientConfig[testClientConfig]{
		Priority: 5,
		Config:   testClientConfig{Name: "roundtrip", Value: "val", Version: 7},
	}
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var decoded ClientConfig[testClientConfig]
	require.NoError(t, yaml.Unmarshal(data, &decoded))

	assert.Equal(t, original.Priority, decoded.Priority)
	assert.Equal(t, original.Config.Name, decoded.Config.Name)
	assert.Equal(t, original.Config.Value, decoded.Config.Value)
	assert.Equal(t, original.Config.Version, decoded.Config.Version)
}

// ── ClientConfig method_authority ─────────────────────────────────────────────

func Test_ClientConfig_MethodAuthority_jsonRoundTrip(t *testing.T) {
	original := ClientConfig[testClientConfig]{
		Priority:        1,
		MethodAuthority: true,
		Config:          testClientConfig{Name: "c1", Value: "v1", Version: 3},
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, true, m["method_authority"])

	var decoded ClientConfig[testClientConfig]
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.True(t, decoded.MethodAuthority)
	assert.True(t, original.Equal(decoded))
}

func Test_ClientConfig_MethodAuthority_omittedWhenFalse(t *testing.T) {
	cc := ClientConfig[testClientConfig]{Priority: 1, Config: testClientConfig{Name: "c1"}}
	data, err := json.Marshal(cc)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	_, has := m["method_authority"]
	assert.False(t, has)
}

func Test_ClientConfig_MethodAuthority_yamlRoundTrip(t *testing.T) {
	original := ClientConfig[testClientConfig]{
		Priority:        1,
		MethodAuthority: true,
		Config:          testClientConfig{Name: "c1", Value: "v1", Version: 3},
	}
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var decoded ClientConfig[testClientConfig]
	require.NoError(t, yaml.Unmarshal(data, &decoded))
	assert.True(t, decoded.MethodAuthority)
	assert.True(t, original.Equal(decoded))
}

func Test_ClientConfig_MethodAuthority_affectsEqual(t *testing.T) {
	a := ClientConfig[testClientConfig]{Priority: 1, Config: testClientConfig{Name: "c1"}}
	b := a
	b.MethodAuthority = true
	assert.False(t, a.Equal(b))
}

func Test_PoolConfig_Trim_preservesMethodAuthority(t *testing.T) {
	trimmed := PoolConfig[testClientConfig]{
		ClientConfigs: []ClientConfig[testClientConfig]{
			{Priority: 0, MethodAuthority: true, Config: testClientConfig{Name: "c1"}},
			{Priority: 1, Config: testClientConfig{Name: "c2"}},
		},
	}.Trim(nil)
	require.Len(t, trimmed.ClientConfigs, 2)
	assert.True(t, trimmed.ClientConfigs[0].MethodAuthority)
	assert.False(t, trimmed.ClientConfigs[1].MethodAuthority)
}
