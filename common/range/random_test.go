package rg

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

// randomRange returns a random finite range within [0, maxVal].
func randomRange(r *rand.Rand, maxVal uint64) Range {
	a := uint64(r.Intn(int(maxVal) + 1))
	b := uint64(r.Intn(int(maxVal) + 1))
	if a > b {
		a, b = b, a
	}
	return NewRange(a, b)
}

// randomRangeSet returns a RangeSet built from up to maxSegments random finite ranges.
func randomRangeSet(r *rand.Rand, maxVal uint64, maxSegments int) RangeSet {
	n := r.Intn(maxSegments) + 1
	ranges := make([]Range, n)
	for i := range ranges {
		ranges[i] = randomRange(r, maxVal)
	}
	return NewRangeSet(ranges...)
}

func Test_IntersectionSetRandom(t *testing.T) {
	const maxVal = 50
	r := rand.New(rand.NewSource(42))

	for i := 0; i < 1000; i++ {
		a := randomRangeSet(r, maxVal, 5)
		b := randomRangeSet(r, maxVal, 5)
		result := a.IntersectionSet(b)

		for x := uint64(0); x <= maxVal; x++ {
			want := a.Contains(x) && b.Contains(x)
			assert.Equalf(t, want, result.Contains(x),
				"iter=%d x=%d a=%s b=%s result=%s", i, x, a, b, result)
		}

		// commutativity: A∩B == B∩A
		resultBA := b.IntersectionSet(a)
		for x := uint64(0); x <= maxVal; x++ {
			assert.Equalf(t, result.Contains(x), resultBA.Contains(x),
				"commutativity iter=%d x=%d a=%s b=%s", i, x, a, b)
		}

		// idempotency: A∩A == A
		resultAA := a.IntersectionSet(a)
		for x := uint64(0); x <= maxVal; x++ {
			assert.Equalf(t, a.Contains(x), resultAA.Contains(x),
				"idempotency iter=%d x=%d a=%s", i, x, a)
		}
	}
}

func Test_UnionSetRandom(t *testing.T) {
	const maxVal = 50
	r := rand.New(rand.NewSource(43))

	for i := 0; i < 1000; i++ {
		a := randomRangeSet(r, maxVal, 5)
		b := randomRangeSet(r, maxVal, 5)
		result := a.UnionSet(b)

		for x := uint64(0); x <= maxVal; x++ {
			want := a.Contains(x) || b.Contains(x)
			assert.Equalf(t, want, result.Contains(x),
				"iter=%d x=%d a=%s b=%s result=%s", i, x, a, b, result)
		}

		// commutativity: A∪B == B∪A
		resultBA := b.UnionSet(a)
		for x := uint64(0); x <= maxVal; x++ {
			assert.Equalf(t, result.Contains(x), resultBA.Contains(x),
				"commutativity iter=%d x=%d a=%s b=%s", i, x, a, b)
		}
	}
}

func Test_RemoveSetRandom(t *testing.T) {
	const maxVal = 50
	r := rand.New(rand.NewSource(44))

	for i := 0; i < 1000; i++ {
		a := randomRangeSet(r, maxVal, 5)
		b := randomRangeSet(r, maxVal, 5)
		result := a.RemoveSet(b)

		for x := uint64(0); x <= maxVal; x++ {
			want := a.Contains(x) && !b.Contains(x)
			assert.Equalf(t, want, result.Contains(x),
				"iter=%d x=%d a=%s b=%s result=%s", i, x, a, b, result)
		}
	}
}

// Test_IntersectionUnionDecomposeRandom verifies (A∩B) ∪ (A\B) == A.
func Test_IntersectionUnionDecomposeRandom(t *testing.T) {
	const maxVal = 50
	r := rand.New(rand.NewSource(45))

	for i := 0; i < 1000; i++ {
		a := randomRangeSet(r, maxVal, 5)
		b := randomRangeSet(r, maxVal, 5)

		inter := a.IntersectionSet(b)
		diff := a.RemoveSet(b)
		union := inter.UnionSet(diff)

		for x := uint64(0); x <= maxVal; x++ {
			assert.Equalf(t, a.Contains(x), union.Contains(x),
				"decompose iter=%d x=%d a=%s b=%s", i, x, a, b)
		}
	}
}

func Test_setRemoveRandom(t *testing.T) {
	const maxVal = 50
	r := rand.New(rand.NewSource(46))

	for i := 0; i < 1000; i++ {
		rs := randomRangeSet(r, maxVal, 5)
		rng := randomRange(r, maxVal)
		result := rs.Remove(rng)

		for x := uint64(0); x <= maxVal; x++ {
			want := rs.Contains(x) && !rng.Contains(x)
			assert.Equalf(t, want, result.Contains(x),
				"iter=%d x=%d rs=%s rng=%s result=%s", i, x, rs, rng, result)
		}
	}
}

func Test_setUnionRandom(t *testing.T) {
	const maxVal = 50
	r := rand.New(rand.NewSource(47))

	for i := 0; i < 1000; i++ {
		rs := randomRangeSet(r, maxVal, 5)
		rng := randomRange(r, maxVal)
		result := rs.Union(rng)

		for x := uint64(0); x <= maxVal; x++ {
			want := rs.Contains(x) || rng.Contains(x)
			assert.Equalf(t, want, result.Contains(x),
				"iter=%d x=%d rs=%s rng=%s result=%s", i, x, rs, rng, result)
		}
	}
}

func Test_setIntersectionRandom(t *testing.T) {
	const maxVal = 50
	r := rand.New(rand.NewSource(48))

	for i := 0; i < 1000; i++ {
		rs := randomRangeSet(r, maxVal, 5)
		rng := randomRange(r, maxVal)
		result := rs.Intersection(rng)

		for x := uint64(0); x <= maxVal; x++ {
			want := rs.Contains(x) && rng.Contains(x)
			assert.Equalf(t, want, result.Contains(x),
				"iter=%d x=%d rs=%s rng=%s result=%s", i, x, rs, rng, result)
		}
	}
}
