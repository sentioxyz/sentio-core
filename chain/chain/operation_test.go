package chain

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"

	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

func newTestDimension() (Dimension[*testSlot], *testRangeStore, *testSimpleSlotStore[*testSlot]) {
	store := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[uint64, *testSlot](),
	}
	rangeStore := &testRangeStore{
		cur: rg.EmptyRange,
	}
	dim := NewSimpleDimension(rangeStore, store)
	return dim, rangeStore, store
}

func newTestSlots(interval rg.Range, hashPrefix string, parentHash ...string) []*testSlot {
	if interval.IsEmpty() {
		return nil
	}
	var slots []*testSlot

	buildHash := func(number uint64) string {
		return fmt.Sprintf("%s%d", hashPrefix, number)
	}

	for n := interval.Start; n <= *interval.End; n++ {
		newSlot := &testSlot{
			Number:     n,
			Hash:       buildHash(n),
			ParentHash: buildHash(n - 1),
		}
		slots = append(slots, newSlot)
	}

	if len(parentHash) > 0 {
		slots[0].ParentHash = parentHash[0]
	}
	return slots
}

func TestRepair(t *testing.T) {
	dim1, rs1, store1 := newTestDimension()
	baseRange := rg.NewRange(100, 200)
	baseSlots := newTestSlots(baseRange, "")
	store1.initFillSlots(baseSlots)
	_, _ = rs1.Update(context.Background(), rg.RangeSetter(baseRange))

	var brokenSlots []*testSlot
	var p = 0
	for i, one := range baseSlots {
		if i == 1<<p {
			p++
			brokenSlots = append(brokenSlots, one)
		}
	}

	dim2, rs2, store2 := newTestDimension()
	store2.initFillSlots(brokenSlots)
	_, _ = rs2.Update(context.Background(), rg.RangeSetter(baseRange))

	// repair missing
	assert.Equal(t, nil, Repair[*testSlot](context.Background(), dim1, dim2, baseRange))

	// final check
	slotChan := make(chan *testSlot, 1000)
	assert.Equal(t, nil, dim2.Load(context.Background(), baseRange, slotChan))
	close(slotChan)
	slots, _ := concurrency.ReadAll(context.Background(), slotChan)

	assert.Equal(t, *baseRange.Size(), uint64(len(slots)))
	assert.Equal(t, baseRange, GetSlotRange(slots))
}

func TestCopy1(t *testing.T) {
	dim1, rs1, store1 := newTestDimension()
	baseRange := rg.NewRange(100, 200)
	baseSlots := newTestSlots(baseRange, "")
	store1.initFillSlots(baseSlots)
	_, _ = rs1.Update(context.Background(), rg.RangeSetter(baseRange))

	dim2, _, _ := newTestDimension()

	{
		// Copy to an empty chain
		assert.Equal(t, nil, Copy[*testSlot](
			context.Background(),
			dim1,
			dim2,
			rg.NewRange(140, 160),
			false,
		))
		r2, _ := dim2.GetRange(context.Background())
		assert.Equal(t, r2, rg.NewRange(140, 160))
	}

	{
		// Copy and overwrite the whole chain
		assert.Equal(t, nil, Copy[*testSlot](
			context.Background(),
			dim1,
			dim2,
			rg.NewRange(130, 170),
			true,
		))
		r2, _ := dim2.GetRange(context.Background())
		assert.Equal(t, r2, rg.NewRange(130, 170))
	}

	{
		// Copy with two part
		assert.Equal(t, nil, Copy[*testSlot](
			context.Background(),
			dim1,
			dim2,
			rg.NewRange(120, 180),
			false,
		))
		r2, _ := dim2.GetRange(context.Background())
		assert.Equal(t, r2, rg.NewRange(120, 180))
	}

	{
		// Copy all missing slots on the left
		assert.Equal(t, nil, Copy[*testSlot](
			context.Background(),
			dim1,
			dim2,
			rg.NewRange(0, 150),
			false,
		))
		r2, _ := dim2.GetRange(context.Background())
		assert.Equal(t, r2, rg.NewRange(100, 180))
	}

	{
		// Copy all missing slots on the right
		assert.Equal(t, nil, Copy[*testSlot](
			context.Background(),
			dim1,
			dim2,
			rg.Range{Start: 150},
			false,
		))
		r2, _ := dim2.GetRange(context.Background())
		assert.Equal(t, r2, rg.NewRange(100, 200))
	}
}

func TestCopy2(t *testing.T) {
	baseRange := rg.NewRange(100, 200)
	baseSlots1 := newTestSlots(baseRange, "ca")
	baseSlots2 := newTestSlots(baseRange, "cb")

	dim1, rs1, store1 := newTestDimension()
	store1.initFillSlots(baseSlots1)
	_, _ = rs1.Update(context.Background(), rg.RangeSetter(baseRange))

	dim2, rs2, store2 := newTestDimension()
	store2.initFillSlots(FilterSlots(baseSlots2, rg.NewRange(140, 160)))
	_, _ = rs2.Update(context.Background(), rg.RangeSetter(rg.NewRange(140, 160)))

	{
		// Copy but number not continuous
		assert.Equal(t, ErrDiscontinuous, Copy[*testSlot](
			context.Background(),
			dim1,
			dim2,
			rg.Range{Start: 162},
			false,
		))
		assert.Equal(t, ErrDiscontinuous, Copy[*testSlot](
			context.Background(),
			dim1,
			dim2,
			rg.NewRange(0, 138),
			false,
		))
	}

	{
		// Copy but not linked
		assert.Equal(t, ErrLink, Copy[*testSlot](
			context.Background(),
			dim1,
			dim2,
			rg.Range{Start: 161},
			false,
		))
		assert.Equal(t, ErrLink, Copy[*testSlot](
			context.Background(),
			dim1,
			dim2,
			rg.Range{Start: 150},
			true,
		))
		assert.Equal(t, nil, Copy[*testSlot](
			context.Background(),
			dim1,
			dim2,
			rg.NewRange(0, 139),
			false,
		))
		assert.Equal(t, nil, Copy[*testSlot](
			context.Background(),
			dim1,
			dim2,
			rg.NewRange(0, 150),
			true,
		))
	}
}

func TestSync_fromScratch(t *testing.T) {
	baseRange := rg.NewRange(0, 300)
	baseSlots := newTestSlots(baseRange, "")

	dim1, rs1, store1 := newTestDimension()
	store1.initFillSlots(FilterSlots(baseSlots, rg.NewRange(100, 199)))
	_, _ = rs1.Update(context.Background(), rg.RangeSetter(rg.NewRange(100, 199)))

	dim2, _, _ := newTestDimension()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		for i := 0; i < 10; i++ {
			time.Sleep(time.Second)
			newRange := rg.NewRange(uint64(200+i*10), uint64(200+(i+1)*10-1))
			store1.initFillSlots(FilterSlots(baseSlots, newRange))
			_, _ = rs1.Update(context.Background(), newRange.Cover)
		}
		time.Sleep(time.Second * 5)
	}()

	assert.Equal(
		t,
		context.Canceled,
		Sync(
			ctx,
			dim1,
			dim2,
			SyncConfig{RoundInterval: time.Second},
		),
	)
	r1, _ := dim1.GetRange(context.Background())
	r2, _ := dim2.GetRange(context.Background())
	assert.Equal(t, r1, r2)
}

func TestSync_continue(t *testing.T) {
	log.ManuallySetLevel(zapcore.DebugLevel)
	log.BindFlag()

	baseRange := rg.NewRange(0, 300)
	baseSlots := newTestSlots(baseRange, "")

	dim1, rs1, store1 := newTestDimension()
	store1.initFillSlots(FilterSlots(baseSlots, rg.NewRange(100, 199)))
	_, _ = rs1.Update(context.Background(), rg.RangeSetter(rg.NewRange(100, 199)))

	dim2, rs2, store2 := newTestDimension()
	store2.initFillSlots(FilterSlots(baseSlots, rg.NewRange(100, 149)))
	_, _ = rs2.Update(context.Background(), rg.RangeSetter(rg.NewRange(100, 149)))

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		for i := 0; i < 10; i++ {
			time.Sleep(time.Second)
			newRange := rg.NewRange(uint64(200+i*10), uint64(200+(i+1)*10-1))
			store1.initFillSlots(FilterSlots(baseSlots, newRange))
			_, _ = rs1.Update(context.Background(), newRange.Cover)
		}
		time.Sleep(time.Second * 5)
	}()

	assert.Equal(
		t,
		context.Canceled,
		Sync(
			ctx,
			dim1,
			dim2,
			SyncConfig{RoundInterval: time.Second * 3},
		),
	)
	r1, _ := dim1.GetRange(context.Background())
	r2, _ := dim2.GetRange(context.Background())
	assert.Equal(t, r1, r2)
}

func TestSync_reorgSome(t *testing.T) {
	log.ManuallySetLevel(zapcore.DebugLevel)
	log.BindFlag()

	baseRange1 := rg.NewRange(0, 300)
	baseRange2 := rg.NewRange(150, 300)
	baseSlots1 := newTestSlots(baseRange1, "ca")
	baseSlots2 := newTestSlots(baseRange2, "cb", "ca149")

	dim1, rs1, store1 := newTestDimension()
	store1.initFillSlots(FilterSlots(baseSlots1, rg.NewRange(100, 199)))
	_, _ = rs1.Update(context.Background(), rg.RangeSetter(rg.NewRange(100, 199)))

	dim2, rs2, store2 := newTestDimension()
	store2.initFillSlots(FilterSlots(baseSlots1, rg.NewRange(100, 149)))
	store2.initFillSlots(FilterSlots(baseSlots2, rg.NewRange(150, 169)))
	_, _ = rs2.Update(context.Background(), rg.RangeSetter(rg.NewRange(100, 169)))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		time.Sleep(time.Second * 3)
	}()
	assert.Equal(
		t,
		context.Canceled,
		Sync(
			ctx,
			dim1,
			dim2,
			SyncConfig{RoundInterval: time.Second},
		),
	)
	r1, _ := dim1.GetRange(context.Background())
	r2, _ := dim2.GetRange(context.Background())
	assert.Equal(t, r1, r2)
	for n := r2.Start; n <= *r2.End; n++ {
		assert.Equal(t, fmt.Sprintf("ca%d", n), store2.slots.GetWithDefault(n, nil).GetHash())
	}
}

func TestSync_reorgAll(t *testing.T) {
	log.ManuallySetLevel(zapcore.DebugLevel)
	log.BindFlag()

	baseRange1 := rg.NewRange(0, 300)
	baseSlots1 := newTestSlots(baseRange1, "ca")
	baseSlots2 := newTestSlots(baseRange1, "cb")

	dim1, rs1, store1 := newTestDimension()
	store1.initFillSlots(FilterSlots(baseSlots1, rg.NewRange(100, 199)))
	_, _ = rs1.Update(context.Background(), rg.RangeSetter(rg.NewRange(100, 199)))

	dim2, rs2, store2 := newTestDimension()
	store2.initFillSlots(FilterSlots(baseSlots2, rg.NewRange(140, 159)))
	_, _ = rs2.Update(context.Background(), rg.RangeSetter(rg.NewRange(140, 159)))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		time.Sleep(time.Second * 3)
	}()
	assert.Equal(
		t,
		context.Canceled,
		Sync(
			ctx,
			dim1,
			dim2,
			SyncConfig{RoundInterval: time.Second},
		),
	)
	r1, _ := dim1.GetRange(context.Background())
	r2, _ := dim2.GetRange(context.Background())
	assert.Equal(t, r1, r2)
	for n := r2.Start; n <= *r2.End; n++ {
		assert.Equal(t, fmt.Sprintf("ca%d", n), store2.slots.GetWithDefault(n, nil).GetHash())
	}
}
