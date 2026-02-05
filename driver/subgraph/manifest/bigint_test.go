package manifest

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func Test_bigIntJSON(t *testing.T) {
	type object struct {
		V *BigInt `json:"v"`
	}
	x := object{
		V: &BigInt{Int: *big.NewInt(65535)},
	}

	s, err := json.Marshal(x)
	assert.NoError(t, err)
	assert.Equal(t, `{"v":"0xffff"}`, string(s))

	testcases := []string{
		`{"v":"0xffff"}`,
		`{"v":65535}`,
		`{"v":"65535"}`,
	}
	var y object
	for i, tc := range testcases {
		err = json.Unmarshal([]byte(tc), &y)
		msg := fmt.Sprintf("#%d %q", i, tc)
		assert.NoError(t, err, msg)
		assert.Equal(t, x, y, msg)
	}

	err = json.Unmarshal([]byte(`{"v":"65535x"}`), &y)
	assert.Equal(t, "not a valid big integer: \"65535x\"", err.Error())

	err = json.Unmarshal([]byte(`{"v": true}`), &y)
	assert.Equal(t, "not a valid big integer: true", err.Error())

	err = json.Unmarshal([]byte(`{"v": 0.1}`), &y)
	assert.Equal(t, "not a valid big integer: 0.1", err.Error())

	err = json.Unmarshal([]byte(`{"v":null}`), &y)
	assert.NoError(t, err)
	assert.Equal(t, object{}, y)
}

func Test_bigIntYAML(t *testing.T) {
	type object struct {
		V *BigInt `yaml:"v"`
	}
	x := object{
		V: &BigInt{Int: *big.NewInt(65535)},
	}

	s, err := yaml.Marshal(x)
	assert.NoError(t, err)
	assert.Equal(t, "v: \"0xffff\"\n", string(s))

	testcases := []string{
		`v: "0xffff"`,
		`v: 65535`,
		`v: "65535"`,
	}
	var y object
	for i, tc := range testcases {
		err = yaml.Unmarshal([]byte(tc), &y)
		msg := fmt.Sprintf("#%d %q", i, tc)
		assert.NoError(t, err, msg)
		assert.Equal(t, x, y, msg)
	}

	err = yaml.Unmarshal([]byte(`v: "65535x"`), &y)
	assert.Equal(t, "not a valid big integer: \"65535x\"", err.Error())

	err = yaml.Unmarshal([]byte(`v: true`), &y)
	assert.Equal(t, "not a valid big integer: \"true\"", err.Error())

	err = yaml.Unmarshal([]byte(`v: null`), &y)
	assert.NoError(t, err)
	assert.Equal(t, object{}, y)
}
