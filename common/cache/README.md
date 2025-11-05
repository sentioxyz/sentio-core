# sentio-core: common/cache

A small, high-performance, generic cache implementation used by the sentio-core project.

This package provides a thread-safe in-memory cache with TTL, auto-refresh, single-flight loading (to avoid duplicate loads), and basic eviction policies (LRU/LFU/TinyLFU fallback to LRU).

Highlights
- Generic API (Go generics) — works with any value type T.
- Cacheable interface for pluggable load/refresh logic.
- Single-flight loading: concurrent Get calls for the same key only load once.
- TTL-based expiration and optional auto-refresh.
- Configurable max size and eviction policy.
- Lightweight stats (hits/misses/evictions/refreshes/errors/load time).

Status
- Implementation is covered by unit tests (see `common/cache/cache_test.go`).

Quick start

Installation / Import

This repository's module path (from `go.mod`) is:

```
sentioxyz/sentio-core
```

Import the cache package:

```go
import "sentioxyz/sentio-core/common/cache"
```

Minimal example (Get or Load)

```go
package main

import (
	"context"
	"fmt"
	"time"

	"sentioxyz/sentio-core/common/cache"
)

// StringItem implements cache.Cacheable[string]
type StringItem struct{ k string }

func (s *StringItem) Key() string                       { return s.k }
func (s *StringItem) TTL() time.Duration                { return time.Minute }
func (s *StringItem) RefreshInterval() time.Duration    { return 0 }
func (s *StringItem) Reload(ctx context.Context) (string, error) {
	// Replace with real load logic (DB/HTTP/etc.)
	return "value-for-" + s.k, nil
}

func main() {
	c := cache.New[string](context.Background(), cache.DefaultConfig())
	defer c.Close()

	item := &StringItem{k: "foo"}
	v, err := c.Get(item) // will call item.Reload on cache miss
	if err != nil {
		panic(err)
	}
	fmt.Println("value:", v)
}
```

Write (preload) example

```go
// Create an implementation of cache.Cacheable[T] and call Write to synchronously load
item := &StringItem{k: "bar"}
if err := c.Write(item); err != nil {
	// Write calls item.Reload and stores the result (returns any load error)
	panic(err)
}
val, ok := c.Read("bar")
if ok {
	fmt.Println(val)
}
```

Auto-refresh example

```go
// Implement RefreshInterval to return a duration > 0 to enable automatic background refreshes.
type RefreshingItem struct{ k string }
func (r *RefreshingItem) Key() string                    { return r.k }
func (r *RefreshingItem) TTL() time.Duration             { return 10 * time.Minute }
func (r *RefreshingItem) RefreshInterval() time.Duration { return 1 * time.Minute }
func (r *RefreshingItem) Reload(ctx context.Context) (string, error) {
	// expensive load
	return "...", nil
}
```

Concurrency and single-flight

Use `Get(item)` when multiple goroutines may try to load the same key concurrently — the cache ensures the underlying `Reload` is executed only once and the result (or error) is shared with waiters.

Configuration

- `DefaultConfig()` returns sensible defaults: max size 1000, LRU policy, cleanup interval 5m, refresh workers = runtime.NumCPU().
- `New[T](ctx, config)` creates a cache instance.

API reference (selected)

- type Cacheable[T any]
  - Key() string
  - TTL() time.Duration
  - RefreshInterval() time.Duration
  - Reload(ctx context.Context) (T, error)

- func DefaultConfig() Config
- func New[T any](ctx context.Context, config Config) *Cache[T]

- Methods on `*Cache[T]`:
  - Read(key string) (T, bool)
  - Write(item Cacheable[T]) error
  - Get(item Cacheable[T]) (T, error) // read or load (single-flight)
  - Delete(key string)
  - Clear()
  - GetStats() Stats
  - Size() int
  - Close()

- Types:
  - type Policy int — PolicyLRU / PolicyLFU / PolicyTinyLFU
  - type Stats — Hits, Misses, Evictions, Refreshes, Errors, ItemCount, LoadTime

Testing

Run the cache package tests:

```bash
# from repo root
go test ./common/cache -v
```

Or run all tests in the module (may take longer):

```bash
go test ./...
```

Notes & assumptions

- The module path in `go.mod` is `sentioxyz/sentio-core`. If you consume this package from another module, import path should match the module path or your fork.
- The cache is intended as a general-purpose building block in this repo — it does not persist to disk or external stores.
- Eviction policies are implemented in a simple way appropriate for in-memory caches; TinyLFU falls back to LRU in this implementation.

Contributing

If you want to improve the cache:
- Add more sophisticated TinyLFU frequency sketches.
- Add instrumentation metrics (Prometheus/OpenTelemetry hooks).
- Make refresh backoff strategies configurable.

License

See the repository LICENSE (if present) for license details.



