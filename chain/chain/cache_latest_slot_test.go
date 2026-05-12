package chain

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

func newCacheTestSlot(number uint64) *testSlot {
	return &testSlot{
		Number:     number,
		Hash:       fmt.Sprintf("hash-%d", number),
		ParentHash: fmt.Sprintf("hash-%d", number-1),
	}
}

func newCachePersistent(start, end uint64) (*SimpleDimension[*testSlot], *testRangeStore, *testSimpleSlotStore[*testSlot]) {
	slotMap := utils.NewSafeMap[uint64, *testSlot]()
	store := &testSimpleSlotStore[*testSlot]{slots: slotMap}
	for i := start; i <= end; i++ {
		slotMap.Put(i, newCacheTestSlot(i))
	}
	rs := &testRangeStore{cur: rg.NewRange(start, end)}
	return NewSimpleDimension[*testSlot](rs, store), rs, store
}

func newStdLatestSlotCache(maxDur, minDur time.Duration, dim Dimension[*testSlot]) *StdLatestSlotCache[*testSlot] {
	return NewStdLatestSlotCache[*testSlot](
		"test", "testnet",
		maxDur, minDur,
		nil, dim,
		nil, 0,
		nil, nil,
	)
}

func TestStdLatestSlotCache_notReadyBeforeGrowth(t *testing.T) {
	dim, _, _ := newCachePersistent(1, 100)
	c := newStdLatestSlotCache(10*time.Second, 5*time.Second, dim)

	_, err := c.GetRange(context.Background())
	assert.ErrorIs(t, err, ErrNotReady)

	_, err = c.GetByNumber(context.Background(), 95)
	assert.ErrorIs(t, err, ErrNotReady)
}

func TestStdLatestSlotCache_initialGrowth(t *testing.T) {
	// bi=1s, minDur=5s → minSize=6
	// persistent=[1..100], newRange=NewRangeByEndAndSize(100,6)∩[1..100]=[95..100]
	dim, _, _ := newCachePersistent(1, 100)
	c := newStdLatestSlotCache(10*time.Second, 5*time.Second, dim)

	assert.NoError(t, c.growth(context.Background(), time.Second))
	assert.True(t, c.ready)

	gotRange, err := c.GetRange(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, rg.NewRange(95, 100), gotRange)

	for sn := uint64(95); sn <= 100; sn++ {
		st, err := c.GetByNumber(context.Background(), sn)
		assert.NoError(t, err)
		assert.Equal(t, sn, st.GetNumber())
	}
	// slot outside window should not be cached
	_, err = c.GetByNumber(context.Background(), 94)
	assert.ErrorIs(t, err, ErrSlotNotFound)
}

func TestStdLatestSlotCache_advanceGrowth(t *testing.T) {
	// initial: persistent=[1..100], bi=1s, minDur=5s → minSize=6 → newRange=[95..100]
	dim, rs, store := newCachePersistent(1, 100)
	c := newStdLatestSlotCache(10*time.Second, 5*time.Second, dim)

	assert.NoError(t, c.growth(context.Background(), time.Second))
	assert.Equal(t, rg.NewRange(95, 100), c.curRange)

	// advance persistent to [1..110]
	for i := uint64(101); i <= 110; i++ {
		store.slots.Put(i, newCacheTestSlot(i))
	}
	rs.cur = rg.NewRange(1, 110)

	// maxDur=10s, bi=1s → maxSize=11
	// newRange = NewRangeByEndAndSize(110,11)∩[1..110] = [100..110]
	// newRange.Start = max(100, curRange.Start=95) = 100
	assert.NoError(t, c.growth(context.Background(), time.Second))

	gotRange, err := c.GetRange(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, rg.NewRange(100, 110), gotRange)

	// old slots outside the new window should be evicted
	for sn := uint64(95); sn <= 99; sn++ {
		_, err := c.GetByNumber(context.Background(), sn)
		assert.ErrorIs(t, err, ErrSlotNotFound, "slot %d should have been evicted", sn)
	}
	// new window slots should be present
	for sn := uint64(100); sn <= 110; sn++ {
		st, err := c.GetByNumber(context.Background(), sn)
		assert.NoError(t, err)
		assert.Equal(t, sn, st.GetNumber())
	}
}

func TestStdLatestSlotCache_sameLatestNoOp(t *testing.T) {
	dim, _, _ := newCachePersistent(1, 100)
	c := newStdLatestSlotCache(10*time.Second, 5*time.Second, dim)

	assert.NoError(t, c.growth(context.Background(), time.Second))
	firstRange := c.curRange

	// growth again without persistent advancing — no change expected
	assert.NoError(t, c.growth(context.Background(), time.Second))
	assert.Equal(t, firstRange, c.curRange)
}

func TestStdLatestSlotCache_GetByHash(t *testing.T) {
	dim, _, _ := newCachePersistent(1, 100)
	c := newStdLatestSlotCache(10*time.Second, 5*time.Second, dim)
	assert.NoError(t, c.growth(context.Background(), time.Second))

	st, err := c.GetByHash(context.Background(), "hash-98")
	assert.NoError(t, err)
	assert.Equal(t, uint64(98), st.GetNumber())

	// slot outside cached window
	_, err = c.GetByHash(context.Background(), "hash-94")
	assert.ErrorIs(t, err, ErrSlotNotFound)
}
