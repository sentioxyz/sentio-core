package queue

import "sync"

type Circular[V any] interface {
	Push(v V)
	Dump(reverse bool) (result []V)
	Total() int
}

type circular[V any] struct {
	maxSize int

	data []V
	c    int
}

type safeCircular[V any] struct {
	circular[V]

	mu sync.Mutex
}

func NewSafeCircular[V any](maxSize int) Circular[V] {
	return &safeCircular[V]{
		circular: circular[V]{
			data:    make([]V, maxSize),
			maxSize: maxSize,
		},
	}
}

func NewCircular[V any](maxSize int) Circular[V] {
	return &circular[V]{
		data:    make([]V, maxSize),
		maxSize: maxSize,
	}
}

func (q *circular[V]) Push(v V) {
	index := q.c
	q.c++
	q.data[index%q.maxSize] = v
}

func (q *circular[V]) Total() int {
	return q.c
}

func (q *circular[V]) Dump(reverse bool) (result []V) {
	if q.c < q.maxSize {
		result = make([]V, q.c)
		copy(result[:], q.data[0:q.c])
	} else {
		result = make([]V, q.maxSize)
		s := q.c % q.maxSize
		copy(result[:q.maxSize-s], q.data[s:])
		copy(result[q.maxSize-s:], q.data[0:s])
	}
	if reverse {
		for i := 0; i < len(result)-1-i; i++ {
			result[i], result[len(result)-1-i] = result[len(result)-1-i], result[i]
		}
	}
	return result
}

func (q *safeCircular[V]) Push(v V) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.circular.Push(v)
}

func (q *safeCircular[V]) Dump(reverse bool) (result []V) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.circular.Dump(reverse)
}

func (q *safeCircular[V]) Total() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.circular.Total()
}
