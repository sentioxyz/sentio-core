package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeReorgClient struct {
	headers map[uint64]testHeader
	calls   int
}

type testHeader struct {
	number uint64
	hash   string
	parent string
}

func (h testHeader) GetBlockNumber() uint64      { return h.number }
func (h testHeader) GetBlockHash() string        { return h.hash }
func (h testHeader) GetBlockParentHash() string  { return h.parent }
func (h testHeader) GetBlockTime() time.Time     { return time.Unix(int64(h.number)*12, 0) }

func linkedHeader(n uint64) testHeader {
	return testHeader{number: n, hash: fmt.Sprintf("0x%x", n), parent: fmt.Sprintf("0x%x", n-1)}
}

func (c *fakeReorgClient) GetLatest(ctx context.Context) (BlockHeader, uint64, error) {
	panic("not used")
}
func (c *fakeReorgClient) Subscribe(ctx context.Context, from BlockHeader, callback func(BlockHeader, error)) {
	panic("not used")
}
func (c *fakeReorgClient) GetHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (BlockHeader, error) {
	c.calls++
	return c.headers[blockNumber], nil
}
func (c *fakeReorgClient) ResetCache(r BlockRange) {}
func (c *fakeReorgClient) Snapshot() any           { return nil }

func TestCheckReorgAdjacentFastPath(t *testing.T) {
	cli := &fakeReorgClient{headers: map[uint64]testHeader{}}
	b := &blockBuilder{client: cli, headerList: []BlockHeader{linkedHeader(100)}}

	// adjacent and linked: free in-memory check, no RPC — this is what makes per-block reorg
	// checking affordable during backfill
	reorg, err := b.checkReorg(context.Background(), linkedHeader(101), false)
	assert.NoError(t, err)
	assert.Nil(t, reorg)
	assert.Equal(t, 0, cli.calls)
}

func TestCheckReorgAdjacentMismatchEscalates(t *testing.T) {
	// the incident shape: block 100 was recorded from an orphan sibling (same parent as the
	// canonical block, different hash), block 101 arrives on the canonical chain
	orphan := testHeader{number: 100, hash: "0xbad", parent: "0x63"}
	cli := &fakeReorgClient{headers: map[uint64]testHeader{100: linkedHeader(100)}}
	b := &blockBuilder{client: cli, headerList: []BlockHeader{orphan}}

	// even with allowScan=false (backfill), an adjacent link mismatch is direct evidence of a
	// fork and must escalate to the re-scan, which points the rollback at block 100
	reorg, err := b.checkReorg(context.Background(), linkedHeader(101), false)
	assert.NoError(t, err)
	if assert.NotNil(t, reorg) {
		assert.Equal(t, uint64(100), *reorg)
	}
	assert.Equal(t, 1, cli.calls)
}

func TestCheckReorgGapSkippedInBackfill(t *testing.T) {
	cli := &fakeReorgClient{headers: map[uint64]testHeader{}}
	b := &blockBuilder{client: cli, headerList: []BlockHeader{linkedHeader(90)}}

	// gap in the header list during backfill: nothing to compare locally, no RPC re-scan
	reorg, err := b.checkReorg(context.Background(), linkedHeader(101), false)
	assert.NoError(t, err)
	assert.Nil(t, reorg)
	assert.Equal(t, 0, cli.calls)

	// the same gap in the watching range keeps the pre-existing RPC re-scan behavior
	cli.headers[90] = linkedHeader(90)
	reorg, err = b.checkReorg(context.Background(), linkedHeader(101), true)
	assert.NoError(t, err)
	assert.Nil(t, reorg)
	assert.Equal(t, 1, cli.calls)
}
