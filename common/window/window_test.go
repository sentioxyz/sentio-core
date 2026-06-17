package window

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_findFirstStartPoint(t *testing.T) {
	winGetter := func(ctx context.Context, n int) (time.Time, error) {
		return time.Unix(int64(n/100), 0), nil
	}

	var r *int
	for s := 1; s <= 100; s++ {
		for e := 100; e < 199; e++ {
			r, _ = FindFirstStartPoint[int](context.Background(), s, e, winGetter)
			assert.Equal(t, 100, *r)
		}
	}
	for s := 101; s <= 199; s++ {
		for e := s; e < 199; e++ {
			r, _ = FindFirstStartPoint[int](context.Background(), s, e, winGetter)
			assert.Nil(t, r)
		}
	}

	var rs []int
	rs, _ = FindStartPoints[int](context.Background(), 1, 99, 3, winGetter)
	assert.Nil(t, rs)
	rs, _ = FindStartPoints[int](context.Background(), 1, 299, 1, winGetter)
	assert.Equal(t, []int{100}, rs)
	rs, _ = FindStartPoints[int](context.Background(), 1, 299, 2, winGetter)
	assert.Equal(t, []int{100, 200}, rs)
	rs, _ = FindStartPoints[int](context.Background(), 1, 299, 3, winGetter)
	assert.Equal(t, []int{100, 200}, rs)
	rs, _ = FindStartPoints[int](context.Background(), 1, 299, -1, winGetter)
	assert.Equal(t, []int{100, 200}, rs)
}
