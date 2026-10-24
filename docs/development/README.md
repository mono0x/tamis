# Development

[README](../../README.md) · [Releasing](releasing.md) · [Repository instructions](../../AGENTS.md)

## Build from a checkout

The toolchain and task definitions live in [mise.toml](../../mise.toml).

```sh
mise trust
mise install
mise run build
```

## Verify changes

```sh
mise run test
mise run check
```

`test` runs all Go tests with the race detector on the host OS. `check` checks formatting, performs static analysis for `darwin/arm64` and `linux/amd64`, verifies dependencies, and validates the release configuration. It does not rewrite files. Run `mise run fix` to apply formatting fixes.

For a focused test or benchmark:

```sh
mise exec -- go test ./internal/meta -run TestSyntheticFixtures
mise run bench
```

## Work with test media

Development must never modify, move, copy, rename, trash, or delete real media or other user data. Never run `tamis apply` against real sources or destinations. Mutation tests must use `t.TempDir()` or an explicitly created directory under the repository or system temporary directory. Read [Real-data safety](../../AGENTS.md#real-data-safety) before working with files.

Committed media fixtures are minimal synthetic files generated from constants:

```sh
mise exec -- go run ./testdata/generate
```

See [Synthetic fixtures](../../testdata/samples/README.md). Do not copy real media or metadata into fixtures, logs, or documentation.

ExifTool is an optional development oracle, separate from runtime metadata extraction:

```sh
mise run verify-meta
mise run verify-meta-diff
```

`verify-meta` runs the `exiftool`-tagged parity test, which currently skips as a placeholder. `verify-meta-diff` compares scan output with ExifTool over `testdata/samples` and requires ExifTool and `jq`. Use synthetic fixtures only.

See [Releasing](releasing.md) for manual releases, versioning, and local snapshot builds.
