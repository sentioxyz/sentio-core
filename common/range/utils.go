package rg

func EqualNilAsInf(a, b *uint64) bool {
	if a == nil && b == nil {
		return true
	}
	if a != nil && b != nil {
		return *a == *b
	}
	return false
}

func MinNilAsInf(ns ...*uint64) *uint64 {
	var r uint64
	var has bool
	for _, n := range ns {
		if n == nil {
			continue
		}
		if !has {
			r, has = *n, true
		} else {
			r = min(r, *n)
		}
	}
	if !has {
		return nil
	}
	return &r
}

func MaxNilAsInf(ns ...*uint64) *uint64 {
	var r uint64
	for _, n := range ns {
		if n == nil {
			return nil
		}
		r = max(r, *n)
	}
	return &r
}

func LessNilAsInf(a, b *uint64) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	return *a < *b
}

func LessEqualNilAsInf(a, b *uint64) bool {
	if a == nil {
		return b == nil
	}
	if b == nil {
		return true
	}
	return *a <= *b
}

func GreaterNilAsInf(a, b *uint64) bool {
	return !LessEqualNilAsInf(a, b)
}

func GreaterEqualNilAsInf(a, b *uint64) bool {
	return !LessNilAsInf(a, b)
}
