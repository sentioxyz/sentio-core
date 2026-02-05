package common

import (
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/wasm"
	"testing"
)

func Test_valueToString1(t *testing.T) {
	v := Value{
		Kind: ValueKindArray,
		Value: &wasm.ObjectArray[*Value]{
			Data: []*Value{{
				Kind:  ValueKindString, // len=100
				Value: wasm.BuildString("1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good1"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good2"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good3"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good4"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good5"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good6"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good7"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good8"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString(""),
			}},
		},
	}
	assert.Equal(t, "Array#10["+
		"String[1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890],"+
		"String[good1],"+
		"String[good2],"+
		"String[good3],"+
		"String[good4],"+
		"String[good5],"+
		"String[good6],"+
		"String[good7],"+
		"String[good8],"+
		"String[]]",
		v.String())
}

func Test_valueToString2(t *testing.T) {
	v := Value{
		Kind: ValueKindArray,
		Value: &wasm.ObjectArray[*Value]{
			Data: []*Value{{
				Kind:  ValueKindString,
				Value: wasm.BuildString("12345678901234567890123456789012345678901234567890x12345678901234567890123456789012345678901234567890"), // len=101
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good1"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good2"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good3"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good4"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good5"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good6"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good7"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good8"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good9"),
			}, {
				Kind:  ValueKindString,
				Value: wasm.BuildString("good10"),
			}},
		},
	}
	assert.Equal(t, "Array#11["+
		"String#101[12345678901234567890123456789012345678901234567890...12345678901234567890123456789012345678901234567890],"+
		"String[good1],"+
		"String[good2],"+
		"String[good3],"+
		"String[good4],"+
		"...,"+
		"String[good6],"+
		"String[good7],"+
		"String[good8],"+
		"String[good9],"+
		"String[good10]]",
		v.String())
}
