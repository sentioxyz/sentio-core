package rg

import (
	"fmt"
	"math"
	"sentioxyz/sentio-core/common/utils"
)

type Range struct {
	StartBlock uint64
	EndBlock   *uint64
}

var EmptyRange = Range{
	StartBlock: math.MaxUint64,
	EndBlock:   utils.WrapPointer[uint64](0),
}

func (r Range) EndOrZero() uint64 {
	if r.EndBlock == nil {
		return 0
	}
	return *r.EndBlock
}

func (r Range) Equal(a Range) bool {
	return r.Include(a) && a.Include(r)
}

func (r Range) Contains(n uint64) bool {
	if r.IsEmpty() {
		return false
	}
	return r.StartBlock <= n && LessEqualNilAsInf(&n, r.EndBlock)
}

func (r Range) Include(a Range) bool {
	if a.IsEmpty() {
		return true
	}
	if r.IsEmpty() {
		return false
	}
	return r.StartBlock <= a.StartBlock && LessEqualNilAsInf(a.EndBlock, r.EndBlock)
}

func (r Range) IsEmpty() bool {
	return r.EndBlock != nil && *r.EndBlock < r.StartBlock
}

func (r Range) String() string {
	if r.EndBlock == nil {
		return fmt.Sprintf("[%d,INF]", r.StartBlock)
	} else if r.StartBlock > *r.EndBlock {
		return fmt.Sprintf("[%d,%d/EMPTY]", r.StartBlock, *r.EndBlock)
	} else {
		return fmt.Sprintf("[%d,%d/%d]", r.StartBlock, *r.EndBlock, *r.EndBlock+1-r.StartBlock)
	}
}

func (r Range) Intersection(a Range) Range {
	if a.IsEmpty() || r.IsEmpty() {
		return EmptyRange
	}
	return Range{
		StartBlock: max(r.StartBlock, a.StartBlock),
		EndBlock:   MinNilAsInf(r.EndBlock, a.EndBlock),
	}
}

func (r Range) Remove(a Range) BlockRangeSet {
	if r.IsEmpty() {
		return EmptyBlockRangeSet
	}
	if a.IsEmpty() {
		return BlockRangeSet{Range: r}
	}
	if a.StartBlock <= r.StartBlock {
		if LessNilAsInf(a.EndBlock, &r.StartBlock) {
			return BlockRangeSet{Range: r} // no intersection, a is to the left of r
		}
		if LessNilAsInf(a.EndBlock, r.EndBlock) {
			return BlockRangeSet{Range: Range{ // left part of r removed
				StartBlock: *a.EndBlock + 1,
				EndBlock:   r.EndBlock,
			}}
		}
		return EmptyBlockRangeSet // all removed
	}
	if LessNilAsInf(r.EndBlock, &a.StartBlock) {
		return BlockRangeSet{Range: r} // no intersection, a is to the right of r
	}
	// now r.StartBlock < a.StartBlock <= r.EndBlock
	if LessNilAsInf(a.EndBlock, r.EndBlock) {
		return BlockRangeSet{ // a middle part removed, remains are two separate part
			Range: r,
			Holes: [][2]uint64{{a.StartBlock, *a.EndBlock}},
		}
	}
	return BlockRangeSet{Range: Range{ // right part of r removed
		StartBlock: r.StartBlock,
		EndBlock:   utils.WrapPointer(a.StartBlock - 1),
	}}
}

// Cover return a minimal Range that both include r and a
func (r Range) Cover(a Range) Range {
	if r.IsEmpty() {
		return a
	}
	if a.IsEmpty() {
		return r
	}
	return Range{
		StartBlock: min(r.StartBlock, a.StartBlock),
		EndBlock:   MaxNilAsInf(r.EndBlock, a.EndBlock),
	}
}
