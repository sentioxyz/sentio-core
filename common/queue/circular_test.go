package queue

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_Circular(t *testing.T) {
	q := NewCircular[int](5)

	q.Push(1)
	assert.Equal(t, []int{1}, q.Dump(false))

	q.Push(2)
	assert.Equal(t, []int{1, 2}, q.Dump(false))

	q.Push(3)
	q.Push(4)
	q.Push(5)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, q.Dump(false))

	q.Push(6)
	assert.Equal(t, []int{2, 3, 4, 5, 6}, q.Dump(false))

	q.Push(7)
	q.Push(8)
	assert.Equal(t, []int{4, 5, 6, 7, 8}, q.Dump(false))

	q.Push(9)
	q.Push(10)
	assert.Equal(t, []int{6, 7, 8, 9, 10}, q.Dump(false))

	q.Push(11)
	assert.Equal(t, []int{7, 8, 9, 10, 11}, q.Dump(false))
	assert.Equal(t, []int{11, 10, 9, 8, 7}, q.Dump(true))
}
