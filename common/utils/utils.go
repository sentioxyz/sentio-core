package utils

import (
	"context"
	lru "github.com/sentioxyz/golang-lru"
	"golang.org/x/exp/constraints"
	"math/big"
	"regexp"
	"time"
)

func MapSlice[T any, M any](a []T, f func(T) (M, error)) ([]M, error) {
	n := make([]M, len(a))
	var err error
	for i, e := range a {
		var m M
		if m, err = f(e); err != nil {
			return n, err
		}
		n[i] = m
	}
	return n, nil
}

func MustMapSlice[T any, M any](a []T, f func(T) (M, error)) []M {
	n := make([]M, len(a))
	var err error
	for i, e := range a {
		var m M
		if m, err = f(e); err != nil {
			panic(err)
		}
		n[i] = m
	}
	return n
}

func MapD2Slice[T any, M any](a [][]T, f func(T) (M, error)) ([][]M, error) {
	n := make([][]M, len(a))
	var err error
	for i, e := range a {
		if n[i], err = MapSlice(e, f); err != nil {
			return n, err
		}
	}
	return n, nil
}

func ReduceMapValues[K comparable, V any](m map[K]V, merge func(a, b V) V) V {
	var r V
	for _, v := range m {
		r = merge(r, v)
	}
	return r
}

func Reduce[V any](a []V, merge func(a, b V) V) V {
	var r V
	if len(a) == 0 {
		return r
	}
	if len(a) == 1 {
		return a[0]
	}
	r = a[0]
	for i := 1; i < len(a); i++ {
		r = merge(r, a[i])
	}
	return r
}

func MapSliceNoErrWithIndex[T any, M any](a []T, f func(int, T) (M, bool)) []M {
	n := make([]M, 0, len(a))
	for i, e := range a {
		if ex, ok := f(i, e); ok {
			n = append(n, ex)
		}
	}
	return n
}

func MapSliceNoError[T any, M any](a []T, f func(T) M) []M {
	n := make([]M, len(a))
	for i, e := range a {
		n[i] = f(e)
	}
	return n
}

func MapMapNoError[K comparable, V1 any, V2 any](origin map[K]V1, f func(V1) V2) map[K]V2 {
	n := make(map[K]V2, len(origin))
	for k, v1 := range origin {
		n[k] = f(v1)
	}
	return n
}

func MapAndMergeNoError[F any, T any](src []F, mapFn func(F) []T) []T {
	var result []T
	for _, item := range src {
		result = append(result, mapFn(item)...)
	}
	return result
}

func MaxBy[T any, R constraints.Ordered](slice []T, f func(x T) R) T {
	var max T
	for i, t := range slice {
		if i == 0 {
			max = t
		}
		if f(t) > f(max) {
			max = t
		}
	}
	return max
}

func MinBy[T any, R constraints.Ordered](slice []T, f func(x T) R) T {
	var min T
	for i, t := range slice {
		if i == 0 {
			min = t
		}
		if f(t) < f(min) {
			min = t
		}
	}
	return min
}

func Min[T constraints.Ordered](items ...T) T {
	var min = items[0]
	for _, it := range items {
		if it < min {
			min = it
		}
	}
	return min
}

func Max[T constraints.Ordered](items ...T) T {
	var max = items[0]
	for _, it := range items {
		if it > max {
			max = it
		}
	}
	return max
}

func FilterMap[K comparable, V any](src map[K]V, filter func(K) bool) map[K]V {
	result := make(map[K]V)
	for k, v := range src {
		if filter(k) {
			result[k] = v
		}
	}
	return result
}

func Group[K comparable, V any](src []V, keyGetter func(V) K) map[K][]V {
	result := make(map[K][]V)
	for _, item := range src {
		key := keyGetter(item)
		result[key] = append(result[key], item)
	}
	return result
}

func BuildSet[T comparable](arr []T) map[T]bool {
	m := make(map[T]bool, len(arr))
	for _, it := range arr {
		m[it] = true
	}
	return m
}

func SetSub[T comparable, V any](a, b map[T]V) map[T]V {
	r := make(map[T]V)
	for k, v := range a {
		if _, has := b[k]; !has {
			r[k] = v
		}
	}
	return r
}

func Select[V any](b bool, trueValue, falseValue V) V {
	if b {
		return trueValue
	}
	return falseValue
}

func Fetch[V any](p *V, defaultValue V) V {
	if p != nil {
		return *p
	}
	return defaultValue
}

func MustReturn[T any](r T, err error) T {
	if err != nil {
		panic(err)
	}
	return r
}

func WrapPointer[T any](v T) *T {
	return &v
}

func WrapPointerForArray[T any](arr []T) []*T {
	if len(arr) == 0 {
		return nil
	}
	r := make([]*T, len(arr))
	for i := range arr {
		r[i] = &arr[i]
	}
	return r
}

func Int32Min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func ZeroOrUInt64(n *big.Int) uint64 {
	if n == nil {
		return 0
	}
	return n.Uint64()
}

func Cmp[V constraints.Ordered](a, b V) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func Sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func TryCloseChan[V any](ch chan V) {
	defer func() {
		_ = recover() // already closed
	}()
	close(ch)
}

func MatchAny(str string, matchers []*regexp.Regexp) bool {
	for _, r := range matchers {
		if r.FindString(str) != "" {
			return true
		}
	}
	return false
}

func CacheSnapshot[K comparable, V any](cache *lru.Cache[K, V], maxCount int, valuePreview func(V) string) any {
	preview := make(map[K]string)
	keys := cache.Keys()
	for i := len(keys) - 1; i >= 0; i-- {
		key := keys[i]
		if val, has := cache.Get(key); has {
			preview[key] = valuePreview(val)
			if len(preview) >= maxCount {
				break
			}
		}
	}
	return map[string]any{
		"size":    cache.Len(),
		"preview": preview,
	}
}
