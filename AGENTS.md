# Repository instructions

Tamis organizes real photo and video libraries in place, so mistakes destroy user data. Read "Real-data safety" before touching anything.

## Real-data safety

- Never modify, move, copy, rename, trash, or delete real media or other user data during development, and never run `tamis apply` against a real source or destination.
- Prefer library documentation, format specifications, and synthetic fixtures over inspecting real data. Reading real-file metadata is allowed only when it is necessary to decide what the application should support.
- Never copy real files into the repository or into temporary fixtures, and never record, print, paste, or otherwise transfer values read from real metadata into source code, tests, fixtures, logs, documentation, commits, or responses.
- Mutation tests must operate only inside `t.TempDir()` or an explicitly created directory under the repository or system temporary directory.
- Test media committed to Git must be minimal synthetic files generated reproducibly from constants (`go run ./testdata/generate`) and contain only meaningless data.

## Commands

The toolchain and development tasks are defined in `mise.toml` (`mise trust && mise install` once).

```sh
mise run build         # CGO_ENABLED=0 static build of ./cmd/tamis
mise run test          # gotestsum -race ./... on the host OS
mise run fix           # hk fix --all (dprint, gofmt, pkl, zizmor) — run before committing
mise run lint          # go vet + staticcheck + errcheck + govulncheck for both targets
hk check --all         # OS-independent: format + pkl + zizmor, no writes
go mod verify         # OS-independent: verify dependency modules
mise run bench         # go test -run '^$' -bench . -benchmem ./...
mise run run -- scan … # go run ./cmd/tamis

go test ./internal/meta -run TestSyntheticFixtures   # single test
go test -race ./internal/rule                        # single package
```

Metadata parity against ExifTool is a development-only oracle, gated behind the `exiftool` build tag:

```sh
mise run verify-meta        # go test -tags exiftool ./internal/meta -run TestExiftoolParity
mise run verify-meta-diff   # diffs `tamis scan` vs `exiftool -j -G -n -r` over testdata/samples
```

`hk check --all` runs OS-independent checks. `mise run lint` runs `go vet`, staticcheck, errcheck, and govulncheck for both `darwin/arm64` and `linux/amd64`, with `CGO_ENABLED=0`, regardless of the host OS. `scripts/check-go.sh` owns these analysis targets. Go analysis runs only through mise tasks (`mise run lint:vet` for go vet alone), not through hk or Git/editor hooks.

CI (`.github/workflows/ci.yml`) runs `hk check --all`, `go mod verify`, and `mise run lint` in separate steps once on Ubuntu, and `mise run test` and `mise run build` on both macOS and Ubuntu. A separate `runtime-exec-boundary` job permits `"os/exec"` only in `internal/apply/shell.go`.

## Architecture

The internal pipeline is `scan → plan → apply`. The CLI exposes `preview` (scan, evaluate, validate, preview) and `apply` (the same steps, then execute without confirmation); neither requires a plan file.

```
scan.Walk → scan.Extract (meta.Chain) → []meta.Record   [filesystem I/O]
rule.Compiled.Evaluate([]meta.Record) → plan.Plan        [pure, no I/O]
apply.Validate(plan.Plan) → apply.Run                    [filesystem I/O]
```

**`internal/meta`** — pure-Go metadata extraction. `meta.NewRecord(path)` builds a full diagnostic record; the chain’s selective path always stats once, sniffs only when required, and reads date_added only when requested; `Provider` is `Supports(Format) + Provides() []string + Extract(ctx, base *Record, reader io.ReadSeeker) (*Record, error)`, where `base` is that shared record — providers must treat it as read-only and work on `base.clone()`, never re-stat or re-sniff. The chain opens the source once, resets the shared reader to the start before each provider, and closes it on return; providers must not close or retain the reader. Sidecars open their own files. `Provides` declares the fields a provider can populate (the sets live in `fields.go`), and is the only thing the chain consults to skip it; no provider list is enumerated anywhere else. `fields.go` also owns the metadata vocabulary — the `meta.Field*` constants and `meta.Vocabulary` — which `Fields` is keyed by and which the `expr:` tags on `rule.Env` must spell exactly (`TestEnvTagsMatchVocabulary`). Never write one of these names as a literal; a drifting name compiles cleanly and silently disables extraction for `preview`/`apply`. `meta.Chain` (`chain.go`) builds the base and walks providers in order — `ImageProvider` (bep/imagemeta: JPEG/TIFF/PNG/WebP/HEIC/AVIF/DNG/CR2/NEF/PEF/ARW) → `ImageExtProvider` (evanoberholster/imagemeta: CR3/CRW/RW2) → `PNGTextProvider` → `VideoProvider` (abema/go-mp4) → `VideoSidecarProvider` (XMP/THM/same-name JPEG) → `FSProvider` (birth time, mtime) — skipping unneeded providers and stopping at the first provider that yields a `Captured` timestamp (or after extraction when no timestamp is requested) and merging `Raw`/`Warnings` from earlier attempts. `meta.NewChain(assume, mvhd)` is the only place that chain is composed. If a requested specialized provider errored before the fallback succeeded, `Record.ExtractionFailed` is set and the planner skips the file and leaves it untouched. `safeExtract` recovers third-party parser panics into errors and is the single recovery point, wrapping both each provider call and `Chain.Extract` as a whole; every provider must stay panic-contained. The base record carries no `Captured`: the filesystem fallback belongs to `FSProvider`, so no other provider has to clear it. `DetectFormatBytes` sniffs magic bytes (ftyp brands for HEIC/AVIF/CR3/MOV/MP4) with extension as a tiebreaker. Video extraction must never read `mdat` or load a whole file into memory.

`ImageProvider` does not combine separate EXIF `SubSecTimeOriginal`/`SubSecTimeDigitized` tags into timestamps or reparse files to recover fractional digits. Other parsers may retain native fractions, but subsecond precision is not guaranteed across formats.

**`internal/scan`** — directory walk with glob include/exclude, then a bounded worker pool (`jobs`, default `NumCPU`) with a per-file timeout. Results are sorted for deterministic output.

**`internal/rule`** — config loading, compilation, and evaluation.

- `config.go`: strict YAML decode (`KnownFields(true)`), defaults, and validation. The config's SHA-256 becomes `Config.Hash` and is included in the Plan's `config_hash`. Each rule owns its required `location`, optional `recursive`/`include`/`exclude`, and unique `name`; there is no separate sources list. CLI scanning and evaluation use all configured rules. Identical scan scopes are walked once and overlapping files are extracted once; evaluation uses each rule’s relative path and preserves per-file terminal behavior. Structural rules live here: exactly one action kind per action; `move` or `trash` must be the last action in a rule. Compilation makes either terminal action stop further rule evaluation for that input automatically; copy-only rules can still use explicit `stop: true`.
- `fields.go`: statically collects `when` references, requires action metadata references to be declared in the same rule, and unions requirements for matching scan scopes and filename captures without I/O. `preview`/`apply` pass these requirements to the chain; `scan`/`explain` retain full diagnostic extraction.
- `rule.go`: `compile` builds the Expr programs (`expr.Env(Env{})`, `MaxNodes(200)`), capture regexps, and path templates **once** at load time. `Evaluate` is the pure core — it must never touch the filesystem. All records with no matching rule land in `Plan.Skipped`, regardless of whether they have a `captured` time.
- `template.go`: the `{{ ... }}` mini-language (not `text/template`) with filters `date`, `slug`, `lower`, `upper`, `pad`, `default`, `trunc`, `slice`. Templates accept a subset of the vocabulary — every list naming variables (`envValue`, `multiComponent`, `validateFields`) uses the `meta.Field*` constants, so a name exists in exactly one place.

**`internal/plan`** — the wire format. NDJSON with one `plan` header line followed by `item` and `skip` lines, so plans stream and diff cleanly. It also owns the destination filename budget (`MaxNameBytes`, `FitName`), shared by the planner that builds names and the applier that resolves them into `name-2.ext`.

**`internal/apply`** — `Validate` runs over the whole Plan before any mutation: source existence, absolute destinations, destination-is-another-source cycles, no operation after a move/trash for the same input, conflict resolution (`rename` picks `name-2.ext`, honoring in-plan reservations), write access, supported operation kinds (`shell`, `copy`, `move`, `trash`), and per-device free-space accumulation. On macOS and Linux, trash with an empty destination uses github.com/mono0x/trash (Foundation or GIO); native libraries and trash availability are checked only during execution; the OS chooses the destination at execution, so destination checks and capacity budgeting do not apply to native trash. An explicit `--trash-dir` retains ordinary move validation. Copies budget full size because cloning is optional. `--force` can downgrade destination and capacity errors to warnings; unsupported actions, source errors, and conflicts still stop execution. `Run` executes ops and records skips. Any skipped copy prevents the final move/trash for that input, including across Plan items, while other copies continue. Errors stop execution and leave completed copies in place; `copyFile` prepares a file in a private temporary directory under the destination parent: clone on macOS, or byte copy when unsupported/cross-volume. Both paths preserve xattrs, mtime, and mode, sync the file, rename exclusively, and sync the destination parent. Platform-specific behavior is split by build tag (`clone_darwin.go` uses `fclonefileat`, `xattr_darwin.go`, `birthtime_*.go`), with no-op `_other.go` counterparts.

`shell` actions precede all copies for an input and edit the original in place. Command text is literal; only environment values are templated. `TAMIS_FILE` is reserved for the absolute source path. After successful execution, refresh the source identity for remaining operations without reevaluating rules. Shell failures stop the run and do not roll back source edits. Commands run as the user, without a sandbox, with a five-minute timeout.

**`cmd/tamis`** — Cobra commands (`scan`, `preview`, `apply`, `check`, `explain`, `doctor`) plus the wiring helpers `provider`, `extractRules`, and the human-readable renderers. `preview`, `apply`, `check`, and `doctor` require a CONFIG positional argument; `explain` takes CONFIG FILE. No configuration is loaded implicitly. `scan` accepts a bare PATH argument, which `rule.New` turns into a synthetic single-rule config so it inherits the same settings defaults, or an explicit `--config CONFIG` to scan rule locations.

## Invariants

- `os/exec` is permitted only in `internal/apply/shell.go` for explicit shell actions. Metadata extraction and ordinary file operations never start external processes. CI enforces this boundary.
- Rule evaluation (`rule.Evaluate` and everything it calls) performs no filesystem I/O.
- Metadata libraries are reached only through `meta.Provider`, and parser panics are recovered at the provider boundary.
- Existing destinations are never overwritten — conflicts resolve via `rename`, `skip`, or `fail`.
- Out of scope unless the user explicitly changes the scope: `watch`, `undo`, backup, journal, built-in metadata writers, duplicate detection, cloud integration.
- Add or update tests with behavior changes, using synthetic fixtures only.
