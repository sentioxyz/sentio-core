package cache

import (
	"context"
	"time"
)

// Cacheable defines the interface for objects that can be cached
type Cacheable[T any] interface {
	// Key returns the unique identifier for this cache entry
	Key() string

	// TTL returns the time-to-live duration for this entry
	TTL() time.Duration

	// RefreshInterval returns how often this entry should be refreshed
	// If 0, the entry will not be auto-refreshed
	RefreshInterval() time.Duration

	// Reload initializes or refreshes the cached value
	Reload(ctx context.Context) (T, error)
}
