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

```sh
cp config.example.yaml organize.yaml

tamis check ./organize.yaml
tamis scan --config ./organize.yaml
tamis preview ./organize.yaml
tamis apply ./organize.yaml
```

Edit the source and destination paths in the configuration before scanning. `preview` scans sources and validates the planned operations without changing files. `apply` scans again, validates all operations, and executes them without confirmation. Its operations reflect the files present at execution time. Existing destinations are never overwritten; choose `rename`, `skip`, or `fail` in the configuration. `--force` does not bypass destination conflicts. If a new conflict appears during execution, the operation stops without overwriting the destination.

`preview` and `apply` extract metadata required by `when`. Action templates may use timestamps and camera metadata only when referenced in the same rule’s `when`. `scan` and `explain` extract full metadata for diagnostics.

Scanning includes regular files only; dotfiles, dot directories and their descendants, symlinks, and special files such as FIFOs are excluded. Before execution, Tamis checks that sources still match their scanned identity, size, and modification time. A changed source or a new rename conflict stops execution; displayed destinations are never silently renamed again.

When a rule has `include` patterns, scanning skips directories that cannot contain a matching file. For example, `Untitled*/PRIVATE/M4ROOT/CLIP/*` under `/Volumes` only explores the specified hierarchy on volumes matching `Untitled*`. Patterns containing `**` retain recursive exploration beneath that wildcard.

Missing rule locations are treated as having no matching files. If part of an existing source is unreadable, `scan` emits the readable records and exits with an error. `preview` and `apply` reject an incomplete scan. Cross-volume moves synchronize the copied file and destination directories before removing the source. On macOS, failure to preserve extended attributes stops the copy so a move cannot silently discard them.

## Configuration

See the [YAML configuration reference](docs/configuration/README.md) for every field, default, condition variable, and template filter.

See [config.example.yaml](config.example.yaml) for a complete example. Rules use [Expr](https://expr-lang.org/) conditions and `{{ ... }}` path templates.

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

Actions are `shell`, `copy`, `move`, and `trash`. `shell` actions must precede the first copy for each input, including across rules. For each input file, any number of `copy` actions may precede at most one `move` or `trash`. A `move` or `trash` must be the last action in its rule and automatically ends rule evaluation for that input; later rules are not evaluated. Copy-only rules continue to later rules unless `stop: true` is set. Available template filters are `date`, `slug`, `lower`, `upper`, `pad`, `default`, `trunc`, and `slice`.

A final `move` or `trash` runs only if every preceding copy for that input succeeded, including copies from earlier rules. If any copy is skipped because its destination exists, remaining copies still run, but the final move/trash is skipped and the source remains. Existing files are not compared for equality. `preview` warns when an existing copy destination will cause this; displayed move/trash operations remain conditional on copy success. `apply` reports skip reasons on stderr and includes skipped operations in the JSON result’s `skipped` array. An error stops execution; completed copies remain and are not rolled back.

`copy` automatically tries cloning on macOS, falling back to a normal copy when cloning is unsupported or crosses volumes. Linux uses normal copying. Both paths preserve file mode, extended attributes (on macOS), and mtime, synchronize the temporary file, and publish it without overwriting an existing destination. Other clone errors stop the operation. Free-space validation always budgets a full copy, even when cloning might use less space. `clone`, `hardlink`, and `symlink` are no longer supported actions; use `copy` for independent files.

In destination templates, metadata such as `model` and `make` must form a single path component; use `{{model | slug}}` for values containing separators. `path`, `relpath`, and date layouts may span directories, but interpolated `.` and `..` components are rejected.

On macOS and Linux, `trash` uses [github.com/mono0x/trash](https://github.com/mono0x/trash) to move files to the system trash without external commands or CGO. macOS uses Foundation; Linux requires `libgio-2.0.so.0` at runtime and uses GIO to manage trash locations and restore metadata. The OS chooses the destination and resolves filename collisions; `preview` displays `system Trash` and JSON omits `dest`. Native libraries and trash availability are checked only during execution, and destination capacity is not checked in advance. A failure stops the run and leaves completed copies in place; files are never directly deleted as a fallback. Finder's Put Back behavior is not guaranteed. Use `preview CONFIG --trash-dir PATH` and `apply CONFIG --trash-dir PATH` to move to an explicit holding directory instead, with destinations displayed and validated before execution. Explicit holding directories do not record desktop restore information. There is no YAML `trash_dir` setting.

The example configuration copies images and videos into separate folders organized by year and month. Metadata editing is available through explicit shell actions.

## Shell actions

A `shell` action edits the original file before copying it. Tamis provides the absolute input path in `TAMIS_FILE`. The following fragment writes a PNG capture time parsed from its filename, then copies and trashes the input. ExifTool is needed only for this example; the command can use any installed tool or script.

```yaml
capture: '^.+? (?P<year>\d{4})_(?P<month>\d{2})_(?P<day>\d{2}) (?P<hour>\d{1,2})_(?P<minute>\d{2})_(?P<second>\d{2})\.png$'
actions:
  - shell:
      run: |
        exiftool -P -overwrite_original \
          "-EXIF:DateTimeOriginal=$CAPTURED" \
          "-EXIF:OffsetTimeOriginal=+09:00" \
          "$TAMIS_FILE"
      env:
        CAPTURED: "{{capture.year}}:{{capture.month}}:{{capture.day}} {{capture.hour | pad 2}}:{{capture.minute}}:{{capture.second}}"
  - copy: "~/Media/Archive/"
  - trash: true
```

`run` is executed literally by `/bin/sh -eu -c`; templates are expanded only in `env` values. Quote environment variables in the script. Commands inherit the working directory and environment, receive no stdin, and have a five-minute timeout. stdout is discarded; up to 16 KiB of stderr is included on failure. Background tasks are unsupported and the process group is terminated on completion or cancellation.

`preview` and `explain` display the command and rendered environment without executing it. Planning uses metadata scanned before the shell runs: writing a new capture time does not change later rule conditions or destinations during that run. Use filename captures in destination templates when needed.

After a successful command, the input must still be a regular file. Tamis refreshes its identity, size, and mtime before later operations, so commands that replace the file are supported. A command failure stops processing; source edits already made by the command are not rolled back. A later copy skip also leaves the edited source in place. Shell-only rules are allowed. Shell runs with the user's permissions and is not sandboxed; only run trusted configurations.

Assigning the same fixed timestamp and offset is idempotent for those tag values. Time shifts such as `+=` are not. Rewriting can still change filesystem attributes or file identity; `-P` preserves mtime in the example. [ExifTool documentation](https://exiftool.org/exiftool_pod2.html)

## Commands

```text
tamis scan PATH             Extract metadata as NDJSON
tamis preview CONFIG       Preview validated operations without changing files
tamis apply CONFIG         Validate and execute operations
tamis explain CONFIG FILE  Explain metadata, matching rules, and destinations for one file
tamis check CONFIG         Validate the configuration
tamis doctor CONFIG        Report providers and configured rule locations
```

Pass the configuration file explicitly as `CONFIG`; Tamis does not search `~/.config` or `$XDG_CONFIG_HOME`. Paths inside the configuration remain relative to the working directory.

`scan PATH` scans a directory without a configuration. To scan rule locations, use `scan --config CONFIG` instead; do not combine it with `PATH`. Common flags are `--jobs` and `--json`.

## Supported metadata

- JPEG, TIFF, PNG, WebP, HEIC/HEIF, AVIF, DNG, CR2, NEF, PEF, and ARW
- CR3, CRW, and RW2
- MOV and MP4, including QuickTime creation dates, `©day`, embedded XML, and `mvhd`
- PNG `eXIf`, `tEXt`, and `iTXt` creation dates
- XMP, THM, and same-name JPEG sidecars
- Filesystem birth time and modification time as final fallbacks

Subsecond precision is not guaranteed. For images other than CR3/CRW/RW2, separate EXIF `SubSecTimeOriginal` and `SubSecTimeDigitized` tags are not combined with the capture time; standard EXIF dates therefore have whole-second precision. Tamis does not reparse files to recover fractional digits. Fractions already returned by other timestamp parsers (such as XMP or the CR3/CRW/RW2 decoder) may be retained. `raw` contains decoder values and does not guarantee the original subsecond digit strings. Do not rely on subseconds for ordering shots or generating unique filenames; use `on_conflict` to handle filename collisions.

Video extraction skips `mdat`, and metadata work runs through a bounded worker pool. Expressions and regular expressions compile once when the configuration is loaded.

XMP sidecars are limited to 8 MiB and PNG text chunks to 1 MiB. Decoder reads and seeks check cancellation; the timeout is cooperative. Filesystem birth time or mtime remains the final timestamp fallback and can be distinguished using `captured_source` in rules.

Tamis intentionally does not provide `watch`, `undo`, backup, journal, built-in metadata writers, duplicate detection, or cloud integration.

The project builds for macOS and Linux with `CGO_ENABLED=0`.
