package clientpool

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"time"
)

type Block struct {
	Number    uint64
	Hash      string
	Timestamp time.Time
}

func (b Block) String() string {
	var hash string
	if len(b.Hash) > 12 {
		hash = "/" + b.Hash[:6] + ".." + b.Hash[len(b.Hash)-6:]
	} else if len(b.Hash) > 0 {
		hash = "/" + b.Hash
	}
	return fmt.Sprintf("%d%s@%s", b.Number, hash, b.Timestamp.Format(time.RFC3339Nano))
}

var (
	ErrInvalidConfig = errors.New("invalid config")
	ErrNoValidClient = errors.New("no valid client")

	// ErrInterrupted is returned by UseClient when any entry in the pool carries one of the
	// tags the caller passed via InterruptWithTags: the tag marks the whole call as pointless
	// (e.g. a method-authority endpoint rejected the method), so the pool gives up immediately
	// instead of probing entries or waiting for a priority downgrade.
	ErrInterrupted = errors.New("interrupted by a tagged client")
)

type Client interface {
	// GetName return the name of this client
	GetName() string

	// Init may return ErrInvalidConfig
	Init(ctx context.Context) (Block, error)

	// SubscribeLatest should not stop until ctx canceled
	SubscribeLatest(ctx context.Context, ch chan<- Block)

	// Snapshot return snapshot of the client, may be client is nil
	Snapshot() any
}

type EntryConfig[CONFIG any] interface {
	GetName() string // as the unique identity of the entry
	Equal(a CONFIG) bool
}

type UsedNotifier func(what string, dur time.Duration, hasErr bool)
