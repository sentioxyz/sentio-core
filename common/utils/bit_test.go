package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_lowBit(t *testing.T) {
	testcases := [][]uint64{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 1},
		{4, 4},
		{5, 1},
		{6, 2},
		{7, 1},
		{8, 8},
		{9, 1},
		{10, 2},
		{11, 1},
		{12, 4},
	}

	for _, testcase := range testcases {
		assert.Equal(t, testcase[1], LowBit(testcase[0]))
	}
}

func Test_highBit(t *testing.T) {
	testcases := [][]uint64{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 2},
		{4, 4},
		{5, 4},
		{6, 4},
		{7, 4},
		{8, 8},
		{9, 8},
		{10, 8},
		{11, 8},
		{12, 8},
	}

	for _, testcase := range testcases {
		assert.Equal(t, testcase[1], HighBit(testcase[0]))
	}
}

func Test_binSearch(t *testing.T) {
	var i, j uint
	for i = 0; i < 100; i++ {
		for j = i; j <= 100; j++ {
			{
				_, has, err := BinarySearch(i, j, func(x uint) (bool, error) {
					return false, nil
				})
				assert.NoError(t, err)
				assert.False(t, has)
			}

			for k := i; k <= j; k++ {
				r, has, err := BinarySearch(i, j, func(x uint) (bool, error) {
					return x >= k, nil
				})
				assert.NoError(t, err)
				assert.True(t, has)
				assert.Equal(t, k, r)
			}
		}
	}
}
