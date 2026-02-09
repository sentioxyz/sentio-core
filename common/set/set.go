package set

import "fmt"

type set[V comparable] map[V]struct{}

func New[V comparable](initItems ...V) Set[V] {
	s := make(set[V])
	for _, item := range initItems {
		s[item] = struct{}{}
	}
	return s
}

func SmartNew[V comparable](initItems ...any) Set[V] {
	s := make(set[V])
	for _, item := range initItems {
		switch x := item.(type) {
		case V:
			s[x] = struct{}{}
		case []V:
			for _, xi := range x {
				s[xi] = struct{}{}
			}
		default:
			panic(fmt.Sprintf("invalid item (%T): %v", item, x))
		}
	}
	return s
}

func (s set[V]) Add(vs ...V) {
	for _, v := range vs {
		s[v] = struct{}{}
	}
}

func (s set[V]) Remove(vs ...V) {
	for _, v := range vs {
		delete(s, v)
	}
}

func (s set[V]) Truncate() {
	for k := range s {
		delete(s, k)
	}
}

func (s set[V]) Contains(v V) bool {
	_, ok := s[v]
	return ok
}

func (s set[V]) DumpValues() []V {
	ret := make([]V, 0, len(s))
	for v := range s {
		ret = append(ret, v)
	}
	return ret
}

func (s set[V]) Size() int {
	return len(s)
}

func (s set[V]) Empty() bool {
	return len(s) == 0
}
