package pool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- test types ---

type testConfig struct {
	Value string
}

func (c testConfig) Equal(other testConfig) bool {
	return c.Value == other.Value
}

type testEntryStatus struct {
	Value int
}

func (s testEntryStatus) Snapshot() any {
	return s.Value
}

type testPoolStatus struct {
	Count int
}

func (s testPoolStatus) Snapshot() any {
	return s.Count
}

// noopRefresher blocks until context is cancelled (simulates a long-running background worker).
func noopRefresher(ctx context.Context, _ testConfig, _ chan<- testEntryStatus) {
	<-ctx.Done()
}

func newTestPool() *Pool[testConfig, testEntryStatus, testPoolStatus] {
	return NewPool[testConfig, testEntryStatus, testPoolStatus](
		"test",
		func(entries map[string]Entry[testConfig, testEntryStatus]) testPoolStatus {
			return testPoolStatus{Count: len(entries)}
		},
		noopRefresher,
	)
}

// --- tests ---

func TestPool_Add_New(t *testing.T) {
	p := newTestPool()
	defer p.RemoveAll()

	added := p.Add("entry1", testConfig{Value: "v1"})
	assert.True(t, added)

	entries, status, _ := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })
	assert.Len(t, entries, 1)
	assert.Equal(t, testConfig{Value: "v1"}, entries["entry1"].Config)
	assert.Equal(t, 1, status.Count)
}

func TestPool_Add_Duplicate(t *testing.T) {
	p := newTestPool()
	defer p.RemoveAll()

	p.Add("entry1", testConfig{Value: "v1"})
	added := p.Add("entry1", testConfig{Value: "v1"})
	assert.False(t, added)

	entries, _, _ := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })
	assert.Len(t, entries, 1)
}

func TestPool_Add_UpdateConfig(t *testing.T) {
	p := newTestPool()
	defer p.RemoveAll()

	p.Add("entry1", testConfig{Value: "v1"})
	added := p.Add("entry1", testConfig{Value: "v2"})
	assert.True(t, added)

	entries, _, _ := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })
	assert.Len(t, entries, 1)
	assert.Equal(t, testConfig{Value: "v2"}, entries["entry1"].Config)
}

func TestPool_Remove(t *testing.T) {
	p := newTestPool()

	p.Add("entry1", testConfig{Value: "v1"})
	removed := p.Remove("entry1")
	assert.True(t, removed)

	entries, status, _ := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })
	assert.Empty(t, entries)
	assert.Equal(t, 0, status.Count)
}

func TestPool_Remove_NonExistent(t *testing.T) {
	p := newTestPool()
	removed := p.Remove("nonexistent")
	assert.False(t, removed)
}

func TestPool_RemoveAll(t *testing.T) {
	p := newTestPool()

	p.Add("entry1", testConfig{Value: "v1"})
	p.Add("entry2", testConfig{Value: "v2"})
	p.RemoveAll()

	entries, status, _ := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })
	assert.Empty(t, entries)
	assert.Equal(t, 0, status.Count)
}

func TestPool_Fetch_Checker(t *testing.T) {
	p := newTestPool()
	defer p.RemoveAll()

	p.Add("entry1", testConfig{Value: "v1"})
	p.Add("entry2", testConfig{Value: "v2"})
	p.Add("entry3", testConfig{Value: "v3"})

	entries, _, _ := p.Fetch(func(name string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool {
		return name == "entry1" || name == "entry3"
	})
	assert.Len(t, entries, 2)
	assert.Contains(t, entries, "entry1")
	assert.Contains(t, entries, "entry3")
	assert.NotContains(t, entries, "entry2")
}

func TestPool_Fetch_CheckerReceivesPoolStatus(t *testing.T) {
	p := newTestPool()
	defer p.RemoveAll()

	p.Add("entry1", testConfig{Value: "v1"})
	p.Add("entry2", testConfig{Value: "v2"})

	// checker sees the current pool status (Count == 2)
	var seenCount int
	p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], ps testPoolStatus) bool {
		seenCount = ps.Count
		return true
	})
	assert.Equal(t, 2, seenCount)
}

func TestPool_Fetch_ReturnsStatusIndex(t *testing.T) {
	p := newTestPool()
	defer p.RemoveAll()

	_, _, idx0 := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })

	p.Add("entry1", testConfig{Value: "v1"})
	_, _, idx1 := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })

	p.Add("entry2", testConfig{Value: "v2"})
	_, _, idx2 := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })

	assert.Less(t, idx0, idx1)
	assert.Less(t, idx1, idx2)
}

func TestPool_Wait_UnblocksOnAdd(t *testing.T) {
	p := newTestPool()
	defer p.RemoveAll()

	_, _, idx := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })

	done := make(chan error, 1)
	go func() {
		done <- p.Wait(context.Background(), idx)
	}()

	time.Sleep(10 * time.Millisecond)
	p.Add("entry1", testConfig{Value: "v1"})

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Wait did not unblock after Add")
	}
}

func TestPool_Wait_UnblocksOnRemove(t *testing.T) {
	p := newTestPool()

	p.Add("entry1", testConfig{Value: "v1"})
	_, _, idx := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })

	done := make(chan error, 1)
	go func() {
		done <- p.Wait(context.Background(), idx)
	}()

	time.Sleep(10 * time.Millisecond)
	p.Remove("entry1")

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Wait did not unblock after Remove")
	}
}

func TestPool_Wait_ContextCancelled(t *testing.T) {
	p := newTestPool()
	defer p.RemoveAll()

	_, _, idx := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- p.Wait(ctx, idx)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("Wait did not unblock after context cancel")
	}
}

func TestPool_EntryStatusUpdated(t *testing.T) {
	statusSent := make(chan struct{})
	p := NewPool[testConfig, testEntryStatus, testPoolStatus](
		"test",
		func(entries map[string]Entry[testConfig, testEntryStatus]) testPoolStatus {
			return testPoolStatus{Count: len(entries)}
		},
		func(ctx context.Context, _ testConfig, ch chan<- testEntryStatus) {
			ch <- testEntryStatus{Value: 42}
			close(statusSent)
			<-ctx.Done()
		},
	)
	defer p.RemoveAll()

	p.Add("entry1", testConfig{Value: "v1"})
	p.Enable("entry1")

	select {
	case <-statusSent:
	case <-time.After(time.Second):
		t.Fatal("refresher did not send status")
	}
	// wait for the status goroutine to process
	time.Sleep(10 * time.Millisecond)

	entries, _, _ := p.Fetch(func(_ string, _ Entry[testConfig, testEntryStatus], _ testPoolStatus) bool { return true })
	assert.Equal(t, 42, entries["entry1"].Status.Value)
}

func TestPool_Snapshot(t *testing.T) {
	p := newTestPool()
	defer p.RemoveAll()

	p.Add("entry1", testConfig{Value: "v1"})

	snap := p.Snapshot()
	snapMap, ok := snap.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "test", snapMap["name"])
	assert.NotNil(t, snapMap["status"])
	assert.NotNil(t, snapMap["statusIndex"])
	assert.NotNil(t, snapMap["entries"])

	entriesSnap, ok := snapMap["entries"].(map[string]map[string]any)
	assert.True(t, ok)
	assert.Contains(t, entriesSnap, "entry1")
}
