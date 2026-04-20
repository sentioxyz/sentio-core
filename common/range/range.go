package rg

import (
	"fmt"
	"math"
	"sentioxyz/sentio-core/common/utils"
)

type Range struct {
	Start uint64
	End   *uint64
}

func NewRangeBySize(start uint64, size *uint64) Range {
	if size == nil {
		return Range{Start: start}
	}
	if *size == 0 {
		return EmptyRange
	}
	end := start + *size - 1
	return Range{Start: start, End: &end}
}

func NewRangeByEndAndSize(end, size uint64) Range {
	if size == 0 {
		return EmptyRange
	}
	if end+1 < size {
		return Range{Start: 0, End: &end}
	}
	return Range{Start: end + 1 - size, End: &end}
}

var EmptyRange = Range{
	Start: math.MaxUint64,
	End:   utils.WrapPointer[uint64](0),
}

func (r Range) EndOrZero() uint64 {
	if r.End == nil {
		return 0
	}
	return *r.End
}

func (r Range) Size() *uint64 {
	if r.End == nil {
		return nil
	}
	var size uint64
	if !r.IsEmpty() {
		size = *r.End - r.Start + 1
	}
	return &size
}

func (r Range) Equal(a Range) bool {
	return r.Include(a) && a.Include(r)
}

func (r Range) Contains(n uint64) bool {
	if r.IsEmpty() {
		return false
	}
	return r.Start <= n && LessEqualNilAsInf(&n, r.End)
}

func (r Range) Include(a Range) bool {
	if a.IsEmpty() {
		return true
	}
	if r.IsEmpty() {
		return false
	}
	return r.Start <= a.Start && LessEqualNilAsInf(a.End, r.End)
}

func (r Range) IsEmpty() bool {
	return r.End != nil && *r.End < r.Start
}

func (r Range) String() string {
	if r.End == nil {
		return fmt.Sprintf("[%d,INF]", r.Start)
	} else if r.Start > *r.End {
		return fmt.Sprintf("[%d,%d/EMPTY]", r.Start, *r.End)
	} else {
		return fmt.Sprintf("[%d,%d/%d]", r.Start, *r.End, *r.End+1-r.Start)
	}
}

func (r Range) MoveLeftBorder(x uint64) Range {
	if r.Start >= x {
		return Range{Start: r.Start - x, End: r.End}
	}
	return Range{Start: 0, End: r.End}
}

func (r Range) Intersection(a Range) Range {
	if a.IsEmpty() || r.IsEmpty() {
		return EmptyRange
	}
	return Range{
		Start: max(r.Start, a.Start),
		End:   MinNilAsInf(r.End, a.End),
	}
}

func (r Range) Remove(a Range) RangeSet {
	if r.IsEmpty() {
		return EmptyBlockRangeSet
	}
	if a.IsEmpty() {
		return RangeSet{Range: r}
	}
	if a.Start <= r.Start {
		if LessNilAsInf(a.End, &r.Start) {
			return RangeSet{Range: r} // no intersection, a is to the left of r
		}
		if LessNilAsInf(a.End, r.End) {
			return RangeSet{Range: Range{ // left part of r removed
				Start: *a.End + 1,
				End:   r.End,
			}}
		}
		return EmptyBlockRangeSet // all removed
	}
	if LessNilAsInf(r.End, &a.Start) {
		return RangeSet{Range: r} // no intersection, a is to the right of r
	}
	// now r.Start < a.Start <= r.End
	if LessNilAsInf(a.End, r.End) {
		return RangeSet{ // a middle part removed, remains are two separate part
			Range: r,
			Holes: [][2]uint64{{a.Start, *a.End}},
		}
	}
	return RangeSet{Range: Range{ // right part of r removed
		Start: r.Start,
		End:   utils.WrapPointer(a.Start - 1),
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
		Start: min(r.Start, a.Start),
		End:   MaxNilAsInf(r.End, a.End),
	}
}
