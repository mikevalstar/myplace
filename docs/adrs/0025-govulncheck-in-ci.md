---
title: ADR-0025 — govulncheck in CI
status: accepted
created: 2026-07-25
updated: 2026-07-25
tags: [ci, security, go, tooling]
supersedes: null
superseded-by: null
---

# ADR-0025: govulncheck in CI

## Context

`ci.yml` runs `gofmt`, `go vet`, and `go test` on every push and pull request. None of those say anything about *known vulnerabilities* in the dependency tree, and myplace has a real one: it pulls the Charm stack, cobra, and a handful of `golang.org/x/*` modules transitively, and it ships a signed binary to every machine in the fleet via `install.sh` / `self-update`. A CVE in a transitive dep would land on all of them silently.

Dependency updates in this repo are also occasional and manual — deps had drifted a full minor across `x/text`, `x/sys`, and friends before anyone looked. Whatever we add has to flag a *vulnerable* dep without demanding that someone remember to run it.

## Options considered

### Option A — Dependabot / Renovate

Automated PRs when a dep publishes a new version. Good at keeping versions fresh, but it's noise-first: it opens PRs for every bump regardless of whether the code is exposed, and it doesn't distinguish "new patch release" from "actively exploitable". For a repo with one maintainer this becomes a PR queue to close, not a signal.

### Option B — `govulncheck` as a CI step

The official Go vulnerability scanner. Queries the Go vulnerability database and — critically — does *reachability* analysis: it reports only vulnerabilities on a call path the binary actually executes, so a CVE in an unreached corner of a transitive dep doesn't fail the build. No PR noise; it stays silent until something is genuinely wrong. Costs one `go run` per CI run.

### Option C — nothing, scan by hand

Zero cost, zero coverage. Relies on remembering, which is exactly what failed.

## Decision

**Option B.** Add `govulncheck` as the last step of the `test` job in `ci.yml`, invoked as `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`, plus a matching `mise run vuln` task so the same check runs locally.

It's run via `go run @latest` rather than pinned or added to `mise.toml`'s `[tools]`: for a scanner, the newest version with the freshest vulnerability database is what you want on every run, and pinning it would mean the tool itself becomes another thing to keep updated. Reachability analysis keeps the false-positive rate low enough that failing the build on a finding is reasonable.

Dependency *freshness* stays manual — deliberately. Reviewing `go list -m -u all` when there's a reason to is a better fit for this repo's pace than a standing PR queue.

## Consequences

- CI now fails when a reachable vulnerability appears in the dependency tree, including one introduced by a dep we didn't touch. That failure can appear on an unrelated PR — the fix is to bump the offending dep, not to work around the check.
- `mise run vuln` gives the same answer locally before pushing.
- Each CI run pays a `go run` compile of govulncheck (tens of seconds, cached by the Go module/build cache between runs).
- Because the tool floats on `@latest`, a govulncheck release could in principle change behaviour without a repo change. Accepted: the alternative is a stale scanner.
- No change to the release pipeline — `release.yml` (ADR-0004) is unaffected; the gate is on `main` and PRs, which a release tag should already have passed.
