package chain

import (
	"context"
	"errors"
	rg "sentioxyz/sentio-core/common/range"
)

type Slot interface {
	GetNumber() uint64
	GetHash() string
	GetParentHash() string
	Linked() bool
}

type SlotLoader[SLOT Slot] interface {
	Load(ctx context.Context, interval rg.Range, slotChan chan<- SLOT) error
}

type RangeStore interface {
	Get(ctx context.Context) (rg.Range, error)
	Update(ctx context.Context, operator rg.RangeOperator) (rg.Range, error)
}

type SlotGetter[SLOT Slot] interface {
	GetSlots(ctx context.Context, number rg.Range) ([]SLOT, error)
	GetSlotHeader(ctx context.Context, number uint64) (slot.Header, error)
}

type Dimension[SLOT Slot] interface {
	Init(ctx context.Context) error
	Load(ctx context.Context, interval rg.Range, slotChan chan<- SLOT) error
	LoadHeader(ctx context.Context, sn uint64) (slot.Header, error) // will return ErrSlotNotFound if not found
	GetRange(ctx context.Context) (rg.Range, error)
	Wait(ctx context.Context, sn uint64) error
	CheckMissing(ctx context.Context, interval rg.Range, missing chan<- rg.Range) error
	Save(ctx context.Context, interval rg.Range, slotChan <-chan SLOT) error
	Delete(ctx context.Context, interval rg.Range) error
}

type SimpleSlotStore[SLOT Slot] interface {
	LoadHeader(ctx context.Context, sn uint64) (slot.Header, error)
	CheckMissing(ctx context.Context, interval rg.Range, missing chan<- rg.Range) error
	Save(ctx context.Context, interval rg.Range, slotChan <-chan SLOT, doneChan chan<- rg.Range) error
	Load(ctx context.Context, interval rg.Range, slotChan chan<- SLOT) error
	Delete(ctx context.Context, interval rg.Range) error
}

var (
	ErrDiscontinuous = errors.New("discontinuous")
	ErrLink          = errors.New("link error")
	ErrSlotNotFound  = errors.New("slot not found")
)
