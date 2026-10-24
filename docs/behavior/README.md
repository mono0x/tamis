# Scanning, execution, and metadata

[日本語](README.ja.md) · [README](../../README.md) · [YAML configuration reference](../configuration/README.md)

This reference describes which files Tamis scans, how it executes operations, and the limits of metadata extraction. For installation and a first configuration, start with the [README](../../README.md#quick-start).

## Contents

- [Scanning](#scanning)
- [Execution](#execution)
- [Metadata](#metadata)

## Scanning

Only regular files are scanned. Dotfiles, dot directories and their descendants, symlinks, and special files such as FIFOs are excluded. Explicit include patterns do not override these exclusions. See [locations and glob patterns](../configuration/README.md#locations-and-glob-patterns) for pattern syntax and rule scopes.

When a rule has `include` patterns, scanning skips directories that cannot contain a matching file. For example, `Untitled*/PRIVATE/M4ROOT/CLIP/*` under `/Volumes` only explores the specified hierarchy on volumes matching `Untitled*`. Patterns containing `**` retain recursive exploration beneath that wildcard.

Missing rule locations are treated as having no matching files. If part of an existing source is unreadable, `scan` emits the readable records and exits with an error. `preview` and `apply` reject an incomplete scan.

`preview` and `apply` extract metadata required by `when`. Action templates may use timestamps and camera metadata only when referenced in the same rule's `when`. `scan` and `explain` extract full metadata for diagnostics. See [rules and evaluation order](../configuration/README.md#rules-and-evaluation-order) for metadata selection and rule matching.

## Execution

`preview` scans inputs and validates the whole plan without changing files or executing shell commands. `apply` scans again, validates all operations, and executes without confirmation. Operations reflect the files present at execution time; a preview is not a saved plan.

Before execution, Tamis checks that sources still match their scanned identity, size, and modification time. A changed source stops execution. Existing destinations are never overwritten, including with `--force`. A new conflict during execution stops the run without silently choosing a different destination from the validated plan.

A final `move` or `trash` runs only if every preceding copy for that input succeeded, including copies from earlier rules. If any copy is skipped because its destination exists, remaining copies still run, but the final move/trash is skipped and the source remains. Existing files are not compared for equality. `preview` warns when an existing copy destination will cause this; displayed move/trash operations remain conditional on copy success. `apply` reports skip reasons on stderr and includes skipped operations in the JSON result's `skipped` array.

Errors stop execution; completed copies and shell edits are not rolled back. A shell action can edit the original before copying, and a later copy skip leaves that edited source in place. See [shell actions](../configuration/README.md#shell) for execution constraints and an example.

Copies preserve mode and mtime, and extended attributes on macOS. They synchronize the temporary file and publish it without overwriting an existing destination. Cross-volume moves synchronize the copied file and destination directories before removing the source. On macOS, failure to preserve extended attributes stops the copy so a move cannot silently discard them.

`copy` automatically tries cloning on macOS, falling back to a normal copy when cloning is unsupported or crosses volumes. Linux uses normal copying. Other clone errors stop the operation. Free-space validation always budgets a full copy, even when cloning might use less space. `clone`, `hardlink`, and `symlink` are unsupported actions; use `copy` for independent files.

For action order and conflict policies, see [conflict policies and action order](../configuration/README.md#conflict-policies-and-action-order). For system trash requirements, checks deferred until execution, and the explicit `--trash-dir` alternative, see [Trash](../configuration/README.md#trash).

## Metadata

Supported formats and timestamp sources:

- JPEG, TIFF, PNG, WebP, HEIC/HEIF, AVIF, DNG, CR2, NEF, PEF, and ARW
- CR3, CRW, and RW2
- MOV and MP4, including QuickTime creation dates, `©day`, embedded XML, and `mvhd`
- PNG `eXIf`, `tEXt`, and `iTXt` creation dates
- XMP, THM, and same-name JPEG sidecars
- Filesystem birth time and modification time as final fallbacks

### Timestamp precision

Subsecond precision is not guaranteed. For images other than CR3/CRW/RW2, separate EXIF `SubSecTimeOriginal` and `SubSecTimeDigitized` tags are not combined with the capture time; standard EXIF dates therefore have whole-second precision. Tamis does not reparse files to recover fractional digits. Fractions already returned by other timestamp parsers (such as XMP or the CR3/CRW/RW2 decoder) may be retained.

`raw` contains decoder values and does not guarantee the original subsecond digit strings. Do not rely on subseconds for ordering shots or generating unique filenames; use `on_conflict` to handle filename collisions.

Filesystem birth time or mtime remains the final timestamp fallback and can be distinguished using `captured_source` in rules. Inputs with failures while extracting requested metadata are skipped, even if a fallback supplied a timestamp. See [condition expressions](../configuration/README.md#condition-expressions) for timestamp variables and [settings](../configuration/README.md#settings) for timezone interpretation.

### Extraction limits

Video extraction skips `mdat`. Metadata work runs through a bounded worker pool. Expressions and regular expressions compile once when the configuration is loaded.

XMP sidecars are limited to 8 MiB and PNG text chunks to 1 MiB. Decoder reads and seeks check cancellation; the timeout is cooperative.

Tamis runs as a single binary with pure Go metadata parsers and requires no ExifTool installation. External commands run only through explicitly configured shell actions. The project builds for macOS and Linux with `CGO_ENABLED=0`; runtime requirements for system trash are listed in [Trash](../configuration/README.md#trash).
