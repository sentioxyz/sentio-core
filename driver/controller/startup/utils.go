package startup

import (
	"sentioxyz/sentio-core/common/sparsify"
	"sentioxyz/sentio-core/common/utils"
)

func cacheSnapshot[T any](cache map[uint64]map[uint64]T, getSize func(T) int) any {
	if len(cache) == 0 {
		return nil
	}
	type segment struct {
		Start      uint64
		End        uint64
		BlockCount int
		DataCount  int
	}
	ss := make([]segment, 0, len(cache))
	for _, bn := range utils.GetOrderedMapKeys(cache) {
		s := segment{Start: bn, End: bn, BlockCount: 1}
		for _, d := range cache[bn] {
			s.DataCount += getSize(d)
		}
		ss = append(ss, s)
	}
	remove := sparsify.Remove(ss, func(s segment) uint64 {
		return s.Start
	})
	merge := func(ss []segment) (r segment) {
		r = ss[0]
		for i := 1; i < len(ss); i++ {
			r.Start = min(ss[i].Start, r.Start)
			r.End = max(ss[i].End, r.End)
			r.BlockCount += ss[i].BlockCount
			r.DataCount += ss[i].DataCount
		}
		return r
	}
	var result []segment
	var s int
	for p := 1; p < len(ss); p++ {
		if !remove[p] {
			result = append(result, merge(ss[s:p]))
			s = p
		}
	}
	return append(result, merge(ss[s:]))
}
