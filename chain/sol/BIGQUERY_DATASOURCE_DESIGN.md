# Solana Super-Node 引入 BigQuery 作为第三数据源 —— 字段映射表与缺口处理方案

> 状态：设计文档（暂不写代码）
> 目标：为 Solana super-node 增加 BigQuery 作为 **第三优先级** 数据源
> 语言/落点：Go，`sentio-core/chain/sol/`（super-node 实现处），装配在 `sentio/launcher/chain_sol.go`

---

## 1. 背景与目标

Solana super-node 当前用两个数据源回答 `sol_*` 数据请求，按优先级：

1. **latest-slot cache**（最新若干 slot，内存，最快）
2. **ClickHouse**（已同步的历史，受 `rangeStore` 的 `[minSlot, maxSlot]` 约束）

ClickHouse 的 Solana 数据**不是从创世开始就有**的（`rangeStore.Get()` 返回的下界 `minSlot` 之前是空的）。对于 **`minSlot` 之前、cache 和 ClickHouse 都覆盖不到的历史 slot**，希望回退到 **BigQuery 公共数据集** 构造返回值：

- 数据集：`bigquery-public-data.crypto_solana_mainnet_us`
  - `.Blocks`、`.Transactions`、`.Instructions`
- BigQuery 作为 **最低优先级** 兜底，只在前两层都不覆盖时使用。
- 鉴权：super-node pod 已挂载 `gcloud-secret-admin`（GCP SA `bigquery@sentio-352722`，已验证有 `bigquery.jobs.create` 与查询该公共数据集的权限），并设置了 `GOOGLE_APPLICATION_CREDENTIALS`。BigQuery client 直接走 ADC 即可。

### ✅ 两个高风险不确定性的验证结论（已用 BigQuery 实测）

> 用 `test/test-sentio-driver-sa` + `gcloud-secret-admin`（`bigquery@sentio-352722`）实测得到。

1. **`Transactions` 表无 `version` 列、无 `loadedAddresses`**（已核对完整 schema）。BQ 根本不区分 legacy / v0，`accounts` 是唯一的账户列表。**定稿**：`version` 一律默认 `legacy(-1)`（见 §5.1）。
2. **`accounts` 是「完全解析后的账户全集」**：用一笔 44 账户的 v0 交易（`4FTMt8f6…`，Jupiter v6）验证 —— `ARRAY_LENGTH(accounts)=44`、`ARRAY_LENGTH(balance_changes)=44`、token `account_index` 最大 29（< 44）。即 LUT 加载的账户**已经被展开进 `accounts`**，且 `balance_changes` 与 token `account_index` 都对齐到这同一个列表。结论：parsed 重建只要**以 BQ 的 `accounts` 顺序作为 `accountKeys`**，preBalances/postBalances/tokenBalances 内部即自洽，无需 `loadedAddresses`（§5.3）。
3. **失败交易**：`status` 取 `"Success"` / `"Fail"`；`err` 是字符串（实测如 `"Error processing Instruction 1: custom program error: 0x14"`），非 RPC 结构化对象。**定稿**：不做语义解析 —— `err` 若是合法 JSON 则转 `json.RawMessage`，否则原样字符串（见 §5.10）。
4. **成本约束（实测元数据）**：`Transactions` 888 TB、`Instructions` 890 TB，二者均 **按 `block_timestamp` DAY 分区且 `requirePartitionFilter=True`（查询必须带 `block_timestamp` 谓词，否则直接报错）**；`Transactions` 按 `signature` 聚簇、`Instructions` 按 `program_id` 聚簇。`Blocks` 71 GB、按 MONTH 分区、无强制过滤（见 §5）。

### 相关代码位置（已核对）

| 角色 | 文件 | 关键符号 |
|------|------|----------|
| 数据源选择核心 | `sentio-core/chain/chain/util.go` | `QueryRangeWithCache`(L61-104)、`CheckRange`(L106-120) |
| super-node RPC | `sentio-core/chain/sol/supernode/rpc.go` | `NewSuperNode`(L35)、`RPCService`、`GetBlock`(L135)、`GetBlocksByInterval`(L171)、`FindTransactions`(L320)、`GetContractStartBlock`(L383)、`previousUnskippedBlock`(L285) |
| 存储接口 | `sentio-core/chain/sol/supernode/storage.go` | `Storage`（5 个方法） |
| ClickHouse 实现（**对照样板**） | `sentio-core/chain/sol/ch/data.go` | `ClickhouseBlock.toBlock`(L52)、`ClickhouseTransaction.toWrappedTransaction`(L78) |
| 领域类型 | `sentio-core/chain/sol/types.go`、`slot.go` | `Block`、`WrappedTransaction`、`BlockTransactions`、`ParsedTransactionWithMeta` |
| 目标 RPC 类型 | `gagliardetto/solana-go@v1.20.0/rpc/types.go`、`getBlock.go` | `GetBlockResult`、`ParsedTransaction`、`ParsedTransactionMeta`、`ParsedInstruction`、`ParsedInnerInstruction`、`ParsedMessageAccount`、`TokenBalance`、`UiTokenAmount` |
| 装配 | `sentio/launcher/chain_sol.go` | `BuildSolMiddlewares`(L89) |

---

## 2. 关键设计结论：用「JSON 形态 → Unmarshal」而不是手填 struct

ClickHouse 路径并**不**逐字段构造 RPC 对象，而是把节点返回的 `transaction` / `meta` 原样存成 JSON 字符串（`transaction_json` / `meta_json`，ZSTD 压缩），读取时直接 unmarshal：

```go
// sentio-core/chain/sol/ch/data.go  toWrappedTransaction()
var transaction *rpc.ParsedTransaction
json.Unmarshal([]byte(ct.TransactionJSON), &transaction)
var meta *rpc.ParsedTransactionMeta
json.Unmarshal([]byte(ct.MetaJSON), &meta)
```

**因此 BigQuery 路径应当采用相同策略**：从 BQ 三张表的行拼出与 RPC `getTransaction(jsonParsed)` **完全相同的 JSON 形态**（即 `raw_tx.json` 中 `result.transaction` 与 `result.meta` 的结构），再 `json.Unmarshal` 成 `*rpc.ParsedTransaction` / `*rpc.ParsedTransactionMeta`。理由：

1. **复用** 已有且测试过的类型与（反）序列化逻辑，与 ClickHouse 路径产出对象一致。
2. **绕开 `InstructionInfoEnvelope` 的死结**：`ParsedInstruction.Parsed` 的类型 `InstructionInfoEnvelope`（`rpc/types.go` L584）字段全部 **不可导出**（`asString` / `asInstructionInfo`），且 **没有导出构造函数**，只实现了 `MarshalJSON` / `UnmarshalJSON`（`rpc/getParsedTransaction.go` L55/L62）。也就是说，跨包**无法直接构造**一个带 parsed info 的指令对象，**只能通过 JSON 反序列化得到**。这使「拼 JSON 再 Unmarshal」成为唯一干净路径。

最终产出的领域对象与 ClickHouse 路径完全一致：

- `sol.Block`（内嵌 `*rpc.GetBlockResult`）
- `sol.WrappedTransaction`（`*rpc.ParsedTransaction` + `*rpc.ParsedTransactionMeta` + `Version`）
- `sol.BlockTransactions`（带 block 头 + `[]WrappedTransaction`）

> 注意：很多 RPC 字段（如 `accountKeys[].source`、`meta.costUnits`）**在 solana-go 的 Go 类型里根本不存在**，所以即便走 ClickHouse 路径也会在 round-trip 中被丢弃。下文「缺口」一节会标明哪些缺失字段其实**无需处理**。

---

## 3. 字段映射表（BigQuery 行 → RPC JSON 形态）

以样例交易 `3WeJDhD1…WsGyywv`（slot `422822279`）为基准，三张表对应 `raw_tx.json`。

### 3.1 `Blocks` → `sol.Block` / `rpc.GetBlockResult`

`sol_getBlock` 只填 header（不含签名）；`sol_getBlocksByInterval` 额外填 `Signatures`（签名需从 `Transactions` 表按 slot 取首签名、按 `index` 排序）。

| BQ `Blocks` 列 | 目标字段（JSON key） | 说明 |
|---|---|---|
| `slot` | `sol.Block.Slot` + 用于查询键 | string→uint64 |
| `block_hash` | `GetBlockResult.blockhash` | base58 |
| `previous_block_hash` | `GetBlockResult.previousBlockhash` | base58 |
| `block_timestamp` | `GetBlockResult.blockTime` | `"2026-05-28 23:42:14 UTC"` → unix 秒 `1780011734` |
| `height` | `GetBlockResult.blockHeight` | string→uint64（指针）|
| `transaction_count` | —（无对应；可用于校验签名条数）| |
| `leader` / `leader_reward` | →（可选）`GetBlockResult.rewards`（见缺口 §5.8） | per-tx meta 一般为空 |
| —（BQ 无此概念）| `GetBlockResult.parentSlot` | **缺**：BQ 无 parentSlot；见 §5.9 |

### 3.2 `Transactions` → `sol.WrappedTransaction` + `rpc.ParsedTransactionMeta` + `message`

| BQ `Transactions` 列 | 目标字段 | 说明 |
|---|---|---|
| `signature` | `transaction.signatures[0]` / `WrappedTransaction.Signature` | base58 |
| `index` | `WrappedTransaction.TransactionIndex` | block 内序号 |
| `recent_block_hash` | `transaction.message.recentBlockhash` | |
| `accounts[]` (`pubkey/signer/writable`) | `transaction.message.accountKeys[]`（`ParsedMessageAccount`：`pubkey/signer/writable`）| **顺序即账户索引**，务必保持原序 |
| `fee` | `meta.fee` | |
| `compute_units_consumed` | `meta.computeUnitsConsumed` | 指针 |
| `status` / `err` | `meta.err` + `meta.status` | `Success`/`""`→ `err=null`, `status={"Ok":null}`；失败见 §5.10 |
| `log_messages[]` | `meta.logMessages` | 原样 |
| `balance_changes[]` (`account/before/after`) | `meta.preBalances[]` / `meta.postBalances[]` | **按 accountKeys 顺序投影**，见 §5.4 |
| `pre_token_balances[]` | `meta.preTokenBalances[]`（`TokenBalance`）| 见 §3.4 |
| `post_token_balances[]` | `meta.postTokenBalances[]` | 见 §3.4 |
| —（BQ **确认无此列**）| `WrappedTransaction.Version` | **缺**：默认 legacy(-1)，见 §5.1 |
| `block_slot/block_hash/block_timestamp` | 关联键 + `GetParsedTransactionResult.slot/blockTime` | |

### 3.3 `Instructions` → `message.instructions` + `meta.innerInstructions`

按 `parent_index` 划分层级，按 `index` 在层内排序：

- `parent_index = null` → **顶层指令**，进 `transaction.message.instructions`，按 `index` 升序。
- `parent_index = N` → **内层指令**，按 `parent_index` 分组进 `meta.innerInstructions[].instructions`，`ParsedInnerInstruction.index = N`，组内按 `index` 升序。

> 已用样例验证：顶层 `index` 0,1,2,3,4,5,6 与 `raw_tx` `message.instructions` 顺序一致；内层 `parent_index=2`（4 条）、`parent_index=3`（7 条）与 `raw_tx` `innerInstructions` 的 `index:2` / `index:3` 完全吻合。

单条指令（`rpc.ParsedInstruction`）映射：

| BQ `Instructions` 列 | 目标字段 | 规则 |
|---|---|---|
| `program_id` | `programId` | 必有 |
| `program` (非 null) | `program` | **判定依据**：`program != null` ⇒ 已解析指令 |
| `instruction_type` | `parsed.type` (`InstructionInfo.InstructionType`) | |
| `params[]` (`key`/`value`) | `parsed.info` (`InstructionInfo.Info` map) | `info[key] = JSON.parse(value)`，见下 |
| `data` | `data`（base58）| 仅**未解析**指令（`program == null`）使用 |
| `accounts[]` | `accounts`（[]pubkey）| 仅**未解析**指令使用；解析指令此处为 `[""]`，忽略 |
| —（推导）| `stackHeight` | 顶层=1；内层默认=2，深层 CPI 需从 log 还原，见 §5.6 |

**`params.value` 解析规则**（value 是 JSON 编码的字符串）：

| BQ `value` 原文 | `JSON.parse` 后写入 `info[key]` |
|---|---|
| `"\"6h3x…\""` | 字符串 `"6h3x…"` |
| `"165"` | 数字 `165` |
| `"[\"immutableOwner\"]"` | 数组 `["immutableOwner"]` |
| `"{\"amount\":\"3014690\",\"decimals\":9,…}"` | 对象 `{amount, decimals, uiAmount, uiAmountString}` |
| `key=null, value=null`（占位）| 跳过；该指令为未解析指令（无 `parsed`）|

### 3.4 token balance：`pre/post_token_balances` → `rpc.TokenBalance`

| BQ 列 | `TokenBalance` 字段 | 说明 |
|---|---|---|
| `account_index` | `accountIndex` | |
| `mint` | `mint` | |
| `owner` | `owner`（*pubkey）| |
| `amount` + `decimals` | `uiTokenAmount`（`UiTokenAmount`）| `amount` 原样字符串；`decimals`；**`uiAmount`/`uiAmountString` 需计算**：`uiAmount = amount / 10^decimals`（`amount==0` 时 `uiAmount=null`，`uiAmountString="0"`，对照 `raw_tx`） |
| —（BQ 无）| `programId`（*pubkey）| **缺**：见 §5.5，可置 nil |

---

## 4. 缺口分析与处理方案

| # | 缺口 | 影响 | 处理方案 |
|---|---|---|---|
| 5.1 | **`version`**（legacy / v0）BQ `Transactions` **确认无此列** | `WrappedTransaction.Version` 会经 `ToParsedTransactionResult` 序列化进 processor 看到的 tx JSON（`block_data.go:68`）；driver 只透传、不分支判断 | **✅ 已定稿：一律默认 `LegacyTransactionVersion = -1`**（solana-go `transaction_version.go`，序列化为 `"legacy"`）。指令/账户/余额都不依赖它，对 v0 交易仅令 `version` 字段显示为 `"legacy"`（接受此保真度瑕疵）。**不做**账户数启发式。 |
| 5.2 | `accountKeys[].source`（transaction / lookupTable）| 无 | **无需处理**：`rpc.ParsedMessageAccount` 没有 `source` 字段，ClickHouse 路径同样丢弃。 |
| 5.3 | `meta.loadedAddresses`（v0 的 LUT 加载地址）| `ParsedTransactionMeta.LoadedAddresses` 存在但 BQ 不直接给 | **✅ 已验证可置空**：BQ `accounts[]` 已是含 LUT 的完整解析列表（44 账户 v0 交易实测 `n_acct==n_bal==44`，token `account_index` 均落在范围内）。以 BQ `accounts` 顺序作为 `accountKeys`，preBalances/postBalances/token `accountIndex` 内部自洽，`loadedAddresses` 置空不影响 parsed 重建。**注**：未与 RPC 交叉核对账户**绝对顺序**（沙箱 slot 在未来、公共 RPC 无此历史），但 parsed 模式指令以 pubkey 引用账户，顺序仅需内部一致即可。 |
| 5.4 | `preBalances/postBalances` 顺序 | 必须与 `accountKeys` 顺序一致，否则余额错位 | BQ `balance_changes[]` 以 `account` 为键 → 构造 `account→{before,after}` map，再**按 `accountKeys` 顺序**投影成两个数组。样例中二者本就同序，但实现不可依赖该巧合。 |
| 5.5 | token balance 的 `programId` | 区分 Token / Token-2022 | **定稿：v1 置 nil**（`*pubkey`，`omitempty`，与 ClickHouse 路径在该字段缺失时行为一致）。增强（后置）：扫描本交易已解析的 token 指令，建 `mint→tokenProgram` 映射回填——同一 mint 归属唯一 token 程序，可由 `program_id` 推出。 |
| 5.6 | 深层 CPI 的 `stackHeight`（>2）| BQ `parent_index` 只有一层，深层嵌套深度从 `Instructions` 表本身**不可恢复** | **定稿：v1 顶层=1、所有内层=2**。指令内容/顺序/分组均正确（已验证），仅嵌套深度近似。增强（后置、依赖 log）：按执行顺序对齐 `log_messages` 的 `Program <id> invoke [n]`，用 `[n]` 赋真实 `stackHeight`——`Instructions` 单独无法做到，logs 是唯一来源。**记录为保真度降级项**。 |
| 5.7 | `meta.returnData` | 个别消费方需要 | **定稿：v1 置空**（零值）。`rpc.ParsedTransactionMeta.ReturnData` 存在但 BQ 无结构化字段；多数消费方不读。增强（后置）：取 `log_messages` 中**最后一条** `Program return: <programId> <base64>` 解析为 `{programId, data}`。 |
| 5.8 | `meta.rewards` | 通常 per-tx 为空 | 置 `[]`（`raw_tx` 即为空）。block 级 `leader_reward` 与 per-tx meta 无关。 |
| 5.9 | `GetBlockResult.parentSlot` | `sol_getBlock` 返回 header 用 | **缺**：BQ `Blocks` 无 parentSlot。Solana 常跳 slot，`slot-1` 未必是父。缓解：查 `Blocks` 中 `< slot` 的最大 `slot` 作为 parentSlot（一次 `MAX(slot)` 查询）。 |
| 5.10 | `err` / `status` 失败态 | 失败交易需正确表达 | **✅ 已定稿**：BQ `status ∈ {"Success","Fail"}`；`err` 是字符串（实测如 `"Error processing Instruction 1: custom program error: 0x14"`）。**不做语义解析**。映射规则：<br>• `status="Success"` ⇒ `meta.err = null`，`meta.status = {"Ok": null}`。<br>• `status="Fail"`：取 BQ `err` 字符串 `s` —— **若 `s` 是合法 JSON**（`json.Valid([]byte(s))`）则作为 `json.RawMessage` 原样嵌入；**否则**作为普通字符串值。该值同时用于 `meta.err` 与 `meta.status = {"Err": <该值>}`。（`rpc.ParsedTransactionMeta.Err` 类型为 `any`，两种都兼容。） |
| 5.11 | `meta.costUnits` | 无 | **无需处理**：`rpc.ParsedTransactionMeta` 无该字段，ClickHouse 路径也丢弃。 |
| 5.12 | **skipped slot** | BQ 只存存在的块，无「skipped」稠密行（ClickHouse `blocks` 表是每 slot 一行含 `skipped`）| `QueryBlock` 命中空 ⇒ 视为 skipped（返回 `sol.Block{Slot}` 且 `GetBlockResult=nil`）。`previousUnskipped` / `getBlocksByInterval` 的窗口逻辑需重写为「`Blocks` 中实际存在的块」语义，见 §5.2 查询。 |

---

## 5. 各 `Storage` 方法 → BigQuery 查询设计

`Storage` 接口 5 个方法（`supernode/storage.go`），BigQuery 实现需逐个对应：

1. **`QueryBlock(slot)`** → `SELECT … FROM Blocks WHERE slot=@slot LIMIT 1`；空 ⇒ skipped（§5.12）；`parentSlot` 见 §5.9。
2. **`QueryBlocksByInterval(from,to,window,limit)`** → 取每个窗口的第一个块。BQ 无 skipped 行，窗口分组（block window: `DIV(slot,W)`；time window: `DIV(UNIX_SECONDS(block_timestamp), W)`）取每组最小 slot。需带签名（关联 `Transactions`）。**较复杂**，注意成本。
3. **`QueryPreviousUnskipped(before)`** → `SELECT slot, block_timestamp FROM Blocks WHERE slot<@before ORDER BY slot DESC LIMIT 1`。
4. **`FindTransactions(from,to,programIDs,limit)`** → 先在 `Instructions` 按 `program_id IN @ids` 且 `block_slot BETWEEN` 选出 `DISTINCT (block_slot, tx_signature)`；再 JOIN `Transactions` + `Instructions`（取全部指令）+ `Blocks`（block 头）组装。**最重的查询**。
5. **`EarliestProgramSlot(address)`** → `SELECT MIN(block_slot) FROM Instructions WHERE program_id=@addr AND <全历史 block_timestamp 谓词>`。⚠️ 因 `requirePartitionFilter=True`，无法做无界 MIN，只能给一个覆盖全数据集的时间下界（如 `block_timestamp >= '2020-03-01'`）；靠 `program_id` 聚簇裁剪到该程序的数据。热门程序可能仍较贵，但此调用罕见（processor 启动时一次），加 `maxBytesBilled` 兜底。

### ⚠️ 成本关键点（定稿）：先用 `Blocks` 表解析 slot→timestamp

实测：`Transactions`/`Instructions` 各约 888/890 TB，按 **`block_timestamp` DAY 分区且 `requirePartitionFilter=True`** —— **查询不带 `block_timestamp` 谓词会直接报错**；按 `slot` 过滤无法裁剪分区。账单走 `bigquery@sentio-352722`（按扫描字节计费）。super-node 入参是 **slot**，必须先拿到时间。

**定稿方案**：
1. **第一跳：查 `Blocks` 解析时间**。`Blocks` 仅 71 GB、MONTH 分区、无强制过滤，按 slot 查很便宜：
   - 单 slot：`SELECT slot, block_timestamp, previous_block_hash, … FROM Blocks WHERE slot=@slot`；
   - slot 区间：`SELECT MIN(block_timestamp) lo, MAX(block_timestamp) hi FROM Blocks WHERE slot BETWEEN @from AND @to`。
   `Blocks` 本就是 `QueryBlock` / `parentSlot`（§5.9）/ skipped 判定（§5.12）所需，**顺带产出精确的 DAY 分区边界**。
2. **第二跳：用精确 `block_timestamp` 谓词查重表**。把第一跳得到的 `[lo, hi]`（按 DAY 向外各扩 1 天容错）作为 `Transactions`/`Instructions` 的分区过滤，扫描收敛到所涉及的少数 DAY 分区。
3. **每个 job 强制设 `maxBytesBilled`**（配置项）作为防爆熔断；超限直接失败而非烧钱。
4. **只 SELECT 必要列**（BQ 列式计费）：组装一笔交易只需 `Transactions` 的标量列 + `accounts`/`*_token_balances`/`balance_changes`/`log_messages`，以及 `Instructions` 的指令列。

> 备选（不推荐作主路径）：用 Solana ~400ms/slot、以 `rangeStore.minSlot` 为锚外推时间窗口，省掉第一跳。误差需放大窗口、且锚点漂移有风险，仅在 `Blocks` 第一跳成为瓶颈时再考虑。

---

## 6. Go 集成方案（仅定位接入点，不写代码）

### 6.1 新增 BigQuery `Storage` 实现
- 新包 `sentio-core/chain/sol/bq`，类型 `bq.Store` 实现 `supernode.Storage` 5 方法。
- 内部按 §2「拼 JSON → `json.Unmarshal`」产出 `sol.Block` / `sol.WrappedTransaction` / `sol.BlockTransactions`，与 `sol/ch` 产物同构。
- 依赖 `cloud.google.com/go/bigquery`（需加入 `MODULE.bazel` 并 `gazelle`）；鉴权走已挂载的 ADC（`bigquery@` SA）。

### 6.2 三层优先级的串接
当前 `QueryRangeWithCache(ctx, interval, slotCache, cachedProc, queryLoader)` 只接 **一个** `queryLoader`，且 `CheckRange` 在范围超出 `rangeStore` 时**直接报错**。要把 BigQuery 接成「ClickHouse 之下的兜底」，两种改法：

- **方案 A（推荐，改动小）**：保持 `QueryRangeWithCache` 签名不变，把传入的 `queryResultLoader` 换成一个**组合 loader**：对请求子区间，先用 `rangeStore` 判断 —— 落在 `[min,max]` 内的部分走 ClickHouse（现 `CheckRange` 逻辑），低于 `min` 的部分走 BigQuery；两段结果合并。即「cache（最新）> ClickHouse（rangeStore 内）> BigQuery（rangeStore 之下的历史）」。
- **方案 B**：扩展 `QueryRangeWithCache`，接受一个 `(rangeStore, loader)` 兜底链（切片），按序消化剩余未覆盖区间。更通用但改动大、影响其他链（evm/sui 共用）。

> 优先级语义：BigQuery 永远**最低**，仅用于更高层不覆盖的 slot（主要是 `< rangeStore.minSlot` 的归档历史）。

### 6.3 `RPCService` / `NewSuperNode`
- `RPCService` 增加可选字段 `bqStore Storage`（未配置时为 `nil`）。
- `NewSuperNode(...)` 增参 `bqStore`（或 `...Option`）。各方法的 `queryResultLoader` 改为 §6.2 的组合 loader；`bqStore==nil` 时退化为现状（完全向后兼容）。
- **方法接入优先级（定稿）**：
  - **P0（核心回填路径，必接）**：`FindTransactions`（按程序回填历史交易，主用途）、`QueryBlock`（取 block 头组装数据，且兼任 §5「第一跳」slot→time 解析）。
  - **P1（必接，含语义微调）**：`EarliestProgramSlot` / `GetContractStartBlock`。**注意优先级方向相反**：该方法求「程序最早出现的 slot」，而 BQ 覆盖的正是 ClickHouse `minSlot` 之下的更老历史，故对它而言 **BQ 的结果可能比 ClickHouse 更早**。定稿语义：先查 BQ（最老），命中即为全局最早；BQ 未命中再退回 ClickHouse、最后 cache。即「最早 slot」场景下 BQ 优先级最高，与其它「最新数据」场景相反——实现时这是一条独立分支，不走 §6.2 的组合 loader。
  - **P2（按需，可后置）**：`QueryPreviousUnskipped`（取某 slot 的链上时间，历史回填偶用）、`QueryBlocksByInterval`（窗口化列块；BQ 上窗口分组跨大分区**最贵**，建议后置，且接入时强制 `maxBytesBilled` + 收紧 slot 跨度）。

### 6.4 装配与配置（`sentio/launcher/chain_sol.go` `BuildSolMiddlewares`）
- 现状：`cache` 或 `clickhouse` 缺任一即退回纯代理；否则 `NewSuperNode(client, slotCache, rangeStore, store)`。
- 新增：`bqConf := c.Get("bigquery")`；`bqConf.IsExist()` 时构造 `bq.NewStore(...)` 并传入 `NewSuperNode`；不存在则传 `nil`（保持兼容）。
- 配置项（yaml）草案：
  ```yaml
  bigquery:
    project: sentio-352722          # 计费项目
    dataset: bigquery-public-data.crypto_solana_mainnet_us
    tables:                          # 可选，便于切换/测试
      blocks: Blocks
      transactions: Transactions
      instructions: Instructions
    maxBytesBilled: 50000000000      # 单 job 扫描上限（熔断）
    # 鉴权走 GOOGLE_APPLICATION_CREDENTIALS（pod 已挂载 gcloud-secret-admin）
  ```

---

## 7. 决策记录（全部已定稿）

| # | 事项 | 定稿 | 章节 |
|---|---|---|---|
| 1 | v0 / `version` | BQ 无 `version`/`loadedAddresses`；`accounts` 已含 LUT 项且与 balance/token 索引自洽；`version` 一律默认 `legacy(-1)`，不做启发式 | §5.1/5.3 |
| 2 | 失败交易 `err` | 不做语义解析；`err` 合法 JSON ⇒ `json.RawMessage`，否则原样字符串；`status` 据此构造 `{"Ok":null}`/`{"Err":…}` | §5.10 |
| 3 | `stackHeight` | v1 顶层=1、内层=2；深层 CPI 还原（依赖 log）后置 | §5.6 |
| 4 | token `programId` | v1 置 nil；`mint→程序` 推导后置 | §5.5 |
| 5 | `returnData` | v1 置空；从 log 末条 `Program return:` 解析后置 | §5.7 |
| 6 | slot→timestamp 成本 | 先查 `Blocks`（便宜）解析精确 DAY 边界，再给重表加分区谓词；每 job 强制 `maxBytesBilled` | §5「成本关键点」 |
| 7 | 方法接入优先级 | P0：`FindTransactions`、`QueryBlock`；P1：`EarliestProgramSlot`（BQ 反向最高优先级）；P2：`QueryPreviousUnskipped`、`QueryBlocksByInterval` | §6.3 |

**唯一遗留的运行期考量（非阻塞，留待实现/灰度时观测）**：BigQuery 查询为「秒级」，会走 super-node 慢请求路径。需评估对 driver 超时/重试的影响，并考虑给 BQ 兜底单独设超时、必要时对结果做本地缓存（归档历史不变，缓存命中率应很高）。

---

## 附：样例文件对照

`my-processors/test/` 下：`blocks.json`（Blocks 行）、`tx.json`（Transactions 行）、`instructions.json`（Instructions 行）三者来自 BQ；`raw_tx.json` 是同一笔交易 `getTransaction(encoding=jsonParsed)` 的 RPC 结果，即本方案要重建的**目标形态**。本文所有映射均以该样例核对。
