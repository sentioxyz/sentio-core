package set

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_SmartNew(T *testing.T) {
	s := SmartNew[string]()
	assert.Equal(T, set[string]{}, s)

	s = SmartNew[string]("")
	assert.Equal(T, set[string]{"": {}}, s)

	s = SmartNew[string]("1", "2")
	assert.Equal(T, set[string]{"1": {}, "2": {}}, s)

	s = SmartNew[string]("1", []string{"2", "3"}, "4")
	assert.Equal(T, set[string]{"1": {}, "2": {}, "3": {}, "4": {}}, s)
}
