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

Main APIs:

- `GetShardByIndex(i int32) Sharding`
- `GetShardByName(name string) Sharding`
- `All() []Sharding`
- `Pick(options PickOptions) (index int32, shard Sharding)`
- `Reload(Config) error`

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

```go
m := ckhmanager.LoadManager("/path/to/config.yaml")
shard := m.GetShardByIndex(0)
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

---

## Repo notes

- The actual ClickHouse connection is created via `clickhouse.ParseDSN`/`clickhouse.Open`.
- Connections are cached by DSN + serialized options, so repeated calls reuse driver connections.

