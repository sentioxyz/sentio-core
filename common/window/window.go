package window

import (
	"context"
	"golang.org/x/exp/constraints"
	"time"
)

// FindFirstStartPoint find the first x in [start,end] that timeWinGetter(x-1) < timeWinGetter(x)
func FindFirstStartPoint[N constraints.Signed](
	ctx context.Context,
	start, end N,
	winGetter func(ctx context.Context, n N) (time.Time, error),
) (*N, error) {
	if start > end {
		return nil, nil
	}

	var t time.Time
	p, err := winGetter(ctx, start-1)
	if err != nil {
		return nil, err
	}

	t, err = winGetter(ctx, end)
	if err != nil {
		return nil, err
	}

	if p.Equal(t) {
		// all x in [start,end] that timeWinGetter(start-1) == timeWinGetter(x), so no result
		return nil, nil
	}

	for start < end {
		mid := (start + end) / 2
		t, err = winGetter(ctx, mid)
		if err != nil {
			return nil, err
		}
		if t.Equal(p) {
			start = mid + 1
		} else {
			end = mid
		}
	}
	return &start, nil
}

// FindStartPoints find first limit points in [start,end] that timeWinGetter(x-1) < timeWinGetter(x)
func FindStartPoints[N constraints.Signed](
	ctx context.Context,
	start, end N,
	limit int,
	winGetter func(ctx context.Context, n N) (time.Time, error),
) ([]N, error) {
	var result []N
	for i := 0; limit <= 0 || i < limit; i++ {
		x, err := FindFirstStartPoint(ctx, start, end, winGetter)
		if err != nil {
			return nil, err
		}
		if x == nil {
			return result, nil
		}
		result = append(result, *x)
		start = *x + 1
	}
	return result, nil
}
