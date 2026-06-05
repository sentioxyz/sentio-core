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

func Test_pushLatestQueue_timestampRegresses_replacesTail(t *testing.T) {
	// Higher block number but earlier timestamp (clock skew): it cannot keep order with the
	// existing tail (number up, timestamp down), so the tail is popped and latest is kept.
	base := time.Unix(1_000_000, 0)
	q, _ := pushLatestQueue(nil, Block{Number: 5, Timestamp: base}, time.Minute)

	latest := Block{Number: 10, Timestamp: base.Add(-time.Second)}
	q, dur := pushLatestQueue(q, latest, time.Minute)
	assert.Equal(t, 1, q.Len())
	assert.Equal(t, time.Duration(0), dur) // single entry → interval unknown
	bc, has := q.Back()
	assert.True(t, has)
	assert.Equal(t, latest, bc) // latest replaced the out-of-order tail
}

func Test_pushLatestQueue_numberStalls_replacesTail(t *testing.T) {
	// Same block number, newer timestamp: number must strictly increase, so the stale-number
	// tail is popped and latest is kept (tracks the most recent observation).
	base := time.Unix(1_000_000, 0)
	q, _ := pushLatestQueue(nil, Block{Number: 5, Timestamp: base}, time.Minute)

	latest := Block{Number: 5, Timestamp: base.Add(time.Second)}
	q, dur := pushLatestQueue(q, latest, time.Minute)
	assert.Equal(t, 1, q.Len())
	assert.Equal(t, time.Duration(0), dur)
	bc, has := q.Back()
	assert.True(t, has)
	assert.Equal(t, latest, bc)
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

// Test_pushLatestQueue_reorg_keepsLatest reproduces the production livelock on the Moonbeam
// super-node (lb pool) and verifies the fix.
//
// When a `latest` arrives whose block number does NOT advance (a reorg/backoff, or an
// inconsistent endpoint in the load-balanced pool) — possibly with a timestamp far ahead of
// the queued window — the old code refused to push it AND the pop-front loop could evict the
// entire window. The queue went empty, but the loop did `fr, _ = q.Front()`, discarding
// has=false: it then read a zero-value Block whose zero timestamp kept the trim condition
// true forever, and PopFront() on the empty queue is a no-op — so the refresher goroutine
// spun on a CPU, never returned to its ctx check, and wedged the whole pool for ~15h.
//
// The fix pops the out-of-order tail, pushes latest, and leaves it in the queue, so the
// queue is genuinely never empty when the front is read.
func Test_pushLatestQueue_reorg_keepsLatest(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	dur := time.Minute

	// A normal window of two advancing blocks.
	q, _ := pushLatestQueue(nil, Block{Number: 100, Timestamp: base}, dur)
	q, _ = pushLatestQueue(q, Block{Number: 101, Timestamp: base.Add(time.Second)}, dur)

	// Reorg: number regresses below the whole window while the timestamp jumps far ahead.
	bad := Block{Number: 50, Timestamp: base.Add(10 * time.Minute)}

	done := make(chan struct{})
	go func() {
		q, _ = pushLatestQueue(q, bad, dur)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pushLatestQueue spun forever instead of pushing latest after the window was evicted")
	}

	// latest must be kept as the (sole, newest) back entry — never drained to empty.
	assert.Equal(t, 1, q.Len())
	bc, has := q.Back()
	assert.True(t, has)
	assert.Equal(t, bad, bc)
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
