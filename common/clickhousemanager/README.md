# ClickHouse Manager (`common/clickhousemanager`)

This package provides a small abstraction layer around ClickHouse connections for Sentio.

It focuses on:

- **Sharding**: choose a ClickHouse shard by project/org/tier (or fall back to a default shard)
- **Connection management**: build ClickHouse DSNs from shard addresses + credentials, and reuse connections
- **Per-connection options**: Dial/Read timeouts, pool sizing, ClickHouse settings, and optional query signing

> Note: this package wraps [`clickhouse-go/v2`](https://github.com/ClickHouse/clickhouse-go) and exposes a thin `Conn` interface with `Exec/Query/QueryRow`.

---

## Key concepts

### `Manager`

A `Manager` owns:

- All configured shards (`[]Sharding`)
- Mapping strategies (by project name, org name, tier)
- A default shard (free tier fallback)
- Loader options (single shard mode, settings overrides, panic behavior)

Main APIs:

- `GetShardByIndex(i ShardingIndex) Sharding` - Get shard by numeric index
- `GetShardByName(name string) Sharding` - Get shard by name
- `All() []Sharding` - Get all configured shards
- `DefaultIndex() ShardingIndex` - Get the default shard index
- `Pick(PickOptions) (ShardingIndex, Sharding)` - Pick shard by project/org/tier
- `Reload(Config) error` - Reload configuration dynamically
- `NewShardByStateIndexer(indexerInfo state.IndexerInfo) Sharding` - Create dynamic shard for indexer

### Creating a Manager

Use `LoadManager()` to create a manager instance:

```go
func LoadManager(configPath string, funcs ...func(o *managerLoaderOptions)) Manager
```

Functional options:

- `LoadSingleSharding(ShardingIndex)` - Load only a specific shard
- `LoadAllowPanic()` - Enable panic on configuration errors
- `LoadSettings(map[string]any)` - Override/add ClickHouse settings at runtime

### `Sharding`

A `Sharding` represents one ClickHouse shard.

Main APIs:

- `GetConn(...func(*ShardingParameter)) (Conn, error)`
- `GetConnAllReplicas(...func(*ShardingParameter)) ([]Conn, error)`

### `Conn`

`Conn` is the wrapper interface around a `clickhouse.Conn`:

- `Exec(ctx, sql, args...)`
- `Query(ctx, sql, args...)`
- `QueryRow(ctx, sql, args...)`

Plus helpers:

- `GetDatabase()`, `GetUsername()`, `GetHost()`, `GetSettings()`, `GetCluster()`

---

## Configuration

The manager loads a YAML/JSON config file into `ckhmanager.Config`.

There is a local dev/test example at:

- `common/clickhousemanager/testdata/test_config.yaml`

### Minimal YAML shape

Top-level fields used by this package:

- `read_timeout`, `dial_timeout`, `max_idle_connections`, `max_open_connections`
- `settings`: map of ClickHouse settings
- `credential`: map of named credentials
- `shards`: list of shards and their addresses/allow-lists

A (simplified) example:

```yaml
read_timeout: 30s
dial_timeout: 10s
max_idle_connections: 50
max_open_connections: 100

settings:
  use_query_cache: 1

credential:
  sentio_admin:
    username: default
    password: password
    database: default

shards:
  - index: 0
    name: shard-0
    allow_organizations: ["sentio", "public"]
    addresses:
      internal_tcp_addr: "localhost:9000"
      internal_tcp_replicas: "localhost:9000,localhost:9000"
      external_tcp_addr: "localhost:9000"
      external_tcp_replicas: "localhost:9000,localhost:9000"
      internal_tcp_proxy: ""
      external_tcp_proxy: ""
```

### Credentials keying

Internally, shard connection picks credentials by:

- `Category` + `_` + `Role`

Example: `sentio_admin` corresponds to:

- `WithCategory("sentio")`
- `WithRole("admin")`

See constants in `roles.go`:

- Categories: `SentioCategory`, `SubgraphCategory` (default)
- Roles: `default_viewer`, `small_viewer`, ... and `admin` (special)

---

## Usage

### Load a manager from config path

Basic usage:

```go
m := ckhmanager.LoadManager("/path/to/config.yaml")
shard := m.GetShardByIndex(0)
```

### Load manager with options

The `LoadManager` function supports functional options for advanced configuration:

#### Single Shard Mode

Load only a specific shard (useful for testing or dedicated environments):

```go
m := ckhmanager.LoadManager("/path/to/config.yaml",
    ckhmanager.LoadSingleSharding(1), // Only load shard with index 1
)
```

#### Runtime Settings Override

Override or add ClickHouse settings at runtime:

```go
m := ckhmanager.LoadManager("/path/to/config.yaml",
    ckhmanager.LoadSettings(map[string]any{
        "max_execution_time":     60,
        "max_memory_usage":       10000000000,
        "use_query_cache":        0,
    }),
)
```

#### Allow Panic Mode

Enable panic on configuration errors (useful for strict validation):

```go
m := ckhmanager.LoadManager("/path/to/config.yaml",
    ckhmanager.LoadAllowPanic(),
)
```

#### Combining Options

Multiple options can be combined:

```go
m := ckhmanager.LoadManager("/path/to/config.yaml",
    ckhmanager.LoadSingleSharding(2),
    ckhmanager.LoadSettings(map[string]any{
        "max_execution_time": 120,
    }),
    ckhmanager.LoadAllowPanic(),
)
```

### Get a connection from a shard

```go
ck, err := shard.GetConn(
    ckhmanager.WithCategory(ckhmanager.SentioCategory),
    ckhmanager.WithRole("admin"),
    func(p *ckhmanager.ShardingParameter) {
        p.InternalOnly = false // use external_tcp_addr
        p.UnderlyingProxy = false
    },
)
if err != nil {
    // handle
}

ctx := context.Background()
_ = ck.Exec(ctx, "SELECT 1")
```

### Settings / dial options

Dial + pool sizing + settings are configured at the manager level (in YAML) and applied when creating shard connections.

If you need query signing, set `PrivateKey` in `ShardingParameter` (see `WithSign`).

#### Settings Priority

Settings are merged with the following priority (lowest to highest):

1. **Code defaults** - Built-in default settings from `newConnSettingsMacro()`
2. **Config file settings** - Settings defined in the YAML `settings` block
3. **Runtime input settings** - Settings passed via `LoadSettings()` option

**Example:**

```yaml
# config.yaml
settings:
  max_execution_time: 30
  use_query_cache: 1
```

```go
// Runtime override
m := ckhmanager.LoadManager("/path/to/config.yaml",
    ckhmanager.LoadSettings(map[string]any{
        "max_execution_time": 60,  // Overrides config value (30)
        "max_memory_usage": 10000000000,  // Adds new setting
    }),
)

// Final settings applied to connections:
// - max_execution_time: 60 (from runtime input)
// - use_query_cache: 1 (from config file)
// - max_memory_usage: 10000000000 (from runtime input)
// - [other defaults from code]
```

This priority system allows for flexible configuration: set sensible defaults in code, environment-specific values in config files, and runtime overrides for special cases or dynamic configuration.

#### Runtime Context Settings

**IMPORTANT:** When passing runtime context settings to ClickHouse queries, always use `ContextMergeSettings()` function instead of the original clickhouse library's `clickhouse.WithSettings()` function.

The original `clickhouse.WithSettings()` has **overwrite behavior** - it completely replaces all existing settings instead of merging them. This is not allowed in our architecture as it can discard important default and configuration-level settings.

We implemented `ContextMergeSettings()` which properly **merges** the new settings with existing ones, preserving the settings priority chain (code defaults → config file → runtime input → context settings).

**Correct usage:**

```go
ctx := context.Background()
ctx = ckhmanager.ContextMergeSettings(ctx, map[string]any{
    "max_execution_time": 120,
    "readonly": 1,
})
err := ck.Query(ctx, "SELECT * FROM table")
```

**Incorrect usage (DO NOT USE):**

```go
// ❌ WRONG - This will overwrite all existing settings!
ctx := clickhouse.WithSettings(context.Background(), clickhouse.Settings{
    "max_execution_time": 120,
})
```

---

## Repo notes

- The actual ClickHouse connection is created via `clickhouse.ParseDSN`/`clickhouse.Open`.
- Connections are cached by DSN + serialized options, so repeated calls reuse driver connections.

