package timehist

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_incr(t *testing.T) {
	p0 := Histogram{}
	p1 := p0.Incr(time.Millisecond)
	p2 := p1.Incr(time.Hour)
	p3 := p1.Add(p2)
	assert.Equal(t, Histogram{0, 0, 0, 0, 0, 0, 0, 0}, p0)
	assert.Equal(t, Histogram{1, 0, 0, 0, 0, 0, 0, 0}, p1)
	assert.Equal(t, Histogram{1, 0, 0, 0, 0, 0, 0, 1}, p2)
	assert.Equal(t, Histogram{2, 0, 0, 0, 0, 0, 0, 1}, p3)
}
