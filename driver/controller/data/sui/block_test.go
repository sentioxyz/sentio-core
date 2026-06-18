package sui

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeSimpleBlockClient implements just the one method attachSimpleBlocks needs. The embedded nil
// Client interface makes any other call panic, keeping the fake honest about what it exercises.
type fakeSimpleBlockClient struct {
	Client
	calls atomic.Int64
}

func (f *fakeSimpleBlockClient) GetSimpleBlock(_ context.Context, bn uint64) (SimpleBlock, error) {
	f.calls.Add(1)
	return SimpleBlock{Checkpoint: bn}, nil
}

// TestAttachSimpleBlocks guards the concurrency contract: it must fetch every block's header exactly
// once and pair each header with the right block, with no concurrent access to the result map. Run
// under -race, the many worker goroutines would trip the detector if the map were touched off the
// main goroutine, or the per-block pairing would break if the index bookkeeping were wrong.
func TestAttachSimpleBlocks(t *testing.T) {
	const n = 1000
	result := make(map[uint64]BlockMainData, n)
	for i := uint64(1); i <= n; i++ {
		result[i] = BlockMainData{}
	}
	client := &fakeSimpleBlockClient{}

	require.NoError(t, attachSimpleBlocks(context.Background(), client, result))

	require.EqualValues(t, n, client.calls.Load(), "each block fetched exactly once")
	for bn, d := range result {
		require.NotNilf(t, d.SimpleBlock, "block %d missing prefetched header", bn)
		require.Equalf(t, bn, d.SimpleBlock.Checkpoint, "block %d paired with wrong header", bn)
	}
}

func TestAttachSimpleBlocksEmpty(t *testing.T) {
	client := &fakeSimpleBlockClient{}
	require.NoError(t, attachSimpleBlocks(context.Background(), client, map[uint64]BlockMainData{}))
	require.Zero(t, client.calls.Load(), "no fetch for an empty result set")
}
