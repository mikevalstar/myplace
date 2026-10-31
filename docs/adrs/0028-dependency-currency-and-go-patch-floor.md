---
title: ADR-0028 — Dependency currency: govulncheck gate, Dependabot, and the Go patch floor
status: accepted
created: 2026-10-31
updated: 2026-10-31
tags: [ci, security, go, tooling, dependencies]
supersedes: "0025"
superseded-by: null
---

# ADR-0028: Dependency currency: govulncheck gate, Dependabot, and the Go patch floor

## Context

[ADR-0025](0025-govulncheck-in-ci.md) added `govulncheck` to CI and said dependency freshness would "stay manual — deliberately", rejecting Dependabot. That was already wrong when it was written: [`.github/dependabot.yml`](../../.github/dependabot.yml) had landed a month earlier ([ADR-0004](0004-release-pipeline-goreleaser.md) mentions it), and it has been opening grouped PRs since.

ADR-0025 also didn't cover the failure mode that actually bit: most findings come from the **Go standard library**, not modules. `go.mod`'s `go` directive pins an exact patch (CI's `setup-go` reads it via `go-version-file`), so whenever Go publishes a stdlib fix, `main` and every open PR go red until someone bumps that line. Dependabot doesn't bump it. In October 2026 this left `main` red for about two weeks, blocking three Dependabot PRs that had nothing wrong with them.

## Options considered

### Option A — keep the exact patch in `go.mod`, bump it when govulncheck flags stdlib

`go.mod` stays the single source of truth for the toolchain CI and releases build with. The cost is a one-line manual bump each time a stdlib vuln is published.

### Option B — CI floats on the newest patch (`go-version: '1.26.x'`, `check-latest`)

Stdlib fixes are picked up with no commit. But CI then builds with a different toolchain than `go.mod` and local builds use, and releases become non-reproducible from the tag.

## Decision

**Option A**, with the record corrected:

- `govulncheck` stays the CI gate, as in ADR-0025. It runs as `go run …@latest` in [`ci.yml`](../../.github/workflows/ci.yml) and as `mise run vuln` locally.
- **Dependabot is the freshness mechanism** for Go modules and workflow Actions: weekly and grouped, with a cooldown, and majors get their own PR.
- **A red govulncheck that names `stdlib` is fixed by bumping the `go` directive** to the patch that fixes it (`go mod edit -go=1.X.Y`). It's never suppressed.

## Consequences

- Stdlib vuln publications still cause a red `main` until the bump lands. The fix is mechanical and the CI log names it.
- While `main` is red, Dependabot PRs fail too. Fix `main` first, then let them rebase. Don't debug the PRs.
- Minor Go version moves (e.g. 1.26 → 1.27) remain a deliberate change. They update `go.mod` and [`mise.toml`](../../mise.toml) together.
