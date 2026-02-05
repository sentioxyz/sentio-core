package wasm

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_extendSize(t *testing.T) {
	testcases := [][]int{
		{0, 1, 0},
		{0, 2, 0},
		{0, 3, 0},

		{1, 1, 1},
		{1, 2, 2},
		{1, 3, 3},

		{2, 1, 2},
		{2, 2, 2},
		{2, 3, 3},

		{3, 1, 3},
		{3, 2, 4},
		{3, 3, 3},

		{4, 1, 4},
		{4, 2, 4},
		{4, 3, 6},

		{5, 1, 5},
		{5, 2, 6},
		{5, 3, 6},

		{6, 1, 6},
		{6, 2, 6},
		{6, 3, 6},

		{7, 1, 7},
		{7, 2, 8},
		{7, 3, 9},

		{8, 1, 8},
		{8, 2, 8},
		{8, 3, 9},

		{9, 1, 9},
		{9, 2, 10},
		{9, 3, 9},
	}

	for i, c := range testcases {
		assert.Equal(t, c[2], extendSize(c[0], c[1]), fmt.Sprintf("case #%d %v", i, c))
	}
}
