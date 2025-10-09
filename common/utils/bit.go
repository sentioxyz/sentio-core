package utils

import (
	"golang.org/x/exp/constraints"
)

func LowBit[T constraints.Unsigned](v T) T {
	return (v ^ (v - 1)) & v
}

func HighBit[T constraints.Unsigned](v T) T {
	for {
		if n := LowBit(v); n == v {
			return v
		} else {
			v -= n
		}
	}
}

// MinP2 return the smallest integer power of 2 that is greater than or equal to v
func MinP2[T constraints.Unsigned](v T) T {
	if n := HighBit(v); n < v {
		return n << 1
	}
	return v
}

// BinarySearch
// checker(i) == true implies checker(i+1) == true.
// `p` is the smallest x in [s,e] and checker(x) is true.
// If all x in [s,e] that checker(x) is false, `has` will be false, or `has` will be true
func BinarySearch[T constraints.Unsigned](s, e T, checker func(T) (bool, error)) (p T, has bool, err error) {
	if has, err = checker(s); err != nil || has {
		return s, has, err
	}
	if has, err = checker(e); err != nil || !has {
		return
	}
	// now checker(s) == false and checker(e) == true, will maintain it in the below binary search process
	for s+1 < e {
		m := (s + e) >> 1
		if m >= e {
			has = true
		} else if m <= s {
			has = false
		} else if has, err = checker(m); err != nil {
			return
		}
		if has {
			e = m
		} else {
			s = m
		}
	}
	return e, true, nil
}
