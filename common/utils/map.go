package utils

import (
	"fmt"
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
	"math/big"
	"sort"
	"strings"
	"sync"
)

type SafeMap[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{data: make(map[K]V)}
}

func (m *SafeMap[K, V]) Put(key K, val V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = val
}

func (m *SafeMap[K, V]) Del(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

func (m *SafeMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, has := m.data[key]
	return val, has
}

func (m *SafeMap[K, V]) GetWithDefault(key K, defVal V) V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, has := m.data[key]
	if !has {
		val = defVal
	}
	return val
}

func (m *SafeMap[K, V]) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

func (m *SafeMap[K, V]) Traverse(fn func(key K, val V)) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.data {
		fn(k, v)
	}
}

func (m *SafeMap[K, V]) Dump() map[K]V {
	r := make(map[K]V)
	m.Traverse(func(key K, val V) {
		r[key] = val
	})
	return r
}

func UpdateK2Map[K1 comparable, K2 comparable, V any](m map[K1]map[K2]V, k1 K1, k2 K2, update func(old V) V) {
	m2, has := m[k1]
	if !has {
		m2 = make(map[K2]V)
		m[k1] = m2
	}
	m2[k2] = update(m2[k2])
}

func PutIntoK2Map[K1 comparable, K2 comparable, V any](m map[K1]map[K2]V, k1 K1, k2 K2, v V) {
	m2, has := m[k1]
	if !has {
		m2 = make(map[K2]V)
		m[k1] = m2
	}
	m2[k2] = v
}

func IncrK2Map[K1 comparable, K2 comparable, V constraints.Integer](m map[K1]map[K2]V, k1 K1, k2 K2, delta V) {
	origin, _ := GetFromK2Map(m, k1, k2)
	PutIntoK2Map(m, k1, k2, origin+delta)
}

func PutAll[K comparable, V any](dst, src map[K]V) {
	for k, v := range src {
		dst[k] = v
	}
}

func MapAdd[K comparable, V constraints.Integer](items ...map[K]V) map[K]V {
	r := make(map[K]V)
	for _, item := range items {
		for k, v := range item {
			r[k] += v
		}
	}
	return r
}

func MergeInto[V comparable](dst map[V]bool, items []V) map[V]bool {
	for _, item := range items {
		dst[item] = true
	}
	return dst
}

func MergeMap[K comparable, V any](dst map[K]V, data map[K]V) map[K]V {
	for k, v := range data {
		dst[k] = v
	}
	return dst
}

func AppendArrMap[K comparable, V any](dst map[K][]V, data map[K][]V) map[K][]V {
	if len(data) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[K][]V)
	}
	for k, v := range data {
		dst[k] = append(dst[k], v...)
	}
	return dst
}

func MergeMapSumByFunc[K comparable, V any](items []map[K]V, add func(a, b V) V) map[K]V {
	if len(items) == 0 {
		return make(map[K]V)
	}
	sum := CopyMap(items[0])
	for i := 1; i < len(items); i++ {
		for k, v := range items[i] {
			sum[k] = add(sum[k], v)
		}
	}
	return sum
}

func MergeMapSum[K comparable, V constraints.Integer](items ...map[K]V) map[K]V {
	if len(items) == 0 {
		return make(map[K]V)
	}
	sum := CopyMap(items[0])
	for i := 1; i < len(items); i++ {
		for k, v := range items[i] {
			sum[k] += v
		}
	}
	return sum
}

func MergeMapSub[K comparable, V constraints.Integer](base map[K]V, ex map[K]V) map[K]V {
	res := CopyMap(base)
	for k, v := range ex {
		res[k] -= v
	}
	return res
}

func MergeMapBigSum[K comparable](dst, another map[K]*big.Int) {
	for k, v := range another {
		if v == nil {
			continue
		}
		dst[k] = AddBigInt(dst[k], v)
	}
}

func MapDelete[K comparable, V any](m map[K]V, filter func(K) bool) {
	for k := range m {
		if filter(k) {
			delete(m, k)
		}
	}
}

func GetFromK2Map[K1 comparable, K2 comparable, V any](m map[K1]map[K2]V, k1 K1, k2 K2) (v V, has bool) {
	m2, has := m[k1]
	if !has {
		return v, false
	}
	v, has = m2[k2]
	return
}

func GetFromMapWithDefault[K comparable, V any](m map[K]V, k K, defaultValue V) V {
	if v, has := m[k]; has {
		return v
	}
	return defaultValue
}

func CountMap[K comparable, V any](m map[K][]V) int {
	var count int
	for _, v := range m {
		count += len(v)
	}
	return count
}

func Stat[V comparable](data []V) map[V]int {
	r := make(map[V]int)
	for _, v := range data {
		r[v] += 1
	}
	return r
}

func SumMap[K comparable, V constraints.Integer](m map[K]V) V {
	var sum V
	for _, v := range m {
		sum += v
	}
	return sum
}

func SumK2Map[K1 comparable, K2 comparable, V constraints.Integer](m map[K1]map[K2]V) V {
	var sum V
	for _, d := range m {
		for _, v := range d {
			sum += v
		}
	}
	return sum
}

func CopyMap[K comparable, V any](m map[K]V) map[K]V {
	n := make(map[K]V, len(m))
	for k, v := range m {
		n[k] = v
	}
	return n
}

func PrintCountMap[K comparable, V any](m map[K][]V) string {
	sec := make([]string, 0, len(m))
	for k, v := range m {
		sec = append(sec, fmt.Sprintf("%v:%d", k, len(v)))
	}
	return strings.Join(sec, ",")
}

func PrintStatMap(m map[string]int, top int) string {
	type entry struct {
		Key string
		Num int
	}
	var entries []entry
	for k, c := range m {
		entries = append(entries, entry{Key: k, Num: c})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Num > entries[j].Num
	})
	sec := make([]string, 0, top)
	for i := 0; i < top && i < len(entries); i++ {
		sec = append(sec, fmt.Sprintf("%s:%d", entries[i].Key, entries[i].Num))
	}
	return strings.Join(sec, ",")
}

func DelFromK2Map[K1 comparable, K2 comparable, V any](m map[K1]map[K2]V, k1 K1, k2 K2) bool {
	m2, has := m[k1]
	if !has {
		return false
	}
	if _, has = m2[k2]; !has {
		return false
	}
	delete(m2, k2)
	return true
}

func TravelK2Map[K1 comparable, K2 comparable, V any](m map[K1]map[K2]V, handler func(k1 K1, k2 K2, v V) error) error {
	for k1, m2 := range m {
		for k2, v := range m2 {
			if err := handler(k1, k2, v); err != nil {
				return err
			}
		}
	}
	return nil
}

func GetMapKeys[K comparable, V any](m map[K]V) []K {
	var keys []K
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func GetOrderedMapKeys[K constraints.Ordered, V any](m map[K]V) []K {
	keys := GetMapKeys(m)
	slices.Sort(keys)
	return keys
}

func GetMapValues[K constraints.Ordered, V any](m map[K]V) []V {
	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

func GetMapValuesOrderByKey[K constraints.Ordered, V any](m map[K]V) []V {
	keys := GetMapKeys(m)
	slices.Sort(keys)
	values := make([]V, len(keys))
	for i, key := range keys {
		values[i] = m[key]
	}
	return values
}

func TraverseMapInOrder[K constraints.Ordered, V any](m map[K]V, f func(k K, v V)) {
	for _, k := range GetOrderedMapKeys(m) {
		f(k, m[k])
	}
}

func CountRich[V any, K constraints.Ordered](items []V, getKey func(V) K) map[K]int {
	count := make(map[K]int)
	for _, item := range items {
		count[getKey(item)] += 1
	}
	return count
}

func CountRichV2[V any, K constraints.Ordered](items []V, getKey func(V) (K, bool)) map[K]int {
	count := make(map[K]int)
	for _, item := range items {
		if key, valid := getKey(item); valid {
			count[key] += 1
		}
	}
	return count
}

func Count[K constraints.Ordered](items []K) map[K]int {
	count := make(map[K]int)
	for _, item := range items {
		count[item] += 1
	}
	return count
}
