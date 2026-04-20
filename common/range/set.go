package rg

import (
	"bytes"
	"fmt"
	"sentioxyz/sentio-core/common/utils"
)

type BlockRangeSet struct {
	Range

	// All the holes must be arranged strictly in ascending order,
	// with no two holes overlapping or adjacent to each other.
	// The left side of the leftmost hole and the right side of the rightmost hole are definitely not empty.
	Holes [][2]uint64
}

var EmptyBlockRangeSet = BlockRangeSet{
	Range: EmptyRange,
}

func (rs BlockRangeSet) Equal(a BlockRangeSet) bool {
	if !rs.Range.Equal(a.Range) {
		return false
	}
	if len(rs.Holes) != len(a.Holes) {
		return false
	}
	for i := 0; i < len(rs.Holes); i++ {
		if rs.Holes[i] != a.Holes[i] {
			return false
		}
	}
	return true
}

func (rs BlockRangeSet) Contains(n uint64) bool {
	if !rs.Range.Contains(n) {
		return false
	}
	for i := range rs.Holes {
		if rs.Holes[i][0] <= n && n <= rs.Holes[i][1] {
			return false
		}
	}
	return true
}

func (rs BlockRangeSet) Include(a Range) bool {
	if !rs.Range.Include(a) {
		return false
	}
	for i := range rs.Holes {
		if !a.Intersection(Range{StartBlock: rs.Holes[i][0], EndBlock: &rs.Holes[i][1]}).IsEmpty() {
			return false
		}
	}
	return true
}

func (rs BlockRangeSet) Last() Range {
	if rs.IsEmpty() {
		return EmptyRange
	}
	if len(rs.Holes) == 0 {
		return rs.Range
	}
	return Range{
		StartBlock: rs.Holes[len(rs.Holes)-1][1] + 1,
		EndBlock:   rs.EndBlock,
	}
}

func (rs BlockRangeSet) String() string {
	var b bytes.Buffer
	total, s := uint64(0), rs.StartBlock
	b.WriteString(fmt.Sprintf("[%d,", rs.StartBlock))
	for _, hole := range rs.Holes {
		leftLen := hole[0] - s
		total += leftLen
		s = hole[1] + 1
		b.WriteString(fmt.Sprintf("%d/%d]+[%d,", hole[0]-1, leftLen, s))
	}
	if rs.EndBlock == nil {
		b.WriteString("INF]")
	} else if s > *rs.EndBlock {
		b.WriteString(fmt.Sprintf("%d/EMPTY]", *rs.EndBlock))
	} else {
		lastLen := *rs.EndBlock + 1 - s
		b.WriteString(fmt.Sprintf("%d/%d]", *rs.EndBlock, lastLen))
		if len(rs.Holes) > 0 {
			b.WriteString(fmt.Sprintf("/%d", total+lastLen))
		}
	}
	return b.String()
}

func (rs BlockRangeSet) Intersection(a Range) BlockRangeSet {
	r := BlockRangeSet{
		Range: rs.Range.Intersection(a),
		Holes: rs.Holes,
	}
	if r.IsEmpty() {
		return EmptyBlockRangeSet
	}
	// remove invalid holes to the left
	//      pl:            *   x
	// r.Holes:      ... { } { } ...
	// r.Range:        [ ...
	// r.Range:           [ ...
	pl := 0
	for pl < len(r.Holes) && r.Holes[pl][1] < r.StartBlock {
		pl++
	}
	if pl == len(r.Holes) {
		r.Holes = nil // all holes are to the left of r
	} else if r.StartBlock < r.Holes[pl][0] {
		r.Holes = r.Holes[pl:]
	} else {
		r.StartBlock = r.Holes[pl][1] + 1
		r.Holes = r.Holes[pl+1:]
	}
	// remove invalid holes to the right
	//      pr:          x   *
	// r.Holes:      ... { } { } ...
	// r.Range:    ... ]
	// r.Range: ... ]
	pr := len(r.Holes) - 1
	for pr >= 0 && LessNilAsInf(r.EndBlock, &r.Holes[pr][0]) {
		pr--
	}
	if pr < 0 {
		r.Holes = nil // all holes are to the right of r
	} else if LessNilAsInf(&r.Holes[pr][1], r.EndBlock) {
		r.Holes = r.Holes[:pr+1]
	} else {
		r.EndBlock = utils.WrapPointer(r.Holes[pr][0] - 1)
		r.Holes = r.Holes[:pr]
	}

	if r.IsEmpty() {
		return EmptyBlockRangeSet
	}
	if len(r.Holes) == 0 {
		r.Holes = nil
	}
	return r
}

func (rs BlockRangeSet) Remove(a Range) (result BlockRangeSet) {
	if a.IsEmpty() || rs.IsEmpty() {
		return rs
	}
	if LessNilAsInf(a.EndBlock, &rs.StartBlock) {
		// rs:     [        ]
		//  a: [  ]
		// no intersection, a is to the left of rs
		return rs
	}
	if LessNilAsInf(rs.EndBlock, &a.StartBlock) {
		// rs: [        ]
		//  a:           [  ]
		// no intersection, a is to the right of rs
		return rs
	}
	// rs: [           ]
	//  a:     [   ]
	//       ^       ^
	//     left    right
	left := EmptyBlockRangeSet
	if rs.StartBlock < a.StartBlock {
		// rs: [       ]
		//  a:     [ ...
		// always have left part
		for i := 0; i < len(rs.Holes); i++ {
			if a.StartBlock < rs.Holes[i][0] {
				// rs: ...  [  ] (rs.Holes[i]) [  ]  [  ]
				//  a:       [ ...
				//  a:         [ ...
				left = BlockRangeSet{
					Range: Range{
						StartBlock: rs.StartBlock,
						EndBlock:   utils.WrapPointer(a.StartBlock - 1),
					},
					Holes: rs.Holes[:i],
				}
				break
			}
			if a.StartBlock <= rs.Holes[i][1]+1 {
				// rs: ... [  ] (rs.Holes[i]) [  ]  ...
				//  a:         [ ...
				//  a:                        [ ...
				left = BlockRangeSet{
					Range: Range{
						StartBlock: rs.StartBlock,
						EndBlock:   utils.WrapPointer(rs.Holes[i][0] - 1),
					},
					Holes: rs.Holes[:i],
				}
				break
			}
		}
		if left.IsEmpty() {
			// rs: ... [  ]  [  ]  [  ]
			//  a:                  [ ...
			//  a:                    [ ...
			left = BlockRangeSet{
				Range: Range{
					StartBlock: rs.StartBlock,
					EndBlock:   utils.WrapPointer(a.StartBlock - 1),
				},
				Holes: rs.Holes,
			}
		}
	}
	right := EmptyBlockRangeSet
	if LessNilAsInf(a.EndBlock, rs.EndBlock) {
		// rs: [       ]
		//  a: ... ]
		// always have right part
		for i := 0; i < len(rs.Holes); i++ {
			if *a.EndBlock < rs.Holes[i][0]-1 {
				// rs: ... [  ] (rs.Holes[i]) [  ]  ...
				//  a:   ... ]
				//  a: ... ]
				right = BlockRangeSet{
					Range: Range{
						StartBlock: *a.EndBlock + 1,
						EndBlock:   rs.EndBlock,
					},
					Holes: rs.Holes[i:],
				}
				break
			}
			if *a.EndBlock <= rs.Holes[i][1] {
				// rs: ... [  ] (rs.Holes[i]) [  ]  ...
				//  a:                   ... ]
				//  a:    ... ]
				right = BlockRangeSet{
					Range: Range{
						StartBlock: rs.Holes[i][1] + 1,
						EndBlock:   rs.EndBlock,
					},
					Holes: rs.Holes[i+1:],
				}
				break
			}
		}
		if right.IsEmpty() {
			// rs: ... [  ]  [  ]  [  ]
			//  a:               ... ]
			//  a:             ... ]
			right = BlockRangeSet{
				Range: Range{
					StartBlock: *a.EndBlock + 1,
					EndBlock:   rs.EndBlock,
				},
			}
		}
	}
	if !left.IsEmpty() && !right.IsEmpty() {
		result = BlockRangeSet{
			Range: rs.Range,
		}
		result.Holes = append(result.Holes, left.Holes...)
		result.Holes = append(result.Holes, [2]uint64{
			*left.EndBlock + 1,
			right.StartBlock - 1,
		})
		result.Holes = append(result.Holes, right.Holes...)
	} else if left.IsEmpty() {
		result = right
	} else {
		result = left
	}
	if result.IsEmpty() {
		return EmptyBlockRangeSet
	}
	if len(result.Holes) == 0 {
		result.Holes = nil
	}
	return result
}

func (rs BlockRangeSet) Union(a Range) BlockRangeSet {
	if a.IsEmpty() {
		return rs
	}
	if rs.IsEmpty() {
		return BlockRangeSet{Range: a}
	}
	if LessNilAsInf(rs.EndBlock, &a.StartBlock) {
		if *rs.EndBlock+1 == a.StartBlock {
			// rs: [   ]
			//  a:      [   ]
			return BlockRangeSet{
				Range: Range{
					StartBlock: rs.StartBlock,
					EndBlock:   a.EndBlock,
				},
				Holes: rs.Holes,
			}
		}
		// rs: [   ]
		//  a:       [   ]
		return BlockRangeSet{
			Range: Range{
				StartBlock: rs.StartBlock,
				EndBlock:   a.EndBlock,
			},
			Holes: append(rs.Holes, [2]uint64{
				*rs.EndBlock + 1,
				a.StartBlock - 1,
			}),
		}
	}
	if LessNilAsInf(a.EndBlock, &rs.StartBlock) {
		if *a.EndBlock+1 == rs.StartBlock {
			// rs:      [   ]
			//  a: [   ]
			return BlockRangeSet{
				Range: Range{
					StartBlock: a.StartBlock,
					EndBlock:   rs.EndBlock,
				},
				Holes: rs.Holes,
			}
		}
		// rs:       [   ]
		//  a: [   ]
		return BlockRangeSet{
			Range: Range{
				StartBlock: a.StartBlock,
				EndBlock:   rs.EndBlock,
			},
			Holes: utils.Prepend(rs.Holes, [2]uint64{
				*a.EndBlock + 1,
				rs.StartBlock - 1,
			}),
		}
	}
	result := BlockRangeSet{Range: a.Cover(rs.Range)}
	for i := 0; i < len(rs.Holes); i++ {
		remain := Range{StartBlock: rs.Holes[i][0], EndBlock: &rs.Holes[i][1]}.Remove(a)
		if remain.IsEmpty() {
			continue // The hole was completely filled.
		}
		if len(remain.Holes) == 1 {
			// The middle part of the hole was filled in, turned into two holes.
			//     rs: ... [ ]   (rs.Holes[i])   [ ] ...
			//      a:           [           ]
			// remain:        [ ]             [ ]
			// result: ... [ ]   [           ]   [ ] ...
			result.Holes = append(result.Holes, [2]uint64{
				rs.Holes[i][0],
				remain.Holes[0][0] - 1,
			}, [2]uint64{
				remain.Holes[0][1] + 1,
				rs.Holes[i][1],
			})
			// append all remain rs.Holes and then break
			result.Holes = append(result.Holes, rs.Holes[i+1:]...)
			break
		}
		// len(remain.Holes) cannot be greater than 1, so here it must be 0,
		// it means the hole was either partially filled on the left or right side, or left completely as it was.
		result.Holes = append(result.Holes, [2]uint64{
			remain.StartBlock,
			*remain.EndBlock,
		})
	}
	return result
}

// CutRangeSet divide the entire range into multiple non-overlapping ranges
// using the endpoints of multiple potentially intersecting ranges.
func CutRangeSet(start uint64, rs []Range) []Range {
	if len(rs) == 0 {
		return nil
	}
	sbn := make(map[uint64]bool)
	var inf bool
	for _, r := range rs {
		if r.EndBlock != nil && *r.EndBlock < start {
			continue
		}
		sbn[max(r.StartBlock, start)] = true
		if r.EndBlock != nil {
			sbn[*r.EndBlock+1] = true
		} else {
			inf = true
		}
	}
	ns := utils.GetOrderedMapKeys(sbn)
	var result []Range
	for i := 0; i+1 < len(ns); i++ {
		end := ns[i+1] - 1
		result = append(result, Range{
			StartBlock: ns[i],
			EndBlock:   &end,
		})
	}
	if inf {
		result = append(result, Range{
			StartBlock: ns[len(ns)-1],
		})
	}
	return result
}
