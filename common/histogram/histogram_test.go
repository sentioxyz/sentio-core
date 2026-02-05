package histogram

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_operator(t *testing.T) {
	ladder := Ladder[int]{10, 30, 100, 300, 1000}

	var hist Histogram
	hist = ladder.Incr(hist, 0)
	hist = ladder.Incr(hist, 10)
	hist = ladder.Incr(hist, 20)
	hist = ladder.Incr(hist, 30)
	hist = ladder.Incr(hist, 100)
	hist = ladder.Incr(hist, 200)
	hist = ladder.Incr(hist, 300)
	hist = ladder.Incr(hist, 10000)

	assert.Equal(t, Histogram{1, 2, 1, 2, 1, 1}, hist)
	assert.Equal(t, 8, ladder.Sum(hist))
	assert.Equal(t, "(-INF,10):1,[10,30):2,[30,100):1,[100,300):2,[300,1000):1,[1000,INF):1", ladder.ToString(hist))

	hist1 := ladder.Incr(nil, 5)
	hist2 := ladder.Merge(ladder.New(), hist, hist1)
	assert.Equal(t, Histogram{1, 2, 1, 2, 1, 1}, hist)
	assert.Equal(t, Histogram{1, 0, 0, 0, 0, 0}, hist1)
	assert.Equal(t, Histogram{2, 2, 1, 2, 1, 1}, hist2)
}
