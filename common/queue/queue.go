package queue

import "container/list"

type Queue[V any] interface {
	PushBack(V)
	PopFront() (V, bool)
	Front() (V, bool)
	Back() (V, bool)
	Len() int
	Reset()
}

type queue[V any] struct {
	list list.List
}

func NewQueue[V any]() Queue[V] {
	q := &queue[V]{}
	q.list.Init()
	return q
}

func (q *queue[V]) PushBack(v V) {
	q.list.PushBack(v)
}

func (q *queue[V]) PopFront() (v V, has bool) {
	e := q.list.Front()
	if e == nil {
		return v, false
	}
	q.list.Remove(e)
	return e.Value.(V), true
}

func (q *queue[V]) Front() (v V, has bool) {
	e := q.list.Front()
	if e == nil {
		return v, false
	}
	return e.Value.(V), true
}

func (q *queue[V]) Back() (v V, has bool) {
	e := q.list.Back()
	if e == nil {
		return v, false
	}
	return e.Value.(V), true
}

func (q *queue[V]) Len() int {
	return q.list.Len()
}

func (q *queue[V]) Reset() {
	q.list.Init()
}
