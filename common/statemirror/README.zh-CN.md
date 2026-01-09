# StateMirror Package

[English Documentation](./README.md)

## 概述

`statemirror` 包提供了一个类型安全的状态镜像系统，用于在 Redis 中存储和同步链上状态数据。该包支持将任意类型的键值对映射到 Redis Hash 结构，并提供了强类型的访问接口。

该包包含两种实现：
- **RedisMirror**: 使用 Redis Hash 结构存储数据（推荐用于生产环境）
- **FileMirror**: 将数据存储在磁盘的 JSON 文件中（适用于开发、测试或本地存储）

## 核心概念

### 1. Mirror

`Mirror` 是底层接口，提供了对 Redis Hash 的基本操作：

- **Upsert**: 同步整个状态，自动计算差异并更新
- **UpsertStreaming**: 流式同步状态，适用于大数据量场景
- **Apply**: 应用增量变更（添加/删除字段）
- **Get/MGet/GetAll**: 读取单个、多个或全部字段
- **Scan**: 游标扫描字段

### 2. TypedMirror

`TypedMirror[K, V]` 是基于 `Mirror` 的泛型封装，提供类型安全的访问：

- 自动处理键和值的序列化/反序列化
- 提供强类型的 Get/MGet/GetAll/Scan 方法
- 通过 `StateCodec` 实现类型转换

### 3. StateCodec

`StateCodec[K, V]` 接口定义了键和值的编解码规则：

```go
type StateCodec[K comparable, V any] interface {
    Field(k K) (string, error)           // 键 -> Redis field 字符串
    ParseField(field string) (K, error)  // Redis field 字符串 -> 键
    Encode(v V) (string, error)          // 值 -> Redis value 字符串
    Decode(s string) (V, error)          // Redis value 字符串 -> 值
}
```

### 4. JSONCodec

`JSONCodec[K, V]` 是 `StateCodec` 的一个常用实现，使用 JSON 序列化值：

```go
type JSONCodec[K comparable, V any] struct {
    FieldFunc func(K) (string, error)      // 键转换函数
    ParseFunc func(string) (K, error)      // 字符串解析函数
}
```

## 使用指南

### 基础用法

#### 1. 创建 Mirror

```go
import (
    "github.com/redis/go-redis/v9"
    "your-project/common/statemirror"
)

// 方式 1: 创建 Redis Mirror（用于生产环境）
client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

// 创建 Mirror（使用默认配置）
mirror := statemirror.NewRedisMirror(client)

// 或使用自定义配置
mirror := statemirror.NewRedisMirror(client,
    statemirror.WithRedisKeyPrefix("myapp:"),
    statemirror.WithScanCount(500),
)

// 方式 2: 创建 File Mirror（用于开发/测试）
mirror, err := statemirror.NewFileMirror("/path/to/data")
if err != nil {
    log.Fatal(err)
}

// 或使用自定义选项
mirror, err := statemirror.NewFileMirror("/tmp",
    statemirror.WithBaseDir("/custom/path"),
    statemirror.WithFileExtension(".data"),
)
if err != nil {
    log.Fatal(err)
}
```

#### 2. 定义数据结构和 Codec

```go
// 定义值类型
type ProcessorInfo struct {
    ProcessorID string    `json:"processor_id"`
    Version     int32     `json:"version"`
    ShardingIdx int32     `json:"sharding_idx"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// 创建 JSONCodec
codec := statemirror.JSONCodec[string, ProcessorInfo]{
    FieldFunc: func(k string) (string, error) {
        // 键转换：添加前缀或进行其他转换
        return fmt.Sprintf("processor:%s", k), nil
    },
    ParseFunc: func(s string) (string, error) {
        // 解析：移除前缀
        return strings.TrimPrefix(s, "processor:"), nil
    },
}
```

#### 3. 使用 TypedMirror

```go
// 创建 TypedMirror
typedMirror := statemirror.TypedMirror[string, ProcessorInfo]{
    m:     mirror,
    key:   statemirror.MappingProcessorAllocations, // 链上映射键
    codec: codec,
}

ctx := context.Background()

// 读取单个值
info, exists, err := typedMirror.Get(ctx, "processor-1")
if err != nil {
    log.Fatal(err)
}
if exists {
    fmt.Printf("Found: %+v\n", info)
}

// 读取多个值
infos, err := typedMirror.MGet(ctx, "processor-1", "processor-2", "processor-3")
if err != nil {
    log.Fatal(err)
}
for k, v := range infos {
    fmt.Printf("%s: %+v\n", k, v)
}

// 读取所有值
allInfos, err := typedMirror.GetAll(ctx, statemirror.MappingProcessorAllocations)
if err != nil {
    log.Fatal(err)
}

// 扫描匹配的值
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

### 高级用法

#### 1. 同步状态（Upsert）

使用 `BuildSyncFunc` 辅助函数构建同步函数：

```go
// 定义数据获取函数
fetchFunc := func(ctx context.Context, key statemirror.OnChainKey) (map[string]ProcessorInfo, error) {
    // 从数据库或 API 获取数据
    data := map[string]ProcessorInfo{
        "processor-1": {ProcessorID: "p1", Version: 1},
        "processor-2": {ProcessorID: "p2", Version: 2},
    }
    return data, nil
}

// 构建 SyncFunc
syncFunc := statemirror.BuildSyncFunc(codec, fetchFunc)

// 执行同步（自动计算差异并更新）
err := mirror.Upsert(ctx, statemirror.MappingProcessorAllocations, syncFunc)
if err != nil {
    log.Fatal(err)
}
```

#### 2. 流式同步（UpsertStreaming）

适用于大数据量场景，避免一次性加载所有数据到内存：

```go
// 定义流式数据发送函数
streamFunc := func(ctx context.Context, key statemirror.OnChainKey, emit func(string, ProcessorInfo) error) error {
    // 逐条发送数据
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

// 构建 StreamingSyncFunc
streamingSyncFunc := statemirror.BuildStreamingSyncFunc(codec, streamFunc)

// 执行流式同步
err := mirror.UpsertStreaming(ctx, statemirror.MappingProcessorAllocations, streamingSyncFunc)
if err != nil {
    log.Fatal(err)
}
```

#### 3. 增量更新（Apply）

只更新变更的字段，不需要全量同步：

```go
// 定义差异计算函数
diffFunc := func(ctx context.Context, key statemirror.OnChainKey) (*statemirror.TypedDiff[string, ProcessorInfo], error) {
    return &statemirror.TypedDiff[string, ProcessorInfo]{
        Added: map[string]ProcessorInfo{
            "processor-1": {ProcessorID: "p1", Version: 2}, // 更新
            "processor-4": {ProcessorID: "p4", Version: 1}, // 新增
        },
        Deleted: []string{"processor-3"}, // 删除
    }, nil
}

// 构建 DiffFunc
diffFuncBuilt := statemirror.BuildDiffFunc(codec, diffFunc)

// 应用差异
err := mirror.Apply(ctx, statemirror.MappingProcessorAllocations, diffFuncBuilt)
if err != nil {
    log.Fatal(err)
}
```

## 完整示例

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
    // 1. 创建 Redis 客户端
    client := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    defer client.Close()

    // 2. 创建 Mirror
    mirror := statemirror.NewRedisMirror(client)

    // 3. 定义 Codec
    codec := statemirror.JSONCodec[string, UserProfile]{
        FieldFunc: func(userID string) (string, error) {
            return "user:" + userID, nil
        },
        ParseFunc: func(field string) (string, error) {
            return strings.TrimPrefix(field, "user:"), nil
        },
    }

    // 4. 创建 TypedMirror
    typedMirror := statemirror.TypedMirror[string, UserProfile]{
        m:     mirror,
        key:   "UserProfiles", // OnChainKey
        codec: codec,
    }

    ctx := context.Background()

    // 5. 同步数据
    syncFunc := statemirror.BuildSyncFunc(codec, func(ctx context.Context, key statemirror.OnChainKey) (map[string]UserProfile, error) {
        return map[string]UserProfile{
            "user1": {UserID: "user1", Name: "Alice", Email: "alice@example.com", CreatedAt: time.Now()},
            "user2": {UserID: "user2", Name: "Bob", Email: "bob@example.com", CreatedAt: time.Now()},
        }, nil
    })

    if err := mirror.Upsert(ctx, "UserProfiles", syncFunc); err != nil {
        log.Fatal(err)
    }

    // 6. 读取数据
    profile, exists, err := typedMirror.Get(ctx, "user1")
    if err != nil {
        log.Fatal(err)
    }
    if exists {
        fmt.Printf("Profile: %+v\n", profile)
    }

    // 7. 读取多个用户
    profiles, err := typedMirror.MGet(ctx, "user1", "user2")
    if err != nil {
        log.Fatal(err)
    }
    for userID, profile := range profiles {
        fmt.Printf("%s: %+v\n", userID, profile)
    }
}
```

## 最佳实践

1. **选择合适的实现**：
   - **RedisMirror**: 用于生产环境，高并发、分布式系统，或需要快速内存访问的场景
   - **FileMirror**: 用于开发、测试、本地存储，或单实例应用中需要持久化到磁盘的场景

2. **选择合适的同步方式**：
   - 数据量小（< 1000 条）：使用 `Upsert`
   - 数据量大（> 10000 条）：使用 `UpsertStreaming`
   - 只有增量变更：使用 `Apply`

3. **Codec 设计**：
   - `FieldFunc` 应该生成唯一的字段名
   - 考虑添加前缀避免字段名冲突
   - 保持 `FieldFunc` 和 `ParseFunc` 的对称性

4. **错误处理**：
   - 所有方法都可能返回错误，务必检查
   - 编解码错误会直接返回，注意数据格式兼容性

5. **性能优化**：
   - 使用 `MGet` 批量读取，减少网络往返
   - 使用 `Scan` 分页处理大数据集
   - 调整 `WithScanCount` 根据数据大小优化性能

6. **键命名**：
   - 使用有意义的 `OnChainKey` 常量
   - 在 `on_chain_mapping_constants.go` 中统一管理

## API 参考

### Mirror 接口

- `Upsert(ctx, key, syncF)` - 全量同步
- `UpsertStreaming(ctx, key, syncF)` - 流式同步
- `Apply(ctx, key, diffF)` - 增量更新
- `Get(ctx, key, field)` - 读取单个字段
- `MGet(ctx, key, fields...)` - 批量读取
- `GetAll(ctx, key)` - 读取所有字段
- `Scan(ctx, key, cursor, match, count)` - 游标扫描

### TypedMirror 方法

- `Get(ctx, k)` - 读取单个值（类型安全）
- `MGet(ctx, ks...)` - 批量读取（类型安全）
- `GetAll(ctx, key)` - 读取所有值（类型安全）
- `Scan(ctx, cursor, match, count)` - 游标扫描（类型安全）

### 辅助函数

- `BuildSyncFunc[K, V](codec, fetch)` - 构建同步函数
- `BuildStreamingSyncFunc[K, V](codec, stream)` - 构建流式同步函数
- `BuildDiffFunc[K, V](codec, typed)` - 构建差异函数

## 常见问题

**Q: 如何处理复杂的键类型（如结构体）？**

A: 在 `FieldFunc` 中将结构体序列化为字符串，在 `ParseFunc` 中反序列化：

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

**Q: 如何自定义序列化格式（不使用 JSON）？**

A: 实现自己的 `StateCodec` 接口：

```go
type MyCodec struct{}

func (c MyCodec) Field(k string) (string, error) { /* ... */ }
func (c MyCodec) ParseField(field string) (string, error) { /* ... */ }
func (c MyCodec) Encode(v MyType) (string, error) { /* 自定义序列化 */ }
func (c MyCodec) Decode(s string) (MyType, error) { /* 自定义反序列化 */ }
```

**Q: Upsert 和 Apply 有什么区别？**

A: 
- `Upsert`: 提供完整的目标状态，系统自动计算差异并更新（包括删除多余的字段）
- `Apply`: 只提供需要变更的部分（添加/更新和删除的字段列表），不影响其他字段

