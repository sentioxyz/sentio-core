package histogram

import (
	"bytes"
	"fmt"
	"golang.org/x/exp/constraints"
)

type Ladder[V constraints.Ordered] []V
type Histogram []int

func (d Ladder[V]) New() Histogram {
	return make(Histogram, len(d)+1)
}

func (d Ladder[V]) Incr(hist Histogram, v V) Histogram {
	if len(hist) == 0 {
		hist = d.New()
	}
	for i := range d {
		if v < d[i] {
			hist[i] += 1
			return hist
		}
	}
	hist[len(d)] += 1
	return hist
}

func (d Ladder[V]) Sum(hist Histogram) (s int) {
	for _, x := range hist {
		s += x
	}
	return
}

func (d Ladder[V]) ToString(hist Histogram) string {
	var buf bytes.Buffer
	for i := range d {
		if i == 0 {
			buf.WriteString(fmt.Sprintf("(-INF,%v):%d,", d[i], hist[i]))
		} else {
			buf.WriteString(fmt.Sprintf("[%v,%v):%d,", d[i-1], d[i], hist[i]))
		}
	}
	buf.WriteString(fmt.Sprintf("[%v,INF):%d", d[len(d)-1], hist[len(d)]))
	return buf.String()
}

func (d Ladder[V]) Merge(result, a, b Histogram) Histogram {
	for i := 0; i <= len(d); i++ {
		result[i] = a[i] + b[i]
	}
	return result
}

func (d Ladder[V]) Snapshot(hist Histogram) any {
	return map[string]any{
		"sum":  d.Sum(hist),
		"dist": d.ToString(hist),
	}
}
