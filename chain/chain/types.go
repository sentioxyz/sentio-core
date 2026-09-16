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
	GetByChecker(ctx context.Context, checker func(SLOT) bool) (SLOT, error) // may return ErrSlotNotFound
	GetByNumber(ctx context.Context, sn uint64) (SLOT, error)                // may return ErrSlotNotFound
	GetByHash(ctx context.Context, hash string) (SLOT, error)                // may return ErrSlotNotFound
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

// SlotRepairer is an optional interface a Dimension can implement to rebuild the degraded
// parts of a slot that entered the cache incomplete (i.e. with a non-empty Features()),
// e.g. an evm slot that got the MissTrace feature because trace loading failed.
type SlotRepairer[SLOT Slot] interface {
	// RepairSlot tries to rebuild the missing parts of st and returns the repaired slot.
	// It must not modify st in place — st is shared with readers of the cache.
	// ok=false with a nil error means the slot cannot (or need not) be repaired and should
	// be skipped; a non-nil error means the attempt failed and may be retried later.
	RepairSlot(ctx context.Context, st SLOT) (repaired SLOT, ok bool, err error)
}

type Dimension[SLOT Slot] interface {
	Init(ctx context.Context) error
	Load(ctx context.Context, interval rg.Range, slotChan chan<- SLOT) error
	LoadHeader(ctx context.Context, sn uint64) (Slot, error) // will return ErrSlotNotFound if not found
	GetRange(ctx context.Context) (rg.Range, error)
	CheckMissing(ctx context.Context, interval rg.Range, missing chan<- rg.Range) error
	Save(ctx context.Context, interval rg.Range, slotChan <-chan SLOT) error
	Delete(ctx context.Context, interval rg.Range) error
}

type SimpleSlotStore[SLOT Slot] interface {
	LoadHeader(ctx context.Context, sn uint64) (Slot, error)
	CheckMissing(ctx context.Context, interval rg.Range, missing chan<- rg.Range) error
	// Save need to clean data in interval before saving, because there may be residual data inside.
	Save(ctx context.Context, interval rg.Range, slotChan <-chan SLOT, doneChan chan<- rg.Range) error
	Load(ctx context.Context, interval rg.Range, slotChan chan<- SLOT) error
	// Delete if failed, the data in interval allow to be incomplete
	Delete(ctx context.Context, interval rg.Range) error
}

var (
	ErrDiscontinuous = errors.New("discontinuous")
	ErrLink          = errors.New("link error")
	ErrSlotNotFound  = errors.New("slot not found")
)

// DuplicateReport describes the duplicate rows a DuplicateChecker found in one table.
type DuplicateReport struct {
	Table     string
	Groups    uint64 // distinct unique keys carried by more than one row
	ExtraRows uint64 // rows beyond the first one of every duplicated key
	First     uint64 // slot number of the first duplicated key
	Last      uint64 // slot number of the last duplicated key
}

// DuplicateChecker is an optional interface of a Dimension (or of the slot store behind it) that
// reports rows sharing a unique key inside interval. Sync runs it periodically on the destination
// (see SyncConfig.DupCheckInterval) as a self-check that a retried save has not left a second
// copy of an earlier flush behind.
type DuplicateChecker interface {
	CheckDuplicates(ctx context.Context, interval rg.Range) ([]DuplicateReport, error)
}

// ErrDuplicateCheckUnsupported is returned by CheckDuplicates when the underlying store cannot check.
var ErrDuplicateCheckUnsupported = errors.New("duplicate check unsupported")
