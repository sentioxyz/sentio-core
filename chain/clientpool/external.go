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
