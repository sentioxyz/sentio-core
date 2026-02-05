package set

type Set[V comparable] interface {
	Add(vs ...V)
	Remove(vs ...V)
	Truncate()

	Contains(v V) bool
	DumpValues() []V
	Size() int
	Empty() bool
}
