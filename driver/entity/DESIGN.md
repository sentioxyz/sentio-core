# driver/entity Design Document

## Overview

`driver/entity` is the core entity persistence and query layer in the Sentio platform.
It uses ClickHouse as the storage backend and provides:

- Dynamic table creation and schema migration driven by a GraphQL schema
- Multi-chain data isolation within a single ClickHouse cluster
- Reorg-safe writes and rollbacks keyed by block number
- Three-tier in-memory read cache (full-data / full-ID-set / LRU)
- "Time-travel" queries by block number and transactional write semantics
- Extended entity types: TimeSeries, Sparse, Cache, Aggregation

---

## Directory Structure

```
driver/entity/
├── DESIGN.md            # this document
├── schema/              # GraphQL schema parsing and entity type definitions
├── persistent/          # Storage-agnostic interface layer and controller
│   ├── controller.go    # ChainStore interface + Controller (read/write/commit)
│   ├── monitor.go       # Monitor interface + MetricsMonitor / ReportMonitor / emptyMonitor
│   ├── box.go           # EntityBox (read/committed) + UncommittedEntityBox (write)
│   ├── box_rich_struct.go  # FromRichStruct / FromEntityUpdateData helpers
│   ├── change.go        # changeSet / changeHistory (per-block uncommitted state)
│   ├── filter.go        # EntityFilter definitions and in-memory evaluation
│   ├── operator.go      # Numeric atomic field operators (NumCalc)
│   └── stat.go          # Per-commit time-window statistics
└── clickhouse/          # ClickHouse storage implementation
    ├── store.go         # Store: multi-chain ClickHouse backend (no per-chain cache)
    ├── chain_store.go   # ChainStore: chain-bound wrapper with 3-tier cache
    ├── entity.go        # getEntity / setEntities / reorg / growthAggregation
    ├── entity_list.go   # listEntities / countEntity / getAllID / getMaxID
    ├── create.go        # InitEntitySchema: create/alter tables and views
    ├── schema.go        # Field scanning and type-build helpers
    └── check_value.go   # CheckValue: pre-write data validation
```

---

## Core Concepts

### Entity Types

Entity types are declared via directives in the GraphQL schema:

| Type | Directive | Characteristics |
|------|-----------|-----------------|
| **Regular** | `@entity` | Default; supports CRUD; historical versions keyed by blockNumber |
| **Sparse** | `@entity(sparse: true)` | Small data set; can be fully loaded into the full-data cache |
| **Immutable** | `@entity(immutable: true)` | Insert only; Update and Delete are rejected |
| **TimeSeries** | `@entity(timeseries: true)` | Insert only; auto-incrementing IDs; required `timestamp` field; implies Immutable |
| **Cache** | `@cache(sizeMB: N)` | In-memory only; nothing is written to ClickHouse; cleared on restart |
| **Aggregation** | `@aggregation` | Derived from TimeSeries entities via time-windowed INSERT-SELECT |

### EntityBox — Committed Entity Carrier

`EntityBox` represents the committed, read-side view of an entity:

```go
type EntityBox struct {
    Entity         string         // entity type name
    ID             string
    Data           map[string]any // nil means deleted
    GenBlockNumber uint64         // block number that produced this version
    GenBlockTime   time.Time
    GenBlockHash   string
}
```

- `Data == nil` indicates a deletion.
- `Copy()` performs a deep copy so callers can modify the result without affecting the cache.
- The chain ID is not stored here; it is implicit in the `ChainStore` that produced the box.

### UncommittedEntityBox — Write-Path Entity Carrier

`UncommittedEntityBox` is used exclusively on the write path (passed to `Controller.SetEntity`):

```go
type UncommittedEntityBox struct {
    EntityBox                       // embedded committed view
    Operator map[string]Operator    // optional atomic numeric operations
}
```

- `Operator` enables increment-style writes on numeric fields without a prior read.
- Callers construct `UncommittedEntityBox` from structured data via `FromRichStruct`
  (upsert/full-replace) or `FromEntityUpdateData` (partial update with operators).

---

## Layer Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  Caller (sentio/driver)                      │
│  startup/entity.go    processor_indexer.go                   │
│                                                              │
│  1. Call clickhouse.Store.InitEntitySchema(ctx) once         │
│  2. Create clickhouse.NewChainStore(store, chainID, ...)     │
│  3. Create persistent.NewController(chainStore, monitor)     │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  persistent.Controller                       │
│  Maintains an uncommitted changeSet indexed by block number  │
│  GetEntity / GetEntityInBlock / ListEntity / ListRelated     │
│  SetEntity (write to changeSet)                              │
│  Commit (flush changeSet to ChainStore)                      │
│  Reorg (discard future changes + delegate to ChainStore)     │
│  Reports operations via Monitor interface                    │
└────────────────────────┬────────────────────────────────────┘
                         │ implements persistent.ChainStore
                         ▼
┌─────────────────────────────────────────────────────────────┐
│               clickhouse.ChainStore                         │
│  Chain-bound wrapper; implements persistent.ChainStore       │
│  Three-tier read cache:                                      │
│    fullCache     — complete entity data for sparse entities  │
│    fullIDCache   — full set of known IDs                     │
│    lruCache      — individual entity LRU (bounded count)     │
│    cacheEntity   — in-memory-only @cache entity storage      │
│  Cache is invalidated/updated on SetEntities and Reorg       │
└────────────────────────┬────────────────────────────────────┘
                         │ holds *Store
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  clickhouse.Store                            │
│  Multi-chain ClickHouse backend; no per-chain cache          │
│  All methods accept a chain string argument internally       │
│  InitEntitySchema — one-time schema setup (NOT on ChainStore)│
│  getEntity / listEntities / setEntities                      │
│  countEntity / getAllID / getMaxID                           │
│  reorg / growthAggregation                                   │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
                    ClickHouse DB
```

### Design Principles

**`InitEntitySchema` lives only on `clickhouse.Store`.** It is a one-time setup operation
that creates or alters ClickHouse tables to match the current entity schema.  It must be
called once before any `ChainStore` is created, not once per chain.  It is intentionally
excluded from the `persistent.ChainStore` interface so that chain-bound code cannot depend
on it.

**`clickhouse.Store` is multi-chain, `clickhouse.ChainStore` is chain-bound.**
`clickhouse.Store` contains no per-chain cache; all caching lives in `clickhouse.ChainStore`.
Multiple `ChainStore` instances can share a single `Store` (and thus a single ClickHouse
connection pool and schema registry).

**`persistent.Controller` is the single transactional facade.**
The previous design split responsibility across `CachedStore`, `Controller`, and `Txn`.
The current design collapses these three into `Controller`, which holds the `ChainStore`
and the `Monitor` and is the only object callers need to keep for a processing cycle.

---

## persistent.ChainStore Interface

```go
type ChainStore interface {
    GetChain() string
    GetEntityType(entity string) *schema.Entity
    GetEntityOrInterfaceType(name string) schema.EntityOrInterface

    // GetEntity returns the entity with the given id.
    // fromCache == true when the result was served entirely from in-memory cache.
    // Returns nil if the entity does not exist or has been deleted.
    GetEntity(ctx context.Context, entityType *schema.Entity, id string) (*EntityBox, bool, error)

    // ListEntities returns entities matching the given filters.
    // fromCache == true when all results were served entirely from in-memory cache.
    ListEntities(ctx context.Context, entityType *schema.Entity,
        filters []EntityFilter, limit int) ([]*EntityBox, bool, error)

    GetTimeSeriesEntityMaxID(ctx context.Context, entityType *schema.Entity) (int64, error)

    // SetEntities writes a batch of entity boxes to persistent storage.
    // Ordering contract: within boxes, entries sharing the same ID are ordered
    // by ascending GenBlockNumber.
    // TimeSeries ID contract: all IDs exceed the value from GetTimeSeriesEntityMaxID.
    SetEntities(ctx context.Context, entityType *schema.Entity, boxes []EntityBox) (int, error)

    GrowthAggregation(ctx context.Context, curBlockTime time.Time) error
    Reorg(ctx context.Context, blockNumber int64) error
    CheckValue(entityType *schema.Entity, data map[string]any) error

    // Snapshot returns cache state for debugging/monitoring.
    Snapshot() any
}
```

All methods are chain-bound: no `chain string` parameter appears in the interface.

---

## Monitor Interface

`Monitor` is the observer through which `Controller` reports key operations to its caller.
Pass `nil` to `NewController` to use the built-in no-op monitor.

```go
type Monitor interface {
    OnGet(ctx, entity, id string, blockNumber uint64, inBlock bool, from string, used time.Duration)
    OnList(ctx, entity string, blockNumber uint64, loadRelated bool, from string,
        resultLen, resultPersistentLen int, used time.Duration)
    OnSet(ctx, entity, id string, blockNumber uint64, remove, hasOperator bool, used time.Duration)
    OnCommit(ctx, blockNumber uint64, created, updated map[string]int, used time.Duration)
}
```

Three built-in implementations:

| Type | Purpose |
|------|---------|
| `MetricsMonitor` | Records operation latency via an OpenTelemetry `Float64Histogram` |
| `ReportMonitor` | Accumulates per-cycle statistics and logs a summary on each `OnCommit`; also delegates to `MetricsMonitor` |
| `emptyMonitor` | No-op (used when `nil` is passed to `NewController`) |

**`ReportMonitor` lifecycle**: call `Reset()` before each processing cycle to clear
accumulated stats and record the cycle start time.  `OnCommit` finalises the cycle report
but does not reset state.

---

## Three-Tier Read Cache (clickhouse.ChainStore)

For non-`@cache` entities, `ChainStore` attempts the following lookup sequence:

```
GetEntity(entityType, id)
  │
  ├─ @cache entity? → look up cacheEntity LRU (weight-limited by sizeMB)
  │
  ├─ 1. Try full-data cache (fullCache) — only for sparse entities
  │     ├─ fullCacheRefused or !IsSparse → skip
  │     ├─ fullCacheLoaded → hit: return fullCache[entity][id].Copy()
  │     └─ not loaded → countEntity() to size-check
  │           ├─ count > fullCacheDataLimit/dataSize → refuse, set fullCacheRefused
  │           └─ within limit → listEntities() and populate fullCache
  │
  ├─ 2. Full-ID cache (fullIDCache) — loaded via getAllID() on first miss
  │     ├─ ID not in set → return nil (no DB roundtrip)
  │     └─ ID in set → try lruCache
  │           ├─ lruCache hit → return copy
  │           └─ lruCache miss → getEntity() from DB, add to lruCache
  │
  └─ fromCache == true only when no DB I/O was performed
```

**Cache invalidation:**
- `SetEntities`: updates fullCache / fullIDCache / lruCache in-place after a successful write.
- `Reorg`: calls `purgeCache()` to reset all cache structures; also removes `@cache` entity
  entries whose `GenBlockNumber` exceeds the reorg target.
- After `SetEntities`, if `fullCache` grows beyond `fullCacheDataLimit/dataSize`, it is
  discarded and `fullCacheRefused` is set — subsequent reads use the LRU + fullIDCache path.

**`@cache` entities** are stored entirely in a per-entity weight-limited LRU (`cacheEntity`).
`SetEntities` for these types is a no-op at the ClickHouse level.

---

## Operator Mechanism (Atomic Numeric Operations)

`SetEntity` supports passing an `Operator` instead of a concrete value for numeric fields,
enabling increment-style writes without a read-before-write:

```go
type Operator struct {
    NumCalc *OperatorNumCalc  // newValue = preValue * Multi + Add
}
```

**Execution**: operators are resolved during `Commit` → `executeAllEntityOperator`.
For the first write to an ID in a block, the pre-value is fetched from the `ChainStore`;
subsequent writes within the same block use the previously resolved value.

**Merging**: two operators on the same (entity, id, blockNumber) triplet are merged
mathematically inside `changeHistory.Push`:

```
(x * m1 + a1) * m2 + a2 = x * (m1 * m2) + (a1 * m2 + a2)
```

A data write following an operator write applies the operator to produce a concrete value,
which is then stored as plain data (Operator is set to nil).

---

## ClickHouse Table Design

### Built-in Meta-fields

Every entity table contains the following reserved columns in addition to user-defined fields:

| Column | ClickHouse Type | Description |
|--------|-----------------|-------------|
| `__genBlockNumber__` | `UInt64` | Block number that produced this row |
| `__genBlockTime__` | `Int64` / `DateTime64` | Block timestamp |
| `__genBlockHash__` | `String` | Block hash |
| `__genBlockChain__` | `String` | Chain ID (primary key component for multi-chain isolation) |
| `__deleted__` | `Bool` | Logical delete marker |
| `__timestamp__` | `DateTime64` | Wall-clock write time (not block time) |
| `__sign__` | `Int8` | VersionedCollapsing only (+1 / -1) |
| `__version__` | `UInt64` | VersionedCollapsing only |

### Table Engines

| Condition | Engine | Notes |
|-----------|--------|-------|
| Regular entity | `ReplacingMergeTree` | Sorted by `(chain, id, blockNumber)`; latest version via `argMax` |
| Immutable entity | `MergeTree` | No `__deleted__`; INSERT only |
| VersionedCollapsing | `VersionedCollapsingMergeTree` | Writes sign=-1 old-value row + sign=+1 new-value row; `HAVING SUM(sign) > 0` selects live rows |
| TimeSeries entity | `MergeTree` | No update; IDs are auto-incremented |
| Aggregation entity | `SummingMergeTree` (or similar) | Populated by periodic `INSERT-SELECT` from the TimeSeries source |

VersionedCollapsing entities additionally create:
- `versionedLatestEntity_{Name}`: a `VersionedCollapsingMergeTree` holding the latest version.
- `versionedLatestEntityMV_{Name}`: a materialized view that keeps the latest table in sync.

### Field Type Mapping

| Schema Type | ClickHouse Type (default) | Notes |
|-------------|--------------------------|-------|
| `ID` / `String` / `Bytes` | `String` | Bytes stored as hex |
| `Boolean` | `Bool` | |
| `Int` | `Int32` | |
| `Int8` | `Int64` | |
| `Timestamp` | `Int64` (µs) or `DateTime64(6)` | Controlled by `Features.TimestampUseDateTime64` |
| `Float` | `Float64` | |
| `BigInt` | `Tuple(Bool,Int8,UInt256)` or `Int256` | Controlled by `Features.BigIntUseInt256` |
| `BigDecimal` | `Decimal256(30)` / `String` / `Decimal512(60)` | Controlled by `Features` flags |
| `Enum` | `Enum(...)` | |
| `[T]` / `[T!]` | `String` (JSON) or `Array(T)` | Controlled by `Features.ArrayUseNativeType` |

`Features` (encoded in `schemaVersion`) selects the concrete type variant; see
`clickhouse/store.go` for details.

---

## Write Flow (Commit)

```
Controller.Commit(ctx, blockNumber, blockTime)
  │
  ├─ executeAllEntityOperator()
  │     Resolves all Operator fields; fetches pre-values from ChainStore as needed.
  │
  ├─ changes.Split(blockNumber)
  │     Splits out entries with blockNumber > commit target; they remain in the Controller
  │     for the next commit cycle.
  │
  ├─ For each entity type with pending changes:
  │   ├─ TimeSeries: GetTimeSeriesEntityMaxID() → assign real IDs to "@"-prefixed entries
  │   ├─ ChainStore.SetEntities(ctx, entityType, boxes)
  │   │     ├─ Batch by BatchInsertSizeLimit
  │   │     ├─ Check existing IDs (skipped when fullCache/fullIDCache is warm)
  │   │     ├─ VersionedCollapsing: write sign=-1 old row + sign=+1 new row
  │   │     └─ Update in-memory caches
  │   └─ Accumulate created / updated counts
  │
  ├─ ChainStore.GrowthAggregation()
  │     Triggers INSERT-SELECT to populate aggregation entity time-windows.
  │
  └─ monitor.OnCommit(ctx, blockNumber, created, updated, elapsed)
```

---

## Read Flow (GetEntity / ListEntity)

```
Controller.GetEntity(ctx, entityType, id, blockNumber)
  │
  ├─ 1. Check changes[entity][id].Latest(blockNumber)
  │       If found and has Operator → execute operator (may need ChainStore for pre-value)
  │       Return result.
  │
  └─ 2. ChainStore.GetEntity(ctx, entityType, id)
            └─ see Three-Tier Read Cache above
```

```
Controller.ListEntity(ctx, entityType, filters, cursor, limit, blockNumber)
  │
  ├─ Collect uncommitted entries from changes[entity] that satisfy filters and cursor.
  │
  ├─ Append "id NOT IN <already-seen IDs>" to filters.
  │
  └─ ChainStore.ListEntities(ctx, entityType, filters+notIn, limit)
            └─ fullCache path → in-memory filter; DB path → SQL WHERE clause
```

---

## Reorg Flow

```
Controller.Reorg(ctx, blockNumberGT)
  │
  ├─ changes.Split(blockNumberGT)
  │     Discards uncommitted entries with blockNumber > blockNumberGT.
  │
  └─ ChainStore.Reorg(ctx, blockNumberGT)
        ├─ purgeCache()
        │     Resets fullCache / fullIDCache / lruCache.
        ├─ Trim cacheEntity LRUs
        │     Remove entries where GenBlockNumber > blockNumberGT.
        └─ clickhouse.Store.reorg(ctx, blockNumberGT, chain)
              ├─ Regular entities: DELETE WHERE __genBlockNumber__ > blockNumberGT
              └─ VersionedCollapsing entities:
                    ├─ DELETE rows with blockNumber > blockNumberGT from both tables
                    └─ Rebuild versionedLatestEntity to repair any collapsed rows
```

---

## Filter System

### EntityFilter

```go
type EntityFilter struct {
    Field *types.FieldDefinition
    Op    EntityFilterOp   // Eq, Ne, Gt, Ge, Lt, Le, In, NotIn, Like, NotLike, HasAll, HasAny
    Value []any
}
```

Supported operators: `=`, `!=`, `>`, `>=`, `<`, `<=`, `IN`, `NOT IN`, `LIKE`, `NOT LIKE`,
`HAS_ALL` (array contains all), `HAS_ANY` (array contains any).

### Dual-path Evaluation

- **In-memory** (`CheckFilters`): used when `fullCache` is warm; no SQL generated.
- **SQL** (`buildCondition`): used when querying ClickHouse directly; converted to WHERE clauses.
  - Oversized `IN` sets (> `HugeIDSetSize`) use a temporary in-memory table to avoid
    parameter length limits.

### NULL Semantics

| Expression | Result |
|------------|--------|
| `field = null` | `field IS NULL` |
| `field != null` | `field IS NOT NULL` (note: `null != null` → false) |
| `field > null` / `< null` | always false |
| `field IN [a, null]` | `field = a OR field IS NULL` |
| `LIKE null` / `NOT LIKE null` | always false |
| `HAS_ALL []` / `HAS_ALL null` | always true |
| `HAS_ANY []` / `HAS_ANY null` | always false |

---

## TimeSeries Entities and Aggregation

### TimeSeries (`@entity(timeseries: true)`)

- Every write creates a new row; IDs auto-increment (or may be explicitly supplied).
- The required `timestamp` field records block time in microseconds.
- Update and Delete are not supported.
- At `Commit` time: `GetTimeSeriesEntityMaxID()` retrieves the current maximum numeric ID,
  and `@`-prefixed auto-IDs are assigned sequentially starting from `maxID + 1`.

### Aggregation (`@aggregation`)

- Derives from a TimeSeries source; declares dimension fields (dim) and aggregate fields (agg).
- Supported aggregation functions: `sum`, `min`, `max`, `count`, `first` (earliest block), `last` (latest block).
- Supports multiple time intervals (e.g. `1h`, `1d`).
- `GrowthAggregation()` fires after each `Commit`; it appends new time-window rows via
  an incremental `INSERT-SELECT` from the source TimeSeries table.

---

## Configuration

| Parameter | Source | Description |
|-----------|--------|-------------|
| `lruCapacity` | `clickhouse.NewChainStore` | Number of entity entries in the LRU cache |
| `fullCacheDataSizeLimit` | `clickhouse.NewChainStore` | Max total bytes kept in the full-data cache; exceeding this falls back to LRU + fullIDCache |
| `schemaVersion` / `Features` | `clickhouse.BuildFeatures` | Controls ClickHouse type variants for BigDecimal, BigInt, Timestamp, and Array fields |
| `BatchInsertSizeLimit` | `clickhouse.TableOption` | Max entities per INSERT batch (default: 1000) |
| `HugeIDSetSize` | `clickhouse.TableOption` | Switch to temp-table for IN filters larger than this (default: 1000) |
| `SENTIO_ENABLE_CLICKHOUSE_LIGHT_DELETE` | env | Use ClickHouse Light Delete for reorg (default: true) |
| `SENTIO_VERSIONED_COLLAPSING_INSERT_QUORUM` | env | Write quorum for VersionedCollapsing tables (default: 1) |
| `SENTIO_DEFAULT_ENTITY_BATCH_INSERT_SIZE` | env | Default batch insert size (default: 1000) |
| `SENTIO_DEFAULT_ENTITY_HUGE_ID_SET_SIZE` | env | Default huge-ID-set threshold (default: 1000) |
