# Tamis

[日本語](README.ja.md)

Tamis is a fast CLI for organizing images and videos with metadata-driven YAML rules. It can rename, copy, move, or trash files and fan one input out to multiple destinations.

Use `preview` to preview operations and `apply` to execute them directly, without an intermediate plan file. It runs as a single binary, uses pure Go metadata parsers, and requires no ExifTool installation. External commands run only through explicitly configured `shell` actions.

## Installation

With [mise](https://mise.jdx.dev/dev-tools/backends/github.html), add the following to your `mise.toml`:

```toml
[tools]
"github:mono0x/tamis" = "latest"
```

Then install and check the version:

```sh
mise install
mise exec -- tamis --version
```

Download the macOS or Linux archive for your architecture (`amd64` or `arm64`) from [GitHub Releases](https://github.com/mono0x/tamis/releases), extract it, and put `tamis` on your `PATH`. Each release includes `checksums.txt` with SHA-256 checksums. Check the installed version with `tamis --version`.

To install from source, Go 1.27.3 or later is required:

```sh
go install github.com/mono0x/tamis/cmd/tamis@latest
```

## Quick start

Copy [config.example.yaml](config.example.yaml) to your own configuration:

```sh
cp config.example.yaml organize.yaml
```

Edit the input and destination paths in `organize.yaml`. Then validate the configuration and preview the operations:

```sh
tamis check ./organize.yaml
tamis preview ./organize.yaml
```

`preview` scans inputs and validates the planned operations without changing files. To inspect all extracted metadata, use `tamis scan --config ./organize.yaml`.

After reviewing the preview, execute the configuration:

```sh
tamis apply ./organize.yaml
```

`apply` scans again, validates all operations, and executes **without confirmation**. Files present at execution time determine the operations. Existing destinations are never overwritten; the conflict policy is `rename`, `skip`, or `fail`. Errors stop execution; completed copies and shell edits are not rolled back.

## Configuration

Rules use [Expr](https://expr-lang.org/) conditions and `{{ ... }}` path templates. This rule copies images into folders by year and month, leaving the originals in place:

```yaml
version: 1

settings:
  on_conflict: rename

rules:
  - name: archive-images
    location: ~/Media/Inbox
    recursive: true
    when: 'kind == "image" && captured != nil'
    actions:
      - copy: '~/Media/Archive/{{captured | date "2006/01"}}/'
        rename: '{{captured | date "20060102_150405"}}_{{basename}}{{ext}}'
```

Actions are `shell`, `copy`, `move`, and `trash`. Metadata used in action templates must be referenced in the same rule's `when`. See the [YAML configuration reference](docs/configuration/README.md) for fields, defaults, action order, and template filters.

## Commands

```text
tamis scan PATH            Extract metadata as NDJSON
tamis preview CONFIG       Preview validated operations without changing files
tamis apply CONFIG         Validate and execute operations
tamis explain CONFIG FILE  Explain metadata, matching rules, and destinations for one file
tamis check CONFIG         Validate the configuration
tamis doctor CONFIG        Report providers and configured rule locations
```

Pass the configuration file explicitly as `CONFIG`; Tamis does not search `~/.config` or `$XDG_CONFIG_HOME`. Paths inside the configuration remain relative to the working directory.

`scan PATH` scans a directory without a configuration. To scan rule locations, use `scan --config CONFIG` instead; do not combine it with `PATH`. Common flags are `--jobs` and `--json`.

## Documentation

| Task                                                              | Document                                                     |
| ----------------------------------------------------------------- | ------------------------------------------------------------ |
| Write rules, conditions, and templates                            | [YAML configuration reference](docs/configuration/README.md) |
| Check scan exclusions, execution guarantees, and metadata support | [Scanning, execution, and metadata](docs/behavior/README.md) |
| Edit metadata before copying                                      | [Shell actions](docs/configuration/README.md#shell)          |
| Configure the system trash or a holding directory                 | [Trash](docs/configuration/README.md#trash)                  |
| Build and test from a checkout                                    | [Development](docs/development/README.md)                    |
| Publish or verify release archives                                | [Releasing](docs/development/releasing.md)                   |

Tamis supports JPEG, TIFF, PNG, WebP, HEIC/HEIF, AVIF, DNG, CR2, CR3, CRW, NEF, PEF, ARW, RW2, MOV, and MP4, with sidecar and filesystem timestamp fallbacks. See [metadata support and limits](docs/behavior/README.md#metadata) for details.

Tamis intentionally does not provide `watch`, `undo`, backup, journal, built-in metadata writers, duplicate detection, or cloud integration.
