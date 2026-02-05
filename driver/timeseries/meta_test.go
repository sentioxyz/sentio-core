package timeseries

import (
	"testing"

	"sentioxyz/sentio-core/common/log"

	"github.com/stretchr/testify/assert"
)

func Test_expr(t *testing.T) {
	testcases := []struct {
		S   string
		OK  bool
		FOK bool
	}{
		{S: "", OK: false, FOK: false},
		{S: "0", OK: false, FOK: false},
		{S: "#a", OK: false, FOK: false},
		{S: "^a", OK: false, FOK: false},
		{S: "$a", OK: false, FOK: false},
		{S: "a#", OK: false, FOK: false},
		{S: "a^", OK: false, FOK: false},
		{S: "a$", OK: false, FOK: false},
		{S: " a", OK: false, FOK: false},
		{S: "a", OK: true, FOK: true},
		{S: "_", OK: true, FOK: true},
		{S: "a0", OK: true, FOK: true},
		{S: "ab", OK: true, FOK: true},
		{S: "a_", OK: true, FOK: true},
		{S: "_a", OK: true, FOK: true},
		{S: "ab_", OK: true, FOK: true},
		{S: "a b", OK: true, FOK: false},
		{S: "ab ", OK: false, FOK: false},
		{S: "a-b", OK: true, FOK: false},
		{S: "ab-", OK: false, FOK: false},
		{S: "-ab", OK: false, FOK: false},
		{S: "a.b_", OK: false, FOK: true},
		{S: ".ab_", OK: false, FOK: false},
		{S: "ab_.", OK: false, FOK: false},
	}
	for i, tc := range testcases {
		assert.Equalf(t, tc.OK, metaNameExpr.MatchString(tc.S), "testcase #%d: %v", i, tc)
		assert.Equalf(t, tc.FOK, fieldNameExpr.MatchString(tc.S), "testcase #%d: %v", i, tc)
	}
}

func Test_CalcFieldsDiff(t *testing.T) {
	original := map[string]Field{
		"a": {
			Name: "a",
			Type: FieldTypeInt,
		},
		"b": {
			Name: "b",
			Type: FieldTypeString,
		},
		"c": {
			Name: "c",
			Type: FieldTypeJSON,
			NestedStructSchema: map[string]FieldType{
				"nested":             FieldTypeString,
				"nested.middle":      FieldTypeJSON,
				"nested.middle.leaf": FieldTypeArray,
			},
		},
		"d": {
			Name: "d",
			Type: FieldTypeToken,
		},
	}

	other := map[string]Field{
		"b": {
			Name: "b",
			Type: FieldTypeString,
		},
		"c": {
			Name: "c",
			Type: FieldTypeJSON,
			NestedStructSchema: map[string]FieldType{
				"nested":                 FieldTypeString,
				"nested.middle":          FieldTypeJSON,
				"nested.middle.leaf":     FieldTypeArray,
				"nested.new_middle":      FieldTypeJSON,
				"nested.new_middle.leaf": FieldTypeArray,
			},
		},
		"e": {
			Name: "e",
			Type: FieldTypeJSON,
			NestedStructSchema: map[string]FieldType{
				"new_nested":             FieldTypeString,
				"new_nested.middle":      FieldTypeJSON,
				"new_nested.middle.leaf": FieldTypeArray,
			},
		},
		"f": {
			Name: "f",
			Type: FieldTypeArray,
		},
	}

	result := CalcFieldsDiff(original, other)
	assert.Equal(t, 2, len(result.AddFields))
	assert.Equal(t, 0, len(result.UpdFields))
	assert.Equal(t, 2, len(result.DelFields))
	assert.Equal(t, 1, len(result.UpdSchema))

	log.Infof("result: %+v", result)
}

func Test_GetFieldType(t *testing.T) {
	meta := &Meta{
		Fields: map[string]Field{
			"a": {
				Name: "a",
				Type: FieldTypeInt,
			},
			"b": {
				Name: "b",
				Type: FieldTypeString,
			},
			"c": {
				Name: "c",
				Type: FieldTypeJSON,
				NestedStructSchema: map[string]FieldType{
					"nested":             FieldTypeString,
					"nested.middle":      FieldTypeJSON,
					"nested.middle.leaf": FieldTypeArray,
				},
			},
		}}
	fieldType, ok := meta.GetFieldType("c.nested.middle.leaf")
	assert.True(t, ok)
	assert.Equal(t, FieldTypeArray, fieldType)
	fieldType, ok = meta.GetFieldType("a")
	assert.True(t, ok)
	assert.Equal(t, FieldTypeInt, fieldType)
	fieldType, ok = meta.GetFieldType("b")
	assert.True(t, ok)
	assert.Equal(t, FieldTypeString, fieldType)

	fieldType, ok = meta.GetFieldType("c.nested.middle.leaf.new")
	assert.False(t, ok)
	fieldType, ok = meta.GetFieldType("c.nested.middle.leaf.new.new")
	assert.False(t, ok)
	fieldType, ok = meta.GetFieldType("c.nested.middle.leaf.new.new.new")
	assert.False(t, ok)
}
