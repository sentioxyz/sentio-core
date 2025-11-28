# RPC Cache

A high-performance, Redis-backed caching library for RPC responses with support for background refresh, concurrency control, and automatic schema handling.

## Features

- **Generic Type-Safe API**: Utilizes Go generics for type-safe cache operations
- **Read-Through Caching**: Automatically loads data on cache miss
- **Background Refresh**: Asynchronously refresh stale cache entries without blocking requests
- **Concurrency Control**: Prevents thundering herd problem with distributed locking
- **Compression Support**: Built-in compression for efficient storage
- **Schema Evolution**: Gracefully handles schema changes with automatic fallback
- **Panic Protection**: Robust error handling prevents crashes from corrupt data
- **Rate Limiting**: Optional token bucket integration for cache bypass protection
- **Compute Stats Tracking**: Built-in metrics for cache hit/miss and refresh status

## Architecture

The library uses a pool of background worker goroutines (default: `runtime.NumCPU()`) to handle asynchronous cache refresh operations. Redis is used as the backing store with Lua scripts for atomic concurrency control.

### Core Components

- **RpcCache Interface**: Main cache operations (`Query`, `Get`, `Set`, `Delete`, `Load`)
- **Request Interface**: Defines cache key, TTL, and refresh interval
- **Response Interface**: Supports compute statistics tracking
- **Loader Function**: User-provided function to load fresh data on cache miss

## Installation

```go
import "sentioxyz/sentio-core/common/rpccache/cache"
```

## Quick Start

### 1. Define Your Request and Response Types

```go
type MyRequest struct {
    ID string
}

func (r *MyRequest) Key() cache.Key {
    return cache.Key{
        Prefix:   "myservice",
        UniqueID: r.ID,
    }
}

func (r *MyRequest) TTL() time.Duration {
    return 5 * time.Minute
}

func (r *MyRequest) RefreshInterval() time.Duration {
    return 1 * time.Minute
}

func (r *MyRequest) Clone() cache.Request {
    return &MyRequest{ID: r.ID}
}

type MyResponse struct {
    Data         string
    ComputeStats *protos.ComputeStats
}

func (r *MyResponse) GetComputeStats() *protos.ComputeStats {
    return r.ComputeStats
}
```

### 2. Create a Cache Instance

```go
import "github.com/redis/go-redis/v9"

redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

rpcCache := cache.NewRpcCache[*MyRequest, *MyResponse](redisClient)
```

### 3. Use the Cache

```go
// Define a loader function
loader := func(ctx context.Context, req *MyRequest, argv ...any) (*MyResponse, error) {
    // Your logic to fetch fresh data
    data, err := fetchFromDatabase(ctx, req.ID)
    if err != nil {
        return nil, err
    }
    return &MyResponse{
        Data:         data,
        ComputeStats: &protos.ComputeStats{},
    }, nil
}

// Query with cache (read-through)
resp, err := rpcCache.Query(ctx, req, loader)
if err != nil {
    log.Fatal(err)
}
```

## API Reference

### Core Methods

#### Query

Read-through cache operation. Returns cached data if available, otherwise loads fresh data.

```go
Query(ctx context.Context, req X, loader Loader[X, Y], options ...*Option) (Y, error)
```

**Behavior:**
- Cache hit: Returns cached response
- Cache miss: Calls loader, caches result, returns response
- Decode error: Treats as miss and reloads
- Supports background refresh when `WithRefreshBackground()` option is set

#### Get

Cache-only operation. Returns cached data without falling back to loader.

```go
Get(ctx context.Context, req X, loader Loader[X, Y], options ...*Option) (Y, bool)
```

**Behavior:**
- Cache hit: Returns `(response, true)`
- Cache miss or error: Returns `(zero value, false)`
- Can trigger background refresh with `WithRefreshBackground()` option

#### Load

Directly loads fresh data, bypassing cache read but still writing to cache.

```go
Load(ctx context.Context, req X, loader Loader[X, Y], option *Option) (Y, error)
```

#### Set

Manually set a cache entry.

```go
Set(ctx context.Context, req X, response Y) error
```

#### Delete

Remove a cache entry.

```go
Delete(ctx context.Context, req X) error
```

## Options

Configure cache behavior with functional options:

### WithRefreshBackground()

Enable asynchronous cache refresh. When a cached entry approaches expiration, it's refreshed in the background while returning the stale data immediately.

```go
resp, err := rpcCache.Query(ctx, req, loader, cache.WithRefreshBackground())
```

### WithConcurrencyControl()

Enable distributed locking to prevent multiple concurrent requests from loading the same data.

```go
resp, err := rpcCache.Query(ctx, req, loader, cache.WithConcurrencyControl())
```

**Behavior:**
- First request acquires lock and loads data
- Subsequent concurrent requests return `ErrResourceExhausted`
- Lock TTL: 60 seconds
- Safe type assertions prevent panics on Redis errors

### WithNoCache()

Bypass cache read and force fresh load. Useful for admin operations or debugging.

```go
resp, err := rpcCache.Query(ctx, req, loader, cache.WithNoCache())
```

**Note:** `Query` will load fresh data; `Get` will return miss.

### WithNoCacheTokenBucket(bucket, config)

Rate limit cache bypass operations to prevent abuse.

```go
resp, err := rpcCache.Query(ctx, req, loader,
    cache.WithNoCache(),
    cache.WithNoCacheTokenBucket(tokenBucket, &tokenbucket.RateLimitConfig{
        Limit:  10,
        Window: time.Minute,
    }),
)
```

Returns `ErrNoCacheNotAllowed` when rate limit is exceeded.

### WithSpecifiedTTL(duration)

Override the default TTL from `Request.TTL()`.

```go
resp, err := rpcCache.Query(ctx, req, loader, cache.WithSpecifiedTTL(10 * time.Minute))
```

### WithSpecifiedRefreshInterval(duration)

Override the default refresh interval from `Request.RefreshInterval()`.

```go
resp, err := rpcCache.Query(ctx, req, loader,
    cache.WithRefreshBackground(),
    cache.WithSpecifiedRefreshInterval(30 * time.Second),
)
```

### WithLoaderArgv(args)

Pass additional arguments to the loader function.

```go
resp, err := rpcCache.Query(ctx, req, loader,
    cache.WithLoaderArgv([]any{"extra", "params"}),
)
```

## Error Handling

### Built-in Errors

- **ErrResourceExhausted**: Returned when concurrency control blocks a request
- **ErrNoCacheNotAllowed**: Returned when cache bypass rate limit is exceeded

### Panic Protection

Both `Query` and `Get` methods include panic recovery:

```go
defer func() {
    if panicErr := recover(); panicErr != nil {
        logger.Errorf("rpc cache query panic: %v", panicErr)
        // Returns safe zero value instead of crashing
    }
}()
```

This prevents crashes from:
- Corrupt cached data
- Unexpected type assertions
- Compression/decompression errors

## Key Structure

Cache keys follow a hierarchical structure:

```
rpccache:{prefix}:{identifier}:{uniqueID}
```

### Special Key Types

- **Data key**: `rpccache:{prefix}:{uniqueID}`
- **Refresh key**: `rpccache:{prefix}:refresh:{uniqueID}`
- **Concurrency key**: `rpccache:{prefix}:concurrency:{uniqueID}`

### Identifier

Optional project-scoped identifier:

```go
type Identifier struct {
    ProjectID    string
    ProjectOwner string
    ProjectSlug  string
    Version      int32
    UserID       *string  // Optional user-specific caching
}
```

## Background Refresh

### How It Works

1. On cache hit, check if refresh interval has elapsed using Redis Lua CAS operation
2. If refresh is needed, enqueue background task to worker pool
3. Return cached data immediately (no blocking)
4. Background worker loads fresh data and updates cache

### Worker Pool

- Pool size: `runtime.NumCPU()` goroutines
- Queue capacity: `NumCPU * 2` tasks
- Tasks are dropped if queue is full (backpressure)
- Idempotent operations tolerate dropped tasks

### Monitoring

Track background refresh activity:

```go
// Access the atomic counter
activeRefreshes := cache.kLoaderCnt.Load()
```

## Response Error Handling

Supports responses that embed error information:

```go
type ResponseWithError interface {
    Response
    GetError() string
}
```

When a response implements `ResponseWithError` and `GetError()` returns non-empty:
- Response is **not cached** (avoids caching transient errors)
- Response is still returned to caller
- Compute stats marked as non-cached

```go
func (r *rpcCache[X, Y]) Load(ctx context.Context, req X, loader Loader[X, Y], option *Option) (resp Y, err error) {
    resp, err = loader(ctx, req, option.loaderArgv...)
    if err != nil {
        return resp, err
    }
    if respWithError, ok := CheckResponseWithError(resp); ok {
        if respWithError.GetError() != "" {
            // Do not cache the internal error
            return resp, nil
        }
    }
    // Cache the successful response
    _ = r.set(ctx, req.Key().String(), resp, ttl)
    return resp, nil
}
```

## Best Practices

### 1. Use Background Refresh for Hot Data

```go
// Frequently accessed data with low latency requirements
resp, err := rpcCache.Query(ctx, req, loader,
    cache.WithRefreshBackground(),
)
```

### 2. Enable Concurrency Control for Expensive Operations

```go
// Expensive database queries or external API calls
resp, err := rpcCache.Query(ctx, req, loader,
    cache.WithConcurrencyControl(),
)
```

### 3. Combine Options for Optimal Performance

```go
// Production-ready configuration
resp, err := rpcCache.Query(ctx, req, loader,
    cache.WithRefreshBackground(),
    cache.WithConcurrencyControl(),
)
```

### 4. Use Get for Optional Caching

```go
// Try cache first, handle miss gracefully
if cachedResp, ok := rpcCache.Get(ctx, req, loader); ok {
    return cachedResp
}
// Fallback to direct load or error
return loadDirectly(ctx, req)
```

### 5. Implement Clone() Properly

Background refresh clones requests to avoid race conditions:

```go
func (r *MyRequest) Clone() cache.Request {
    // Deep copy all fields
    return &MyRequest{
        ID:   r.ID,
        Data: append([]byte(nil), r.Data...),
    }
}
```

## Schema Evolution

The cache gracefully handles schema changes:

1. **Decode Failure Detection**: When `compression.Decode` fails, it's logged as a potential schema change
2. **Automatic Reload**: `Query` treats decode errors as cache miss and reloads fresh data
3. **No Panic**: Decode errors never crash the application
4. **Gradual Migration**: Old cached data expires naturally based on TTL

```go
decoded, err := compression.Decode[Y](string(data))
if err != nil {
    logger.Warnf("decode response failed, maybe there has schema changed: %v", err)
    // Treat as cache miss and reload
    option.force = true
    return r.Load(ctx, req, loader, option)
}
```

## Concurrency Safety

### Redis Context Usage

All Redis operations use `context.Background()` instead of request context:

```go
// use background context to avoid canceling by gateway
err = r.client.SetEx(context.Background(), key, responseBytes, ttl).Err()
```

**Rationale:**
- Ensures cache operations complete even if client disconnects
- Maintains cache consistency
- Prevents partial state from canceled operations

### Safe Type Assertions

Redis Lua script results are safely type-checked:

```go
code, ok := statusCode.(int64)
if !ok {
    logger.Warnf("unexpected status code type: %T, value: %v; allow by default", statusCode, statusCode)
    return true  // Fail-open to avoid blocking traffic
}
```

## Performance Considerations

- **Compression**: Responses are compressed before storing in Redis (see `compression` package)
- **Connection Pooling**: Use Redis client connection pooling for optimal performance
- **TTL Strategy**: Balance between freshness and load (typical: 5-15 minutes)
- **Refresh Interval**: Set to 20-50% of TTL for smooth background refresh
- **Worker Pool**: Automatically sized to `runtime.NumCPU()`, handles concurrent refreshes efficiently

## Troubleshooting

### High Cache Miss Rate

- Check if TTL is too short
- Verify key generation is deterministic
- Check Redis connectivity and memory

### Background Refresh Not Working

- Ensure `WithRefreshBackground()` is set
- Check refresh interval is less than TTL
- Monitor worker queue: if `kQueue` is full, increase capacity or reduce refresh frequency

### ErrResourceExhausted Errors

- Too many concurrent requests for same key with concurrency control enabled
- Consider increasing concurrency lock TTL (currently 60s)
- Review if concurrency control is needed for this endpoint

### Decode Errors

- Schema change detected: old cached data with new code
- Wait for TTL expiration or manually invalidate with `Delete()`
- Consider versioning in cache keys for breaking changes

## Example: Complete Integration

```go
package main

import (
    "context"
    "time"

    "sentioxyz/sentio-core/common/rpccache/cache"
    "github.com/redis/go-redis/v9"
)

type UserRequest struct {
    UserID string
}

func (r *UserRequest) Key() cache.Key {
    return cache.Key{
        Prefix:   "user",
        UniqueID: r.UserID,
    }
}

func (r *UserRequest) TTL() time.Duration {
    return 10 * time.Minute
}

func (r *UserRequest) RefreshInterval() time.Duration {
    return 2 * time.Minute
}

func (r *UserRequest) Clone() cache.Request {
    return &UserRequest{UserID: r.UserID}
}

type UserResponse struct {
    Name         string
    Email        string
    ComputeStats *protos.ComputeStats
}

func (r *UserResponse) GetComputeStats() *protos.ComputeStats {
    if r.ComputeStats == nil {
        r.ComputeStats = &protos.ComputeStats{}
    }
    return r.ComputeStats
}

func main() {
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    
    userCache := cache.NewRpcCache[*UserRequest, *UserResponse](redisClient)
    
    loader := func(ctx context.Context, req *UserRequest, argv ...any) (*UserResponse, error) {
        // Simulate database query
        user, err := database.GetUser(ctx, req.UserID)
        if err != nil {
            return nil, err
        }
        return &UserResponse{
            Name:         user.Name,
            Email:        user.Email,
            ComputeStats: &protos.ComputeStats{},
        }, nil
    }
    
    ctx := context.Background()
    req := &UserRequest{UserID: "user123"}
    
    // Query with background refresh and concurrency control
    resp, err := userCache.Query(ctx, req, loader,
        cache.WithRefreshBackground(),
        cache.WithConcurrencyControl(),
    )
    if err != nil {
        panic(err)
    }
    
    // Check if response was from cache
    if resp.GetComputeStats().IsCached {
        println("Cache hit!")
    }
    
    // Check if background refresh was triggered
    if resp.GetComputeStats().IsRefreshing {
        println("Background refresh in progress")
    }
}
```

## License

Internal library for Sentio platform.

