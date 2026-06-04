# Solana Super-Node: BigQuery as a Third Data Source — Field Mapping & Gap Handling

> Status: implemented in `chain/sol/bq` and the super-node (`chain/sol/supernode`).
> Goal: add BigQuery as the **lowest-priority** data source for the Solana super-node.

---

## 1. Background & Goal

The Solana super-node currently answers `sol_*` data requests from two sources, in priority order:

1. **latest-slot cache** — the newest slots, in memory, fastest.
2. **ClickHouse** — synced history, bounded by the `rangeStore` interval `[minSlot, maxSlot]`.

ClickHouse does **not** hold Solana data back to genesis (everything below `rangeStore.Get()`'s lower bound `minSlot` is empty). For **slots below `minSlot` that neither the cache nor ClickHouse cover**, we fall back to the **BigQuery public dataset** to reconstruct the response:

- Dataset: `bigquery-public-data.crypto_solana_mainnet_us`
  - `.Blocks`, `.Transactions`, `.Instructions`
- BigQuery is the **lowest-priority** tier, used only when the two higher tiers do not cover the slot.
- Auth: the BigQuery client uses Application Default Credentials (`GOOGLE_APPLICATION_CREDENTIALS`), provided by the deployment environment.

### Two high-risk uncertainties — verified against BigQuery

> Verified by querying the public dataset directly.

1. **The `Transactions` table has no `version` column and no `loadedAddresses`** (full schema confirmed). BigQuery does not distinguish legacy vs v0; `accounts` is the only account list. **Decision**: always default `version` to `legacy(-1)` (see §5.1).
2. **`accounts` is the fully-resolved account set.** Verified on a 44-account v0 transaction (`4FTMt8f6…`, Jupiter v6): `ARRAY_LENGTH(accounts)=44`, `ARRAY_LENGTH(balance_changes)=44`, max token `account_index` = 29 (< 44). Address-lookup-table accounts are **already expanded into `accounts`**, and `balance_changes` and the token `account_index` all align to this same list. Conclusion: as long as the reconstruction uses BigQuery's `accounts` order as `accountKeys`, preBalances/postBalances/tokenBalances are internally consistent, with no need for `loadedAddresses` (§5.3).
3. **Failed transactions**: `status` is `"Success"` / `"Fail"`; `err` is a human-readable string (e.g. `"Error processing Instruction 1: custom program error: 0x14"`), not the RPC structured object. **Decision**: no semantic parsing — embed `err` as `json.RawMessage` when it is valid JSON, otherwise as the raw string (see §5.10).
4. **Cost constraints (verified metadata)**: `Transactions` is ~888 TB and `Instructions` ~890 TB, both **partitioned by `block_timestamp` (DAY) with `requirePartitionFilter=True`** (a query must carry a `block_timestamp` predicate or it errors); `Transactions` is clustered by `signature`, `Instructions` by `program_id`. `Blocks` is ~71 GB, MONTH-partitioned, with no required partition filter (see §5).

### Relevant code (verified)

| Role | File | Key symbols |
|------|------|-------------|
| Data-source selection core | `sentio-core/chain/chain/util.go` | `QueryRangeWithCache`, `CheckRange`, `CheckRangeWithFallback` |
| Super-node RPC | `sentio-core/chain/sol/supernode/rpc.go` | `NewSuperNode`, `RPCService`, `GetBlock`, `GetBlocksByInterval`, `FindTransactions`, `GetContractStartBlock`, `previousUnskippedBlock` |
| Storage interface | `sentio-core/chain/sol/supernode/storage.go` | `Storage` (5 methods) |
| ClickHouse impl (**reference template**) | `sentio-core/chain/sol/ch/data.go` | `ClickhouseBlock.toBlock`, `ClickhouseTransaction.toWrappedTransaction` |
| Domain types | `sentio-core/chain/sol/types.go`, `slot.go` | `Block`, `WrappedTransaction`, `BlockTransactions`, `ParsedTransactionWithMeta` |
| Target RPC types | `gagliardetto/solana-go@v1.20.0/rpc/types.go`, `getBlock.go` | `GetBlockResult`, `ParsedTransaction`, `ParsedTransactionMeta`, `ParsedInstruction`, `ParsedInnerInstruction`, `ParsedMessageAccount`, `TokenBalance`, `UiTokenAmount` |
| BigQuery impl | `sentio-core/chain/sol/bq/` | `Store` (`store.go`), conversions (`convert.go`), stats (`stat.go`) |

---

## 2. Key design decision: build the JSON shape and Unmarshal, not field-by-field structs

The ClickHouse path does **not** construct the RPC objects field by field. It stores the node's `transaction` / `meta` verbatim as JSON strings (`transaction_json` / `meta_json`, ZSTD-compressed) and unmarshals them on read:

```go
// sentio-core/chain/sol/ch/data.go  toWrappedTransaction()
var transaction *rpc.ParsedTransaction
json.Unmarshal([]byte(ct.TransactionJSON), &transaction)
var meta *rpc.ParsedTransactionMeta
json.Unmarshal([]byte(ct.MetaJSON), &meta)
```

**The BigQuery path therefore uses the same strategy**: from the three tables' rows, build the **exact same JSON shape** as RPC `getTransaction(jsonParsed)` (i.e. the structure of `result.transaction` and `result.meta` in `raw_tx.json`), then `json.Unmarshal` into `*rpc.ParsedTransaction` / `*rpc.ParsedTransactionMeta`. Rationale:

1. **Reuse** the existing, tested types and (de)serialization, producing objects identical to the ClickHouse path.
2. **Sidestep the `InstructionInfoEnvelope` dead end**: the type of `ParsedInstruction.Parsed`, `InstructionInfoEnvelope` (`rpc/types.go`), has only **unexported** fields (`asString` / `asInstructionInfo`) and **no exported constructor** — only `MarshalJSON` / `UnmarshalJSON` (`rpc/getParsedTransaction.go`). A parsed instruction therefore **cannot be constructed directly** from another package; it can only be produced by unmarshalling JSON. This makes "build JSON, then Unmarshal" the only clean path.

The resulting domain objects are identical to the ClickHouse path:

- `sol.Block` (embeds `*rpc.GetBlockResult`)
- `sol.WrappedTransaction` (`*rpc.ParsedTransaction` + `*rpc.ParsedTransactionMeta` + `Version`)
- `sol.BlockTransactions` (block header + `[]WrappedTransaction`)

> Note: several RPC fields (e.g. `accountKeys[].source`, `meta.costUnits`) **do not exist in solana-go's Go types**, so they are dropped on the ClickHouse round-trip too. The gap section below marks which missing fields need **no handling**.

Per-instruction JSON round-trip lives in `convert.go` (`buildInstruction`); everything else (account keys, balances, token balances, inner-instruction grouping) is built as typed structs directly.

---

## 3. Field mapping (BigQuery rows → RPC JSON shape)

Based on sample transaction `3WeJDhD1…WsGyywv` (slot `422822279`), the three tables map to `raw_tx.json`.

### 3.1 `Blocks` → `sol.Block` / `rpc.GetBlockResult`

`sol_getBlock` fills only the header (no signatures); `sol_getBlocksByInterval` additionally fills `Signatures` (the first signature of each transaction, taken from `Transactions` ordered by `index`).

| BQ `Blocks` column | Target field (JSON key) | Notes |
|---|---|---|
| `slot` | `sol.Block.Slot` + query key | string→uint64 |
| `block_hash` | `GetBlockResult.blockhash` | base58 |
| `previous_block_hash` | `GetBlockResult.previousBlockhash` | base58 |
| `block_timestamp` | `GetBlockResult.blockTime` | `"2026-05-28 23:42:14 UTC"` → unix seconds `1780011734` |
| `height` | `GetBlockResult.blockHeight` | string→uint64 (pointer) |
| `transaction_count` | — (no target; can validate signature count) | |
| `leader` / `leader_reward` | → (optional) `GetBlockResult.rewards` (see §5.8) | per-tx meta is usually empty |
| — (no such concept in BQ) | `GetBlockResult.parentSlot` | **missing**; not populated (see §5.9) |

### 3.2 `Transactions` → `sol.WrappedTransaction` + `rpc.ParsedTransactionMeta` + `message`

| BQ `Transactions` column | Target field | Notes |
|---|---|---|
| `signature` | `transaction.signatures[0]` / `WrappedTransaction.Signature` | base58 |
| `index` | `WrappedTransaction.TransactionIndex` | in-block index |
| `recent_block_hash` | `transaction.message.recentBlockhash` | |
| `accounts[]` (`pubkey/signer/writable`) | `transaction.message.accountKeys[]` (`ParsedMessageAccount`: `pubkey/signer/writable`) | **order is the account index** — must be preserved |
| `fee` | `meta.fee` | |
| `compute_units_consumed` | `meta.computeUnitsConsumed` | pointer |
| `status` / `err` | `meta.err` + `meta.status` | `Success`/`""` → `err=null`, `status={"Ok":null}`; failures see §5.10 |
| `log_messages[]` | `meta.logMessages` | verbatim |
| `balance_changes[]` (`account/before/after`) | `meta.preBalances[]` / `meta.postBalances[]` | **projected onto accountKeys order**, see §5.4 |
| `pre_token_balances[]` | `meta.preTokenBalances[]` (`TokenBalance`) | see §3.4 |
| `post_token_balances[]` | `meta.postTokenBalances[]` | see §3.4 |
| — (no such column in BQ) | `WrappedTransaction.Version` | **missing**; defaults to legacy(-1), see §5.1 |
| `block_slot/block_hash/block_timestamp` | join keys + `GetParsedTransactionResult.slot/blockTime` | |

### 3.3 `Instructions` → `message.instructions` + `meta.innerInstructions`

Split into levels by `parent_index`, ordered within each level by `index`:

- `parent_index = null` → **top-level instruction**, goes into `transaction.message.instructions`, ascending by `index`.
- `parent_index = N` → **inner instruction**, grouped by `parent_index` into `meta.innerInstructions[].instructions`, with `ParsedInnerInstruction.index = N` and ascending by `index` within the group.

> Verified on the sample: top-level `index` 0,1,2,3,4,5,6 matches `raw_tx` `message.instructions` order; inner groups `parent_index=2` (4 instructions) and `parent_index=3` (7 instructions) match `raw_tx` `innerInstructions` entries `index:2` / `index:3` exactly.

Single instruction (`rpc.ParsedInstruction`) mapping:

| BQ `Instructions` column | Target field | Rule |
|---|---|---|
| `program_id` | `programId` | always present |
| `program` (non-null) | `program` | **discriminator**: `program != null` ⇒ parsed instruction |
| `instruction_type` | `parsed.type` (`InstructionInfo.InstructionType`) | |
| `params[]` (`key`/`value`) | `parsed.info` (`InstructionInfo.Info` map) | `info[key] = JSON.parse(value)`, see below |
| `data` | `data` (base58) | used only for **unparsed** instructions (`program == null`) |
| `accounts[]` | `accounts` ([]pubkey) | used only for **unparsed** instructions; for parsed ones this is `[""]` and is ignored |
| — (derived) | `stackHeight` | top-level=1; inner defaults to 2; deeper CPI needs log reconstruction, see §5.6 |

**`params.value` parsing rule** (each value is a JSON-encoded string):

| BQ `value` literal | After `JSON.parse`, written to `info[key]` |
|---|---|
| `"\"6h3x…\""` | string `"6h3x…"` |
| `"165"` | number `165` |
| `"[\"immutableOwner\"]"` | array `["immutableOwner"]` |
| `"{\"amount\":\"3014690\",\"decimals\":9,…}"` | object `{amount, decimals, uiAmount, uiAmountString}` |
| `key=null, value=null` (placeholder) | skip; this is an unparsed instruction (no `parsed`) |

### 3.4 Token balances: `pre/post_token_balances` → `rpc.TokenBalance`

| BQ column | `TokenBalance` field | Notes |
|---|---|---|
| `account_index` | `accountIndex` | |
| `mint` | `mint` | |
| `owner` | `owner` (*pubkey) | |
| `amount` + `decimals` | `uiTokenAmount` (`UiTokenAmount`) | `amount` kept as string; `decimals`; **`uiAmount`/`uiAmountString` are computed**: `uiAmount = amount / 10^decimals` (when `amount==0`, `uiAmount=null`, `uiAmountString="0"`, matching `raw_tx`) |
| — (not in BQ) | `programId` (*pubkey) | **missing**; see §5.5, left nil |

---

## 4. Gap analysis & handling

| # | Gap | Impact | Handling |
|---|---|---|---|
| 5.1 | **`version`** (legacy / v0); `Transactions` **has no such column** | `WrappedTransaction.Version` flows into the parsed transaction result (`ToParsedTransactionResult`) and thus into the serialized `version` field | **Decision: always default to `LegacyTransactionVersion = -1`** (solana-go `transaction_version.go`, serializes to `"legacy"`). Instructions/accounts/balances do not depend on it; for a v0 tx this only makes the `version` field read `"legacy"` (accepted fidelity gap). **No** account-count heuristic. |
| 5.2 | `accountKeys[].source` (transaction / lookupTable) | none | **No handling needed**: `rpc.ParsedMessageAccount` has no `source` field, so the ClickHouse path drops it too. |
| 5.3 | `meta.loadedAddresses` (v0 LUT-loaded addresses) | `ParsedTransactionMeta.LoadedAddresses` exists but BQ does not provide it directly | **Verified safe to leave empty**: BQ `accounts[]` is already the fully-resolved list including LUT accounts (verified on a 44-account v0 tx: `n_acct==n_bal==44`, token `account_index` within range). Using BQ `accounts` order as `accountKeys` keeps preBalances/postBalances/token `accountIndex` internally consistent; leaving `loadedAddresses` empty does not affect the parsed reconstruction. **Note**: the **absolute** account order was not cross-checked against RPC (the sandbox slots are in the future and public RPCs lack that history), but in parsed mode instructions reference accounts by pubkey, so only internal consistency matters. |
| 5.4 | `preBalances/postBalances` order | must match `accountKeys` order, or balances are misaligned | Build an `account→{before,after}` map from BQ `balance_changes[]`, then project it onto the two arrays **in `accountKeys` order**. (In the sample they happen to share order, but the implementation must not rely on that.) |
| 5.5 | token balance `programId` | distinguishes Token vs Token-2022 | **Decision: v1 leaves it nil** (`*pubkey`, `omitempty`, matching the ClickHouse path when the field is absent). Enhancement (deferred): scan the transaction's parsed token instructions to build a `mint→tokenProgram` map and backfill — a mint belongs to a single token program, derivable from `program_id`. |
| 5.6 | deeper-CPI `stackHeight` (>2) | BQ `parent_index` is one level only; deeper nesting depth is **unrecoverable from the `Instructions` table alone** | **Decision: v1 uses top-level=1, all inner=2.** Instruction content/order/grouping are all correct (verified); only nesting depth is approximate. Enhancement (deferred, log-dependent): align `log_messages` `Program <id> invoke [n]` in execution order and assign the real `stackHeight` from `[n]` — `Instructions` alone cannot do this; the logs are the only source. **Recorded as a fidelity downgrade.** |
| 5.7 | `meta.returnData` | a few consumers need it | **Decision: v1 leaves it zero.** `rpc.ParsedTransactionMeta.ReturnData` exists but BQ has no structured field; most consumers don't read it. Enhancement (deferred): parse the **last** `Program return: <programId> <base64>` from `log_messages` into `{programId, data}`. |
| 5.8 | `meta.rewards` | usually empty per-tx | Set to `[]` (`raw_tx` is empty). Block-level `leader_reward` is unrelated to per-tx meta. |
| 5.9 | `GetBlockResult.parentSlot` | used by the `sol_getBlock` header | **Not populated.** BQ `Blocks` has no parent-slot column; deriving it (`MAX(slot)` below the block) costs a full-column scan of `Blocks` per block, which is too expensive for a value downstream consumers of the archival tier do not need. Left zero (documented in `toBlock`). |
| 5.10 | `err` / `status` for failures | failed transactions must be represented correctly | **Decision (verified)**: BQ `status ∈ {"Success","Fail"}`; `err` is a string (e.g. `"Error processing Instruction 1: custom program error: 0x14"`). **No semantic parsing.** Mapping:<br>• `status="Success"` ⇒ `meta.err = null`, `meta.status = {"Ok": null}`.<br>• `status="Fail"`: take the BQ `err` string `s` — if `s` is valid JSON (`json.Valid([]byte(s))`), embed it as `json.RawMessage`; otherwise use it as a plain string. The same value is used for both `meta.err` and `meta.status = {"Err": <value>}`. (`rpc.ParsedTransactionMeta.Err` is `any`, so both forms work.) |
| 5.11 | `meta.costUnits` | none | **No handling needed**: `rpc.ParsedTransactionMeta` has no such field; the ClickHouse path drops it too. |
| 5.12 | **skipped slots** | BQ only stores produced blocks; there is no dense "skipped" row (the ClickHouse `blocks` table has one row per slot with a `skipped` flag) | `QueryBlock` with no hit ⇒ treated as skipped (returns `sol.Block{Slot}` with `GetBlockResult=nil`). `previousUnskipped` / `getBlocksByInterval` window logic is expressed in terms of "blocks that actually exist in `Blocks`", see §5 queries. |

---

## 5. Per-`Storage`-method BigQuery query design

The `Storage` interface has 5 methods (`supernode/storage.go`); the BigQuery implementation covers each:

1. **`QueryBlock(slot)`** → `SELECT … FROM Blocks WHERE slot=@slot LIMIT 1`; empty ⇒ skipped (§5.12); `parentSlot` not populated (§5.9).
2. **`QueryBlocksByInterval(from,to,window,limit)`** → first block of each window. BQ has no skipped rows; window grouping (block window: `DIV(slot,W)`; time window: `DIV(UNIX_SECONDS(block_timestamp), W)`) takes the min slot per group. Signatures attached via a join on `Transactions`. **More complex; watch cost.**
3. **`QueryPreviousUnskipped(before)`** → `SELECT slot, block_timestamp FROM Blocks WHERE slot<@before ORDER BY slot DESC LIMIT 1`.
4. **`FindTransactions(from,to,programIDs,limit)`** → select `DISTINCT (block_slot, tx_signature)` from `Instructions` where `program_id IN @ids` and `block_slot BETWEEN`, then assemble from `Transactions` (by `signature`) + `Instructions` + `Blocks` (block header). **Returns only the queried programs' instructions, not the full instruction set — see the cost note below.**
5. **`EarliestProgramSlot(address)`** → `SELECT MIN(block_slot) FROM Instructions WHERE program_id=@addr AND <whole-history block_timestamp predicate>`. ⚠️ Because of `requirePartitionFilter=True`, an unbounded MIN is impossible; supply a dataset-wide lower bound (e.g. `block_timestamp >= '2020-03-01'`) and rely on `program_id` clustering to prune to that program. Popular programs can still be expensive, but this call is rare (once at processor start) and is bounded by `maxBytesBilled`.

### ⚠️ Cost-critical point (decided): resolve slot→timestamp via `Blocks` first

Measured: `Transactions`/`Instructions` are ~888/890 TB, partitioned by **`block_timestamp` (DAY) with `requirePartitionFilter=True`** — a query without a `block_timestamp` predicate errors outright; a `slot` filter cannot prune partitions. Queries are charged by bytes scanned against the configured billing project. The super-node input is a **slot**, so we must obtain the time first.

**Decided approach**:
1. **First hop: resolve time from `Blocks`.** `Blocks` is only ~71 GB, MONTH-partitioned, with no required filter, so a slot lookup is cheap:
   - single slot: `SELECT slot, block_timestamp, previous_block_hash, … FROM Blocks WHERE slot=@slot`;
   - slot range: `SELECT MIN(block_timestamp) lo, MAX(block_timestamp) hi FROM Blocks WHERE slot BETWEEN @from AND @to`.
   `Blocks` is needed anyway for `QueryBlock` / skipped detection (§5.12), and **incidentally yields the exact DAY partition bounds**.
   (`MIN/MAX BETWEEN` is also robust to skipped endpoints — `slot IN (@from,@to)` would miss data when an endpoint slot is skipped, and at the same scan cost since `Blocks` is not slot-clustered.)
2. **Second hop: query the heavy tables with the exact `block_timestamp` predicate.** Use the `[lo, hi]` from the first hop (padded by one DAY on each side) as the partition filter on `Transactions`/`Instructions`, narrowing the scan to the few DAY partitions involved.
3. **Set `maxBytesBilled` on every job** (configurable) as a circuit breaker; over-limit fails fast instead of burning money.
4. **Select only the needed columns** (BigQuery bills by column): assembling a transaction needs the `Transactions` scalar columns + `accounts`/`*_token_balances`/`balance_changes`/`log_messages`, plus the `Instructions` columns.

> Alternative (not the primary path): extrapolate a time window from Solana's ~400ms/slot anchored at `rangeStore.minSlot`, skipping the first hop. It requires a wide window for the error margin and risks anchor drift; consider only if the `Blocks` first hop becomes a bottleneck.

### ⚠️ Cost-critical point (decided): `FindTransactions` returns only the queried programs' instructions

Measured against a busy sample day, fetching a transaction's **full** instruction set (filter `tx_signature IN @sigs` on `Instructions`) scans **~1.34 TB** — because `Instructions` is clustered by `program_id`, not by signature/slot, so the per-transaction filter cannot prune and reads the whole DAY partition. (`Transactions`, clustered by `signature`, is fine: `signature IN @sigs` prunes to ~MBs. `QueryBlock` on `Blocks` scans ~10–50 GB, the whole table's referenced columns, since it is unclustered and the lookup is by slot.)

**Decision**: `FindTransactions` fetches instructions filtered by **`program_id IN @programIDs`** (the cluster key), so the scan is pruned to just those programs. The returned transactions therefore carry **only the queried programs' instructions** (top-level and inner), not the full set. This is acceptable because an instruction handler only inspects instructions of the program it targets, and all other transaction data (accounts, balances, token balances, logs, status/err, fee, compute units) is complete. See the `bq.Store.FindTransactions` doc comment, and the matching note where the driver builds the raw transaction for instruction handlers. Consumers needing the full instruction set of an arbitrary transaction must not use this store.

---

## 6. Go integration (as implemented)

### 6.1 BigQuery `Storage` implementation
- New package `sentio-core/chain/sol/bq`; `bq.Store` implements `supernode.Storage` (5 methods).
- Internally uses §2 ("build JSON → `json.Unmarshal`") to produce `sol.Block` / `sol.WrappedTransaction` / `sol.BlockTransactions`, isomorphic to `sol/ch` output.
- Depends on `cloud.google.com/go/bigquery` (added to `MODULE.bazel` + gazelle); auth via Application Default Credentials.
- Per-query statistics (latency / returned count / bytes billed) recorded in `stat.go`, mirroring `sol/ch.statistic`, exposed via `Snapshot()` for tracking.

### 6.2 Three-tier chaining
`QueryRangeWithCache(ctx, interval, slotCache, cachedProc, queryLoader)` takes **one** `queryLoader`, and `CheckRange` **errors** when the range exceeds `rangeStore`. To plug BigQuery in as the "below ClickHouse" fallback, the implemented approach (Option A — smallest change) keeps `QueryRangeWithCache` unchanged and introduces `chain.CheckRangeWithFallback`:

- The sub-range within `rangeStore` `[min,max]` is served by ClickHouse (the original `CheckRange` logic).
- The sub-range strictly **below** `min` is served by BigQuery.
- A request extending **above** `max` still **errors** (ClickHouse has not synced that far — the caller must retry; the fallback only extends coverage downward, never upward).
- With a nil fallback it degrades to `CheckRange`, preserving the original behavior.

Priority: cache (newest) > ClickHouse (within `rangeStore`) > BigQuery (archival history below `rangeStore.minSlot`).

### 6.3 `RPCService` / `NewSuperNode`
- `RPCService` has an optional `bqStore Storage` field (`nil` when not configured).
- `NewSuperNode(...)` takes a `bqStore` argument; each method's loader uses the `CheckRangeWithFallback` composite. `bqStore == nil` reverts to the original behavior (fully backward compatible).
- **Method wiring**:
  - **P0 (core backfill path)**: `FindTransactions` (backfill historical program activity — the main use), `QueryBlock` (block header for assembly; also serves as the §5 "first hop" slot→time resolution).
  - **P1 (with a semantic twist)**: `EarliestProgramSlot` / `GetContractStartBlock`. **The priority direction is reversed here**: this method asks for a program's *earliest* slot, and BigQuery covers exactly the older history below ClickHouse's `minSlot`, so **BigQuery may have an earlier answer**. Semantics: consult ClickHouse first; if it finds the program at exactly its lower bound (so the program might extend below the range), check BigQuery for an earlier slot; if ClickHouse doesn't find it at all, BigQuery (then the cache) is consulted. This is a dedicated branch, not the §6.2 composite loader, and is cost-aware (only queries BigQuery when the program may actually extend below the ClickHouse range).
  - **P2**: `QueryPreviousUnskipped` (chain time as of a slot), `QueryBlocksByInterval` (windowed block listing; window grouping over large partitions is the **most expensive**, bounded by `maxBytesBilled` and the slot-span cap).

### 6.4 Configuration (`bq.Config`)
`NewStore(ctx, bq.Config)` is constructed by the caller and passed to `NewSuperNode`; passing `nil` disables the tier. `bq.Config` fields:
- `ProjectID` — billing/job project (queries are billed here even though the dataset is public).
- `Dataset` — `"<project>.<dataset>"` qualifier; defaults to `bigquery-public-data.crypto_solana_mainnet_us`.
- `MaxBytesBilled` — per-query bytes-scanned cap (cost circuit breaker; 0 = unlimited).
- `BlocksTable` / `TransactionsTable` / `InstructionsTable` — table-name overrides (default `Blocks`/`Transactions`/`Instructions`).
- `PartitionPaddingDays` — widens the resolved `[lo, hi]` window at DAY boundaries (default 1).
- `HistoryStart` — dataset-wide lower `block_timestamp` bound for whole-history scans (`EarliestProgramSlot`); default 2020-03-01.

A construction error should degrade gracefully (disable the tier) rather than fail the super-node.

---

## 7. Decision log (all settled)

| # | Topic | Decision | Section |
|---|---|---|---|
| 1 | v0 / `version` | BQ has no `version`/`loadedAddresses`; `accounts` already includes LUT entries and is self-consistent with balance/token indices; `version` always defaults to `legacy(-1)`, no heuristic | §5.1/5.3 |
| 2 | failed-tx `err` | no semantic parsing; valid-JSON `err` ⇒ `json.RawMessage`, else raw string; `status` built as `{"Ok":null}`/`{"Err":…}` | §5.10 |
| 3 | `stackHeight` | v1 top-level=1, inner=2; deeper-CPI reconstruction (log-dependent) deferred | §5.6 |
| 4 | token `programId` | v1 nil; `mint→program` derivation deferred | §5.5 |
| 5 | `returnData` | v1 empty; parse from the last `Program return:` log deferred | §5.7 |
| 6 | `parentSlot` | not populated (per-block full-column scan too costly; unused by archival consumers) | §5.9 |
| 7 | slot→timestamp cost | resolve exact DAY bounds via `Blocks` (cheap) first, then add the partition predicate to the heavy tables; set `maxBytesBilled` per job | §5 "cost-critical point" |
| 8 | method wiring priority | P0: `FindTransactions`, `QueryBlock`; P1: `EarliestProgramSlot` (BigQuery has reverse-highest priority); P2: `QueryPreviousUnskipped`, `QueryBlocksByInterval` | §6.3 |
| 9 | `FindTransactions` instruction scope | return only the queried programs' instructions (filter `program_id`, the cluster key) instead of the full set; avoids the ~1.34 TB per-transaction day-partition scan. Tx-level data stays complete | §5 "returns only the queried programs' instructions" |

**Remaining runtime consideration (non-blocking; observe during rollout)**: BigQuery queries are seconds-scale and take the super-node's slow-request path. Evaluate the impact on driver timeouts/retries, and consider a dedicated timeout for the BigQuery tier and local caching of results (archival history is immutable, so the hit rate should be high).

---

## Appendix: validation sample

The mappings above were checked against one sample transaction: its `Blocks` / `Transactions` / `Instructions` rows from BigQuery versus the RPC `getTransaction(encoding=jsonParsed)` result for the same transaction (the **target shape** this design reconstructs). The same sample shapes are exercised by the unit tests in `convert_test.go`.
