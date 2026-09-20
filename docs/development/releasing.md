# Releasing

Run the **Release** workflow manually from GitHub Actions with `main` selected. After checks and tests pass, GoReleaser builds macOS and Linux binaries for `amd64` and `arm64`, packages them, and publishes a GitHub Release with generated release notes and SHA-256 checksums.

Tags use `v0.YYYYMMDD.N`: the date in `Asia/Tokyo` and the total commit count from `git rev-list --count HEAD`, for example `v0.20260920.123`. Versions are assigned automatically, without version-bump commits or manual major/minor/patch decisions. The workflow releases the commit selected at dispatch time. Running it again on the same day and commit stops if the tag already exists.

The tag is created locally first. GoReleaser creates the remote tag at the exact build commit when publishing the release. Only the built-in `GITHUB_TOKEN` with `contents: write` is needed; no personal access token is required. Release runs are serialized.

GoReleaser embeds the version in `tamis --version`. Ordinary development builds report `dev`. Archives include only the user-facing documents under `docs/configuration/`; `docs/development/` is not packaged.

## Local verification

GoReleaser is pinned as a tool dependency in `go.mod` and invoked with `go tool goreleaser` by the workflow and mise tasks.

```sh
mise run check:goreleaser
mise run release:snapshot
```

Archives are written to `dist/` without publishing a release or creating a tag. Snapshot versions are for local verification, not the calendar versions assigned by the workflow.
