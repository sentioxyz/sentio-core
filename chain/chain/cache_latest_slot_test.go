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

type testRepairDimension struct {
	*SimpleDimension[*testSlot]
	repairErr error
	attempted []uint64
	onRepair  func(st *testSlot)
}

func (d *testRepairDimension) RepairSlot(
	ctx context.Context,
	st *testSlot,
) (*testSlot, bool, error) {
	d.attempted = append(d.attempted, st.GetNumber())
	if d.onRepair != nil {
		d.onRepair(st)
	}
	if d.repairErr != nil {
		return st, false, d.repairErr
	}
	fixed := *st
	fixed.Feas = nil
	return &fixed, true, nil
}

func newRepairableCache(start, end uint64, degraded ...uint64) (
	*StdLatestSlotCache[*testSlot],
	*testRepairDimension,
) {
	dim, _, store := newCachePersistent(start, end)
	for _, sn := range degraded {
		st, _ := store.slots.Get(sn)
		st.Feas = []string{"MissTrace"}
	}
	repairDim := &testRepairDimension{SimpleDimension: dim}
	return newStdLatestSlotCache(10*time.Second, 5*time.Second, repairDim), repairDim
}

func TestStdLatestSlotCache_repairRound(t *testing.T) {
	// window is [95..100] (see TestStdLatestSlotCache_initialGrowth); 90 is degraded but
	// outside the window, so only 96 and 99 should be repaired
	c, repairDim := newRepairableCache(1, 100, 90, 96, 99)
	assert.NoError(t, c.growth(context.Background(), time.Second))

	for _, sn := range []uint64{96, 99} {
		st, err := c.GetByNumber(context.Background(), sn)
		assert.NoError(t, err)
		assert.NotEmpty(t, st.Features())
	}

	c.repairRound(context.Background(), repairDim)

	assert.Equal(t, []uint64{96, 99}, repairDim.attempted)
	for sn := uint64(95); sn <= 100; sn++ {
		st, err := c.GetByNumber(context.Background(), sn)
		assert.NoError(t, err)
		assert.Empty(t, st.Features(), "slot %d should have no features after repair", sn)
	}

	// nothing degraded left — the next round should not attempt anything
	c.repairRound(context.Background(), repairDim)
	assert.Equal(t, []uint64{96, 99}, repairDim.attempted)
}

func TestStdLatestSlotCache_repairRoundStopsOnFirstError(t *testing.T) {
	c, repairDim := newRepairableCache(1, 100, 96, 99)
	assert.NoError(t, c.growth(context.Background(), time.Second))

	repairDim.repairErr = fmt.Errorf("still broken")
	c.repairRound(context.Background(), repairDim)
	// the round probes the first degraded slot only and leaves the rest untouched
	assert.Equal(t, []uint64{96}, repairDim.attempted)
	for _, sn := range []uint64{96, 99} {
		st, err := c.GetByNumber(context.Background(), sn)
		assert.NoError(t, err)
		assert.NotEmpty(t, st.Features())
	}

	// upstream recovered — both get repaired in the next round
	repairDim.repairErr = nil
	c.repairRound(context.Background(), repairDim)
	assert.Equal(t, []uint64{96, 96, 99}, repairDim.attempted)
	for _, sn := range []uint64{96, 99} {
		st, err := c.GetByNumber(context.Background(), sn)
		assert.NoError(t, err)
		assert.Empty(t, st.Features())
	}
}

func TestStdLatestSlotCache_repairRoundDiscardsOnReorg(t *testing.T) {
	c, repairDim := newRepairableCache(1, 100, 96)
	assert.NoError(t, c.growth(context.Background(), time.Second))

	// simulate a reorg replacing slot 96 while its repair is in flight: the repaired result
	// carries the pre-reorg hash and must not overwrite the reorged slot
	replaced := &testSlot{Number: 96, Hash: "hash-96-reorg", ParentHash: "hash-95"}
	repairDim.onRepair = func(st *testSlot) {
		c.lock.Lock()
		c.memCache[96] = replaced
		c.lock.Unlock()
	}

	c.repairRound(context.Background(), repairDim)
	assert.Equal(t, []uint64{96}, repairDim.attempted)
	st, err := c.GetByNumber(context.Background(), 96)
	assert.NoError(t, err)
	assert.Equal(t, "hash-96-reorg", st.GetHash())
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
