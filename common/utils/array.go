package utils

import (
	"bytes"
	"fmt"
	"strings"
)

func CountD2Arr[T any](arr [][]T) (c int) {
	for _, items := range arr {
		c += len(items)
	}
	return
}

func FilterArr[T any](src []T, f func(T) bool) []T {
	var r []T
	for _, item := range src {
		if f(item) {
			r = append(r, item)
		}
	}
	return r
}

// RemoveByIndex remove the items with the index in the set remove, will destroy the origin arr
func RemoveByIndex[T any](arr []T, remove map[int]bool) []T {
	if len(remove) == 0 {
		return arr
	}
	var n int
	for i := range arr {
		if !remove[i] {
			arr[n] = arr[i]
			n++
		}
	}
	return arr[:n]
}

// RemoveSubSeq remove arr[from:from+num], will destroy the origin arr
func RemoveSubSeq[T any](arr []T, from, num int) []T {
	if num == 0 {
		return arr
	}
	if num < 0 {
		panic(fmt.Errorf("num %d should not be negative", num))
	}
	var n = from
	for i := from + num; i < len(arr); i++ {
		arr[n] = arr[i]
		n++
	}
	return arr[:n]
}

func HasAny[T any](origin []T, checker func(T) bool) bool {
	for _, item := range origin {
		if checker(item) {
			return true
		}
	}
	return false
}

func FilterArrWithErr[T any](src []T, f func(T) (bool, error)) ([]T, error) {
	var r []T
	for _, item := range src {
		if ok, err := f(item); err != nil {
			return nil, err
		} else if ok {
			r = append(r, item)
		}
	}
	return r, nil
}

func FilterD2Arr[T any](src [][]T, f func(T) bool) [][]T {
	var r [][]T
	for _, items := range src {
		r = append(r, FilterArr(items, f))
	}
	return r
}

func FilterArrBuffered[T any](src []T, f func(*T) bool) []T {
	r := make([]T, 0, len(src))
	for i := range src {
		if f(&src[i]) {
			r = append(r, src[i])
		}
	}
	return r
}

func MergeArr[T any](arrs ...[]T) []T {
	switch len(arrs) {
	case 0:
		return nil
	case 1:
		return arrs[0]
	default:
		var totalLen int
		for _, arr := range arrs {
			totalLen += len(arr)
		}
		result := make([]T, 0, totalLen)
		for _, arr := range arrs {
			result = append(result, arr...)
		}
		return result
	}
}

func Prepend[T any](arr []T, heads ...T) []T {
	newArr := make([]T, len(arr)+len(heads))
	copy(newArr[0:], heads)
	copy(newArr[len(heads):], arr[:])
	return newArr
}

func NotNull[T any](obj T) bool {
	var a any = obj
	return a != nil
}

func ArrEqual[T comparable](a1, a2 []T) bool {
	if len(a1) != len(a2) {
		return false
	}
	for i, it := range a1 {
		if it != a2[i] {
			return false
		}
	}
	return true
}

func ArrSummary[T any](arr []T, headerAndTail ...int) string {
	var headerCount, tailCount int
	switch len(headerAndTail) {
	case 0:
		headerCount, tailCount = 3, 3
	case 1:
		headerCount, tailCount = headerAndTail[0], headerAndTail[0]
	case 2:
		headerCount, tailCount = headerAndTail[0], headerAndTail[1]
	default:
		panic("too many arguments")
	}
	count := len(arr)
	if count <= headerCount+tailCount {
		return fmt.Sprintf("%v", arr)
	}
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < headerCount; i++ {
		b.WriteString(fmt.Sprintf("%v,", arr[i]))
	}
	b.WriteString(fmt.Sprintf("...(ignored %d items)...", count-headerCount-tailCount))
	for i := tailCount; i > 0; i-- {
		b.WriteString(fmt.Sprintf(",%v", arr[count-i]))
	}
	b.WriteString("]")
	return b.String()
}

func ToAnyArray[T any](arr []T) []any {
	r := make([]any, 0, len(arr))
	for _, item := range arr {
		r = append(r, item)
	}
	return r
}

func In[T comparable](x T, set ...T) bool {
	return IndexOf(set, x) >= 0
}

func IndexOf[T comparable](arr []T, target T) int {
	for i, item := range arr {
		if item == target {
			return i
		}
	}
	return -1
}

func HasPrefix[T comparable](orig []T, prefix []T) bool {
	if len(orig) < len(prefix) {
		return false
	}
	for i := range prefix {
		if orig[i] != prefix[i] {
			return false
		}
	}
	return true
}

func Reverse[T any](arr []T) []T {
	for i := 0; i*2 < len(arr); i++ {
		arr[i], arr[len(arr)-i-1] = arr[len(arr)-i-1], arr[i]
	}
	return arr
}

func ShowArray[T fmt.Stringer](arr []T, delimiter string) string {
	var buf bytes.Buffer
	for i, item := range arr {
		if i > 0 {
			buf.WriteString(delimiter)
		}
		buf.WriteString(item.String())
	}
	return buf.String()
}

func Dedup[T any](raw []T, hashFunc func(T) string) []T {
	has := make(map[string]bool)
	result := make([]T, 0, len(raw))
	for _, item := range raw {
		hash := hashFunc(item)
		if has[hash] {
			continue
		}
		has[hash] = true
		result = append(result, item)
	}
	return result
}
