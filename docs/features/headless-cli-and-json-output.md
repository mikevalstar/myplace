---
title: Headless CLI with JSON output
status: accepted
created: 2026-06-12
updated: 2026-06-13
tags: [cli, json, headless, automation]
phase: 1
---

# Headless CLI with JSON output

## Summary

Every myplace capability works two ways: as an interactive TUI for a human at the keyboard, and as a plain CLI command with `--json` output for scripts, cron, SSH loops, and AI tooling. Both are thin clients over the same core packages.

## Motivation

The same person is both audiences: at a Mac, you want the TUI; across twelve servers, you want `ssh host myplace status --json` in a loop. Phase 2's reporting agent is just the headless mode plus an HTTP POST, so the JSON contract has to exist — and be trustworthy — from day one.

## Scope

### In scope

- The command surface and which commands support headless operation.
- JSON output conventions (stream discipline, schema versioning, timestamps).
- Exit codes.
- Non-interactive behavior rules (what happens when a prompt would be needed).

### Out of scope

- Uploading reports to a server (phase 2 — but this schema is what gets uploaded).
- Watch/daemon mode (`myplace status --watch`) — possible later, not spec'd here.
- YAML or other output formats — JSON only until a real need appears.

## Behavior

### Command surface

| Command | Interactive (default) | Headless form |
|---------|----------------------|---------------|
| `myplace` | TUI dashboard; routes to bootstrap wizard on a fresh machine | — |
| `myplace bootstrap` | guided wizard (huh form) | `--repo <url> --profile <name> --yes` |
| `myplace status` | human-readable summary (plain text, no TUI) | `--json` |
| `myplace outdated` | per-source list of upgradable packages (plain text) | `--json` — informational; own exit codes (see below) |
| `myplace update` | TUI review-and-apply flow | `--yes [--dotfiles] [--tools]`, plus `--json` for the result report |
| `myplace self-update` | confirm prompt | `--yes`, `--json` |
| `myplace version` | plain text | `--json` |
| `myplace help` | cobra help (opens with an agent pointer) | `--llm` brief, `--json` manifest — see [LLM-friendly help](llm-friendly-help.md) |

`myplace bootstrap --repo git@... --profile server --yes` is the one-command server bootstrap: installer one-liner, then this, done.

### JSON conventions

- With `--json`, **stdout carries exactly one JSON document and nothing else**. All progress, warnings, and logs go to stderr. A consumer may always `myplace ... --json | jq .` safely.
- Every document has a top-level `"schema": 1` integer, bumped only on breaking changes. Evolution is additive where possible; consumers must tolerate unknown fields.
- Timestamps are ISO-8601 UTC (`2026-06-12T20:00:00Z`).
- No ANSI/color ever appears in JSON mode. Plain-text mode honors `NO_COLOR` and disables color when stdout is not a TTY.

### Non-interactive rule

If a headless invocation reaches a point that would require a prompt (a conflict choice, a missing flag, a git credential), it **fails fast with a descriptive error and exit code 3** — it never hangs waiting for input that cron/SSH will never provide. The error names the flag or interactive flow that resolves it.

### Exit codes

| Code | `status` | `update` / `bootstrap` | `outdated` |
|------|----------|------------------------|------------|
| 0 | in sync | success (or nothing to do) | all current |
| 1 | drifted | completed with per-item failures (e.g. one tool failed) | updates available |
| 2 | unknown (some checks couldn't run, e.g. offline) | — | — |
| 3 | error (couldn't produce a report) | error / would-need-prompt in headless mode | error (no source could be queried) |

So `ssh host myplace status --json` gives the verdict in `$?` before you even parse the body.

`outdated` is **informational** and has its own contract (above): unmanaged/brew packages being behind never affects the `status` drift verdict — `0`/`1`/`3` there mean "is anything upgradable?", not "is the managed setup in sync?". See [Outdated packages](outdated-packages.md) and [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md).

### Canonical example

```console
$ myplace status --json
{
  "schema": 1,
  "machine": "hostname",
  "profile": "server",
  "checked_at": "2026-06-12T20:00:00Z",
  "verdict": "drifted",
  "dotfiles": { "behind_origin": 2, "to_apply": ["dot_zshrc"], "local_modified": [], "unpushed_commits": 0 },
  "tools": { "missing": [], "outdated": [{ "name": "node", "current": "22.1.0", "wanted": "22.3.0" }] },
  "myplace": { "current": "0.3.0", "latest": "0.4.0" }
}
$ echo $?
1
```

Full field semantics live in the [status workflow](../workflows/check-machine-status.md); this document owns the envelope (`schema`, stream discipline, exit codes), the workflow owns the drift semantics.

## Acceptance criteria

- [ ] `myplace status --json | jq .` succeeds; nothing but the document on stdout, even with verbose logging enabled.
- [ ] Exit codes match the table for in-sync, drifted, offline-unknown, and broken-install cases.
- [ ] `myplace bootstrap --repo <url> --profile server --yes` completes on a fresh Linux container with no TTY attached.
- [ ] A headless `update` hitting a conflict exits 3 with an error naming the interactive flow, rather than hanging.
- [ ] Every JSON document contains `schema: 1`.
- [ ] Core packages (`internal/...`) contain no TUI imports (enforces the architecture in ADR-0002).

## Decisions made during implementation

- **`update` is converge-only (both headless and TUI for now):** it applies incoming dotfiles and converges tools, but never auto-captures outgoing drift — pushing unreviewed local edits from cron is a footgun. Outgoing capture (re-add / commit / push with per-file review) will be an interactive flow, tracked in the update workflow doc.

## Open questions
- Should `--json` errors also be JSON on stdout (an error document) or plain text on stderr only? Lean: error document with `"verdict": "error"`, keeping stdout parseable always.

## Related

- [ADR-0002](../adrs/0002-go-and-charm-for-the-tui.md) — the core/TUI separation that makes this free
- [Check machine status](../workflows/check-machine-status.md) — semantics of the status report
- [Outdated packages](outdated-packages.md) — the `outdated` command, its envelope and distinct exit codes
- [Update a machine](../workflows/update-machine.md), [Bootstrap a new machine](../workflows/bootstrap-new-machine.md)
