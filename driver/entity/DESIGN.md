# driver/entity 设计文档

## 概览

`driver/entity` 是 Sentio 平台中负责实体（Entity）数据持久化与查询的核心驱动层。它以 ClickHouse 为存储后端，提供以下能力：

- GraphQL Schema 驱动的动态表结构创建与变更
- 面向区块链场景的多链（multi-chain）数据隔离
- 区块重组（Reorg）安全的写入与回滚
- 多层读缓存（内存全量缓存 / ID 集合缓存 / LRU 缓存）
- 基于区块号（blockNumber）的"时间旅行"查询与事务语义
- 时序实体（TimeSeries）、稀疏实体（Sparse）、缓存实体（Cache）等扩展实体类型

---

## 目录结构

```
driver/entity/
├── schema/          # Schema 解析与实体类型定义
├── persistent/      # 存储无关的接口层、控制器、事务
│   ├── persistent.go    # Store 接口（多链版）
│   ├── cache.go         # CachedStore：单链绑定的缓存封装
│   ├── controller.go    # Controller：事务读写控制器
│   ├── transaction.go   # Txn：事务报告 + 指标上报
│   ├── box.go           # EntityBox：实体数据载体
│   ├── filter.go        # 过滤器定义与内存过滤逻辑
│   └── operator.go      # 数值型字段的原子操作（NumCalc）
└── clickhouse/      # ClickHouse 存储实现
    ├── store.go         # Store：多链 ClickHouse 后端
    ├── entity.go        # GetEntity / SetEntities / Reorg / GrowthAggregation
    ├── entity_list.go   # ListEntities / CountEntity / GetAllID / GetMaxID
    ├── create.go        # InitEntitySchema：建表 / 建视图 / 字段映射
    ├── schema.go        # Entity 实体的字段扫描与构建辅助
    └── check_value.go   # CheckValue：写入前数据校验
```

---

## 核心概念

### 实体类型（Entity 分类）

实体类型通过 GraphQL Schema 中的指令（Directive）声明，共有以下几类：

| 分类 | 指令 | 特性 |
|------|------|------|
| **普通实体** | `@entity` | 默认类型，支持 CRUD，按 blockNumber 保存历史版本 |
| **稀疏实体** | `@entity(sparse: true)` | 数据量少，可全量加载到内存缓存 |
| **不可变实体** | `@entity(immutable: true)` | 只允许 Insert，不允许 Update/Delete |
| **时序实体** | `@entity(timeseries: true)` | 只允许 Insert，自动递增 ID，含 timestamp 字段；隐含 immutable |
| **缓存实体** | `@cache(sizeMB: N)` | 数据**仅存内存**，不写 ClickHouse；重启后清空 |
| **聚合实体** | `@aggregation` | 由时序实体按时间窗口聚合生成，定期以 INSERT-SELECT 写入 |

### EntityBox — 实体数据载体

```go
type EntityBox struct {
    ID             string
    Data           map[string]any          // 字段名 → 值（nil 表示删除）
    Operator       map[string]Operator     // 字段名 → 原子操作（可选）
    Entity         string                  // 实体类型名
    GenBlockNumber uint64                  // 产生该版本的区块号
    GenBlockTime   time.Time
    GenBlockHash   string
    GenBlockChain  string                  // 产生该版本的链 ID
}
```

- `Data == nil`：代表删除操作
- `Operator`：数值型字段的原子增量操作，见 [Operator 机制](#operator-机制)
- `Copy()`：深拷贝，拷贝后修改不影响原始数据（nil Operator 保持 nil）

---

## 层级架构

```
┌──────────────────────────────────────────────────────┐
│                  调用方（sentio/driver）               │
│  startup/entity.go   processor_indexer.go            │
└──────────────┬───────────────────────────────────────┘
               │ NewTxn()
               ▼
┌─────────────────────────────────────────┐
│            persistent.Txn               │
│  嵌入 Controller；收集操作指标报告       │
└──────────────┬──────────────────────────┘
               │ 持有
               ▼
┌─────────────────────────────────────────┐
│          persistent.Controller          │
│  uncommitted changeSet（区块号索引）    │
│  GetEntity / ListEntity / SetEntity     │
│  Commit / Reorg                         │
└──────────────┬──────────────────────────┘
               │ 持有
               ▼
┌─────────────────────────────────────────┐
│         persistent.CachedStore          │  ← 单链绑定
│  三层读缓存：                           │
│    fullCache（稀疏实体全量数据）         │
│    fullIDCache（全量 ID 集合）           │
│    lruCache（LRU 个体实体）             │
│    cacheEntity（Cache 实体内存存储）     │
└──────────────┬──────────────────────────┘
               │ 持有（多链接口）
               ▼
┌─────────────────────────────────────────┐
│          clickhouse.Store               │  ← 多链，无缓存
│  GetEntity / ListEntities / SetEntities │
│  CountEntity / GetAllID / GetMaxID      │
│  Reorg / GrowthAggregation             │
│  InitEntitySchema                       │
└─────────────────────────────────────────┘
               │
               ▼
          ClickHouse DB
```

### persistent.Store（多链接口）

```go
type Store interface {
    InitEntitySchema(ctx context.Context) error
    GetChain() string
    GetEntityType(entity string) *schema.Entity
    GetEntityOrInterfaceType(name string) schema.EntityOrInterface

    GetEntity(ctx, entityType, chain, id string) (*EntityBox, error)
    ListEntities(ctx, entityType, chain, filters, limit) ([]*EntityBox, error)
    SetEntities(ctx, entityType, boxes) (created int, error)
    GetAllID(ctx, entityType, chain) ([]string, error)
    GetMaxID(ctx, entityType, chain) (int64, error)
    CountEntity(ctx, entityType, chain) (uint64, error)
    GrowthAggregation(ctx, chain, curBlockTime) error
    Reorg(ctx, blockNumber, chain) error
    CheckValue(entityType, data) error
}
```

所有方法携带 `chain string` 参数，适合多链共享同一 ClickHouse 实例的场景。

### persistent.CachedStore（单链缓存封装）

`CachedStore` 将 `persistent.Store`（多链）包装为单链绑定视图，并在读路径上增加三层内存缓存：

```go
type CachedStore struct {
    store    Store        // 底层多链存储
    chain    string       // 绑定的链

    // 三层缓存
    cache        *simplelru.LRU[string, *EntityBox]        // LRU 缓存，key = "entityName/id"
    cacheEvicted int
    fullIDCache       map[string]map[string]bool            // 全量 ID 集合，key = entityName
    fullIDCacheLoaded map[string]bool
    fullCache        map[string]map[string]*EntityBox       // 全量数据，key = entityName
    fullCacheLoaded  map[string]bool
    fullCacheRefused map[string]bool                        // 超出大小限制，拒绝全量缓存

    // Cache 类型实体的专属存储（按内存大小限制的 LRU）
    cacheEntity map[string]*lru.Cache[string, *EntityBox]
}
```

提供 `NewTxn()` 方法用于创建绑定该链的事务对象。

### persistent.Controller（事务读写控制器）

Controller 是事务的核心，维护一个"已提交 → 未提交"的双层视图：

```go
type Controller struct {
    store     *CachedStore
    changes   changeSet  // map[entityName][id] → changeHistory（按区块号排序）
    committed *uint64
    timeStat  *timewin.TimeWindowsManager[*timeStatWindow]
    noticeCtl NoticeController
}
```

**读取语义**：对于给定 `blockNumber`，优先从 `changes` 中取该区块号及之前的最新版本，若无则从 `CachedStore`（进而 ClickHouse）读取。

**写入语义**：`SetEntity` 仅写入内存 `changes`，`Commit` 时批量写入 ClickHouse。

### persistent.Txn（事务 + 指标）

`Txn` 嵌入 `Controller` 并实现 `NoticeController` 接口：

```go
type Txn struct {
    start             time.Time
    storeCacheEvicted int
    report            TxnReport
    recordMetric      SimpleNoticeController
    *Controller
}
```

`TxnReport` 汇总整个事务周期内的所有 get/list/set/commit 操作统计，在 `NoticeCommit` 时打印到日志。

---

## 三层读缓存详解

`CachedStore` 为非 Cache 类型实体提供三层读缓存，命中策略按以下顺序尝试：

```
GetEntity(entityType, id)
  │
  ├─ 1. IsCache 实体？→ 直接查 cacheEntity（LRU，按内存大小限制）
  │
  ├─ 2. 尝试全量数据缓存（fullCache）
  │     ├─ 已拒绝（fullCacheRefused）或非稀疏（!IsSparse）→ 跳过
  │     ├─ 已加载（fullCacheLoaded）→ 命中，返回 fullCache[entity][id].Copy()
  │     └─ 未加载 → CountEntity() 检查数量
  │           ├─ 超出 fullCacheDataSizeLimit / dataSize → 拒绝，设 fullCacheRefused
  │           └─ 在限额内 → ListEntities() 全量加载到 fullCache
  │
  ├─ 3. 全量 ID 缓存（fullIDCache）
  │     ├─ 未加载 → GetAllID() 加载所有 ID
  │     ├─ ID 不存在 → 返回 nil（无需 DB 查询）
  │     └─ ID 存在 → 尝试 lruCache
  │           ├─ 命中 lruCache → 返回副本
  │           └─ 未命中 → GetEntity() 从 DB 读取，写入 lruCache
  │
  └─ fromCache 标志：true=全程命中缓存，false=产生了 DB 访问
```

**缓存失效**：
- `SetEntities` 成功后同步更新对应缓存层（fullCache / fullIDCache / lruCache）
- `Reorg` 和 `InitEntitySchema` 时清空所有缓存（`purgeCache()`）
- fullCache 写入后实体数超限 → 删除 fullCache，设置 fullCacheRefused

**Cache 实体（`@cache`）** 的存储完全在内存中，`SetEntities` 不写 ClickHouse，数据不持久化。

---

## Operator 机制（原子数值操作）

`SetEntity` 支持传入 `Operator` 而非直接覆盖值，用于对数值字段做增量操作，无需先读后写：

```go
type Operator struct {
    NumCalc *OperatorNumCalc  // newValue = preValue * Multi + Add
}
```

**执行时机**：`Commit` 前的 `executeAllEntityOperator` 阶段。对同一字段的多个 Operator 可以合并：

```
(x * m1 + a1) * m2 + a2 = x * (m1*m2) + (a1*m2 + a2)
```

**合并**：同一区块号、同一 ID 的两次 `SetEntity`，通过 `changeHistory.Push` → `EntityBox.Merge` 合并：
- 旧有数据（`===`）+ 新来 Operator（`+++`）→ 执行 Operator，字段变为数据（`===`）
- 旧有 Operator + 新来 Operator → 数学合并为一个 Operator

---

## ClickHouse 表设计

### 内置元字段

每张实体表都包含以下元字段（用户字段之外）：

| 字段名 | 类型 | 含义 |
|--------|------|------|
| `__genBlockNumber__` | UInt64 | 产生该行的区块号 |
| `__genBlockTime__` | Int64/DateTime64 | 产生该行的区块时间 |
| `__genBlockHash__` | String | 产生该行的区块哈希 |
| `__genBlockChain__` | String | 链 ID（多链隔离的主键之一） |
| `__deleted__` | Bool | 标记删除 |
| `__timestamp__` | DateTime64 | 行写入时间（非区块时间） |
| `__sign__` | Int8 | 仅 VersionedCollapsing 表使用（+1/-1） |
| `__version__` | UInt64 | 仅 VersionedCollapsing 表使用 |

### 表类型

根据实体特性，ClickHouse 侧会创建不同类型的表：

| 条件 | 主表引擎 | 特点 |
|------|----------|------|
| 普通实体 | `ReplacingMergeTree` | 以 `(chain, id, blockNumber)` 排序，通过 argMax 取最新版 |
| 不可变实体 | `MergeTree` | 无 `__deleted__`，无更新，直接 INSERT |
| VersionedCollapsing | `VersionedCollapsingMergeTree` | 写入时附带 sign=-1 的旧值行，HAVING SUM(sign)>0 取有效行 |
| 时序实体 | `MergeTree` | 不可更新，id 自动递增，含 timestamp |
| 聚合实体 | `SummingMergeTree`/等 | 按时间窗口聚合，定期 INSERT-SELECT 填充 |

VersionedCollapsing 实体还会额外创建：
- `versionedLatestEntity_{Name}`：VersionedCollapsingMergeTree，存最新版本
- `versionedLatestEntityMV_{Name}`：物化视图，从原始表同步到 latest 表

### 数据类型映射

| Schema 类型 | ClickHouse 类型（默认） | 备注 |
|-------------|------------------------|------|
| `ID` / `String` / `Bytes` | `String` | Bytes 存储 hex 字符串 |
| `Boolean` | `Bool` | |
| `Int` | `Int32` | |
| `Int8` | `Int64` | |
| `Timestamp` | `Int64`（微秒）或 `DateTime64(6)` | 由 schemaVersion bit 控制 |
| `Float` | `Float64` | |
| `BigInt` | `Tuple(Bool,Int8,UInt256)` 或 `Int256` | 由 schemaVersion bit 控制 |
| `BigDecimal` | `Decimal256(30)` / `String` / `Decimal512(60)` | 由 schemaVersion bit 控制 |
| `Enum` | `Enum(...)` | |
| `[T]` / `[T!]` | `String`（JSON）或 `Array(T)` | 由 schemaVersion bit 控制 |

`Features`（由 schemaVersion 编码）控制具体的类型选择，详见 `clickhouse/store.go`。

---

## 写入流程（Commit）

```
Controller.Commit(ctx, blockNumber, blockTime)
  │
  ├─ executeAllEntityOperator()   # 展开所有 Operator，计算出最终 Data
  │
  ├─ changes.Split(blockNumber)   # 分离出 >blockNumber 的变更（留待下次）
  │
  ├─ 对每种实体类型：
  │   ├─ 若时序实体：GetMaxID() 获取当前最大 ID，为 @ 前缀 ID 分配真实 ID
  │   ├─ SetEntities(ctx, entityType, entities)
  │   │   ├─ 按 batchSize 分批
  │   │   ├─ 检查新旧 ID（queryExistEntity / listEntities for versioned）
  │   │   ├─ 对 VersionedCollapsing：写入 sign=-1 旧值行 + sign=+1 新值行
  │   │   └─ 更新 CachedStore 缓存
  │   └─ 统计 created / updated
  │
  └─ GrowthAggregation()         # 驱动聚合实体 INSERT-SELECT
```

---

## 读取流程（GetEntity / ListEntity）

```
Controller.GetEntity(ctx, entityType, id, blockNumber)
  │
  ├─ 1. 查 changes[entity][id].Latest(blockNumber) → 若命中，展开 Operator 返回
  │
  └─ 2. 查 CachedStore.GetEntity(ctx, entityType, id)
        └─ 见"三层读缓存详解"
```

```
Controller.ListEntity(ctx, entityType, filters, cursor, limit, blockNumber)
  │
  ├─ 从 changes[entity] 中收集满足 filters 且未过 cursor 的 uncommitted 数据
  │
  ├─ 拼接 "id NOT IN <已检查 ID 集合>" 过滤条件
  │
  └─ CachedStore.ListEntities(ctx, entityType, filters+notIn, limit)
        └─ 见"三层读缓存详解"（命中 fullCache 则内存过滤）
```

---

## Reorg 流程

当区块发生重组时，需回滚到某个区块号之前的状态：

```
Controller.Reorg(ctx, blockNumberGT)
  │
  ├─ changes.Split(blockNumberGT) 丢弃区块号 > blockNumberGT 的未提交变更
  │
  └─ CachedStore.Reorg(ctx, blockNumberGT)
        ├─ purgeCache()           # 清空所有读缓存
        ├─ 修剪 cacheEntity LRU（移除 genBlockNumber > blockNumberGT 的 Cache 实体）
        └─ clickhouse.Store.Reorg(ctx, blockNumberGT, chain)
              ├─ 对普通实体：DELETE WHERE __genBlockNumber__ > blockNumberGT
              └─ 对 VersionedCollapsing 实体：
                    ├─ DELETE 原始表中 > blockNumberGT 的行
                    └─ 检查并重建 versionedLatestEntity（防止 collapsing 后数据丢失）
```

---

## 过滤器系统

### EntityFilter

```go
type EntityFilter struct {
    Field *types.FieldDefinition
    Op    EntityFilterOp       // Eq, Ne, Gt, Ge, Lt, Le, In, NotIn, Like, NotLike, HasAll, HasAny
    Value []any
    idSet map[string]bool      // 仅 id IN/NOT IN 使用，预构建的 id 集合
}
```

支持操作符：`=`、`!=`、`>`、`>=`、`<`、`<=`、`IN`、`NOT IN`、`LIKE`、`NOT LIKE`、`HAS_ALL`（数组包含全部）、`HAS_ANY`（数组包含任意）。

### 双路过滤

- **内存过滤**（`checkFilter`）：当 `CachedStore` 从 fullCache 提供数据时，在内存中执行过滤
- **SQL 过滤**（`buildCondition`）：当需要访问 ClickHouse 时，转换为 WHERE 条件
  - 超大 IN 集合（超过 `HugeIDSetSize`）时使用临时内存表避免 SQL 参数过长

### NULL 语义

过滤器对 NULL 值有明确语义（与 SQL 不同）：

- `field = null` → field IS NULL
- `field != null` → field IS NOT NULL（**注意**：null != null 返回 false）
- `field > null` / `< null` → 始终 false
- `field IN [a, null]` → field = a OR field IS NULL
- `LIKE null` / `NOT LIKE null` → 始终 false
- `HAS_ALL []` / `HAS_ALL null` → 始终 true
- `HAS_ANY []` / `HAS_ANY null` → 始终 false

---

## 时序实体与聚合

### 时序实体（`@entity(timeseries: true)`）

- 每次写入都是一条新记录，ID 自动递增（或手动指定）
- 含必填 `timestamp` 字段，记录区块时间（微秒）
- 不支持 Update / Delete
- `Commit` 时：`GetMaxID()` 获取当前最大 ID，为 `@` 前缀的自动 ID 分配真实递增 ID

### 聚合实体（`@aggregation`）

- 以时序实体为数据源，定义维度字段（dim）和聚合字段（agg）
- 支持聚合函数：`sum`、`min`、`max`、`count`、`first`（最早区块）、`last`（最新区块）
- 支持多个时间窗口（interval），如 `1h`、`1d`
- `GrowthAggregation()` 在每次 Commit 后触发，以 INSERT-SELECT 增量追加新时间窗口的数据

---

## 配置参数

| 参数 | 来源 | 含义 |
|------|------|------|
| `capacity`（lruCapacity） | `NewCachedStore` | LRU 缓存容量（实体个数） |
| `fullCacheDataSizeLimit` | `NewCachedStore` | 全量数据缓存的总大小上限（字节），超过则退化到 LRU + ID 缓存 |
| `schemaVersion` | `BuildFeatures` | 控制 BigDecimal/BigInt/Timestamp/Array 在 ClickHouse 中的存储类型 |
| `BatchInsertSizeLimit` | `TableOption` | 每批写入的实体数上限（默认 1000） |
| `HugeIDSetSize` | `TableOption` | IN 条件超过此数量时改用临时表（默认 1000） |
| `SENTIO_ENABLE_CLICKHOUSE_LIGHT_DELETE` | env | 是否使用 ClickHouse Light Delete（默认 true） |
| `SENTIO_VERSIONED_COLLAPSING_INSERT_QUORUM` | env | VersionedCollapsing 写入一致性级别（默认 1） |
