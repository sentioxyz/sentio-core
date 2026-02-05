package common

import (
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/wasm"
	"testing"
)

func Test_jsonFromBytes(t *testing.T) {
	var value JSONValue

	assert.NoError(t, value.FromBytes([]byte(`null`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindNull, Value: nil}, value)

	assert.NoError(t, value.FromBytes([]byte(`true`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindBool, Value: wasm.Bool(true)}, value)

	assert.NoError(t, value.FromBytes([]byte(`false`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindBool, Value: wasm.Bool(false)}, value)

	assert.NoError(t, value.FromBytes([]byte(`0`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindNumber, Value: wasm.BuildString("0")}, value)

	assert.NoError(t, value.FromBytes([]byte(`123`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindNumber, Value: wasm.BuildString("123")}, value)

	assert.NoError(t, value.FromBytes([]byte(`1234567890123456789012345678901234567890`)))
	assert.Equal(t, JSONValue{
		Kind:  JSONValueKindNumber,
		Value: wasm.BuildString("1234567890123456789012345678901234567890"),
	}, value)

	assert.NoError(t, value.FromBytes([]byte(`-123.123`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindNumber, Value: wasm.BuildString("-123.123")}, value)

	assert.NoError(t, value.FromBytes([]byte(`""`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindString, Value: wasm.BuildString("")}, value)

	assert.NoError(t, value.FromBytes([]byte(`"123"`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindString, Value: wasm.BuildString("123")}, value)

	assert.NoError(t, value.FromBytes([]byte(`"123-abc"`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindString, Value: wasm.BuildString("123-abc")}, value)

	assert.NoError(t, value.FromBytes([]byte(`"123-\"abc\""`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindString, Value: wasm.BuildString("123-\"abc\"")}, value)

	assert.NoError(t, value.FromBytes([]byte(`[]`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindArray, Value: BuildJSONArray()}, value)

	assert.NoError(t, value.FromBytes([]byte(`[123]`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindArray, Value: BuildJSONArray(
		&JSONValue{Kind: JSONValueKindNumber, Value: wasm.BuildString("123")},
	)}, value)

	assert.NoError(t, value.FromBytes([]byte(
		`[123, 0, 1234567890123456789012345678901234567890, "abc", "", true, null]`,
	)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindArray, Value: BuildJSONArray(
		&JSONValue{Kind: JSONValueKindNumber, Value: wasm.BuildString("123")},
		&JSONValue{Kind: JSONValueKindNumber, Value: wasm.BuildString("0")},
		&JSONValue{Kind: JSONValueKindNumber, Value: wasm.BuildString("1234567890123456789012345678901234567890")},
		&JSONValue{Kind: JSONValueKindString, Value: wasm.BuildString("abc")},
		&JSONValue{Kind: JSONValueKindString, Value: wasm.BuildString("")},
		&JSONValue{Kind: JSONValueKindBool, Value: wasm.Bool(true)},
		&JSONValue{Kind: JSONValueKindNull},
	)}, value)

	assert.NoError(t, value.FromBytes([]byte(`
		{
			"key1": 123,
			"key2": null,
			"key3": "abc",
			"key4": [3,2,"1"],
			"key5": {
				"key51": "def",
				"key52": 0
			}
		}`)))
	assert.Equal(t, JSONValue{Kind: JSONValueKindObject, Value: BuildJSONObject(map[string]*JSONValue{
		"key1": {Kind: JSONValueKindNumber, Value: wasm.BuildString("123")},
		"key2": {Kind: JSONValueKindNull},
		"key3": {Kind: JSONValueKindString, Value: wasm.BuildString("abc")},
		"key4": {Kind: JSONValueKindArray, Value: BuildJSONArray(
			&JSONValue{Kind: JSONValueKindNumber, Value: wasm.BuildString("3")},
			&JSONValue{Kind: JSONValueKindNumber, Value: wasm.BuildString("2")},
			&JSONValue{Kind: JSONValueKindString, Value: wasm.BuildString("1")},
		)},
		"key5": {Kind: JSONValueKindObject, Value: BuildJSONObject(map[string]*JSONValue{
			"key51": {Kind: JSONValueKindString, Value: wasm.BuildString("def")},
			"key52": {Kind: JSONValueKindNumber, Value: wasm.BuildString("0")},
		})},
	})}, value)
}
