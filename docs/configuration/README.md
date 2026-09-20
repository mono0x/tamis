# YAML configuration reference

[日本語](README.ja.md) · [README](../../README.md) · [Example configuration](../../config.example.yaml)

This reference describes configuration version `1`. Pass the file explicitly to `tamis check CONFIG`, `tamis preview CONFIG`, or `tamis apply CONFIG`. Tamis does not discover a configuration automatically.

## Contents

- [Complete example](#complete-example)
- [Top-level fields](#top-level-fields)
- [Settings](#settings)
- [Path variables](#path-variables)
- [Locations and glob patterns](#locations-and-glob-patterns)
- [Rules and evaluation order](#rules-and-evaluation-order)
- [Condition expressions](#condition-expressions)
- [Actions](#actions)
- [Templates and filters](#templates-and-filters)
- [Validation](#validation)

## Complete example

Replace the input and destination paths before scanning. This example copies images and videos and leaves the originals in place.

```yaml
version: 1

settings:
  assume_timezone: Asia/Tokyo
  video:
    mvhd_timezone: local
  on_conflict: rename
  jobs: 0
  windows_names: false

vars:
  archive: ~/Media/Archive

rules:
  - name: archive-images
    location: ~/Media/Inbox
    recursive: true
    include: ["**/*.jpg", "**/*.JPG", "**/*.png", "**/*.mov", "**/*.mp4"]
    exclude: ["**/private"]
    when: 'kind == "image" && captured != nil'
    actions:
      - copy: '{{archive}}/Images/{{captured | date "2006/01"}}/'
        rename: '{{captured | date "20060102_150405"}}_{{basename}}{{ext}}'
    stop: true

  - name: archive-videos
    location: ~/Media/Inbox
    recursive: true
    include: ["**/*.jpg", "**/*.JPG", "**/*.png", "**/*.mov", "**/*.mp4"]
    exclude: ["**/private"]
    when: 'kind == "video" && captured != nil'
    actions:
      - copy: '{{archive}}/Videos/{{captured | date "2006/01"}}/'
```

## Top-level fields

Unknown field names in configuration objects are errors. `vars` and `shell.env` are maps with user-defined keys.

| Field      | Type            | Default / requirement | Meaning                                              |
| ---------- | --------------- | --------------------- | ---------------------------------------------------- |
| `version`  | integer         | Required; must be `1` | Configuration version.                               |
| `settings` | object          | Defaults below        | Metadata extraction and file operation settings.     |
| `vars`     | map of strings  | Empty                 | Named paths for templates.                           |
| `rules`    | list of objects | Empty                 | Rules evaluated in declaration order for each input. |

## Settings

| Field                 | Type    | Default   | Meaning                                                                                                                           |
| --------------------- | ------- | --------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `assume_timezone`     | string  | `local`   | Timezone for offset-free image, PNG text, and sidecar timestamps. Accepts `local`, `utc`, or an IANA name such as `Asia/Tokyo`.   |
| `video.mvhd_timezone` | string  | `local`   | Interpretation of QuickTime `mvhd` creation time: `local` or `utc` only. Also used for offset-free embedded video date strings.   |
| `on_conflict`         | string  | `rename`  | Default destination conflict policy: `rename`, `skip`, or `fail`.                                                                 |
| `jobs`                | integer | CPU count | Metadata extraction workers. Omitted, zero, or negative values use the CPU count. A positive CLI `--jobs` overrides this setting. |
| `windows_names`       | boolean | `false`   | Prefix Windows reserved filename stems with `_` when Tamis normalizes a filename.                                                 |

`local` means the process's local timezone. `assume_timezone` does not convert timestamps that already contain an offset, and it does not set the timezone used by `video.mvhd_timezone`. With `mvhd_timezone: local`, the decoded date and clock fields are interpreted as local wall time; `utc` interprets them as UTC. The `date` filter formats the timestamp in its existing timezone.

`windows_names` covers `CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, and `LPT1`–`LPT9`, case-insensitively, including names with an extension. It is not a complete Windows filename sanitizer. It applies to names appended to directory destinations not every component of an explicit destination path.

Unmatched inputs are skipped and left untouched, regardless of whether they have a capture timestamp. Inputs marked as metadata extraction failures are skipped and left untouched, even if a fallback supplied a timestamp.

## Path variables

```yaml
vars:
  archive: ~/Media/Archive
  exports: ./exports
```

Each value is a path string. Tamis expands a leading `~/` and resolves relative values against the **process's working directory**, not the configuration file's directory. This also applies to `rules[].location`. Environment variables such as `$HOME` are not expanded in these paths.

Use the exact forms `{{archive}}` or `{{vars.archive}}` in `copy`, `move`, `rename`, or `shell.env` templates. Path variables are literal substitutions performed before metadata template parsing. Whitespace inside these forms and filters on path variables are unsupported: use `{{archive}}/{{model | slug}}/`, not `{{ archive }}` or `{{archive | lower}}`. Prefer `{{vars.NAME}}` if a variable name overlaps a metadata field. Do not nest variable references.

`vars` is not a general-purpose string map and is not available in `when`. For literal text such as a label or timezone offset, write it directly in the template or `shell.env` value.

## Locations and glob patterns

| Field       | Type            | Default / requirement           | Meaning                                                                                        |
| ----------- | --------------- | ------------------------------- | ---------------------------------------------------------------------------------------------- |
| `location`  | string          | Required; nonempty              | Input path for this rule.                                                                      |
| `recursive` | boolean         | `false`                         | Descend into subdirectories.                                                                   |
| `include`   | list of strings | Empty: include all              | A file must match at least one pattern when the list is nonempty.                              |
| `exclude`   | list of strings | Empty: no additional exclusions | Exclude entries matching any pattern; a matching directory and its entire subtree are skipped. |

Only regular files are scanned; symlinks and special files are skipped. Exclusion takes precedence over inclusion. Patterns match the whole path relative to the rule’s `location`, with `/` as the separator, and are case-sensitive. Files and directories whose names start with `.` are always excluded, including descendants of hidden directories. Explicit `include`, `location`, or `scan PATH` values do not override this exclusion. This check uses path names, not OS-specific hidden attributes.

| Pattern syntax | Meaning                                      | Example                                                  |
| -------------- | -------------------------------------------- | -------------------------------------------------------- |
| `*`            | Zero or more characters within one component | `*.jpg` matches root-level JPEG filenames only.          |
| `?`            | One character other than `/`                 | `image-?.png`                                            |
| `**`           | Zero or more characters, including `/`       | `archive/**` matches descendants of `archive`.           |
| `**/`          | Zero or more directory levels                | `**/*.jpg` matches root-level and nested JPEG filenames. |

Character classes such as `[a-z]`, brace expansion such as `*.{jpg,png}`, and negation such as `!keep.jpg` are unsupported; those characters are literal. Use separate patterns for extensions and case variants. Quote patterns beginning with `*` so YAML does not interpret them as aliases. To prune any directory named `private`, use `exclude: ["**/private"]`.

Set `location`, `recursive`, `include`, and `exclude` directly on each rule. Identical scan scopes are walked once; files in overlapping scopes have metadata extracted once. Each file is evaluated in rule order, with `relpath` relative to the current rule’s `location`. In scan JSON, `relpath` is relative to the first rule that discovers the file.

Scanning and evaluation use all rules in the configuration. The old `sources`, `source`, and `--source` forms are rejected. Scan JSON no longer includes `source_id`.

## Rules and evaluation order

| Field     | Type            | Default / requirement         | Meaning                                                                                                                |
| --------- | --------------- | ----------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `name`    | string          | Required; nonempty and unique | Name shown in plans and explanations.                                                                                  |
| `when`    | string          | `"true"`                      | Expr expression that must return a boolean.                                                                            |
| `capture` | string          | None                          | Go regular expression matched against the full filename, including its extension.                                      |
| `actions` | list of objects | Empty                         | Operations to plan in order. An empty list performs no operation.                                                      |
| `stop`    | boolean         | `false`                       | Stop evaluating later rules for this input after this rule matches. Implicitly true for a rule with `move` or `trash`. |

For each input, Tamis checks rules in YAML order. It first checks `location`, `recursive`, `include`, and `exclude`, then matches `capture` if present, then evaluates `when`. A failed capture match skips the rule. Named groups such as `(?P<year>\d{4})` become strings in `capture.year`, available to both `when` and templates for that rule. Capture values are not shared between rules. The regex is a search unless anchored with `^` and `$`.

This rule fragment organizes a synthetic filename such as `2001_02_sample.png` by its captured strings:

```yaml
name: filename-date
location: ~/Media/Inbox
capture: '^(?P<year>\d{4})_(?P<month>\d{2})_.*\.png$'
when: 'kind == "image" && capture.year == "2001"'
actions:
  - copy: "~/Media/Archive/{{capture.year}}/{{capture.month}}/"
stop: true
```

Capture groups do not parse dates or update `captured`. A rule with empty `actions` still counts as matched and can use `stop: true` to leave matching inputs untouched. All unmatched inputs are skipped.

`preview` and `apply` derive metadata requirements from references in `when`. For each file, they union requirements from rules whose scan scope and `capture` match, then extract once. Conditions and terminal rules are evaluated afterward, so extraction can cover more fields than the branch that ultimately matches.

Filename and filesystem-only filters do not invoke image or video parsers. `kind`, `format`, and `mime` need only format sniffing; `date_added` is read only when referenced. Images select required tags and metadata sources; videos select boxes according to date and duration requirements. Referencing `raw` requires broader metadata extraction. The CR3/CRW/RW2 parser does not support tag selection and still decodes metadata together when needed, but skips converting all metadata to JSON unless `raw` is requested. Structural headers must still be read to locate requested fields.

Failures while extracting requested metadata still skip the file. Corruption in unrequested metadata may go undetected. `scan` and `explain` retain full metadata extraction for diagnostics. `$env` references are unsupported; use field names directly.

## Condition expressions

`when` uses Expr, not the template language. Conditions compile when the configuration loads and must return a boolean. The expression limit is 200 AST nodes. Examples:

```yaml
when: 'kind == "image" && captured != nil'
```

```yaml
when: 'ext in [".jpg", ".jpeg"] && age_hours >= 24'
```

```yaml
when: 'captured != nil && captured_source not in ["fs:birthtime", "fs:mtime"]'
```

The complete application-provided expression vocabulary is below. Optional values can be `nil`; guard them before comparisons or member access. Missing strings are empty and missing dimensions are zero. The vocabulary differs from `scan` JSON field names.

| Variable                | Type              | Meaning                                                                                                                            |
| ----------------------- | ----------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `path`                  | string            | Absolute input path.                                                                                                               |
| `relpath`               | string            | Path relative to the rule’s `location`, using `/`.                                                                                 |
| `basename`              | string            | Filename without its final extension.                                                                                              |
| `ext`                   | string            | Lowercase final extension, including `.`, or empty.                                                                                |
| `dir`                   | string            | Name of the immediate parent directory, not its full path.                                                                         |
| `size`                  | integer           | File size in bytes.                                                                                                                |
| `kind`                  | string            | `image`, `video`, or `other`.                                                                                                      |
| `format`                | string            | `JPEG`, `TIFF`, `PNG`, `WEBP`, `HEIC`, `AVIF`, `DNG`, `CR2`, `CR3`, `CRW`, `NEF`, `PEF`, `ARW`, `RW2`, `MOV`, `MP4`, or `UNKNOWN`. |
| `mime`                  | string            | MIME type; `application/octet-stream` when no specific mapping is provided.                                                        |
| `captured`              | optional time     | Selected capture timestamp, possibly from the filesystem fallback.                                                                 |
| `captured_source`       | string            | Timestamp provenance, such as `fs:birthtime`, `fs:mtime`, or `qt:mvhd.creation_time`; empty without a timestamp.                   |
| `mtime`                 | time              | Filesystem modification time.                                                                                                      |
| `ctime`                 | time              | Filesystem birth time on macOS when available; otherwise mtime. This is not POSIX inode change time.                               |
| `date_added`            | optional time     | Time the file was added to its containing directory on macOS, when supported. No birthtime or mtime fallback.                      |
| `age`                   | duration          | `now - ctime`, represented as a Go duration in nanoseconds.                                                                        |
| `age_hours`             | number            | `now - ctime` in fractional hours.                                                                                                 |
| `now`                   | time              | Evaluation time, shared across inputs in the plan.                                                                                 |
| `make`, `model`, `lens` | string            | Camera maker, camera model, and lens model.                                                                                        |
| `iso`                   | optional integer  | ISO speed.                                                                                                                         |
| `fnumber`               | optional number   | Aperture f-number.                                                                                                                 |
| `focal`                 | optional number   | Focal length in millimeters.                                                                                                       |
| `width`, `height`       | integer           | Image dimensions in pixels when available.                                                                                         |
| `gps`                   | optional object   | `gps.lat`, `gps.lon` in degrees; optional `gps.alt` in meters.                                                                     |
| `duration`              | optional duration | Video duration as a Go duration in nanoseconds.                                                                                    |
| `raw`                   | map               | Provider-specific decoded metadata. Keys and value types depend on the parser and format.                                          |
| `capture`               | map of strings    | Named regex groups from the current rule.                                                                                          |
| `seq`                   | integer           | One-based index in the evaluated input list. CLI scans use absolute path order after deduplication.                                |

`seq` is not a persistent counter and can change with configured rule locations or input files; `explain` evaluates a single input with `seq = 1`. Timestamp precision varies by format. Separate EXIF subsecond tags are not combined for images other than CR3/CRW/RW2; do not use subseconds to guarantee unique names.

### Time added to a folder

To act on files added to a folder more than three days ago:

```yaml
when: "date_added != nil && now.Sub(date_added).Hours() > 72"
```

`date_added` uses the macOS filesystem attribute `ATTR_CMN_ADDEDTIME`: the time the file was created or renamed into its containing directory. It is separate from `ctime`, `mtime`, and `age_hours`. It reads the filesystem directly rather than the Spotlight `kMDItemDateAdded` index used by organize, so indexed values can differ. [Apple's getattrlist specification](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/getattrlist.2)

On Linux, unsupported filesystems, or when the value is missing, `date_added` is `nil`. Unlinked files and files with multiple hard links also yield `nil` because the containing directory is ambiguous. No other timestamp is substituted, and `date_added` never supplies a capture-time fallback. Guard it with `date_added != nil` before comparisons or `date` formatting. `scan` includes the value in JSON when available; `explain` shows it or `unavailable`.

The condition above uses the plan's fixed `now` and requires strictly more than 72 hours; a newly added file with an old creation or modification time does not match. `duration` is already a video metadata variable, so use the time methods shown above rather than `duration("72h")`.

## Actions

Each action object must select exactly one kind: nonempty `copy`, nonempty `move`, `trash: true`, or a `shell` object. `rename` is an option on a copy or move, not a standalone action.

| Field         | Type            | Meaning                                                                                                                            |
| ------------- | --------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `copy`        | string template | Copy to a file or directory destination.                                                                                           |
| `move`        | string template | Move to a file or directory destination; must be the rule's last action.                                                           |
| `trash`       | boolean         | Use `true` to move to the trash holding directory; must be the rule's last action.                                                 |
| `shell`       | object          | Run a command on the original input before any copies. See below.                                                                  |
| `rename`      | string template | Optional filename for `copy` or `move` destinations ending in `/`; otherwise ignored. Ignored for `trash`, prohibited for `shell`. |
| `on_conflict` | string          | Optional `rename`, `skip`, or `fail`; defaults to `settings.on_conflict`. Prohibited for `shell`.                                  |

### Destinations and renaming

A destination ending in `/` is a directory: Tamis appends the original filename or the rendered `rename` value. Without `/`, the destination is the complete file path, even if it names an existing directory. `rename` does not append the original extension automatically; use `{{ext}}` when needed. The default original filename retains its extension's case, while `{{ext}}` is lowercase.

```yaml
actions:
  - copy: "~/Media/Archive/"
    rename: "{{basename | lower}}{{ext}}"
  - move: "~/Media/Processed/{{basename}}{{ext}}"
```

Rendered destinations must be absolute. A leading literal `~/` is expanded in `copy` and `move`; other relative destinations are rejected. Use an absolute path or a path variable to anchor a template. Destination parent directories are created during execution when needed.

For directory destinations, the appended filename is normalized to Unicode NFC; `/`, `\`, and NUL are replaced with `-`, and the filename is shortened to at most 255 UTF-8 bytes while preserving the extension when possible. A `rename` result of empty text, `.`, or `..` is an error. Explicit complete file paths do not receive this filename normalization.

### Conflict policies and action order

| Policy   | Behavior                                                                                                     |
| -------- | ------------------------------------------------------------------------------------------------------------ |
| `rename` | Choose a free name such as `name-2.jpg`, `name-3.jpg`, including destinations already reserved by this plan. |
| `skip`   | Skip the conflicting operation. Existing contents are not compared.                                          |
| `fail`   | Reject the plan if the destination is occupied.                                                              |

Existing destinations are never overwritten, including with CLI `--force`. A new conflict detected during execution stops the run without choosing a different name from the validated plan.

All `shell` actions for an input must precede its first copy, including across matching rules. Any number of copies may precede at most one final move or trash. Move and trash stop later rule evaluation even if `stop: false` is explicit. Copy-only and shell-only rules continue unless `stop: true` is set.

If any copy for an input is skipped, remaining copies continue but its final move or trash is skipped so the original remains. This also applies across rules. Errors stop execution; completed copies and shell edits are not rolled back.

`copy` attempts cloning on macOS and falls back to byte copying when unsupported or across volumes. Linux uses byte copying. Copies preserve mode, mtime, and macOS extended attributes. Capacity validation budgets the full file size even when cloning may save space.

### Trash

On macOS and Linux, `trash` uses [github.com/mono0x/trash](https://github.com/mono0x/trash) to move files to the system trash without external commands or CGO. macOS uses Foundation; Linux requires `libgio-2.0.so.0` at runtime and uses GIO to manage trash locations and restore metadata. The OS chooses the destination and resolves filename collisions; `preview` displays `system Trash` and JSON omits `dest`. Native libraries and trash availability are checked only during execution, and destination capacity is not checked in advance. A failure stops the run and leaves completed copies in place; files are never directly deleted as a fallback. Finder's Put Back behavior is not guaranteed. Use `preview CONFIG --trash-dir PATH` and `apply CONFIG --trash-dir PATH` to move to an explicit holding directory instead, with destinations displayed and validated before execution. Explicit holding directories do not record desktop restore information. There is no YAML `trash_dir` setting.

### Shell

| Field       | Type           | Default / requirement      | Meaning                                                                                                                           |
| ----------- | -------------- | -------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `shell.run` | string         | Required; nonblank, no NUL | Literal script executed by `/bin/sh -eu -c`. No template expansion.                                                               |
| `shell.env` | map of strings | Empty                      | Environment values rendered with the template language. Names must match `^[A-Za-z_][A-Za-z0-9_]*$`; values must not contain NUL. |

`TAMIS_FILE` is reserved and contains the absolute original input path. It cannot be set in `env`. Quote environment variable expansions in scripts. The following action fragment illustrates passing a metadata value to an installed script:

```yaml
when: 'model matches ".*"'
actions:
  - shell:
      run: 'sh ./prepare-media.sh "$TAMIS_FILE" "$CAMERA"'
      env:
        CAMERA: '{{model | default "unknown"}}'
  - copy: "~/Media/Archive/"
```

The command inherits the process's working directory and environment, with `env` entries overriding inherited values. It receives no stdin. stdout is discarded; failure messages include up to the first 16 KiB of stderr. The timeout is five minutes and is not configurable in YAML. Background tasks are unsupported; the process group is terminated on completion or cancellation.

Shell runs as the user without a sandbox and may edit the original in place; execute only trusted configurations. `preview` and `explain` display commands and rendered environment values without executing them. After success, the input must still be a regular file; Tamis refreshes its identity, size, and mtime for subsequent operations. Metadata, conditions, and destinations are not reevaluated after a shell action. Command failures stop the run without rolling back edits; a later copy skip also leaves the edited original in place.

## Templates and filters

Templates apply to `copy`, `move`, `rename`, and `shell.env` values. They use a small `{{ variable | filter arguments }}` language, not Expr or Go `text/template`. Arbitrary expressions, arithmetic, conditionals, and function calls are unsupported.

Action metadata variables `captured`, `mtime`, `ctime`, `date_added`, `make`, `model`, and `lens` must be referenced in the same rule's `when`. Configuration loading rejects unreferenced fields in destinations, `rename`, and `shell.env`; references in other rules do not qualify. Basic variables `path`, `relpath`, `basename`, `ext`, `dir`, and `seq` remain always available, and `capture.NAME` comes from that rule's `capture`. A string literal such as `"model"` is not a metadata reference.

Only these metadata variables are available in templates: `path`, `relpath`, `basename`, `ext`, `dir`, `captured`, `mtime`, `ctime`, `date_added`, `make`, `model`, `lens`, `seq`, and `capture.NAME`. Their meanings are given in the expression table. Other expression variables such as `kind`, `size`, `iso`, `raw`, `now`, and `captured_source` are unavailable in templates. Path variables use the separate literal substitution described above.

Filters run left to right. Arguments containing spaces use double quotes; wrapping the whole YAML value in single quotes makes this easier. The template parser does not support escaped quotes or literal `|` within arguments. A missing capture group yields an empty string. Missing `captured` yields empty text without filters; `date` on it is an error.

| Filter            | Meaning                                                                                                                  | Example                                  |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------- |
| `date "LAYOUT"`   | Format a time with a Go date layout. Requires a time value.                                                              | `{{captured \| date "2006/01/02"}}`      |
| `slug`            | NFKD-normalize and lowercase; keep Unicode letters and digits, replace other runs with `-`, trim outer `-`.              | `{{model \| slug}}`                      |
| `lower`           | Lowercase text.                                                                                                          | `{{basename \| lower}}`                  |
| `upper`           | Uppercase text.                                                                                                          | `{{capture.label \| upper}}`             |
| `pad WIDTH`       | Left-pad with zeroes to the minimum width; does not truncate.                                                            | `{{seq \| pad 4}}`                       |
| `default "TEXT"`  | Substitute for `nil` or an empty string, not zero.                                                                       | `{{model \| default "unknown" \| slug}}` |
| `trunc LENGTH`    | Keep at most this many Unicode code points; length must be nonnegative.                                                  | `{{basename \| trunc 40}}`               |
| `slice START END` | Extract Unicode code points from zero-based START, inclusive, to END, exclusive. Requires `0 <= START <= END <= length`. | `{{basename \| slice 0 4}}`              |

`slug` retains non-Latin letters and does not transliterate them into ASCII.

Go date layouts use the reference date `2006-01-02 15:04:05`, not `YYYY-MM-DD` or `strftime` directives:

| Layout            | Output for 2001-02-03 04:05:06 |
| ----------------- | ------------------------------ |
| `2006/01`         | `2001/02`                      |
| `2006-01-02`      | `2001-02-03`                   |
| `20060102_150405` | `20010203_040506`              |

In destination templates, interpolated metadata must form a single path component. Use `slug` for values containing `/` or `\`. Only `path`, `relpath`, `captured`, `mtime`, `ctime`, and `date_added` may expand across `/` separators, allowing layouts such as `2006/01`. All interpolated values reject NUL, `\`, and path components equal to `.` or `..`. `rename` and `shell.env` use text rendering instead; filename normalization applies to the `rename` result as described above.

## Validation

```sh
tamis check ./organize.yaml
tamis preview ./organize.yaml
tamis explain ./organize.yaml /absolute/path/to/input.jpg
```

`check` loads the YAML, rejects unknown fields and invalid action structures, validates timezones and conflict policies, and compiles conditions and capture regexes. It does not scan inputs, render metadata-dependent templates, or check destination existence, permissions, or capacity. An accepted configuration can still fail during evaluation or operation validation.

`preview` scans and evaluates the configured rule locations, then validates the whole plan without changing files or executing shell commands. `explain` extracts metadata and evaluates rules for one input; it does not perform the whole-plan filesystem validation that `preview` does. Use `apply CONFIG` to rescan, validate, and execute the resulting operations.
