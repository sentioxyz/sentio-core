package rg

import (
	"context"
	"fmt"
)

type RangeCutter struct {
	Size      uint64
	AlignZero bool
}

func (c RangeCutter) String() string {
	if c.AlignZero {
		return fmt.Sprintf("0/%d", c.Size)
	}
	return fmt.Sprintf("<RangeLeft>/%d", c.Size)
}

func (c RangeCutter) Cut(r Range, num int) []Range {
	var base uint64
	if !c.AlignZero {
		base = r.Start
	}
	return r.CutByFixedSize(base, c.Size, num)
}

func (c RangeCutter) CutAll(r Range) []Range {
	return c.Cut(r, 0)
}

func (c RangeCutter) First(r Range) Range {
	if r.IsEmpty() {
		return r
	}
	return c.Cut(r, 1)[0]
}

func (c RangeCutter) CutSet(s RangeSet) []Range {
	return s.CutByFixedSize(c.Size, c.AlignZero)
}

func (c RangeCutter) BuildProducer(r Range) RangeProducer {
	return func(ctx context.Context, ch chan<- Range) error {
		for !r.IsEmpty() {
			first := c.First(r)
			r = Range{Start: *first.End + 1, End: r.End}
			select {
			case ch <- first:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
}
