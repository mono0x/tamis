# Releasing

[Development](README.md) · [README](../../README.md)

Wait for the latest push CI run on the target `main` commit to succeed, then run the **Release** workflow manually from GitHub Actions with `main` selected. If `main` advances before preparation, start a new workflow run.

The workflow runs three jobs:

1. **Prepare** checks CI and pushes a tag pointing directly to the selected commit.
2. **Build** calls the reusable **Release build** workflow. GoReleaser builds macOS and Linux binaries for `amd64` and `arm64`, packages them, and generates SHA-256 checksums. The job attests the archives and uploads them as a workflow artifact retained for seven days.
3. **Publish** downloads the artifact, verifies its checksums, creates a draft GitHub Release with generated notes, uploads the archives, and publishes it.

Tags use `v0.YYYYMMDD.N`: the workflow creation date in `Asia/Tokyo` and `GITHUB_RUN_NUMBER`, for example `v0.20261031.12`. Versions are assigned automatically without version-bump commits. Re-running the same workflow keeps both the date and number, even across midnight.

If a job fails, re-run the failed jobs or the whole workflow. Preparation reuses the tag only if it points to the original selected commit and remains an ancestor of `main`. Publication resumes an existing draft and replaces its assets. An already published release is left unchanged, and a full re-run skips its build and publication.

Only the built-in `GITHUB_TOKEN` is needed; no personal access token is required. Preparation has `actions: read` and `contents: write`; the build has `contents: read`, `id-token: write`, and `attestations: write`; publication has `contents: write`. Release runs are serialized.

GoReleaser embeds the version in `tamis --version`. Ordinary development builds report `dev`. Archives include only the user-facing documents under `docs/configuration/` and `docs/behavior/`; `docs/development/` is not packaged.

## Local verification

GoReleaser is pinned as a tool dependency in `go.mod` and invoked with `go tool goreleaser` by the workflow and mise tasks.

```sh
mise run check:goreleaser
mise run release:snapshot
```

Archives are written to `dist/` without publishing a release or creating a tag. Snapshot versions are for local verification, not the calendar versions assigned by the workflow.
