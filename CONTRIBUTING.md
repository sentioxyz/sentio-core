# Contributing to sentio-core

Thanks for your interest in improving sentio-core. Bug reports, small fixes and larger features
are all welcome.

## Before you start

- For anything non-trivial, open an issue first so we can agree on the approach before you write
  code. This avoids duplicated work — parts of this repo are driven by internal roadmap work.
- For security issues, do **not** open an issue or PR; follow [SECURITY.md](SECURITY.md).
- By contributing you agree that your contribution is licensed under the
  [Apache License 2.0](LICENSE), per section 5 of the license.

## Development setup

Install [Bazelisk](https://github.com/bazelbuild/bazelisk) (it picks up the Bazel version pinned
in `.bazelversion`), Python 3.11, and Node 22+ with [pnpm](https://pnpm.io). Bazel provides the
remaining toolchains.

```bash
bazel build //...   # build everything
bazel test //...    # run all tests
```

`AGENTS.md` documents the rest of the build system: per-language commands, protobuf regeneration,
OCI image targets and dependency upgrades. A few rules worth repeating:

- `BUILD.bazel` files are generated. After adding or removing Go files or dependencies, run
  `scripts/deps-update.sh` and commit the regenerated `BUILD.bazel` / `MODULE.bazel` / `go.mod`
  changes with your source changes — do not hand-edit them.
- Go tests use [testify](https://github.com/stretchr/testify); tests live next to the code
  (`foo.go` → `foo_test.go`).
- TypeScript/JS/YAML/JSON is formatted with Prettier: `pnpm format`.
- The `nogo` static analyzer (`nogo-config.json`) runs over all Go code as part of the build.

## Pull requests

1. Branch off `main`; never commit to `main` directly.
2. Keep the change focused. Unrelated refactors make review slower.
3. Add or update tests for behaviour changes.
4. Make sure `bazel test //...` passes locally for the packages you touched.
5. Use a [Conventional Commits](https://www.conventionalcommits.org/) PR title — it is enforced
   by `.github/semantic.yml`, and the scope is usually the top-level package, e.g.
   `fix(driver): ...`, `feat(chain): ...`, `chore(deps): ...`.
6. Fill in the PR template: what changed, why, and how you verified it.
7. A maintainer review and green CI are required before merge.

Note for external contributors: CI runs on self-hosted runners, so a maintainer has to approve
workflow runs on PRs from forks. Expect a short delay before checks appear.

## Reporting bugs

Open an issue with the reproduction steps, the chain/network involved if relevant, expected vs
actual behaviour, and any log output. Sentio project URLs and processor versions are very helpful
for platform-level reports — see the
[support guide](https://docs.sentio.xyz/docs/getting-support).
