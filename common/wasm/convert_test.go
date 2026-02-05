package wasm

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wasmerio/wasmer-go/wasmer"
)

type testObject struct {
}

func (obj *testObject) Dump(*MemoryManager) Pointer {
	panic("not implemented")
}

func (obj *testObject) Load(*MemoryManager, Pointer) {
	panic("not implemented")
}

type testNonObject struct {
}

func Test_ConvertType(t *testing.T) {
	testcases := [][]any{
		{I8(0), wasmer.I32, true},
		{I16(0), wasmer.I32, true},
		{I32(0), wasmer.I32, true},
		{I64(0), wasmer.I64, true},
		{F32(0), wasmer.F32, true},
		{F64(0), wasmer.F64, true},
		{Bool(false), wasmer.I32, true},
		{&testObject{}, wasmer.I32, true},
		{testObject{}, wasmer.I32, false},
		{&testNonObject{}, wasmer.I32, false},
		{testNonObject{}, wasmer.I32, false},
	}

	for i, c := range testcases {
		wvk, ok := ConvertType(reflect.TypeOf(c[0]))
		assert.Equal(t, c[1], wvk, fmt.Sprintf("kind case #%d: %v", i, c))
		assert.Equal(t, c[2], ok, fmt.Sprintf("ok case #%d: %v", i, c))
	}
}

func Test_newNil(t *testing.T) {
	var tpl *String
	tpe := reflect.TypeOf(tpl)
	assert.Equal(t, "*wasm.String", tpe.String())
	assert.Equal(t, "wasm.String", tpe.Elem().String())
	z := reflect.Zero(tpe)
	vv, ok := z.Interface().(*String)
	assert.True(t, ok)
	assert.True(t, vv == nil)
	t.Logf("%T|%#v|%v, %v, %T|%#v|%v", vv, vv, vv == nil, ok, tpl, tpl, tpl == nil)
}
