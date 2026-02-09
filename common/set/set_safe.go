package set

import "sync"

type safeSet[V comparable] struct {
	data Set[V]
	mu   sync.RWMutex
}

func NewSafe[V comparable](initItems ...V) Set[V] {
	return &safeSet[V]{
		data: New(initItems...),
	}
}

func SmartNewSafe[V comparable](initItems ...any) Set[V] {
	return &safeSet[V]{
		data: SmartNew[V](initItems...),
	}
}

func (s *safeSet[V]) Add(vs ...V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Add(vs...)
}

func (s *safeSet[V]) Remove(vs ...V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Remove(vs...)
}

func (s *safeSet[V]) Truncate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Truncate()
}

func (s *safeSet[V]) Contains(v V) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Contains(v)
}

func (s *safeSet[V]) DumpValues() []V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.DumpValues()
}

func (s *safeSet[V]) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Size()
}

func (s *safeSet[V]) Empty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Empty()
}
