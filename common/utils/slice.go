package utils

import (
	"golang.org/x/exp/constraints"
	"slices"
	"sync"
)

type SafeSlice[T any] struct {
	arr []T
	mu  sync.Mutex
}

func NewSafeSlice[T any](cap int) *SafeSlice[T] {
	return &SafeSlice[T]{arr: make([]T, 0, cap)}
}

func (s *SafeSlice[T]) Set(index int, item T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var zero T
	for index >= len(s.arr) {
		s.arr = append(s.arr, zero)
	}
	s.arr[index] = item
}

func (s *SafeSlice[T]) Append(items ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.arr = append(s.arr, items...)
}

func (s *SafeSlice[T]) Dump() []T {
	s.mu.Lock()
	defer s.mu.Unlock()
	arr := s.arr
	s.arr = nil
	return arr
}

type SafeMapSlice[K constraints.Ordered, V any] struct {
	mu   sync.Mutex
	data map[K][]V
}

func NewSafeMapSlice[K constraints.Ordered, V any]() *SafeMapSlice[K, V] {
	return &SafeMapSlice[K, V]{data: make(map[K][]V)}
}

func (s *SafeMapSlice[K, V]) Append(k K, v V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[k] = append(s.data[k], v)
}

func (s *SafeMapSlice[K, V]) Dump() []V {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := GetMapKeys(s.data)
	slices.Sort(keys)
	var values []V
	for _, key := range keys {
		values = append(values, s.data[key]...)
	}
	return values
}
