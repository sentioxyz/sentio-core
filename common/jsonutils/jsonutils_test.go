package jsonutils

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPatch_Succeed(t *testing.T) {
	testcases := [][]string{
		{
			`{}`,
			`{"a":1,"b":true,"c":1.1,"d":"abcdefg"}`,
			`{"a":1,"b":true,"c":1.1,"d":"abcdefg"}`,
		},
		{
			`{"a":123}`,
			`{"a":1,"b":true,"c":1.1,"d":"abcdefg"}`,
			`{"a":1,"b":true,"c":1.1,"d":"abcdefg"}`,
		},
		{
			`{"a":456,"e":"xxx"}`,
			`{"a":1,"b":true,"c":1.1,"d":"abcdefg"}`,
			`{"a":1,"b":true,"c":1.1,"d":"abcdefg","e":"xxx"}`,
		},
		{
			`{"a":456,"e":"xxx","f":{"g":123,"h":"yyy"}}`,
			`{"a":1,"b":true,"c":1.1,"d":"abcdefg","f":{"h":"zzz","i":true}}`,
			`{"a":1,"b":true,"c":1.1,"d":"abcdefg","e":"xxx","f":{"g":123,"h":"zzz","i":true}}`,
		},
	}
	for i, testcase := range testcases {
		r, err := Patch([]byte(testcase[0]), []byte(testcase[1]), func(path string, or, pa any) {
			fmt.Printf("#%d %s: %v => %v\n", i, path, or, pa)
		})
		assert.NoError(t, err)
		assert.Equal(t, testcase[2], string(r), fmt.Sprintf("testcase #%d %v", i, testcase))
	}
}

func TestPatch_WildcardSucceed(t *testing.T) {
	testcases := [][]string{
		{
			`{}`,
			`{"*":1}`,
			`{}`,
		},
		{
			`{"a":123,"b":456,"c":"ccc"}`,
			`{"*":1}`,
			`{"a":1,"b":1,"c":"ccc"}`,
		},
		{
			`{"a":{"x":1},"b":{"x":2},"c":3}`,
			`{"*":{"x":4}}`,
			`{"a":{"x":4},"b":{"x":4},"c":3}`,
		},
		{
			`{"a":{"x":1},"b":{"y":2},"c":3}`,
			`{"*":{"x":4}}`,
			`{"a":{"x":4},"b":{"x":4,"y":2},"c":3}`,
		},
	}
	for i, testcase := range testcases {
		r, err := Patch([]byte(testcase[0]), []byte(testcase[1]), func(path string, or, pa any) {
			fmt.Printf("#%d %s: %v => %v\n", i, path, or, pa)
		})
		assert.NoError(t, err)
		assert.Equal(t, testcase[2], string(r), fmt.Sprintf("testcase #%d %v", i, testcase))
	}
}

func TestPatch_TypeMismatch(t *testing.T) {
	var err error

	_, err = Patch([]byte(`{"a":true}`), []byte(`{"a":"abc"}`), nil)
	assert.ErrorContains(t, err, "cannot patch .a/bool by string")

	_, err = Patch([]byte(`{"a":123}`), []byte(`{"a":"abc"}`), nil)
	assert.ErrorContains(t, err, "cannot patch .a/json.Number by string")

	_, err = Patch([]byte(`{"a":{"b":123}}`), []byte(`{"a":"abc"}`), nil)
	assert.ErrorContains(t, err, "cannot patch .a/map[string]interface {} by string")

	_, err = Patch([]byte(`{"a":123}`), []byte(`{"a":{"b":true}}`), nil)
	assert.ErrorContains(t, err, "cannot patch .a/json.Number by map[string]interface {}")
}

func TestPatch_WildcardTypeMismatch(t *testing.T) {
	var err error

	_, err = Patch([]byte(`{"a":{"x":true}}`), []byte(`{"*":{"x":"abc"}}`), nil)
	assert.ErrorContains(t, err, "cannot patch .a.x/bool by string")

	_, err = Patch([]byte(`{"a":{"x":123}}`), []byte(`{"*":{"x":"abc"}}`), nil)
	assert.ErrorContains(t, err, "cannot patch .a.x/json.Number by string")

	_, err = Patch([]byte(`{"a":{"x":{"y":123}}}`), []byte(`{"*":{"x":"abc"}}`), nil)
	assert.ErrorContains(t, err, "cannot patch .a.x/map[string]interface {} by string")

	_, err = Patch([]byte(`{"a":{"x":123}}`), []byte(`{"*":{"x":{"y":123}}}`), nil)
	assert.ErrorContains(t, err, "cannot patch .a.x/json.Number by map[string]interface {}")
}
