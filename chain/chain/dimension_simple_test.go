package chain

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio/chain/slot"
	"sentioxyz/sentio/common/number"
)

type testSlot struct {
	Number     number.Number
	Hash       string
	ParentHash string
	Filler     []byte
}

func (b *testSlot) GetNumber() number.Number {
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

type testSimpleSlotStore[SLOT slot.Slot] struct {
	slots *utils.SafeMap[number.Number, SLOT]
}

func (s *testSimpleSlotStore[SLOT]) initFillSlots(slots []SLOT) {
	for _, st := range slots {
		s.slots.Put(st.GetNumber(), st)
	}
}

func (s *testSimpleSlotStore[SLOT]) LoadHeader(ctx context.Context, sn number.Number) (slot.Header, error) {
	st, has := s.slots.Get(sn)
	if has {
		return st, nil
	}
	return nil, ErrSlotNotFound
}

func (s *testSimpleSlotStore[SLOT]) CheckMissing(ctx context.Context, interval number.Range, missing chan<- number.Range) error {
	for sn := interval.L(); sn <= interval.R(); sn++ {
		if _, has := s.slots.Get(sn); !has {
			select {
			case missing <- number.NewSingleRange(sn):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}

func (s *testSimpleSlotStore[SLOT]) Save(ctx context.Context, interval number.Range, slotChan <-chan SLOT, doneChan chan<- number.Range) error {
	return concurrency.ForEach(ctx, slotChan, func(ctx context.Context, index int, st SLOT) error {
		s.slots.Put(st.GetNumber(), st)
		select {
		case doneChan <- number.NewSingleRange(st.GetNumber()):
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
}

func (s *testSimpleSlotStore[SLOT]) Load(ctx context.Context, interval number.Range, slotChan chan<- SLOT) error {
	for sn := interval.L(); sn <= interval.R(); sn++ {
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

func (s *testSimpleSlotStore[SLOT]) Delete(ctx context.Context, interval number.Range) error {
	var sns []number.Number
	s.slots.Traverse(func(st number.Number, val SLOT) {
		if interval.ContainsNumber(st) {
			sns = append(sns, st)
		}
	})
	for _, sn := range sns {
		s.slots.Del(sn)
	}
	return nil
}

type testRangeStore struct {
	cur number.Range
}

func (s *testRangeStore) Get(ctx context.Context) (number.Range, error) {
	return s.cur, nil
}

func (s *testRangeStore) Update(ctx context.Context, operator number.RangeOperator) (number.Range, error) {
	s.cur = operator(s.cur)
	return s.cur, nil
}

func (s *testRangeStore) Wait(ctx context.Context, sn number.Number) error {
	if s.cur.ContainsNumber(sn) {
		return nil
	}
	return fmt.Errorf("%d out of range %s", sn, s.cur.String())
}

func TestSimpleDimension_Save1(t *testing.T) {
	// test save succeed
	newTestSlot := func(number number.Number) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("hash-%d", number),
			ParentHash: fmt.Sprintf("hash-%d", number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: number.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[number.Number, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 11; i <= 20; i++ {
		slotChan <- newTestSlot(number.Number(i))
	}
	close(slotChan)
	assert.NoError(t, dim.Save(context.Background(), number.NewRange(11, 20), slotChan))
}

func TestSimpleDimension_Save2(t *testing.T) {
	// miss the last slot, and the range will be correct
	newTestSlot := func(number number.Number) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("hash-%d", number),
			ParentHash: fmt.Sprintf("hash-%d", number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: number.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[number.Number, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 11; i <= 19; i++ {
		slotChan <- newTestSlot(number.Number(i))
	}
	close(slotChan)
	err := dim.Save(context.Background(), number.NewRange(11, 20), slotChan)
	assert.NoError(t, err)
	assert.Equal(t, number.NewRange(10, 19), rangeStore.cur)
}

func TestSimpleDimension_SaveErrDiscontinuous1(t *testing.T) {
	// test ErrDiscontinuous because miss first slot
	newTestSlot := func(number number.Number) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("hash-%d", number),
			ParentHash: fmt.Sprintf("hash-%d", number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: number.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[number.Number, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 12; i <= 20; i++ {
		slotChan <- newTestSlot(number.Number(i))
	}
	close(slotChan)
	err := dim.Save(context.Background(), number.NewRange(11, 20), slotChan)
	assert.True(t, errors.Is(err, ErrDiscontinuous), "expected ErrDiscontinuous, got %v", err)
}

func TestSimpleDimension_SaveErrDiscontinuous2(t *testing.T) {
	// test ErrDiscontinuous because miss middle slot
	newTestSlot := func(number number.Number) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("hash-%d", number),
			ParentHash: fmt.Sprintf("hash-%d", number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: number.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[number.Number, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 11; i <= 20; i++ {
		if i == 15 {
			continue
		}
		slotChan <- newTestSlot(number.Number(i))
	}
	close(slotChan)
	err := dim.Save(context.Background(), number.NewRange(11, 20), slotChan)
	assert.True(t, errors.Is(err, ErrDiscontinuous), "expected ErrDiscontinuous, got %v", err)
}

func TestSimpleDimension_SaveErrDiscontinuous3(t *testing.T) {
	// test ErrDiscontinuous because order
	newTestSlot := func(number number.Number) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("hash-%d", number),
			ParentHash: fmt.Sprintf("hash-%d", number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: number.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[number.Number, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 20; i >= 11; i-- {
		slotChan <- newTestSlot(number.Number(i))
	}
	close(slotChan)
	err := dim.Save(context.Background(), number.NewRange(11, 20), slotChan)
	assert.True(t, errors.Is(err, ErrDiscontinuous), "expected ErrDiscontinuous, got %v", err)
}

func TestSimpleDimension_SaveErrLink1(t *testing.T) {
	// test ErrLink at the head
	newTestSlot := func(number number.Number, hashPrefix string) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("%s-%d", hashPrefix, number),
			ParentHash: fmt.Sprintf("%s-%d", hashPrefix, number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: number.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[number.Number, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10, "bad"))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 11; i <= 20; i++ {
		slotChan <- newTestSlot(number.Number(i), "good")
	}
	close(slotChan)
	err := dim.Save(context.Background(), number.NewRange(11, 20), slotChan)
	assert.True(t, errors.Is(err, ErrLink), "expected ErrLink, got %v", err)
}

func TestSimpleDimension_SaveErrLink2(t *testing.T) {
	// test ErrLink at the tail
	newTestSlot := func(number number.Number, hashPrefix string) *testSlot {
		return &testSlot{
			Number:     number,
			Hash:       fmt.Sprintf("%s-%d", hashPrefix, number),
			ParentHash: fmt.Sprintf("%s-%d", hashPrefix, number-1),
		}
	}
	rangeStore := &testRangeStore{
		cur: number.NewRange(10, 10),
	}
	slotStore := &testSimpleSlotStore[*testSlot]{
		slots: utils.NewSafeMap[number.Number, *testSlot](),
	}
	slotStore.slots.Put(10, newTestSlot(10, "good"))
	dim := NewSimpleDimension[*testSlot](rangeStore, slotStore)

	slotChan := make(chan *testSlot, 100)
	for i := 11; i <= 19; i++ {
		slotChan <- newTestSlot(number.Number(i), "good")
	}
	slotChan <- newTestSlot(20, "bad")
	close(slotChan)
	err := dim.Save(context.Background(), number.NewRange(11, 20), slotChan)
	assert.True(t, errors.Is(err, ErrLink), "expected ErrLink, got %v", err)
}
