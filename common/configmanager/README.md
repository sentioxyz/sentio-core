# Config Manager

A flexible configuration management library based on [koanf](https://github.com/knadh/koanf) for Go that supports multiple data sources (PostgreSQL, Redis) with automatic reloading capabilities.

## Features

- 🔄 **Auto-reload**: Automatically watch for configuration changes and reload
- 🗄️ **Multiple Backends**: Support for PostgreSQL and Redis
- 📝 **Multiple Formats**: JSON, YAML, and TOML encoding support
- 🔒 **Thread-safe**: Safe for concurrent access
- 🎯 **Type-safe**: Strongly typed getters for common types
- 🔍 **Easy Access**: Simple API for retrieving nested configuration values

## Quick Start

### Basic Usage with PostgreSQL

```go
package main

import (
    "time"
    "sentioxyz/sentio-core/common/configmanager"
    "github.com/knadh/koanf/parsers/json"
    "gorm.io/gorm"
)

func main() {
    // Initialize your database connection
    db := /* your gorm.DB instance */
    
    // Load configuration from PostgreSQL
    err := configmanager.Set(
        "myconfig",                                      // config name
        configmanager.NewPgProvider(
            db,
            configmanager.WithPgKey("my_config_key"),    // key in database
        ),
        json.Parser(),                                   // parser for JSON data
        &configmanager.LoadParams{
            EnableReload: false,                         // disable auto-reload
        },
    )
    if err != nil {
        panic(err)
    }
    
    // Retrieve and use configuration
    config, ok := configmanager.Get("myconfig")
	if !ok {
		panic("config not found")
	}
    port := config.Int("server.port")
    host := config.String("server.host")
}
```

### Basic Usage with Redis

```go
package main

import (
    "sentioxyz/sentio-core/common/configmanager"
    "github.com/knadh/koanf/parsers/json"
    "github.com/redis/go-redis/v9"
)

func main() {
    // Initialize Redis client
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    
    // Load configuration from Redis
    err := configmanager.Set(
        "myconfig",
        configmanager.NewRedisProvider(
            rdb,
            configmanager.WithRedisKey("my_config_key"),
        ),
        json.Parser(),
        &configmanager.LoadParams{},
    )
    if err != nil {
        panic(err)
    }
    
    config, ok := configmanager.Get("myconfig")
	if !ok {
		panic("config not found")
	}
    apiKey := config.String("api.key")
}
```

## Auto-Reload Configuration

Enable automatic configuration reloading to pick up changes without restarting your application:

```go
err := configmanager.Set(
    "myconfig",
    configmanager.NewPgProvider(db, configmanager.WithPgKey("my_config_key")),
    json.Parser(),
    &configmanager.LoadParams{
        EnableReload: true,                    // enable auto-reload
        ReloadPeriod: 10 * time.Second,       // check every 10 seconds
    },
)
```

When enabled, the library will:
1. Poll the data source at the specified interval
2. Detect changes by comparing data
3. Automatically reload the configuration when changes are detected
4. Log reload events for monitoring

## Configuration Providers

### PostgreSQL Provider

The PostgreSQL provider reads configuration from a database table.

#### Default Table (`_sentio_configs`)

```go
provider := configmanager.NewPgProvider(
    db,
    configmanager.WithPgKey("config_key"),
)
```

The default table structure:
```sql
CREATE TABLE _sentio_configs (
    key VARCHAR PRIMARY KEY,
    data BYTEA,
    updated_at TIMESTAMP
);
```

#### Custom Table

```go
provider := configmanager.NewPgProvider(
    db,
    configmanager.WithPgTable(
        "custom_configs",    // table name
        "config_name",       // key column
        "config_data",       // data column
    ),
    configmanager.WithPgKey("my_config"),
)
```

#### With Custom Encoder

```go
provider := configmanager.NewPgProvider(
    db,
    configmanager.WithPgKey("config_key"),
    configmanager.WithPgEncoder(configmanager.ConfigEncoderJSON),
)

// Read parsed data directly
data, err := provider.Read() // returns map[string]any
```

### Redis Provider

The Redis provider reads configuration from Redis keys.

```go
provider := configmanager.NewRedisProvider(
    redisClient,
	configmanager.WithRedisCategory("production"),                    // support for multiple categories
    configmanager.WithRedisKey("limiter"),
    configmanager.WithRedisEncoder(configmanager.ConfigEncoderJSON),
)
```

## Parsers

Support for multiple configuration formats:

```go
import (
    "github.com/knadh/koanf/parsers/json"
    "github.com/knadh/koanf/parsers/yaml"
    "github.com/knadh/koanf/parsers/toml"
)

// JSON
configmanager.Set("config", provider, json.Parser(), params)

// YAML
configmanager.Set("config", provider, yaml.Parser(), params)

// TOML
configmanager.Set("config", provider, toml.Parser(), params)
```

## Configuration Access API

The library provides type-safe getters for accessing configuration values:

### String Values

```go
config := configmanager.Get("myconfig")

// Safe access (returns zero value if not found)
host := config.String("server.host")
hosts := config.Strings("server.hosts")
hostMap := config.StringMap("server.region_hosts")

// Must access (panics if not found)
host := config.MustString("server.host")
hosts := config.MustStrings("server.hosts")
hostMap := config.MustStringMap("server.region_hosts")
```

### Numeric Values

```go
// Integer
port := config.Int("server.port")
ports := config.Ints("server.ports")
portMap := config.IntMap("service.ports")

// Int64
id := config.Int64("user.id")
ids := config.Int64s("user.ids")
idMap := config.Int64Map("user.mappings")

// Float64
rate := config.Float64("rate.limit")
rates := config.Float64s("rate.limits")
rateMap := config.Float64Map("rate.mappings")
```

### Boolean Values

```go
enabled := config.Bool("feature.enabled")
flags := config.Bools("feature.flags")
flagMap := config.BoolMap("feature.mappings")
```

### Special Types

```go
// Duration (e.g., "5s", "10m", "1h")
timeout := config.Duration("timeout")

// Time with custom layout
createdAt := config.Time("created_at", time.RFC3339)

// Raw bytes
data := config.Bytes("raw.data")
```

### Nested Configuration

Use dot notation to access nested values:

```json
{
  "database": {
    "primary": {
      "host": "localhost",
      "port": 5432
    }
  }
}
```

```go
host := config.String("database.primary.host")     // "localhost"
port := config.Int("database.primary.port")        // 5432
```

### Custom Delimiter

Change the delimiter for nested keys:

```go
configmanager.Set("config", provider, parser, &configmanager.LoadParams{
    Delim: "/",  // use "/" instead of "."
})

// Access with custom delimiter
value := config.String("database/primary/host")
```

## Advanced Features

### Strict Mode

Enable strict mode to prevent accidental overwrites during reload:

```go
configmanager.Set("config", provider, parser, &configmanager.LoadParams{
    StrictMode: true,  // fail on key conflicts during merge
})
```

### Custom Merge Handler

Implement custom logic when configuration changes:

```go
configmanager.Set("config", provider, parser, &configmanager.LoadParams{
    EnableReload: true,
    ReloadPeriod: 5 * time.Second,
    MergeFunc: func(src, dest map[string]any) error {
        // Custom validation or transformation
		dest["key_v2"] = src["key"]
        
        // Return nil to accept merge, or error to reject
        return nil
    },
})
```

### Load Time Tracking

Track when configuration was last loaded:

```go
config := configmanager.Get("myconfig")
loadedAt := config.LoadAt()
fmt.Printf("Config loaded at: %v\n", loadedAt)
```

### Graceful Shutdown

Properly cleanup resources when shutting down:

```go
defer func() {
    if err := configmanager.Shutdown(); err != nil {
        log.Printf("Error shutting down config manager: %v", err)
    }
}()
```

### Lazy Load

Support for lazy loading of configuration:

```go
func init() {
   configmanager.Register("my_config", provider, parser, params) // register config on init function
}

configmanager.Load("my_config") // load config on demand
```

This will:
- Stop all watch goroutines
- Close all provider connections
- Clear all cached configurations

## Complete Example

```go
package main

import (
    "context"
    "log"
    "time"
    
    "sentioxyz/sentio-core/common/configmanager"
    "github.com/knadh/koanf/parsers/json"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

type AppConfig struct {
    ServerPort int
    ServerHost string
    DBMaxConns int
    Debug      bool
}

func main() {
    // Setup database
    db, err := gorm.Open(postgres.Open("postgres://..."), &gorm.Config{})
    if err != nil {
        log.Fatal(err)
    }
    
    // Load configuration with auto-reload
    err = configmanager.Set(
        "app",
        configmanager.NewPgProvider(
            db,
            configmanager.WithPgKey("production_config"),
        ),
        json.Parser(),
        &configmanager.LoadParams{
            EnableReload: true,
            ReloadPeriod: 30 * time.Second,
        },
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Ensure graceful shutdown
    defer configmanager.Shutdown()
    
    // Access configuration
    config, ok := configmanager.Get("app")
	if !ok {
		log.Fatal("config not found")
	}
    
    appConfig := AppConfig{
        ServerPort: config.Int("server.port"),
        ServerHost: config.String("server.host"),
        DBMaxConns: config.Int("database.max_connections"),
        Debug:      config.Bool("debug.enabled"),
    }
    
    log.Printf("Starting server on %s:%d", appConfig.ServerHost, appConfig.ServerPort)
    
    // Your application logic here
    // Configuration will automatically reload every 30 seconds
}
```

## Best Practices

1. **Use descriptive names**: Give your configs meaningful names for easier debugging
   ```go
   configmanager.Set("auth-service", ...)
   configmanager.Set("analytic-service", ...)
   ```

2. **Handle missing values gracefully**: Use non-Must methods when values are optional
   ```go
   timeout := config.Duration("optional.timeout")
   if timeout == 0 {
       timeout = 30 * time.Second // default
   }
   ```

3. **Set appropriate reload periods**: Balance between responsiveness and load
   - Development: 5-10 seconds
   - Production: 30-60 seconds

4. **Use strict mode in production**: Prevent unexpected configuration overwrites
   ```go
   StrictMode: os.Getenv("ENV") == "production"
   ```

5. **Always call Shutdown**: Ensure resources are properly cleaned up
   ```go
   defer configmanager.Shutdown()
   ```

6. **Monitor reload errors**: Watch logs for reload failures to catch issues early

7. **Version your configs**: Include version fields to track configuration changes
   ```json
   {
       "version": "1.2.3",
       "config": { ... }
   }
   ```

## Thread Safety

All operations are thread-safe. Multiple goroutines can safely:
- Load configurations
- Access configuration values
- Watch for changes

## Error Handling

```go
// Loading errors
err := configmanager.Load("config", provider, parser, params)
if err != nil {
    // Handle database connection issues
    // Handle parsing errors
    // Handle duplicate config names
}

// Access errors (Must* methods panic)
defer func() {
    if r := recover(); r != nil {
        log.Printf("Config access error: %v", r)
    }
}()
value := config.MustString("required.value")
```

## Encoders

Available encoders for parsing configuration data:

- `ConfigEncoderJSON` - JSON format
- `ConfigEncoderYAML` - YAML format
- `ConfigEncoderTOML` - TOML format
- `ConfigEncoderTEXT` - Plain text (not supported for parsing)

## License

Part of the Sentio Core library.

