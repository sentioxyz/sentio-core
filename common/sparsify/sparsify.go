package sparsify

import (
	"golang.org/x/exp/constraints"
	"sentioxyz/sentio-core/common/utils"
)

// Remove collect the items index from origin which can be removed, and make sure len(result) is O(ln(origin))
func Remove[T any, N constraints.Integer](origin []T, getNum func(T) N) map[int]bool {
	return RemoveWithMultiplier(origin, getNum, 1, 2)
}

func RemoveWithMultiplier[T any, N constraints.Integer](origin []T, getNum func(T) N, minDist, distMultiplier N) map[int]bool {
	if len(origin) <= 2 {
		return make(map[int]bool)
	}
	p, distance := len(origin)-1, minDist
	pn := getNum(origin[p])
	remove := make(map[int]bool)
	for p > 1 {
		q := p - 1
		for q > 0 && pn-getNum(origin[q-1]) <= distance {
			remove[q] = true
			q--
		}
		p, pn, distance = q, getNum(origin[q]), distance*distMultiplier
	}
	return remove
}

// Sparsify collect the items from origin, and make sure len(result) is O(ln(origin)), will destroy the origin array
func Sparsify[T any, N constraints.Integer](origin []T, getNum func(T) N) []T {
	return utils.RemoveByIndex(origin, Remove(origin, getNum))
}
