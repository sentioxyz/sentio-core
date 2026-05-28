package set

type Set[V comparable] interface {
	Add(vs ...V)
	Remove(vs ...V)
	Truncate()

	Contains(v V) bool
	DumpValues() []V
	Traverse(f func(v V))
	Size() int
	Empty() bool
}
