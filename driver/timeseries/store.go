package timeseries

import (
	"context"
	"errors"
	"time"
)

type StoreMeta interface {
	GetHash() string
	GetAllMeta() map[MetaType]map[string]Meta
	Different(other StoreMeta) bool
	String() string

	MetaTypes() []MetaType
	MetaNames(t MetaType) []string
	MetaByType(t MetaType) map[string]Meta
	Meta(t MetaType, name string) (Meta, bool)
	MustMeta(t MetaType, name string) Meta
}

type Store interface {
	Init(ctx context.Context) error
	CleanAll(ctx context.Context) error

	// Meta related methods
	Meta() StoreMeta
	ReloadMeta(ctx context.Context) error

	// AppendData Will first synchronize all table structures, then insert data rows, and finally calculate the new
	// rows of aggregation tables.
	// If a table added fields, the Aggregation use this table as source should also add fields manually.
	AppendData(ctx context.Context, data []Dataset, chainID string, curTime time.Time) error
	DeleteData(ctx context.Context, chainID string, slotNumber int64) error
}

var (
	ErrInvalidMetaDiff   = errors.New("invalid meta diff")
	ErrInvalidMeta       = errors.New("invalid meta")
	ErrTooManyMetrics    = errors.New("too many metrics")
	ErrTooManyEventTypes = errors.New("too many event types")
	ErrTooManySeries     = errors.New("too many time series")
)
