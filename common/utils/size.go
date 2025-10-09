package utils

import (
	"math"
	"reflect"
)

// DeepSize returns an estimated size of the given value, including the size of any referenced values.
// https://github.com/creachadair/misctools/blob/4c938d6077e47ed42f6e7d6c8794b54e236dd23d/sizeof/size.go
func DeepSize(v interface{}) int64 {
	return int64(valueSize(reflect.ValueOf(v), make(map[uintptr]bool)))
}

func valueSize(v reflect.Value, seen map[uintptr]bool) uintptr {
	if !v.IsValid() {
		return 0
	}

	base := v.Type().Size()
	switch v.Kind() {
	case reflect.Ptr:
		p := v.Pointer()
		if !seen[p] && !v.IsNil() {
			seen[p] = true
			return base + valueSize(v.Elem(), seen)
		}
	case reflect.Interface:
		if v.IsValid() && !v.IsNil() {
			return base + valueSize(v.Elem(), seen)
		}
	case reflect.Slice:
		n := v.Len()
		for i := 0; i < n; i++ {
			base += valueSize(v.Index(i), seen)
		}

		// Account for the parts of the array not covered by this slice.  Since
		// we can't get the values directly, assume they're zeroes. That may be
		// incorrect, in which case we may underestimate.
		if cap := v.Cap(); cap > n {
			base += v.Type().Size() * uintptr(cap-n)
		}
	case reflect.Array:
		n := v.Len()
		for i := 0; i < n; i++ {
			base += valueSize(v.Index(i), seen)
		}
	case reflect.Map:
		// A map m has len(m) / 6.5 buckets, rounded up to a power of two, and
		// a minimum of one bucket. Each bucket is 16 bytes + 8*(keysize + valsize).
		//
		// We can't tell which keys are in which bucket by reflection, however,
		// so here we count the 16-byte header for each bucket, and then just add
		// in the computed key and value sizes.
		nb := uintptr(math.Pow(2, math.Ceil(math.Log(float64(v.Len())/6.5)/math.Log(2))))
		if nb == 0 {
			nb = 1
		}
		base = 16 * nb
		for _, key := range v.MapKeys() {
			base += valueSize(key, seen)
			base += valueSize(v.MapIndex(key), seen)
		}

		// We have nb buckets of 8 slots each, and v.Len() slots are filled.
		// The remaining slots we will assume contain zero key/value pairs.
		zk := v.Type().Key().Size()  // a zero key
		zv := v.Type().Elem().Size() // a zero value
		base += (8*nb - uintptr(v.Len())) * (zk + zv)

	case reflect.Struct:
		// Chase pointer and slice fields and add the size of their members.
		for i := 0; i < v.NumField(); i++ {
			kind := v.Field(i).Kind()
			if kind == reflect.Ptr || kind == reflect.Slice ||
				kind == reflect.Map || kind == reflect.Interface ||
				kind == reflect.Array || kind == reflect.Struct ||
				kind == reflect.String {
				base += valueSize(v.Field(i), seen)
			}
		}

	case reflect.String:
		return base + uintptr(v.Len())

	}
	return base
}
