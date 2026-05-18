# registry Package

The `registry` package provides type-safe access to network state mirrored from on-chain data via Redis/file state mirror. It offers three focused registry interfaces backed by `statemirror.Mirror`.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Registry                              │
│  ┌──────────────┐  ┌──────────────────┐  ┌──────────────┐  │
│  │  DbRegistry  │  │ ProcessorRegistry│  │IndexerRegistry│  │
│  └──────┬───────┘  └────────┬─────────┘  └──────┬───────┘  │
└─────────┼──────────────────┼───────────────────┼───────────┘
          │                  │                   │
          └──────────────────┼───────────────────┘
                             │
                    ┌────────▼────────┐
                    │ statemirror.Mirror│
                    └─────────────────┘
```

## Interfaces

### DbRegistry

Manages database metadata and access permissions.

```go
type DbRegistry interface {
    RetrieveDatabaseInfo(ctx context.Context, database Database) (state.DatabaseInfo, error)
    RetrievePermissionsByAccount(ctx context.Context, address Address) (map[Database]DbAuth, error)
    AccountHasPermission(ctx context.Context, address Address, database Database, action Action) (bool, error)
    RetrieveAllDatabaseInfos(ctx context.Context) (map[Database]state.DatabaseInfo, error)
}
```

**Usage:**
```go
reg := NewDbRegistry(mirror)
info, err := reg.RetrieveDatabaseInfo(ctx, "my_database")
hasRead, err := reg.AccountHasPermission(ctx, "0x123...", "my_database", Read)
```

### ProcessorRegistry

Accesses processor allocations and metadata.

```go
type ProcessorRegistry interface {
    RetrieveProcessorInfo(ctx context.Context, processorId ProcessorId) (state.ProcessorInfo, error)
    RetrieveProcessorAllocations(ctx context.Context, processorId ProcessorId) ([]state.ProcessorAllocation, error)
}
```

**Usage:**
```go
reg := NewProcessorRegistry(mirror)
info, err := reg.RetrieveProcessorInfo(ctx, "processor_123")
allocations, err := reg.RetrieveProcessorAllocations(ctx, "processor_123")
```

### IndexerRegistry

Accesses indexer configuration and metadata.

```go
type IndexerRegistry interface {
    RetrieveIndexerInfo(ctx context.Context, indexerId IndexerId) (state.IndexerInfo, error)
    RetrieveAllIndexers(ctx context.Context) (map[IndexerId]state.IndexerInfo, error)
}
```

**Usage:**
```go
reg := NewIndexerRegistry(mirror)
info, err := reg.RetrieveIndexerInfo(ctx, 42)
allIndexers, err := reg.RetrieveAllIndexers(ctx)
```

## Permission System

The registry implements a hierarchical permission system:

| Level | Bit | Implies |
|-------|-----|---------|
| Owner | 8   | Admin, Write, Read |
| Write | 2   | Read |
| Admin | 4   | (nothing) |
| Read  | 1   | (nothing) |

**Key behavior:** `Admin` alone does NOT imply `Write` or `Read`. Only `Owner` grants all permissions.

### Wildcard Permissions

The wildcard address (`0x0000000000000000000000000000000000000000`) defines public permissions. All accounts inherit these permissions unioned with their specific grants.

## Type Definitions

```go
type Database string        // Database identifier
type Address string         // Ethereum address (will be lowercased)
type ProcessorId string     // Processor identifier
type IndexerId uint64       // Indexer numeric ID

type DbAuth int64           // Permission bitmap
type Action int64           // Requested action (Read=1, Write=2)
```

## State Mirror Integration

All registries are backed by `statemirror.Mirror`, which keeps Redis/File state in sync with on-chain data. Each registry uses `TypedMirror[K,V]` for type-safe access:

| Registry | Mapping | Key Type | Value Type |
|----------|---------|----------|------------|
| DbRegistry | MappingDatabases | string | DatabaseInfo |
| DbRegistry | MappingDatabasePermissions | string | map[string]string |
| ProcessorRegistry | MappingProcessorAllocations | string | []ProcessorAllocation |
| ProcessorRegistry | MappingProcessorInfos | string | ProcessorInfo |
| IndexerRegistry | MappingIndexerInfos | string | IndexerInfo |

## Error Handling

All methods return errors for:
- Uninitialized mirror (nil mirror passed to constructor)
- Missing entries (not found returns error, not (nil, false))
- Mirror read failures

## Migration Notes

This package replaces the monolithic `processor_registry.go`. Key changes:

| Old | New | Notes |
|-----|-----|-------|
| `RetrieveProcessorAllocation` | `RetrieveProcessorAllocations` | Plural name, type-safe ID |
| `RetrieveShardingByProcessor` | *removed* | Use allocations + indexer lookup |

## Testing

Run tests with:
```bash
bazel test //network/registry/...
```
