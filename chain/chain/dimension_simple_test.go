package chain

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/concurrency"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

type testSlot struct {
	Number     uint64
	Hash       string
	ParentHash string
	Filler     []byte
}

func (b *testSlot) GetNumber() uint64 {
	return b.Number
}

func (b *testSlot) GetHash() string {
	return b.Hash
}

func (b *testSlot) GetParentHash() string {
	return b.ParentHash
}

func (b *testSlot) Linked() bool {
	return true
}

type testSimpleSlotStore[SLOT Slot] struct {
	slots *utils.SafeMap[uint64, SLOT]
}

func (s *testSimpleSlotStore[SLOT]) initFillSlots(slots []SLOT) {
	for _, st := range slots {
		s.slots.Put(st.GetNumber(), st)
	}
}

func (s *testSimpleSlotStore[SLOT]) LoadHeader(ctx context.Context, sn uint64) (Slot, error) {
	st, has := s.slots.Get(sn)
	if has {
		return st, nil
	}
	return nil, ErrSlotNotFound
}

func (s *testSimpleSlotStore[SLOT]) CheckMissing(ctx context.Context, interval rg.Range, missing chan<- rg.Range) error {
	for sn := interval.Start; sn <= *interval.End; sn++ {
		if _, has := s.slots.Get(sn); !has {
			select {
			case missing <- rg.NewSingleRange(sn):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}

func (s *testSimpleSlotStore[SLOT]) Save(ctx context.Context, interval rg.Range, slotChan <-chan SLOT, doneChan chan<- rg.Range) error {
	return concurrency.ForEach(ctx, slotChan, func(ctx context.Context, index int, st SLOT) error {
		s.slots.Put(st.GetNumber(), st)
		select {
		case doneChan <- rg.NewSingleRange(st.GetNumber()):
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
}

func (s *testSimpleSlotStore[SLOT]) Load(ctx context.Context, interval rg.Range, slotChan chan<- SLOT) error {
	for sn := interval.Start; sn <= *interval.End; sn++ {
		st, has := s.slots.Get(sn)
		if !has {
			continue
		}
		select {
		case slotChan <- st:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *testSimpleSlotStore[SLOT]) Delete(ctx context.Context, interval rg.Range) error {
	var sns []uint64
	s.slots.Traverse(func(st uint64, val SLOT) {
		if interval.Contains(st) {
			sns = append(sns, st)
		}
	})
	for _, sn := range sns {
		s.slots.Del(sn)
	}
	return nil
}

type testRangeStore struct {
	cur rg.Range
}

func (s *testRangeStore) Get(ctx context.Context) (rg.Range, error) {
	return s.cur, nil
}

func (s *testRangeStore) Update(ctx context.Context, operator rg.RangeOperator) (rg.Range, error) {
	s.cur = operator(s.cur)
	return s.cur, nil
}

func TestSimpleDimension_Save1(t *testing.T) {
	// test save succeed
	newTestSlot := func(number uint64) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("hash-%d", number),
			ParentHash: fmt.Sprintf("hash-%d", number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: rg.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[uint64, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 11; i <= 20; i++ {
		slotChan <- newTestSlot(uint64(i))
	}
	close(slotChan)
	assert.NoError(t, dim.Save(context.Background(), rg.NewRange(11, 20), slotChan))
}

func TestSimpleDimension_Save2(t *testing.T) {
	// miss the last slot, and the range will be correct
	newTestSlot := func(number uint64) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("hash-%d", number),
			ParentHash: fmt.Sprintf("hash-%d", number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: rg.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[uint64, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 11; i <= 19; i++ {
		slotChan <- newTestSlot(uint64(i))
	}
	close(slotChan)
	err := dim.Save(context.Background(), rg.NewRange(11, 20), slotChan)
	assert.NoError(t, err)
	assert.Equal(t, rg.NewRange(10, 19), rangeStore.cur)
}

func TestSimpleDimension_SaveErrDiscontinuous1(t *testing.T) {
	// test ErrDiscontinuous because miss first slot
	newTestSlot := func(number uint64) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("hash-%d", number),
			ParentHash: fmt.Sprintf("hash-%d", number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: rg.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[uint64, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 12; i <= 20; i++ {
		slotChan <- newTestSlot(uint64(i))
	}
	close(slotChan)
	err := dim.Save(context.Background(), rg.NewRange(11, 20), slotChan)
	assert.True(t, errors.Is(err, ErrDiscontinuous), "expected ErrDiscontinuous, got %v", err)
}

func TestSimpleDimension_SaveErrDiscontinuous2(t *testing.T) {
	// test ErrDiscontinuous because miss middle slot
	newTestSlot := func(number uint64) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("hash-%d", number),
			ParentHash: fmt.Sprintf("hash-%d", number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: rg.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[uint64, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 11; i <= 20; i++ {
		if i == 15 {
			continue
		}
		slotChan <- newTestSlot(uint64(i))
	}
	close(slotChan)
	err := dim.Save(context.Background(), rg.NewRange(11, 20), slotChan)
	assert.True(t, errors.Is(err, ErrDiscontinuous), "expected ErrDiscontinuous, got %v", err)
}

func TestSimpleDimension_SaveErrDiscontinuous3(t *testing.T) {
	// test ErrDiscontinuous because order
	newTestSlot := func(number uint64) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("hash-%d", number),
			ParentHash: fmt.Sprintf("hash-%d", number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: rg.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[uint64, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 20; i >= 11; i-- {
		slotChan <- newTestSlot(uint64(i))
	}
	close(slotChan)
	err := dim.Save(context.Background(), rg.NewRange(11, 20), slotChan)
	assert.True(t, errors.Is(err, ErrDiscontinuous), "expected ErrDiscontinuous, got %v", err)
}

func TestSimpleDimension_SaveErrLink1(t *testing.T) {
	// test ErrLink at the head
	newTestSlot := func(number uint64, hashPrefix string) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("%s-%d", hashPrefix, number),
			ParentHash: fmt.Sprintf("%s-%d", hashPrefix, number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: rg.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[uint64, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10, "bad"))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 11; i <= 20; i++ {
		slotChan <- newTestSlot(uint64(i), "good")
	}
	close(slotChan)
	err := dim.Save(context.Background(), rg.NewRange(11, 20), slotChan)
	assert.True(t, errors.Is(err, ErrLink), "expected ErrLink, got %v", err)
}

func TestSimpleDimension_SaveErrLink2(t *testing.T) {
	// test ErrLink at the tail
	newTestSlot := func(number uint64, hashPrefix string) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("%s-%d", hashPrefix, number),
			ParentHash: fmt.Sprintf("%s-%d", hashPrefix, number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: rg.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[uint64, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10, "good"))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 11; i <= 19; i++ {
		slotChan <- newTestSlot(uint64(i), "good")
	}
	slotChan <- newTestSlot(20, "bad")
	close(slotChan)
	err := dim.Save(context.Background(), rg.NewRange(11, 20), slotChan)
	assert.True(t, errors.Is(err, ErrLink), "expected ErrLink, got %v", err)
}
