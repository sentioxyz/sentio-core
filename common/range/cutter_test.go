package rg

import (
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/utils"
	"testing"
)

func Test_cutter(t *testing.T) {
	assert.Equal(t,
		[]Range{
			{Start: 120, End: utils.WrapPointer[uint64](129)},
			{Start: 130, End: utils.WrapPointer[uint64](139)},
			{Start: 140, End: utils.WrapPointer[uint64](149)},
		},
		RangeCutter{Size: 10, AlignZero: true}.CutAll(Range{Start: 120, End: utils.WrapPointer[uint64](149)}))
	assert.Equal(t,
		[]Range{
			{Start: 125, End: utils.WrapPointer[uint64](129)},
			{Start: 130, End: utils.WrapPointer[uint64](139)},
			{Start: 140, End: utils.WrapPointer[uint64](149)},
			{Start: 150, End: utils.WrapPointer[uint64](155)},
		},
		RangeCutter{Size: 10, AlignZero: true}.CutAll(Range{Start: 125, End: utils.WrapPointer[uint64](155)}))
	assert.Equal(t,
		[]Range{
			{Start: 125, End: utils.WrapPointer[uint64](134)},
			{Start: 135, End: utils.WrapPointer[uint64](144)},
			{Start: 145, End: utils.WrapPointer[uint64](154)},
			{Start: 155, End: utils.WrapPointer[uint64](155)},
		},
		RangeCutter{Size: 10, AlignZero: false}.CutAll(Range{Start: 125, End: utils.WrapPointer[uint64](155)}))
}
