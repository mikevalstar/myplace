---
title: ADR-0004 — Releases via goreleaser, GitHub Actions, and a curl installer
status: accepted
created: 2026-06-12
updated: 2026-06-12
tags: [release, ci, installer, self-update]
supersedes: null
superseded-by: null
---

# ADR-0004: Releases via goreleaser, GitHub Actions, and a curl installer

## Context

Bootstrap's first step is "get the myplace binary onto a machine that has nothing" — so binaries must be downloadable without auth, for darwin/linux × amd64/arm64, from a URL stable enough to bake into an installer script and the `self-update` command. The repo is public on GitHub, which makes GitHub Releases the obvious host.

## Options considered

### Option A — goreleaser on a tag-push GitHub Action

Industry standard for Go CLIs: one YAML, cross-compiles the matrix, stamps the version via ldflags, uploads archives to a GitHub Release. Tag-driven (`git tag v0.2.0 && git push --tags`).

### Option B — hand-rolled build matrix in Actions

No extra tool, but reimplements what goreleaser does (naming, checksums, release creation) with more YAML and more drift.

### Option C — Homebrew tap / package managers

Wrong fit as the *primary* channel: servers don't have brew, and bootstrap must not depend on a package manager (workflow rule). Could be added later on top of A.

## Decision

Option A. Specifics that other code depends on:

- **Archive naming is version-less**: `myplace_<os>_<arch>.tar.gz`. This makes the permanent URL `https://github.com/mikevalstar/myplace/releases/latest/download/myplace_<os>_<arch>.tar.gz` predictable, so both `install.sh` and `self-update` need no API call to download.
- **Version stamping**: `-ldflags -X .../internal/version.Version={{.Version}}` — `myplace version` reports the tag.
- **`install.sh` lives at the repo root**, served raw: `curl -fsSL https://raw.githubusercontent.com/mikevalstar/myplace/main/install.sh | sh`. It detects OS/arch, downloads the latest archive, and installs to `~/.local/bin`.
- **`self-update`** compares the running version against the GitHub API's `releases/latest` tag and swaps the binary in place from the same latest/download URL.
- A separate `ci.yml` runs `go test`/`go vet`/`gofmt` on every push — release tags should never be the first time code is built in CI.

## Consequences

- Cutting a release = pushing a tag; no manual steps.
- The latest/download URL always serves the newest release — machines that bootstrap or self-update get the latest tag, which matches the "config is always latest-of-main" model from ADR-0003.
- `status` can now populate `myplace.latest` (best-effort GitHub API call; null when offline) — drift includes an outdated myplace binary.
- Checksums ship with each release (goreleaser default); installer verification is follow-up work, noted in the script.
