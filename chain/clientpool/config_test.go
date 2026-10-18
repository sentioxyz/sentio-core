package clientpool

import (
	"encoding/json"
	"testing"
	"time"

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

// ── ClientConfig Provider ─────────────────────────────────────────────────────

func Test_ClientConfig_Provider_JSON(t *testing.T) {
	data, err := json.Marshal(ClientConfig[testClientConfig]{Config: testClientConfig{Name: "c1"}})
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	_, hasProvider := m["provider"]
	assert.False(t, hasProvider, "empty provider must be omitted")

	original := ClientConfig[testClientConfig]{Priority: 1, Provider: "drpc/k1", Config: testClientConfig{Name: "c1"}}
	data, err = json.Marshal(original)
	require.NoError(t, err)
	var decoded ClientConfig[testClientConfig]
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "drpc/k1", decoded.Provider)
	assert.Equal(t, "c1", decoded.Config.Name)
}

func Test_ClientConfig_Provider_YAML(t *testing.T) {
	raw := "priority: 2\nprovider: drpc/k1\nname: c1\n"
	var cc ClientConfig[testClientConfig]
	require.NoError(t, yaml.Unmarshal([]byte(raw), &cc))
	assert.Equal(t, uint32(2), cc.Priority)
	assert.Equal(t, "drpc/k1", cc.Provider)
	assert.Equal(t, "c1", cc.Config.Name)

	data, err := yaml.Marshal(ClientConfig[testClientConfig]{Config: testClientConfig{Name: "c1"}})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "provider", "empty provider must be omitted")
}

func Test_ClientConfig_Equal_comparesProvider(t *testing.T) {
	a := ClientConfig[testClientConfig]{Priority: 1, Provider: "drpc/k1", Config: testClientConfig{Name: "c1"}}
	b := a
	assert.True(t, a.Equal(b))
	b.Provider = "drpc/k2"
	assert.False(t, a.Equal(b))
}

func Test_PoolConfig_Trim_keepsProvider(t *testing.T) {
	c := PoolConfig[testClientConfig]{ClientConfigs: []ClientConfig[testClientConfig]{
		{Priority: 1, Provider: "drpc/k1", Config: testClientConfig{Name: "c1"}},
	}}
	trimmed := c.Trim(nil)
	require.Len(t, trimmed.ClientConfigs, 1)
	assert.Equal(t, "drpc/k1", trimmed.ClientConfigs[0].Provider)
}

// ── PoolConfig.Trim defaults ──────────────────────────────────────────────────

func Test_PoolConfig_Trim_defaultTagDuration(t *testing.T) {
	trimmed := PoolConfig[testClientConfig]{}.Trim(nil)
	assert.Equal(t, time.Minute*30, trimmed.TagDuration)
}

func Test_PoolConfig_Trim_reInitInterval(t *testing.T) {
	// 0 (the default) disables periodic re-init; a default interval, if any, is up to the
	// service loading the configuration
	assert.Equal(t, time.Duration(0), PoolConfig[testClientConfig]{}.Trim(nil).ReInitInterval)
	// explicit value is kept
	assert.Equal(t, time.Minute*10,
		PoolConfig[testClientConfig]{ReInitInterval: time.Minute * 10}.Trim(nil).ReInitInterval)
	// negative is clamped to 0
	assert.Equal(t, time.Duration(0),
		PoolConfig[testClientConfig]{ReInitInterval: -time.Second}.Trim(nil).ReInitInterval)
}
