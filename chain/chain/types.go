package chain

import (
	"context"
	"errors"
	rg "sentioxyz/sentio-core/common/range"
)

type LatestSlotCache[SLOT Slot] interface {
	GetRange(ctx context.Context) (rg.Range, error)
	Traverse(
		ctx context.Context,
		interval rg.Range,
		fn func(ctx context.Context, st SLOT) error,
	) (cached rg.Range, err error)
	Wait(ctx context.Context, latestGt uint64) (latest uint64, err error)
	GetByNumber(ctx context.Context, sn uint64) (SLOT, error) // will return ErrSlotNotFound if not found in cache
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
	GetSlotHeader(ctx context.Context, number uint64) (Slot, error)
}

type Dimension[SLOT Slot] interface {
	Init(ctx context.Context) error
	Load(ctx context.Context, interval rg.Range, slotChan chan<- SLOT) error
	LoadHeader(ctx context.Context, sn uint64) (Slot, error) // will return ErrSlotNotFound if not found
	GetRange(ctx context.Context) (rg.Range, error)
	Wait(ctx context.Context, sn uint64) error
	CheckMissing(ctx context.Context, interval rg.Range, missing chan<- rg.Range) error
	Save(ctx context.Context, interval rg.Range, slotChan <-chan SLOT) error
	Delete(ctx context.Context, interval rg.Range) error
}

type SimpleSlotStore[SLOT Slot] interface {
	LoadHeader(ctx context.Context, sn uint64) (Slot, error)
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
