package clientpool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ── pushLatestQueue ───────────────────────────────────────────────────────────

func Test_pushLatestQueue_nilQueue_seedsFirstBlock(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	q, dur := pushLatestQueue(nil, Block{Number: 1, Timestamp: base}, time.Minute)
	assert.NotNil(t, q)
	assert.Equal(t, 1, q.Len())
	assert.Equal(t, time.Duration(0), dur) // single entry → interval unknown
}

func Test_pushLatestQueue_bothAdvance_pushes(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	q, _ := pushLatestQueue(nil, Block{Number: 1, Timestamp: base}, time.Minute)

	q, dur := pushLatestQueue(q, Block{Number: 11, Timestamp: base.Add(10 * time.Second)}, time.Minute)
	assert.Equal(t, 2, q.Len())
	assert.Equal(t, time.Second, dur) // 10s / 10 blocks = 1s per block
}

func Test_pushLatestQueue_skipWhenOnlyNumberAdvances(t *testing.T) {
	// Bug-fix: if timestamp goes backwards, entry must NOT be pushed.
	base := time.Unix(1_000_000, 0)
	q, _ := pushLatestQueue(nil, Block{Number: 5, Timestamp: base}, time.Minute)

	// Higher block number but earlier timestamp (clock skew) → skip
	q, dur := pushLatestQueue(q, Block{Number: 10, Timestamp: base.Add(-time.Second)}, time.Minute)
	assert.Equal(t, 1, q.Len())
	assert.Equal(t, time.Duration(0), dur)
}

func Test_pushLatestQueue_skipWhenOnlyTimestampAdvances(t *testing.T) {
	// Same block number, newer timestamp → skip (number must advance too).
	base := time.Unix(1_000_000, 0)
	q, _ := pushLatestQueue(nil, Block{Number: 5, Timestamp: base}, time.Minute)

	q, dur := pushLatestQueue(q, Block{Number: 5, Timestamp: base.Add(time.Second)}, time.Minute)
	assert.Equal(t, 1, q.Len())
	assert.Equal(t, time.Duration(0), dur)
}

func Test_pushLatestQueue_trimsEntriesOutsideWindow(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	window := 5 * time.Second

	q, _ := pushLatestQueue(nil, Block{Number: 1, Timestamp: base}, window)                         // t=0
	q, _ = pushLatestQueue(q, Block{Number: 2, Timestamp: base.Add(2 * time.Second)}, window)       // t=2
	q, _ = pushLatestQueue(q, Block{Number: 3, Timestamp: base.Add(4 * time.Second)}, window)       // t=4
	assert.Equal(t, 3, q.Len())                                                                      // all within 5s of latest (t=4)

	// New block at t=8: entries at t=0 (8s ago) and t=2 (6s ago) are beyond the 5s window → trimmed.
	// Entry at t=4 is 4s ago ≤ 5s → kept. Interval from t=4 to t=8 over 1 block = 4s.
	q, d := pushLatestQueue(q, Block{Number: 4, Timestamp: base.Add(8 * time.Second)}, window)
	assert.Equal(t, 2, q.Len()) // only t=4 and t=8 remain
	assert.Equal(t, 4*time.Second, d)
}

func Test_pushLatestQueue_intervalMonotonicallyCalculated(t *testing.T) {
	base := time.Unix(1_000_000, 0)

	q, d := pushLatestQueue(nil, Block{Number: 100, Timestamp: base}, time.Minute)
	assert.Equal(t, time.Duration(0), d)

	// +10 blocks in 10s → 1s/block
	q, d = pushLatestQueue(q, Block{Number: 110, Timestamp: base.Add(10 * time.Second)}, time.Minute)
	assert.Equal(t, time.Second, d)

	// +20 more blocks in 20s (total: 30s / 30 blocks from front) → still 1s/block
	q, d = pushLatestQueue(q, Block{Number: 130, Timestamp: base.Add(30 * time.Second)}, time.Minute)
	assert.Equal(t, time.Second, d)
	_ = q
}
