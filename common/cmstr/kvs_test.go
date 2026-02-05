package cmstr

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_load(t *testing.T) {
	var kv KVS

	assert.Equal(t, "", kv.String())
	assert.NoError(t, kv.Load(""))
	assert.Equal(t, "", kv.String())

	kv.Add("Foo", "Bar")
	assert.Equal(t, "Foo(Bar)", kv.String())
	kv.Add("Foo", "(Bar)")
	assert.Equal(t, "Foo(Bar) Foo((Bar))", kv.String())

	kv.Add("A", "")
	assert.Equal(t, "Foo(Bar) Foo((Bar)) A()", kv.String())

	assert.NoError(t, kv.Load(" Foo(Bar)  Foo((Bar)) A "))
	assert.Equal(t, "Foo(Bar) Foo((Bar)) A()", kv.String())
}
