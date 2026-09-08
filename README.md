# sentio-core

Core libraries and services behind [Sentio](https://sentio.xyz) — a multi-chain blockchain
indexing, analytics and observability platform.

This repository holds the shared, lower-level building blocks: chain clients, storage drivers,
protobuf/gRPC contracts, and the services that turn raw blockchain data into queryable
timeseries and entities. It is a Bazel monorepo spanning Go, TypeScript, Python and C++.

## Repository layout

| Path | Contents |
| --- | --- |
| `chain/` | Chain-specific clients and codecs (EVM, Sui, Aptos, Solana, Move, Fuel) and the RPC `clientpool` |
| `driver/` | Indexing drivers: `controller/`, `entity/` and `timeseries/` (ClickHouse-backed), `subgraph/` |
| `service/` | gRPC services: `processor/`, `analytic/`, `project/`, `rewriter/`, `webhook/`, `observability/`, `launcher/` |
| `common/` | Shared libraries: ClickHouse management, caching, state mirroring, logging, monitoring, flags |
| `network/` | Sentio Network indexer/processor registry and shared network state |
| `processor/` | Protobuf definitions and helpers for the processor data pipeline |
| `packages/` | TypeScript packages, including [`@sentio/chain`](https://www.npmjs.com/package/@sentio/chain) and the `ui-*` component libraries |
| `tools/`, `scripts/`, `third_party/` | Build tooling, dependency scripts and vendored patches |

Key technologies: ClickHouse (analytical storage), PostgreSQL (transactional), Redis
(cache/state mirror), gRPC + protobuf, OpenTelemetry, and WebAssembly via wasmer-go.

## Building

Requirements: [Bazelisk](https://github.com/bazelbuild/bazelisk) (the pinned Bazel version is in
`.bazelversion`), Python 3.11, Node 22+ with [pnpm](https://pnpm.io). Bazel manages everything
else, including Go and Node toolchains.

```bash
bazel build //...   # build everything
bazel test //...    # run all tests
```

Build a single target or run a service:

```bash
bazel build //service/processor:processor
bazel run //service/processor:processor
```

More build, codegen and dependency-management commands are documented in
[AGENTS.md](AGENTS.md); contribution workflow is in [CONTRIBUTING.md](CONTRIBUTING.md).

## Related repositories

- [sentio-sdk](https://github.com/sentioxyz/sentio-sdk) — TypeScript SDK for writing processors
- [sentio-processors](https://github.com/sentioxyz/sentio-processors) — example processors
- [typemove](https://github.com/sentioxyz/typemove) — TypeScript bindings for Move chains
- [docs](https://github.com/sentioxyz/docs) — user-facing documentation

## Support

Questions and bug reports: [open an issue](https://github.com/sentioxyz/sentio-core/issues), ask
on [Telegram](https://t.me/sentioxyz), or email support@sentio.xyz. To report a security
vulnerability, follow [SECURITY.md](SECURITY.md) instead of filing a public issue.

## License

Apache License 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE). Some TypeScript packages under
`packages/` declare their own license in their `package.json`.
