package rg

import (
	"context"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/utils"
	"testing"
)

func Test_cutterFirst(t *testing.T) {
	c := RangeCutter{Size: 10, AlignZero: true}

	assert.Equal(t, EmptyRange, c.First(EmptyRange))
	assert.Equal(t, NewRange(120, 129), c.First(NewRange(120, 149)))

	// non-aligned start → first chunk is partial
	assert.Equal(t, NewRange(125, 129), c.First(Range{Start: 125, End: utils.WrapPointer[uint64](155)}))
}

func Test_cutterCutWithNum(t *testing.T) {
	c := RangeCutter{Size: 10, AlignZero: true}
	r := NewRange(120, 149)

	// num=0 → all 3 chunks
	assert.Len(t, c.Cut(r, 0), 3)

	// num=2 → first 2 chunks only
	two := c.Cut(r, 2)
	assert.Equal(t, []Range{NewRange(120, 129), NewRange(130, 139)}, two)
}

func Test_cutterCutSet(t *testing.T) {
	c := RangeCutter{Size: 5, AlignZero: false}

	// [1][4-6][10]
	set := RangeSet{
		Range: NewRange(1, 10),
		Holes: [][2]uint64{{2, 3}, {7, 9}},
	}
	// GetRanges: [1,1], [4,6], [10,10]
	// each segment fits in one chunk of size 5
	result := c.CutSet(set)
	assert.Equal(t, []Range{NewRange(1, 1), NewRange(4, 6), NewRange(10, 10)}, result)
}

func Test_cutterBuildProducer(t *testing.T) {
	c := RangeCutter{Size: 10, AlignZero: true}
	producer := c.BuildProducer(NewRange(120, 149))

	ch := make(chan Range, 10)
	err := producer(context.Background(), ch)
	close(ch)

	assert.NoError(t, err)
	var result []Range
	for item := range ch {
		result = append(result, item)
	}
	assert.Equal(t, []Range{NewRange(120, 129), NewRange(130, 139), NewRange(140, 149)}, result)
}

func Test_cutterBuildProducerCancellation(t *testing.T) {
	c := RangeCutter{Size: 10, AlignZero: true}
	producer := c.BuildProducer(NewRange(120, 149))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so the first send will pick ctx.Done()

	ch := make(chan Range) // unbuffered — send would block
	err := producer(ctx, ch)
	assert.ErrorIs(t, err, context.Canceled)
}

func Test_cutter(t *testing.T) {
	assert.Equal(t,
		[]Range{
			NewRange(120, 129),
			NewRange(130, 139),
			NewRange(140, 149),
		},
		RangeCutter{Size: 10, AlignZero: true}.CutAll(Range{Start: 120, End: utils.WrapPointer[uint64](149)}))
	assert.Equal(t,
		[]Range{
			NewRange(125, 129),
			NewRange(130, 139),
			NewRange(140, 149),
			NewRange(150, 155),
		},
		RangeCutter{Size: 10, AlignZero: true}.CutAll(Range{Start: 125, End: utils.WrapPointer[uint64](155)}))
	assert.Equal(t,
		[]Range{
			NewRange(125, 134),
			NewRange(135, 144),
			NewRange(145, 154),
			NewRange(155, 155),
		},
		RangeCutter{Size: 10, AlignZero: false}.CutAll(Range{Start: 125, End: utils.WrapPointer[uint64](155)}))
}
