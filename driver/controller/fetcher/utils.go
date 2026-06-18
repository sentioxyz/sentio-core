package fetcher

import (
	"golang.org/x/exp/constraints"

	"sentioxyz/sentio-core/driver/controller"
)

func sumSize[T controller.FetchTarget](dict map[uint64]T) (size int) {
	for _, item := range dict {
		size += item.Size()
	}
	return
}

func _min[V constraints.Integer](a V, b *V) V {
	if b == nil {
		return a
	}
	return min(a, *b)
}
