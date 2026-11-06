package memsize

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func heapAlloc() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// measure returns the real heap bytes held by one value built by build, averaged over n copies
func measure(n int, build func(i int) any) float64 {
	keep := make([]any, n)
	before := heapAlloc()
	for i := range keep {
		keep[i] = build(i)
	}
	after := heapAlloc()
	runtime.KeepAlive(keep)
	return float64(after-before) / float64(n)
}

func randomJSON(r *rand.Rand, depth int) any {
	if depth <= 0 {
		switch r.Intn(5) {
		case 0:
			return r.Float64() * 1e6
		case 1:
			return r.Intn(2) == 0
		case 2:
			return fmt.Sprintf("0x%064x", r.Uint64())
		case 3:
			return strings.Repeat("a", r.Intn(40))
		default:
			return fmt.Sprintf("%d", r.Int63())
		}
	}
	if r.Intn(3) == 0 {
		arr := make([]any, r.Intn(6))
		for i := range arr {
			arr[i] = randomJSON(r, depth-1)
		}
		return arr
	}
	m := make(map[string]any)
	for i, n := 0, 1+r.Intn(12); i < n; i++ {
		m[fmt.Sprintf("field_%d", r.Intn(30))] = randomJSON(r, depth-1)
	}
	return m
}

type item struct {
	ID     uint64
	Name   string
	Tags   []string
	Attrs  map[string]string
	Next   *item
	Values []float64
}

type bigValue struct {
	A [20]uint64
	S string
}

func TestOfMatchesHeap(t *testing.T) {
	cases := []struct {
		name  string
		n     int
		build func(i int) any
	}{
		{"decoded json", 200, func(i int) any {
			raw, _ := json.Marshal(randomJSON(rand.New(rand.NewSource(int64(i%20))), 5))
			var v any
			_ = json.Unmarshal(raw, &v)
			return &v
		}},
		{"decoded json slice", 50, func(i int) any {
			r := rand.New(rand.NewSource(int64(i % 10)))
			docs := make([]any, 30)
			for j := range docs {
				docs[j] = randomJSON(r, 3)
			}
			raw, _ := json.Marshal(docs)
			var v []any
			_ = json.Unmarshal(raw, &v)
			return &v
		}},
		{"struct graph", 500, func(i int) any {
			head := &item{ID: 1, Name: strings.Repeat("n", 30)}
			cur := head
			for j := 0; j < 20; j++ {
				cur.Next = &item{
					ID:     uint64(j),
					Name:   fmt.Sprintf("item-%d-%d", i, j),
					Tags:   []string{fmt.Sprintf("t%d", j), fmt.Sprintf("tag-%d", i)},
					Attrs:  map[string]string{"k": fmt.Sprintf("v%d", j), "kk": strings.Repeat("v", j)},
					Values: make([]float64, j),
				}
				cur = cur.Next
			}
			return head
		}},
		{"small map", 2000, func(i int) any {
			return map[string]int{"a": i, "b": 2, "c": 3}
		}},
		{"medium map", 200, func(i int) any {
			m := make(map[string]int)
			for j := 0; j < 300; j++ {
				m[fmt.Sprintf("map-key-%04d-%05d", i, j)] = j
			}
			return m
		}},
		{"split map", 10, func(i int) any {
			m := make(map[int64]int64)
			for j := 0; j < 20000; j++ {
				m[int64(j)] = int64(i)
			}
			return m
		}},
		{"map with big values", 50, func(i int) any {
			m := make(map[int]bigValue)
			for j := 0; j < 50; j++ {
				m[j] = bigValue{S: fmt.Sprintf("value-%d-%d", i, j)}
			}
			return m
		}},
		{"large byte slice", 50, func(i int) any {
			b := make([]byte, 100000+i)
			return &b
		}},
		{"string slice", 200, func(i int) any {
			s := make([]string, 1000)
			for j := range s {
				s[j] = fmt.Sprintf("string-%08d-%08d", i, j)
			}
			return &s
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			real := measure(c.n, c.build)
			var est float64
			for i := 0; i < c.n; i++ {
				est += float64(Of(c.build(i)))
			}
			est /= float64(c.n)
			ratio := est / real
			t.Logf("real=%.0f estimate=%.0f estimate/real=%.3f", real, est, ratio)
			assert.InDelta(t, 1, ratio, 0.1)
		})
	}
}

func TestOfSharedReferencesCountedOnce(t *testing.T) {
	s := strings.Repeat("x", 1000)
	shared := &item{Name: s}
	one := Of(&[]*item{shared})
	two := Of(&[]*item{shared, shared})
	assert.Equal(t, one+AllocSize(16, true)-AllocSize(8, true), two)

	m := map[string]any{"a": s, "b": s}
	// the keys are literals, not on the heap, but are counted like any other string (1 byte each)
	assert.Equal(t, mapSize(2, reflectTypeOf[string](), reflectTypeOf[any]())+AllocSize(1000, false)+2*AllocSize(16, true)+2,
		Of(m))
}

func TestOfStaticBoxes(t *testing.T) {
	header := AllocSize(24, true) // the slice header the pointer points to
	assert.Equal(t, header, Of(&[]any{}))
	// small values in an interface are taken from static memory
	v := []any{true, uint8(7), int64(3), 0.0, ""}
	assert.Equal(t, header+AllocSize(5*16, true), Of(&v))
	// other values are boxed
	w := []any{int64(300), 1.5, "abc"}
	assert.Equal(t, header+AllocSize(3*16, true)+2*AllocSize(8, false)+AllocSize(16, true)+AllocSize(3, false), Of(&w))
}

func TestAllocSize(t *testing.T) {
	assert.Equal(t, uint64(0), AllocSize(0, true))
	assert.Equal(t, uint64(5), AllocSize(5, false)) // tiny allocator
	assert.Equal(t, uint64(8), AllocSize(5, true))
	assert.Equal(t, uint64(48), AllocSize(33, false))
	assert.Equal(t, uint64(512), AllocSize(512, true))  // no malloc header up to 512 bytes
	assert.Equal(t, uint64(576), AllocSize(513, false)) // pointer-free objects have no header
	assert.Equal(t, uint64(576), AllocSize(568, true))  // 568 + 8-byte header = 576
	assert.Equal(t, uint64(640), AllocSize(569, true))
	assert.Equal(t, uint64(40960), AllocSize(32769, false))
}

func reflectTypeOf[T any]() reflect.Type { return reflect.TypeOf((*T)(nil)).Elem() }
