# 连接 Distributed 门面时自动创建 range 表的问题

## 概述

当链数据 writer 连接到 compute 端点时，可写 range store 的初始化流程可能把已经存在的 `Distributed` 表误判为不存在，进而尝试创建同名的 MergeTree 系列表。

以 Zircuit 为例，受影响的数据库是 `chain_db`，完整表名是包含点号的 `zircuit.range`。实际 SQL 中应写为：

```sql
chain_db.`zircuit.range`
```

如果正确的门面已经存在，同名 CREATE 通常会因“表已存在”而失败；错误继续向上传递，导致 launcher 启动失败。如果门面确实缺失，在权限和 DDL 执行条件允许的情况下，这条自动建表路径则可能在 compute 上创建实体表。

本文依据已检查的本地源码和 Chain0 目标拓扑编写，不代表当前线上镜像或实际表结构。本文仅说明问题和建议，不包含代码修复或生产变更。

## 目标拓扑

此次迁移将查询路由与物理存储分开：

| 角色 | range 表 | 引擎及用途 |
| --- | --- | --- |
| 两个 compute 实例，各自一张 | `chain_db` 数据库中的 `zircuit.range` | Distributed 门面 |
| storage 第 0 分片的两个副本，各自一张 | `chain_db` 数据库中的 `zircuit.range_local` | ReplicatedMergeTree，保存实际 range 记录 |

门面指向 `clickhouse-storage-chain-0-control`。这个集群仅包含 storage 第 0 分片的两个副本，不会把 range 记录分散到四个数据分片上。

目标门面使用以下引擎定义：

```sql
Distributed(
    'clickhouse-storage-chain-0-control',
    'chain_db',
    'zircuit.range_local',
    rand()
)
```

`Distributed` 负责向配置的远端表转发操作，而当前 range store 初始化逻辑预期管理的是物理 MergeTree 表，两者的职责不同。

## 启动调用链

launcher 根据配置的数据库和表名前缀构造 ClickHouse controller。当数据库为 `chain_db`、前缀为 `zircuit.`、range 表名为 `range` 时，最终访问的是：

```sql
chain_db.`zircuit.range`
```

相关初始化路径如下：

```text
launcher.BuildClickhouseRangeStore
  -> clickhouse.NewRangeStore(..., readOnly=false)
     -> 构造预期的 MergeTree 系列 range 表结构
     -> ctrl.LoadOne(ctx, "range", false)
        -> Controller.Load / load
           -> 查询 system.tables
           -> buildEngineFromString(engineWithArgs)
     -> has 为 true：ctrl.Sync(...)
        has 为 false：ctrl.Create(...)
```

`NewRangeStore` 中的结构管理逻辑仅在 `readOnly=false` 时执行。本文描述的是可写初始化路径。

## 已存在的 Distributed 表为什么会被判断为“不存在”

这里涉及两个独立行为。

### 1. 引擎解析器不支持 Distributed

`common/chx/table_engine.go` 中的 `buildEngineFromString` 支持若干 MergeTree 系列引擎和 `Memory`，但没有 `Distributed` 分支，最终进入默认分支：

```go
return nil, errors.Errorf("unknown engine %q", str)
```

因此，解析器返回的是错误，并不是“成功返回一个 nil 引擎”。

### 2. 元数据加载流程跳过该表，并丢失了解析错误

`common/chx/operator.go` 中，处理查询结果的回调包含：

```go
table.Config.Engine, err = buildEngineFromString(engineWithArgs)
if err != nil {
    logger.Warnfe(err, "build engine of table %s failed, will be ignored", rawName)
    return nil
}
```

回调在把表加入结果 map 之前返回 nil，导致这张表被跳过。`Controller.Query` 把回调返回 nil 视为成功，并可能最终返回 nil；外层的 `err = c.Query(...)` 又将捕获到的解析错误覆盖为 nil。

在其余元数据查询也正常完成的情况下，`LoadOne` 得到空结果，返回：

```go
nil, false, nil
```

因此，调用方的下面这段检查不会中止初始化：

```go
if err != nil {
    return nil, errors.Wrapf(err, "load table %s failed", tableName)
}
```

程序会继续按 `has == false` 进入 CREATE 分支。

## CREATE 之后会发生什么

`NewRangeStore` 通过 `NewDefaultMergeTreeEngine` 构造预期表结构。根据 controller 的集群配置，目标引擎是 MergeTree 或 ReplicatedMergeTree，而不是 Distributed。

`Controller.buildCreateTableSQL` 生成的语句是 `CREATE TABLE`，没有 `IF NOT EXISTS`。

| 情况 | 预期结果 |
| --- | --- |
| 正确的门面已经存在 | 同名 CREATE 通常失败，不会自动替换已有门面。 |
| 门面缺失且允许建表 | compute 上可能以逻辑 range 表名创建 MergeTree 系列实体表。 |
| 没有建表权限 | 初始化返回 ClickHouse 错误。 |
| DDL 带 ON CLUSTER 且部分执行成功 | 返回错误不能证明所有目标都未变化，需要逐个实例核查。 |

`Controller.Create` 返回执行错误，`NewRangeStore` 继续返回该错误，launcher 包装层调用 `panic(c.BuildErr(err))`。在 Kubernetes 重启策略下，持续的启动失败可能造成反复重启。

这条路径不会自动执行补偿性 DROP，也不会自动覆盖已有门面。忽略错误或添加 `IF NOT EXISTS`，都不能证明最终表结构和拓扑正确。

## 为什么只增加 Distributed 解析支持还不够

识别 Distributed 只能解决元数据解析问题。range store 初始化仍然会构造预期的实体表结构，并在发现已有表后进入 `ctrl.Sync`；可写初始化成功后，还会启动物理分区保留清理任务。

`chain/clickhouse/simple_slot_store.go` 中的数据表初始化也采用 LoadOne/Create/Sync 模式。因此，迁移需要明确区分“外部管理的门面”和“程序管理的实体表”。

分区维护、启动清理、reorg 清理、repair 和副本同步等操作，同样需要在适用时明确指向物理 storage 表。仅修改 DSN 不会自动完成这些路由适配。

## 建议行为：以下是待实现要求

1. 区分“表确实不存在”与“表存在但引擎不受支持或元数据无效”，不能把解析错误静默转换成不存在。
2. 提供明确的外部管理门面模式：验证 Distributed 门面，而不是在 compute 上创建或同步实体表结构。
3. 分别验证两个 compute 上门面的字段、目标集群、数据库、远端表名和分片表达式。
4. 在该模式下，门面缺失或定义错误时应明确拒绝启动，不能在 compute 上自动创建实体替代表。
5. 将物理维护路由到正确的 storage local 表及副本集合；range 应使用 control 分片，而不是四分片数据集群。
6. 对原有直接连接 storage、由程序管理 MergeTree 表的部署，保留适当的兼容行为。

这些是实现建议，不代表代码中已经存在对应配置项。

## range 接管是另一个独立前置条件

解决自动 DDL 问题，并不代表预复制目标可以带空 range 安全启动。

`chain/chain/dimension_simple.go` 的 `Init` 在当前 range 为空时使用清理起点 `0`；range 非空时使用 `End + 1`。

迁移后的 writer 启动前，必须建立经过验证的接管边界，并使用经过测试的 range 发布协议。不能为了绕过初始化而随意写入 range：记录的范围必须对应已经验证的连续数据覆盖。

## 后续修复需要验证的项目

- 已有正确门面能够被识别，不会在 compute 上触发 CREATE 或结构 ALTER。
- 不支持的引擎及元数据读取错误能够传递给调用方。
- 外部管理模式下，门面缺失或路由错误时明确启动失败。
- 两个 compute 分别完成验证。
- 直接连接 storage 的原有初始化行为符合预期。
- 分区保留及其他物理维护作用于 storage local 表。
- 已有预复制数据但 range 为空或未验证时，不会触发破坏性初始化。
- 生产接管前完成重启、副本落后、分布式部分写入成功及写入结果未知等场景的演练。

## 源码依据

除另有说明，以下路径均相对于 `sentio-core` 仓库根目录：

- `common/chx/table_engine.go`：`buildEngineFromString`、默认 MergeTree 引擎选择。
- `common/chx/operator.go`：`load`、`LoadOne`、`buildCreateTableSQL`、`Create`、`Sync`。
- `common/chx/query.go`：`Controller.Query` 及回调错误处理。
- `chain/clickhouse/range_store.go`：`NewRangeStore` 及分区保留清理。
- `chain/clickhouse/simple_slot_store.go`：数据表初始化及维护。
- `chain/chain/dimension_simple.go`：根据 range 执行启动清理。
- 同级仓库 `sentio/launcher/chain.go`：`BuildClickhouseRangeStore` 及 launcher panic 传播。
- 同级仓库 `production/k8s-sea/clickhouse-chain-0/zircuit-precopy/zircuit_precopy.py`：`facade_ddl` 和 `expected_objects`。

本文基于静态源码检查，未进行运行时复现或生产表结构验证。
