# Automatic Range Table Creation Against a Distributed Facade

## Summary

When the chain writer connects to a compute endpoint, its writable range-store initialization can mistake an existing `Distributed` table for a missing table and attempt to create a MergeTree-family table with the same logical name.

For Zircuit, the affected logical table is `chain_db.` followed by the quoted table name `zircuit.range`, written in SQL as:

```sql
chain_db.`zircuit.range`
```

An existing facade normally causes the attempted CREATE to fail with a table-already-exists error. The initialization error then propagates to the launcher and prevents startup. If the facade is actually missing, the same automatic creation path may instead create a physical table on compute, subject to permissions and DDL execution conditions.

This document describes the checked local source and the intended Chain0 topology. It does not establish which application image or schema is currently deployed. No code fix or production change is included.

## Intended topology

The migration separates query routing from physical storage:

| Role | Range table | Engine / purpose |
| --- | --- | --- |
| Each of the two compute instances | `chain_db`.`zircuit.range` | Distributed facade |
| Both replicas of storage shard 0 | `chain_db`.`zircuit.range_local` | ReplicatedMergeTree, physical range records |

The facade routes to the `clickhouse-storage-chain-0-control` cluster, which contains only the two replicas of storage shard 0. It does not distribute range records across all four data shards.

The intended facade engine is:

```sql
Distributed(
    'clickhouse-storage-chain-0-control',
    'chain_db',
    'zircuit.range_local',
    rand()
)
```

The `Distributed` engine routes operations to the configured remote tables. It is not the physical MergeTree table that the current range-store initializer expects to own.

## Startup path

The launcher constructs a ClickHouse controller with the configured database and table-name prefix. With database `chain_db`, prefix `zircuit.`, and range table name `range`, the controller addresses ``chain_db.`zircuit.range` ``.

The relevant initialization path is:

```text
launcher.BuildClickhouseRangeStore
  -> clickhouse.NewRangeStore(..., readOnly=false)
     -> build expected MergeTree-family range schema
     -> ctrl.LoadOne(ctx, "range", false)
        -> Controller.Load / load
           -> query system.tables
           -> buildEngineFromString(engineWithArgs)
     -> if has: ctrl.Sync(...)
        otherwise: ctrl.Create(...)
```

The schema-management block in `NewRangeStore` runs only when `readOnly` is false. The startup issue described here concerns the writable path.

## Why an existing Distributed table becomes "missing"

There are two separate behaviors involved.

### 1. The engine parser rejects Distributed

`common/chx/table_engine.go`, function `buildEngineFromString`, recognizes several MergeTree-family engines and `Memory`. It has no `Distributed` case. The default branch returns an error:

```go
return nil, errors.Errorf("unknown engine %q", str)
```

The parser does not report successful parsing with a nil engine.

### 2. The metadata loader skips the table and loses that error

In `common/chx/operator.go`, the row callback contains:

```go
table.Config.Engine, err = buildEngineFromString(engineWithArgs)
if err != nil {
    logger.Warnfe(err, "build engine of table %s failed, will be ignored", rawName)
    return nil
}
```

This returns nil from the callback before the table is added to the result map. `Controller.Query` treats that callback return as success and can finish with nil. The surrounding `err = c.Query(...)` assignment then overwrites the captured parser error with nil.

Assuming the remaining metadata queries also succeed, `LoadOne` receives an empty result and returns:

```go
nil, false, nil
```

Consequently, this caller-side error check does not stop initialization:

```go
if err != nil {
    return nil, errors.Wrapf(err, "load table %s failed", tableName)
}
```

The caller instead follows `has == false` and attempts CREATE.

## Result of the CREATE attempt

`NewRangeStore` builds its expected schema using `NewDefaultMergeTreeEngine`. Depending on the controller's cluster configuration, this is a MergeTree or ReplicatedMergeTree engine, not a Distributed engine.

`Controller.buildCreateTableSQL` generates `CREATE TABLE` without `IF NOT EXISTS`.

| Situation | Expected consequence |
| --- | --- |
| Correct facade already exists | Same-name CREATE normally fails; the existing table is not automatically replaced. |
| Facade is missing and creation is permitted | A physical MergeTree-family table may be created on compute under the logical range name. |
| Creation is denied | Initialization fails with the returned ClickHouse error. |
| DDL uses ON CLUSTER and execution is partial | A returned error does not prove that no target instance changed; inspect each target. |

`Controller.Create` returns its execution error. `NewRangeStore` returns that error, and the launcher wrapper calls `panic(c.BuildErr(err))`. Under a Kubernetes restart policy, repeated startup failure may lead to repeated restarts.

The application does not send a compensating DROP or replace the existing facade in this path. Ignoring the error or adding `IF NOT EXISTS` would not establish that the expected topology is correct.

## Why a parser-only fix is insufficient

Recognizing Distributed would address metadata parsing, but the range-store initializer would still construct an expected physical-table schema and enter `ctrl.Sync` for an existing table. It also starts physical partition-retention maintenance after successful writable initialization.

The data-table initializer in `chain/clickhouse/simple_slot_store.go` uses the same LoadOne/Create/Sync pattern. The migration therefore requires a deliberate distinction between externally managed facades and application-managed physical tables.

Other operations must also respect that distinction: partition maintenance, startup cleanup, reorg cleanup, repair, and replica synchronization need explicit physical storage targets where required. Merely changing the DSN does not provide that routing.

## Recommended behavior — proposed, not implemented here

1. Distinguish a genuinely missing table from an existing unsupported or invalid table. Metadata parsing failures must not silently become absence.
2. Add an explicit way to validate externally managed Distributed facades without creating or synchronizing physical schemas on compute.
3. Validate facade columns, destination cluster, database, remote table, and sharding expression on both compute instances.
4. In this mode, fail startup if a facade is missing or invalid; do not automatically create a physical replacement on compute.
5. Route physical maintenance to the correct storage local tables and replica set. Range operations must use the control shard rather than the four-shard data cluster.
6. Keep existing direct-storage behavior available for deployments that intentionally use application-managed MergeTree tables.

These are implementation requirements, not configuration keys that already exist.

## Range takeover is a separate requirement

Fixing automatic DDL does not make an empty range safe for a pre-copied target. `chain/chain/dimension_simple.go`, function `Init`, uses a cleanup start of zero when the current range is empty; with a nonempty range it uses `End + 1`.

Before starting the migrated writer, establish a validated takeover boundary and a tested range-publication protocol. Do not insert an arbitrary range merely to bypass initialization: the recorded range must correspond to verified continuous data coverage.

## Verification required for a future fix

- An existing valid facade is recognized without CREATE or schema ALTER on compute.
- Unsupported-engine and metadata-read errors remain visible to the caller.
- A missing or incorrectly routed facade causes a clear startup failure in externally managed mode.
- Both compute instances are checked independently.
- Existing direct-storage initialization still behaves as intended.
- Partition retention and other physical maintenance target storage local tables.
- A pre-copied target with an empty or unverified range cannot trigger destructive initialization.
- Restart, replica lag, partial distributed writes, and unknown write outcomes are exercised before production takeover.

## Source references

Paths below are relative to the `sentio-core` repository root unless otherwise stated:

- `common/chx/table_engine.go`: `buildEngineFromString`, default MergeTree engine selection.
- `common/chx/operator.go`: `load`, `LoadOne`, `buildCreateTableSQL`, `Create`, and `Sync`.
- `common/chx/query.go`: `Controller.Query` and callback error handling.
- `chain/clickhouse/range_store.go`: `NewRangeStore` and partition retention.
- `chain/clickhouse/simple_slot_store.go`: data-table initialization and maintenance.
- `chain/chain/dimension_simple.go`: range-dependent startup cleanup.
- Sibling repository `sentio/launcher/chain.go`: `BuildClickhouseRangeStore` and launcher panic propagation.
- Sibling repository `production/k8s-sea/clickhouse-chain-0/zircuit-precopy/zircuit_precopy.py`: `facade_ddl` and `expected_objects`.

Evidence is based on static source inspection. No runtime reproduction or production schema verification was performed for this document.
