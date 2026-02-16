# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Important: Pull Request Workflow

**ALWAYS submit changes via pull requests (PRs) - never commit directly to main.**

### Git Shortcuts (defined in `.github/.gitconfig`)

This repository has custom git aliases configured:
- **`git dev <branch-name>`** - Create and switch to `dev/$user/$branch-name`, auto-synced with origin/main
- **`git sync`** - Rebase current branch with origin/main
- **`git pr <number>`** - Checkout a PR locally to `pr/$number`
- **`git dev-clean`** - Clean up your dev/* branches (except current)
- **`git pr-clean`** - Clean up all pr/* branches
- **`git patch <name>`** - Create a patch branch (see `.github/.gitconfig` for details)

### Recommended Workflow

1. Create a dev branch: `git dev my-feature`
2. Make your changes and commit: `git commit -m "description"`
3. Push: `git push` (auto-setup remote is enabled)
4. Create a pull request on GitHub using the printed URL
5. Wait for review and approval before merging

Direct commits to main are not allowed and may be rejected by branch protection rules.

## Build System

This is a **Bazel monorepo** supporting multiple languages (Go, TypeScript/JavaScript, Python, C++). Bazel manages all dependencies, builds, and tests.

### Common Bazel Commands

```bash
# Build everything
bazel build //...

# Build a specific target
bazel build //service/processor:processor
bazel build //common/statemirror:statemirror

# Run tests
bazel test //...                    # All tests (OCI targets are manual, won't run)
bazel test //service/processor/...  # Tests in a package and subpackages
bazel test //common/cache:cache_test  # Single test target

# Run a specific service
bazel run //service/processor:processor

# Clean build artifacts
bazel clean
bazel clean --expunge  # Deep clean including external dependencies

# Update Go dependencies after changing go.mod
bazel run //:gazelle-update-repos

# Regenerate BUILD files (after adding new Go files)
bazel run //:gazelle

# Sync go.mod versions with MODULE.bazel (after upgrading Bazel modules)
bazel run @rules_go//go -- get github.com/grpc-ecosystem/grpc-gateway/v2@v2.27.6

# Update Python requirements lock files
bazel run //:generate_requirements_lock

# Regenerate proto files (after protobuf version upgrades)
bazel run //processor/protos:protos.update_go_pb

# Build OCI images (marked as manual, requires Linux platform)
bazel build //service/launcher:launcher_image
bazel run //service/launcher:launcher_load     # Load image into Docker
bazel run //service/launcher:launcher_push     # Push to registry
```

### Language-Specific Commands

**Go:**
- Tests use `github.com/stretchr/testify` for assertions
- Mock generation uses `gomock`
- After modifying `go.mod`, run `bazel run //:gazelle-update-repos` then `bazel run //:gazelle`

**TypeScript/JavaScript:**
- Uses `pnpm` as package manager (enforced by preinstall hook)
- Node version: 22+ (specified in package.json engines)
- Format code: `pnpm format` (runs prettier)
- Packages are in `packages/` directory (ui-core, ui-web3, chain, etc.)

**Protocol Buffers:**
- Proto files generate code for multiple languages (Go, Python, TypeScript, C++)
- After modifying `.proto` files, rebuild affected targets
- Proto definitions are in `*/protos/` directories
- Uses built-in protobuf toolchain (since protobuf@33.4) with `--@protobuf//bazel/toolchains:prefer_prebuilt_protoc`
- After protobuf version upgrades, regenerate proto files: `bazel run //processor/protos:protos.update_go_pb`

## Architecture Overview

### High-Level Structure

The codebase is organized into several major components:

**`service/`** - Microservices architecture with gRPC APIs:
- `processor/` - Main data processing service that handles blockchain data ingestion and transformation
- `analytic/` - Analytics query service with SQL rewriting capabilities
- `project/` - Project management and configuration
- `launcher/` - Service orchestration and deployment
- `rewriter/` - SQL query rewriting for multi-tenant data access
- `webhook/` - Webhook delivery system

**`driver/`** - Storage and data access drivers:
- `entity/` - Entity storage with ClickHouse backend, handles complex data types including Decimal512
- `timeseries/` - Time-series data storage and querying in ClickHouse
- `subgraph/` - Subgraph indexing support (Ethereum ABI utilities, manifest parsing)

**`common/`** - Shared libraries and utilities:
- `statemirror/` - Type-safe state mirroring system for Redis (supports both RedisMirror and FileMirror)
- `clickhousemanager/` - ClickHouse connection and query management
- `rpccache/` - RPC response caching layer
- `cache/` - Multi-level caching abstractions
- `db/` - Database utilities and connection management
- `log/`, `monitoring/`, `flags/` - Observability and configuration

**`processor/`** - Data processing pipeline:
- Contains proto definitions for processor entities
- Integrates with the service/processor for data transformation

**`network/`** - Network and blockchain abstractions:
- `registry/` - Chain and network registry
- `state/` - State management
- `sqlrewriter/` - SQL rewriting for network-specific data access

**`chain/`** - Blockchain-specific implementations:
- `evm/` - Ethereum Virtual Machine support

**`packages/`** - Frontend TypeScript/JavaScript packages:
- `chain/` - Chain-specific utilities
- `ui-core/`, `ui-web3/` - UI components
- `browser-extension/`, `remix-plugin/` - Browser integrations

### Key Technologies

- **ClickHouse**: Primary analytical database for time-series and entity data
- **Redis**: Caching and state mirroring (see `common/statemirror/`)
- **PostgreSQL**: Transactional data storage (via GORM)
- **gRPC**: Inter-service communication
- **OpenTelemetry**: Distributed tracing and metrics
- **Protocol Buffers**: Service and data schema definitions
- **WASM**: WebAssembly support via wasmer-go

### Data Flow

1. Blockchain data enters through the **processor service**
2. Data is transformed and stored via **drivers** (entity, timeseries)
3. **ClickHouse** stores analytical data
4. **State mirroring** keeps frequently accessed state in Redis
5. **Analytic service** provides query interface with SQL rewriting for multi-tenancy
6. **Webhooks** deliver events to external systems

## Development Workflow

### Adding New Code

1. **Go packages**: Add files, then run `bazel run //:gazelle` to update BUILD files
2. **Go dependencies**: Update `go.mod`, then `bazel run //:gazelle-update-repos` and `bazel run //:gazelle`
3. **Proto changes**: Modify `.proto` files, rebuild will auto-generate code
4. **TypeScript packages**: Use pnpm workspace, dependencies managed in package.json files

### Testing

- Tests are co-located with source files (e.g., `foo.go` has `foo_test.go`)
- Use `bazel test //path/to/package:target_test` for specific tests
- Integration tests may use embedded databases (see `fergusstrange/embedded-postgres`)
- Test output is streamed by default (see `.bazelrc`)

### Configuration

- Most services use `koanf` for configuration management
- Environment-specific config via flags (see `common/flags/`)
- Workspace status is tracked via `workspace_status.sh` (git commit, branch, version)

### Cross-Compilation

- Linux AMD64 binaries can be built on macOS via hermetic Zig toolchain
- Use `--platforms=//:linux_amd64` for cross-compilation
- CI builds use `--config=ci` (see `.bazelrc`)

## Important Patterns

### State Mirroring

When working with on-chain state that needs Redis caching, use the `statemirror` package (see `common/statemirror/README.md`):
- `TypedMirror[K, V]` provides type-safe access
- `JSONCodec` handles serialization
- Choose `Upsert` for small datasets, `UpsertStreaming` for large datasets
- `RedisMirror` for production, `FileMirror` for development/testing

### ClickHouse Queries

- Use `clickhousemanager` for connection pooling and query execution
- Time-series data follows specific schema patterns (see `driver/timeseries/`)
- Entity data supports complex types including Decimal512 (see `driver/entity/clickhouse/`)

### Service Implementation

- Services expose gRPC APIs defined in `*/protos/`
- Use `grpc-ecosystem/go-grpc-middleware` for common middleware
- OpenTelemetry instrumentation is standard (otelgrpc, otelhttp)

## Dependency Management

### Bazel Module Upgrades

- Check latest versions at [Bazel Central Registry](https://registry.bazel.build/)
- Bazel Central Registry MCP server is configured in `.mcp.json` for easy module discovery
- After upgrading modules in MODULE.bazel, sync Go dependencies:
  ```bash
  bazel run @rules_go//go -- get <package>@<version>
  ```
- Watch for version mismatches between MODULE.bazel and go.mod (Gazelle will warn)

### Known Compatibility Issues

- **Bazel 9**: Not yet supported due to `rules_foreign_cc` incompatibility (pulled in via grpc → opencensus-cpp → google_benchmark → libpfm)
- **OCI Images**: Container image builds require Linux platform. OCI targets (in `service/launcher/BUILD.bazel`) are marked as `manual` and must be built explicitly with `bazel build //service/launcher:launcher_image`
- **gRPC Python**: Latest grpc versions may require Python versions not yet in rules_python

## Notes

- The repository uses a custom git config (`.github/.gitconfig`) and hooks (`.github/.githooks/`)
- Some Go dependencies have custom patches (see `third_party/` and MODULE.bazel)
- The `nogo` static analyzer runs on all Go code (config in `nogo-config.json`)
- Bazel disk cache is in `~/.cache/bazel-disk` (max 50GB)
- Current Bazel version: 8.5.1 (see `.bazelversion`)
