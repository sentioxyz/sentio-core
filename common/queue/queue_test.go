package queue

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_queue(t *testing.T) {
	q := NewQueue[int]()

	assert.Equal(t, 0, q.Len())

	v, has := q.Front()
	assert.False(t, has)
	assert.Equal(t, 0, v)

	v, has = q.Back()
	assert.False(t, has)
	assert.Equal(t, 0, v)

	q.PushBack(1)

	assert.Equal(t, 1, q.Len())

	v, has = q.Front()
	assert.True(t, has)
	assert.Equal(t, 1, v)

	v, has = q.Back()
	assert.True(t, has)
	assert.Equal(t, 1, v)

	q.PushBack(2)

	assert.Equal(t, 2, q.Len())

	v, has = q.Front()
	assert.True(t, has)
	assert.Equal(t, 1, v)

	v, has = q.Back()
	assert.True(t, has)
	assert.Equal(t, 2, v)

	v, has = q.PopFront()
	assert.True(t, has)
	assert.Equal(t, 1, v)
	assert.Equal(t, 1, q.Len())

	v, has = q.PopFront()
	assert.True(t, has)
	assert.Equal(t, 2, v)
	assert.Equal(t, 0, q.Len())

	v, has = q.PopFront()
	assert.False(t, has)
	assert.Equal(t, 0, v)
	assert.Equal(t, 0, q.Len())

	q.PushBack(3)
	assert.Equal(t, 1, q.Len())

	v, has = q.Front()
	assert.True(t, has)
	assert.Equal(t, 3, v)

	v, has = q.Back()
	assert.True(t, has)
	assert.Equal(t, 3, v)

	q.Reset()
	assert.Equal(t, 0, q.Len())
}
