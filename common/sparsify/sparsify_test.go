package sparsify

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_sparsify(t *testing.T) {
	getNum := func(x int) int {
		return x
	}
	assert.Equal(t, []int{}, Sparsify([]int{}, getNum))
	assert.Equal(t, []int{0}, Sparsify([]int{0}, getNum))
	assert.Equal(t, []int{0, 1}, Sparsify([]int{0, 1}, getNum))
	assert.Equal(t, []int{0, 10}, Sparsify([]int{0, 10}, getNum))
	assert.Equal(t, []int{0, 1, 10}, Sparsify([]int{0, 1, 10}, getNum))
	assert.Equal(t, []int{0, 2, 10}, Sparsify([]int{0, 1, 2, 10}, getNum))
	assert.Equal(t, []int{0, 8, 9, 10}, Sparsify([]int{0, 8, 9, 10}, getNum))
	assert.Equal(t, []int{0, 7, 9, 10}, Sparsify([]int{0, 7, 9, 10}, getNum))
	assert.Equal(t, []int{0, 6, 9, 10}, Sparsify([]int{0, 6, 9, 10}, getNum))
	assert.Equal(t, []int{0, 7, 9, 10}, Sparsify([]int{0, 7, 8, 9, 10}, getNum))
	assert.Equal(t, []int{0, 6, 7, 9, 10}, Sparsify([]int{0, 6, 7, 8, 9, 10}, getNum))
	assert.Equal(t, []int{0, 5, 7, 9, 10}, Sparsify([]int{0, 5, 6, 7, 8, 9, 10}, getNum))
	assert.Equal(t, []int{0, 4, 7, 9, 10}, Sparsify([]int{0, 4, 5, 6, 7, 8, 9, 10}, getNum))
	assert.Equal(t, []int{0, 3, 7, 9, 10}, Sparsify([]int{0, 3, 4, 5, 6, 7, 8, 9, 10}, getNum))
	assert.Equal(t, []int{0, 3, 7, 9, 10}, Sparsify([]int{0, 2, 3, 4, 5, 6, 7, 8, 9, 10}, getNum))
	assert.Equal(t, []int{0, 3, 7, 9, 10}, Sparsify([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, getNum))
}
