# StateMirror Package

[中文文档](./README.zh-CN.md)

## Overview

The `statemirror` package provides a type-safe state mirroring system for storing and synchronizing on-chain state data in Redis. It supports mapping arbitrary key-value pairs to Redis Hash structures and provides strongly-typed access interfaces.

## Core Concepts

### 1. Mirror

`Mirror` is the low-level interface providing basic operations on Redis Hashes:

- **Upsert**: Synchronize the entire state, automatically calculating and updating differences
- **UpsertStreaming**: Stream-based synchronization for large datasets
- **Apply**: Apply incremental changes (add/delete fields)
- **Get/MGet/GetAll**: Read single, multiple, or all fields
- **Scan**: Cursor-based field scanning

### 2. TypedMirror

`TypedMirror[K, V]` is a generic wrapper around `Mirror` providing type-safe access:

- Automatically handles key and value serialization/deserialization
- Provides strongly-typed Get/MGet/GetAll/Scan methods
- Type conversion through `StateCodec`

### 3. StateCodec

`StateCodec[K, V]` interface defines encoding/decoding rules for keys and values:

```go
type StateCodec[K comparable, V any] interface {
    Field(k K) (string, error)           // key -> Redis field string
    ParseField(field string) (K, error)  // Redis field string -> key
    Encode(v V) (string, error)          // value -> string
    Decode(s string) (V, error)          // string -> value
}
```

### 4. JSONCodec

`JSONCodec[K, V]` is a common implementation of `StateCodec` that uses JSON serialization for values:

```go
type JSONCodec[K comparable, V any] struct {
    FieldFunc func(K) (string, error)      // Key conversion function
    ParseFunc func(string) (K, error)      // String parsing function
}
```

## Usage Guide

### Basic Usage

#### 1. Creating a Mirror

```go
import (
    "github.com/redis/go-redis/v9"
    "your-project/common/statemirror"
)

// Create Redis client
client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

// Create Mirror (with default configuration)
mirror := statemirror.NewRedisMirror(client)

// Or with custom configuration
mirror := statemirror.NewRedisMirror(client,
    statemirror.WithRedisKeyPrefix("myapp:"),
    statemirror.WithScanCount(500),
)
```

#### 2. Define Data Structure and Codec

```go
// Define value type
type ProcessorInfo struct {
    ProcessorID string    `json:"processor_id"`
    Version     int32     `json:"version"`
    ShardingIdx int32     `json:"sharding_idx"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// Create JSONCodec
codec := statemirror.JSONCodec[string, ProcessorInfo]{
    FieldFunc: func(k string) (string, error) {
        // Key conversion: add prefix or perform other transformations
        return fmt.Sprintf("processor:%s", k), nil
    },
    ParseFunc: func(s string) (string, error) {
        // Parse: remove prefix
        return strings.TrimPrefix(s, "processor:"), nil
    },
}
```

#### 3. Using TypedMirror

```go
// Create TypedMirror
typedMirror := statemirror.TypedMirror[string, ProcessorInfo]{
    m:     mirror,
    key:   statemirror.MappingProcessorAllocations, // On-chain mapping key
    codec: codec,
}

ctx := context.Background()

// Read single value
info, exists, err := typedMirror.Get(ctx, "processor-1")
if err != nil {
    log.Fatal(err)
}
if exists {
    fmt.Printf("Found: %+v\n", info)
}

// Read multiple values
infos, err := typedMirror.MGet(ctx, "processor-1", "processor-2", "processor-3")
if err != nil {
    log.Fatal(err)
}
for k, v := range infos {
    fmt.Printf("%s: %+v\n", k, v)
}

// Read all values
allInfos, err := typedMirror.GetAll(ctx, statemirror.MappingProcessorAllocations)
if err != nil {
    log.Fatal(err)
}

// Scan matching values
cursor := uint64(0)
for {
    nextCursor, kvs, err := typedMirror.Scan(ctx, cursor, "processor:*", 100)
    if err != nil {
        log.Fatal(err)
    }
    
    for k, v := range kvs {
        fmt.Printf("%s: %+v\n", k, v)
    }
    
    if nextCursor == 0 {
        break
    }
    cursor = nextCursor
}
```

### Advanced Usage

#### 1. Synchronizing State (Upsert)

Use the `BuildSyncFunc` helper to build sync functions:

```go
// Define data fetch function
fetchFunc := func(ctx context.Context, key statemirror.OnChainKey) (map[string]ProcessorInfo, error) {
    // Fetch data from database or API
    data := map[string]ProcessorInfo{
        "processor-1": {ProcessorID: "p1", Version: 1},
        "processor-2": {ProcessorID: "p2", Version: 2},
    }
    return data, nil
}

// Build SyncFunc
syncFunc := statemirror.BuildSyncFunc(codec, fetchFunc)

// Execute synchronization (automatically calculates differences and updates)
err := mirror.Upsert(ctx, statemirror.MappingProcessorAllocations, syncFunc)
if err != nil {
    log.Fatal(err)
}
```

#### 2. Streaming Synchronization (UpsertStreaming)

For large datasets, avoids loading all data into memory at once:

```go
// Define streaming data emission function
streamFunc := func(ctx context.Context, key statemirror.OnChainKey, emit func(string, ProcessorInfo) error) error {
    // Emit data one by one
    for i := 0; i < 10000; i++ {
        info := ProcessorInfo{
            ProcessorID: fmt.Sprintf("p%d", i),
            Version:     int32(i),
        }
        if err := emit(fmt.Sprintf("processor-%d", i), info); err != nil {
            return err
        }
    }
    return nil
}

// Build StreamingSyncFunc
streamingSyncFunc := statemirror.BuildStreamingSyncFunc(codec, streamFunc)

// Execute streaming synchronization
err := mirror.UpsertStreaming(ctx, statemirror.MappingProcessorAllocations, streamingSyncFunc)
if err != nil {
    log.Fatal(err)
}
```

#### 3. Incremental Updates (Apply)

Only update changed fields, no need for full synchronization:

```go
// Define diff calculation function
diffFunc := func(ctx context.Context, key statemirror.OnChainKey) (*statemirror.TypedDiff[string, ProcessorInfo], error) {
    return &statemirror.TypedDiff[string, ProcessorInfo]{
        Added: map[string]ProcessorInfo{
            "processor-1": {ProcessorID: "p1", Version: 2}, // Update
            "processor-4": {ProcessorID: "p4", Version: 1}, // Add
        },
        Deleted: []string{"processor-3"}, // Delete
    }, nil
}

// Build DiffFunc
diffFuncBuilt := statemirror.BuildDiffFunc(codec, diffFunc)

// Apply differences
err := mirror.Apply(ctx, statemirror.MappingProcessorAllocations, diffFuncBuilt)
if err != nil {
    log.Fatal(err)
}
```

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "strings"
    "time"

    "github.com/redis/go-redis/v9"
    "your-project/common/statemirror"
)

type UserProfile struct {
    UserID    string    `json:"user_id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

func main() {
    // 1. Create Redis client
    client := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    defer client.Close()

    // 2. Create Mirror
    mirror := statemirror.NewRedisMirror(client)

    // 3. Define Codec
    codec := statemirror.JSONCodec[string, UserProfile]{
        FieldFunc: func(userID string) (string, error) {
            return "user:" + userID, nil
        },
        ParseFunc: func(field string) (string, error) {
            return strings.TrimPrefix(field, "user:"), nil
        },
    }

    // 4. Create TypedMirror
    typedMirror := statemirror.TypedMirror[string, UserProfile]{
        m:     mirror,
        key:   "UserProfiles", // OnChainKey
        codec: codec,
    }

    ctx := context.Background()

    // 5. Synchronize data
    syncFunc := statemirror.BuildSyncFunc(codec, func(ctx context.Context, key statemirror.OnChainKey) (map[string]UserProfile, error) {
        return map[string]UserProfile{
            "user1": {UserID: "user1", Name: "Alice", Email: "alice@example.com", CreatedAt: time.Now()},
            "user2": {UserID: "user2", Name: "Bob", Email: "bob@example.com", CreatedAt: time.Now()},
        }, nil
    })

    if err := mirror.Upsert(ctx, "UserProfiles", syncFunc); err != nil {
        log.Fatal(err)
    }

    // 6. Read data
    profile, exists, err := typedMirror.Get(ctx, "user1")
    if err != nil {
        log.Fatal(err)
    }
    if exists {
        fmt.Printf("Profile: %+v\n", profile)
    }

    // 7. Read multiple users
    profiles, err := typedMirror.MGet(ctx, "user1", "user2")
    if err != nil {
        log.Fatal(err)
    }
    for userID, profile := range profiles {
        fmt.Printf("%s: %+v\n", userID, profile)
    }
}
```

## Best Practices

1. **Choose the right synchronization method**:
   - Small datasets (< 1000 items): Use `Upsert`
   - Large datasets (> 10000 items): Use `UpsertStreaming`
   - Incremental changes only: Use `Apply`

2. **Codec Design**:
   - `FieldFunc` should generate unique field names
   - Consider adding prefixes to avoid field name conflicts
   - Keep `FieldFunc` and `ParseFunc` symmetric

3. **Error Handling**:
   - All methods can return errors, always check them
   - Encoding/decoding errors are returned directly, ensure data format compatibility

4. **Performance Optimization**:
   - Use `MGet` for batch reads to reduce network round trips
   - Use `Scan` for paginated processing of large datasets
   - Adjust `WithScanCount` based on data size for optimal performance

5. **Key Naming**:
   - Use meaningful `OnChainKey` constants
   - Manage them centrally in `on_chain_mapping_constants.go`

## API Reference

### Mirror Interface

- `Upsert(ctx, key, syncF)` - Full synchronization
- `UpsertStreaming(ctx, key, syncF)` - Streaming synchronization
- `Apply(ctx, key, diffF)` - Incremental update
- `Get(ctx, key, field)` - Read single field
- `MGet(ctx, key, fields...)` - Batch read
- `GetAll(ctx, key)` - Read all fields
- `Scan(ctx, key, cursor, match, count)` - Cursor-based scan

### TypedMirror Methods

- `Get(ctx, k)` - Read single value (type-safe)
- `MGet(ctx, ks...)` - Batch read (type-safe)
- `GetAll(ctx, key)` - Read all values (type-safe)
- `Scan(ctx, cursor, match, count)` - Cursor-based scan (type-safe)

### Helper Functions

- `BuildSyncFunc[K, V](codec, fetch)` - Build sync function
- `BuildStreamingSyncFunc[K, V](codec, stream)` - Build streaming sync function
- `BuildDiffFunc[K, V](codec, typed)` - Build diff function

## FAQ

**Q: How to handle complex key types (like structs)?**

A: Serialize structs to strings in `FieldFunc`, and deserialize in `ParseFunc`:

```go
type CompositeKey struct {
    ChainID   int64
    Address   string
}

codec := statemirror.JSONCodec[CompositeKey, Value]{
    FieldFunc: func(k CompositeKey) (string, error) {
        return fmt.Sprintf("%d:%s", k.ChainID, k.Address), nil
    },
    ParseFunc: func(s string) (CompositeKey, error) {
        parts := strings.SplitN(s, ":", 2)
        chainID, _ := strconv.ParseInt(parts[0], 10, 64)
        return CompositeKey{ChainID: chainID, Address: parts[1]}, nil
    },
}
```

**Q: How to customize serialization format (not using JSON)?**

A: Implement your own `StateCodec` interface:

```go
type MyCodec struct{}

func (c MyCodec) Field(k string) (string, error) { /* ... */ }
func (c MyCodec) ParseField(field string) (string, error) { /* ... */ }
func (c MyCodec) Encode(v MyType) (string, error) { /* custom serialization */ }
func (c MyCodec) Decode(s string) (MyType, error) { /* custom deserialization */ }
```

**Q: What's the difference between Upsert and Apply?**

A: 
- `Upsert`: Provide the complete target state, system automatically calculates differences and updates (including removing extra fields)
- `Apply`: Only provide the changes (fields to add/update and delete), doesn't affect other fields


