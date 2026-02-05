package timewin

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type testWin struct {
	startAt time.Time
	total   int
}

func (w *testWin) GetStartAt() time.Time {
	return w.startAt
}

func (w *testWin) Merge(a *testWin) {
	w.total += a.total
}

func (w *testWin) Snapshot(endAt time.Time) any {
	return map[string]any{
		"startAt": w.startAt,
		"endAt":   endAt,
		"total":   w.total,
	}
}

func Test_append(t *testing.T) {
	mustParseTime := func(s string) time.Time {
		tt, _ := time.Parse(time.RFC3339, s)
		return tt
	}

	m := NewTimeWindowsManager[*testWin](time.Hour * 2)
	for h := 0; h < 50; h++ {
		tt := mustParseTime(fmt.Sprintf("2025-08-%02dT%02d:01:01Z", h/24+1, h%24))
		m.Append(&testWin{startAt: tt, total: 1})
	}

	assert.Equal(t, []*testWin{
		{startAt: mustParseTime("2025-08-01T00:01:01Z"), total: 32},
		{startAt: mustParseTime("2025-08-02T08:01:01Z"), total: 8},
		{startAt: mustParseTime("2025-08-02T16:01:01Z"), total: 4},
		{startAt: mustParseTime("2025-08-02T20:01:01Z"), total: 2},
		{startAt: mustParseTime("2025-08-02T22:01:01Z"), total: 2},
		{startAt: mustParseTime("2025-08-03T00:01:01Z"), total: 2},
	}, m.wins)
}
