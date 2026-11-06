// Package memsize estimates the Go heap memory held by a value, the way the Go runtime allocates it.
//
// Plain deep-size walkers add up type sizes and content lengths, which on decoded JSON undercounts the
// real heap by about half. This package models what the allocator does instead:
//   - every allocation is rounded up to its size class (read from runtime/metrics, so it follows the Go
//     version), objects above 32 KiB to whole pages
//   - objects with pointers above 512 bytes carry an 8-byte malloc header
//   - pointer-free objects under 16 bytes go through the tiny allocator and are counted by their size
//   - maps are counted as Swiss tables (Go 1.24+): header, groups of 8 slots with a control word, a 7/8
//     load factor, tables of at most 1024 slots and the table directory
//   - non-pointer values stored in an interface are boxed in their own allocation, except the values the
//     runtime serves from static memory (zero-length strings, single bytes, small integers)
//
// Every pointer, slice array, string data and map is counted once even if it is reachable many times.
//
// Runtime state shared by many values is not walked (see opaqueTypes): the state of protobuf messages,
// which points to the per-type MessageInfo and descriptors that protobuf initializes lazily, so walking
// it would both count the shared metadata for every message and race with protobuf operations on other
// messages; and time.Location, which is shared by all times in that location.
// On decoded Aptos blocks the estimate is within 2% of the real heap; plain deep-size walkers report
// about half of it.
//
// Known gaps, it is an estimate:
//   - pointers into the middle of another object and data the runtime does not put on the heap (e.g.
//     string literals) are counted as separate allocations
//   - a 16-byte tiny block stays alive while any object in it does, so tiny objects interleaved with
//     short-lived ones take more than their size; data made mostly of strings under 16 bytes is
//     undercounted (by up to a quarter in the worst case measured)
//
// The model is for 64-bit platforms.
package memsize

import (
	"math"
	"math/bits"
	"reflect"
	"runtime/metrics"
	"sort"
	"sync"
	"time"
	"unsafe"

	"google.golang.org/protobuf/runtime/protoimpl"
)

const (
	maxSmallSize           = 32768
	pageSize               = 8192
	tinySize               = 16
	minSizeForMallocHeader = 512
	mallocHeaderSize       = 8

	mapHeaderSize         = 48 // internal/runtime/maps.Map
	mapTableSize          = 32 // internal/runtime/maps.table
	mapGroupSlots         = 8
	mapCtrlSize           = 8
	mapMaxAvgGroupLoad    = 7
	mapMaxTableCapacity   = 1024
	mapMinTableCapacity   = 2 * mapGroupSlots
	mapMaxKeyOrElemInline = 128
)

var sizeClasses = loadSizeClasses()

// loadSizeClasses reads the size classes from the buckets of the allocs-by-size histogram, whose
// boundaries are the size classes shifted up by 1.
func loadSizeClasses() []uint64 {
	samples := []metrics.Sample{{Name: "/gc/heap/allocs-by-size:bytes"}}
	metrics.Read(samples)
	var classes []uint64
	if samples[0].Value.Kind() == metrics.KindFloat64Histogram {
		for _, b := range samples[0].Value.Float64Histogram().Buckets {
			if b > 1 && !math.IsInf(b, 0) && b <= maxSmallSize+1 {
				classes = append(classes, uint64(b)-1)
			}
		}
	}
	if len(classes) == 0 {
		// fallback: multiples of 8 up to maxSmallSize
		for s := uint64(8); s <= maxSmallSize; s += 8 {
			classes = append(classes, s)
		}
	}
	return classes
}

// AllocSize returns the memory taken by one heap allocation of n bytes.
func AllocSize(n uint64, hasPointers bool) uint64 {
	if n == 0 {
		return 0
	}
	if !hasPointers && n < tinySize {
		return n
	}
	if hasPointers && n > minSizeForMallocHeader && n <= maxSmallSize-mallocHeaderSize {
		n += mallocHeaderSize
	}
	if n > maxSmallSize {
		return (n + pageSize - 1) / pageSize * pageSize
	}
	i := sort.Search(len(sizeClasses), func(i int) bool { return sizeClasses[i] >= n })
	if i == len(sizeClasses) {
		return (n + pageSize - 1) / pageSize * pageSize
	}
	return sizeClasses[i]
}

// Of returns the estimated heap memory held by v: the object v points to (if v is a pointer) and
// everything reachable from it. For a non-pointer v, the memory of v itself is counted as one allocation.
func Of(v any) uint64 {
	if v == nil {
		return 0
	}
	w := walker{visited: make(map[uintptr]struct{})}
	rv := reflect.ValueOf(v)
	if isPointerShaped(rv.Kind()) {
		return w.refs(rv)
	}
	return AllocSize(uint64(rv.Type().Size()), hasPointers(rv.Type())) + w.refs(rv)
}

type walker struct {
	visited map[uintptr]struct{}
}

// visit reports whether p is seen for the first time
func (w *walker) visit(p uintptr) bool {
	if _, has := w.visited[p]; has {
		return false
	}
	w.visited[p] = struct{}{}
	return true
}

// refs returns the heap memory referenced by v, not counting the inline memory of v itself
func (w *walker) refs(v reflect.Value) uint64 {
	switch v.Kind() {
	case reflect.String:
		n := v.Len()
		if n == 0 || !w.visit(uintptr(unsafe.Pointer(unsafe.StringData(v.String())))) {
			return 0
		}
		return AllocSize(uint64(n), false)
	case reflect.Slice:
		if v.Cap() == 0 || !w.visit(v.Pointer()) {
			return 0
		}
		et := v.Type().Elem()
		total := AllocSize(uint64(v.Cap())*uint64(et.Size()), hasPointers(et))
		if hasPointers(et) {
			for i := 0; i < v.Len(); i++ {
				total += w.refs(v.Index(i))
			}
		}
		return total
	case reflect.Array:
		var total uint64
		if hasPointers(v.Type().Elem()) {
			for i := 0; i < v.Len(); i++ {
				total += w.refs(v.Index(i))
			}
		}
		return total
	case reflect.Struct:
		var total uint64
		for i := 0; i < v.NumField(); i++ {
			if f := v.Field(i); hasPointers(f.Type()) {
				total += w.refs(f)
			}
		}
		return total
	case reflect.Pointer:
		if v.IsNil() || opaqueTypes[v.Type().Elem()] || !w.visit(v.Pointer()) {
			return 0
		}
		et := v.Type().Elem()
		return AllocSize(uint64(et.Size()), hasPointers(et)) + w.refs(v.Elem())
	case reflect.Interface:
		if v.IsNil() {
			return 0
		}
		e := v.Elem()
		if isPointerShaped(e.Kind()) {
			return w.refs(e)
		}
		return boxSize(e) + w.refs(e)
	case reflect.Map:
		if v.IsNil() || !w.visit(v.Pointer()) {
			return 0
		}
		t := v.Type()
		total := mapSize(v.Len(), t.Key(), t.Elem())
		kp, ep := hasPointers(t.Key()), hasPointers(t.Elem())
		if kp || ep {
			it := v.MapRange()
			for it.Next() {
				if kp {
					total += w.refs(it.Key())
				}
				if ep {
					total += w.refs(it.Value())
				}
			}
		}
		return total
	default:
		return 0
	}
}

func isPointerShaped(k reflect.Kind) bool {
	switch k {
	case reflect.Pointer, reflect.Map, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return true
	default:
		return false
	}
}

// boxSize returns the allocation made to store the non-pointer value e in an interface,
// 0 for the values the runtime takes from static memory
func boxSize(e reflect.Value) uint64 {
	size := uint64(e.Type().Size())
	if size <= 1 {
		return 0
	}
	switch e.Kind() {
	case reflect.String:
		if e.Len() == 0 {
			return 0
		}
	case reflect.Slice:
		if e.Pointer() == 0 {
			return 0
		}
	case reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		if uint64(e.Int()) < 256 {
			return 0
		}
	case reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
		if e.Uint() < 256 {
			return 0
		}
	case reflect.Float32:
		if math.Float32bits(float32(e.Float())) < 256 {
			return 0
		}
	case reflect.Float64:
		if math.Float64bits(e.Float()) < 256 {
			return 0
		}
	default:
	}
	return AllocSize(size, hasPointers(e.Type()))
}

// mapSize returns the memory of the map structure holding n entries, not counting what keys and
// elems reference
func mapSize(n int, kt, et reflect.Type) uint64 {
	ksize, esize := uint64(kt.Size()), uint64(et.Size())
	kalign, ealign := uint64(kt.Align()), uint64(et.Align())
	slotHasPointers := hasPointers(kt) || hasPointers(et)
	var indirect uint64
	if ksize > mapMaxKeyOrElemInline {
		indirect += AllocSize(ksize, hasPointers(kt))
		ksize, kalign, slotHasPointers = 8, 8, true
	}
	if esize > mapMaxKeyOrElemInline {
		indirect += AllocSize(esize, hasPointers(et))
		esize, ealign, slotHasPointers = 8, 8, true
	}
	slotAlign := max(kalign, ealign, 1)
	slotSize := alignUp(alignUp(ksize, ealign)+esize, slotAlign)
	groupSize := alignUp(mapCtrlSize, slotAlign) + mapGroupSlots*slotSize

	total := AllocSize(mapHeaderSize, true) + uint64(n)*indirect
	switch {
	case n == 0:
		// make(map) without a hint allocates the group on the first insert
	case n <= mapGroupSlots:
		total += AllocSize(groupSize, slotHasPointers)
	default:
		capacity, tables := tableLayout(uint64(n))
		perTable := AllocSize(mapTableSize, true) + AllocSize(capacity/mapGroupSlots*groupSize, slotHasPointers)
		total += tables*perTable + AllocSize(tables*8, true)
	}
	return total
}

// tableLayout returns the capacity of each table and the number of tables for n entries
func tableLayout(n uint64) (capacity, tables uint64) {
	maxPerTable := uint64(mapMaxTableCapacity * mapMaxAvgGroupLoad / mapGroupSlots)
	if n <= maxPerTable {
		capacity = mapMinTableCapacity
		for capacity*mapMaxAvgGroupLoad/mapGroupSlots < n {
			capacity *= 2
		}
		return capacity, 1
	}
	// a full table splits into two full-capacity tables, the directory grows by powers of 2
	need := (n + maxPerTable - 1) / maxPerTable
	return mapMaxTableCapacity, uint64(1) << bits.Len64(need-1)
}

func alignUp(n, a uint64) uint64 {
	return (n + a - 1) / a * a
}

// opaqueTypes hold runtime state shared by many values, their inline size is counted with the value
// containing them but what they reference is neither walked nor counted
var opaqueTypes = map[reflect.Type]bool{
	reflect.TypeFor[protoimpl.MessageState](): true,
	reflect.TypeFor[time.Location]():          true,
}

var pointerCache sync.Map // reflect.Type -> bool

// hasPointers reports whether values of t may reference heap memory that should be walked
func hasPointers(t reflect.Type) bool {
	if cached, ok := pointerCache.Load(t); ok {
		return cached.(bool)
	}
	// opaque types are reported pointer-free so that nothing walks into them
	result := !opaqueTypes[t] && hasPointersByKind(t)
	pointerCache.Store(t, result)
	return result
}

func hasPointersByKind(t reflect.Type) bool {
	var result bool
	switch t.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Chan, reflect.Func, reflect.UnsafePointer,
		reflect.Interface, reflect.Slice, reflect.String:
		result = true
	case reflect.Array:
		result = t.Len() > 0 && hasPointers(t.Elem())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if hasPointers(t.Field(i).Type) {
				result = true
				break
			}
		}
	default:
	}
	return result
}
