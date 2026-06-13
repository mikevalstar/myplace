---
title: Outdated packages (cross–package-manager inventory)
status: accepted
created: 2026-06-13
updated: 2026-06-13
tags: [outdated, packages, mise, homebrew, cli, json, tui]
phase: 1
---

# Outdated packages (cross–package-manager inventory)

## Summary

`myplace outdated` lists packages with a newer version available, grouped by package manager — **mise** today, **Homebrew** when it's present, and more managers as they're added. It's available headlessly (`--json`) and as a TUI view (a summary pane on the dashboard plus a scrollable detail screen). It is **informational and read-only**: it reports what's upgradable — including packages myplace doesn't manage — and never changes the machine.

## Motivation

The machine has software from several sources: mise (dev tools/runtimes), Homebrew (CLI formulae + casks the owner installed by hand), and eventually apt/npm/cargo. `status` already flags outdated *mise* tools as drift because `update` can fix them — but it deliberately says nothing about the dozens of brew formulae behind their latest version, because myplace never upgrades those ([ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md)). There was no single "what's upgradable on this box?" view. This adds one, without polluting the drift verdict (see [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md)).

## Scope

### In scope

- A cross-manager inventory of outdated packages, grouped by source.
- Sources: **mise** (reuses `mise outdated`) and **Homebrew** (`brew outdated --json=v2`, formulae + casks). brew is included only when it's on PATH.
- Packages myplace does **not** manage (most brew formulae) are shown — this is inventory, not just managed drift.
- Headless `myplace outdated --json` and a TUI view + a dashboard summary pane.
- A pluggable source interface so new managers (apt/dnf, npm, pipx, cargo) are one adapter each.

### Out of scope

- **Upgrading anything.** This feature only reads. brew in particular is never upgraded (ADR-0008/0009). Converging *mise* tools to their pinned versions remains `myplace update`'s job.
- **Affecting the `status`/drift verdict or its exit codes.** Outdated inventory is informational; the status verdict stays mise-only. See [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md).
- **Running `brew update`** to refresh brew's view (mutating/slow). Freshness reflects the user's last `brew update`.
- **Non-mac Homebrew install** — brew is read-if-present, never installed here.

## Behavior

### Command

`myplace outdated` prints a per-source summary in plain text; `myplace outdated --json` emits one JSON document (logs/progress to stderr, per the [headless contract](headless-cli-and-json-output.md)). The command never prompts and never mutates, so it's fully agent-runnable off a TTY.

Each source is queried independently and degrades gracefully: a source that isn't installed is reported as unavailable (not an error); a source that errors captures its message and doesn't stop the others.

### Exit codes

Distinct from the drift codes — this is its own contract:

| Code | Meaning |
|------|---------|
| 0 | all current — every available source produced a result, nothing outdated |
| 1 | updates available — at least one source reports ≥1 outdated package |
| 3 | error — no source could produce a result (e.g. none installed, or all errored) |

So `myplace outdated --json; echo $?` tells an agent "is anything upgradable here?" in `$?` before parsing the body. (There's no `2`/unknown: a partial failure where some source still produced a result resolves to `0`/`1` with the failure captured per-source in the JSON.)

### JSON envelope

```json
{
  "schema": 1,
  "machine": "hostname",
  "checked_at": "2026-06-13T20:00:00Z",
  "sources": [
    {
      "name": "mise",
      "available": true,
      "packages": [
        { "name": "node", "current": "22.1.0", "latest": "22.3.0" }
      ]
    },
    {
      "name": "brew",
      "available": true,
      "packages": [
        { "name": "htop", "current": "3.5.0", "latest": "3.5.1" },
        { "name": "gnupg", "current": "2.5.19", "latest": "2.5.20" }
      ]
    }
  ]
}
```

- `schema` — bumped only on breaking changes (mirrors the drift envelope).
- `sources[]` — one entry per source, in a stable display order (mise, then brew, then future managers).
- `sources[].available` — `false` when the manager isn't on PATH; its `packages` is then `[]`.
- `sources[].error` — present (string) only when that source was available but failed; other sources are unaffected.
- `packages[].current` / `latest` — installed version and the newer one offered. For mise, `latest` is the version mise would converge to; for brew it's `current_version` from `brew outdated`.

### TUI

- **Dashboard home** gains an **"Updates available"** pane next to Dotfiles and Tools, showing per-source counts (`mise: N`, `brew: M`, or `n/a` when a source is absent) and a `press o for details` hint. It loads asynchronously alongside the status report; until it lands the pane shows `checking…`. It does **not** change the verdict badge.
- **`o`** opens a dedicated, scrollable outdated view (a `bubbles` viewport) listing every outdated package grouped by source. `↑/↓`/`pgup`/`pgdn` scroll; `esc`/`q` returns to the dashboard; `ctrl+c` quits.

## Acceptance criteria

- [x] `myplace outdated --json | jq .` succeeds; exactly one document on stdout; contains `schema`, `machine`, `checked_at`, `sources[]`.
- [x] On a Mac with brew present, the `brew` source is `available: true` and lists outdated formulae and casks; on a machine without brew it's `available: false` with empty packages and is not an error.
- [x] Exit code is `1` when anything is outdated, `0` when nothing is, `3` when no source could be queried.
- [x] `myplace status --json` verdict and exit code are **unchanged** by brew/unmanaged packages being outdated (proves the informational separation).
- [x] `myplace help --json`/`--llm` lists `outdated` with its exit codes and this doc as its output schema.
- [x] Dashboard shows the "Updates available" pane with per-source counts; `o` opens a scrollable detail view; `esc` returns.
- [x] Nothing in this feature ever installs or upgrades a package.

## Open questions

- The dashboard runs `mise outdated` twice on open (once via `drift.Compute`, once via the inventory). Accepted for v1; a shared result is a future optimization. ([ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md) Consequences.)
- "All sources unavailable → exit 3" treats "couldn't check anything" as an error (consistent with status's `unknown`/error stance). Revisit if a real machine legitimately has neither mise nor brew.

## Related

- [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md) — the decision: informational, pluggable, read-only, separate from drift
- [Headless CLI + JSON](headless-cli-and-json-output.md) — the envelope/stream/exit-code contract this follows
- [TUI dashboard layout](tui-dashboard.md) — the home pane + the `o` detail view
- [Review outdated packages workflow](../workflows/review-outdated-packages.md)
- [ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md) / [ADR-0009](../adrs/0009-homebrew-casks-macos.md) — why brew is read-only/informational here
