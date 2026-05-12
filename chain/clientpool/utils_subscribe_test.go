package clientpool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startSubscribe runs Subscribe in a goroutine and returns a done channel that
// is closed when Subscribe returns.
func startSubscribe(
	ctx context.Context,
	checkInterval time.Duration,
	latestChan <-chan Block,
	interval time.Duration,
	getLatest func(context.Context) (Block, error),
	stop func(Block) bool,
	out chan<- Block,
) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		Subscribe(ctx, checkInterval, latestChan, interval, getLatest, stop, out)
	}()
	return done
}

// ── Subscribe ─────────────────────────────────────────────────────────────────

func Test_Subscribe_contextCancellation_exits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan Block, 10)

	getLatest := func(_ context.Context) (Block, error) {
		return Block{Number: 1, Timestamp: time.Now()}, nil
	}
	done := startSubscribe(ctx, time.Minute, make(chan Block), time.Millisecond, getLatest, nil, out)

	time.Sleep(5 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe did not return after context cancellation")
	}
}

func Test_Subscribe_stopFunction_exits(t *testing.T) {
	ctx := context.Background()
	out := make(chan Block, 10)

	callCount := 0
	getLatest := func(_ context.Context) (Block, error) {
		callCount++
		return Block{Number: uint64(callCount), Timestamp: time.Now()}, nil
	}
	stop := func(b Block) bool { return b.Number >= 2 }

	done := startSubscribe(ctx, time.Minute, make(chan Block), time.Millisecond, getLatest, stop, out)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe did not return after stop returned true")
	}
}

func Test_Subscribe_forwardsBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Block, 10)

	ts := time.Unix(1_000_000, 0)
	getLatest := func(_ context.Context) (Block, error) {
		return Block{Number: 10, Timestamp: ts}, nil
	}
	startSubscribe(ctx, time.Minute, make(chan Block), time.Millisecond, getLatest, nil, out)

	select {
	case b := <-out:
		assert.Equal(t, uint64(10), b.Number)
	case <-time.After(2 * time.Second):
		t.Fatal("expected block not received in out channel")
	}
}

func Test_Subscribe_getLatestError_doesNotForward(t *testing.T) {
	ctx := context.Background()
	out := make(chan Block, 10)

	callCount := 0
	ts := time.Unix(1_000_000, 0)
	getLatest := func(_ context.Context) (Block, error) {
		callCount++
		if callCount < 3 {
			return Block{}, fmt.Errorf("rpc unavailable")
		}
		// third call succeeds but stop fires before forwarding
		return Block{Number: 1, Timestamp: ts}, nil
	}
	// stop on the third call (after success is returned), before forwarding
	stop := func(_ Block) bool { return callCount >= 3 }

	done := startSubscribe(ctx, time.Minute, make(chan Block), time.Millisecond, getLatest, stop, out)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe did not stop")
	}

	assert.Equal(t, 0, len(out), "errored getLatest calls must not forward a block")
}

func Test_Subscribe_latestChan_forwardsBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Block, 10)
	latestChan := make(chan Block, 1)

	// getLatest should not be reached because latestChan delivers first
	getLatest := func(_ context.Context) (Block, error) {
		return Block{Number: 1, Timestamp: time.Now()}, nil
	}
	// long polling interval so the latestChan select branch fires first
	startSubscribe(ctx, time.Minute, latestChan, time.Hour, getLatest, nil, out)

	ts := time.Unix(1_000_000, 0)
	latestChan <- Block{Number: 42, Timestamp: ts}

	select {
	case b := <-out:
		assert.Equal(t, uint64(42), b.Number)
	case <-time.After(2 * time.Second):
		t.Fatal("block from latestChan not forwarded to out")
	}
}

func Test_Subscribe_latestChanClosed_fallsBackToPolling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Block, 10)

	// Close the channel before passing it in — Subscribe should reset and fall back.
	latestChan := make(chan Block)
	close(latestChan)

	ts := time.Unix(1_000_000, 0)
	getLatest := func(_ context.Context) (Block, error) {
		return Block{Number: 99, Timestamp: ts}, nil
	}
	startSubscribe(ctx, time.Minute, latestChan, time.Millisecond, getLatest, nil, out)

	select {
	case b := <-out:
		assert.Equal(t, uint64(99), b.Number)
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe did not fall back to polling after latestChan closed")
	}
}

func Test_Subscribe_multipleBlocks_allForwarded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Block, 10)

	base := time.Unix(1_000_000, 0)
	callCount := 0
	getLatest := func(_ context.Context) (Block, error) {
		callCount++
		return Block{
			Number:    uint64(callCount),
			Timestamp: base.Add(time.Duration(callCount) * time.Second),
		}, nil
	}
	stop := func(_ Block) bool { return callCount >= 3 }

	done := startSubscribe(ctx, time.Minute, make(chan Block), time.Millisecond, getLatest, stop, out)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe did not stop")
	}

	require.GreaterOrEqual(t, len(out), 2, "at least 2 blocks should have been forwarded")
	for i := 0; i < len(out); i++ {
		b := <-out
		assert.GreaterOrEqual(t, b.Number, uint64(1))
	}
}
