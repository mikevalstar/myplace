---
title: Outdated packages (cross–package-manager inventory)
status: accepted
created: 2026-06-13
updated: 2026-07-03
tags: [outdated, packages, mise, homebrew, shelly, cachyos, skills, ai, cli, json, tui]
phase: 1
---

# Outdated packages (cross–package-manager inventory)

## Summary

`myplace outdated` lists packages with a newer version available, grouped by package manager — **mise** today, **Homebrew** when it's present, **Shelly** on CachyOS, **skills** (AI agent skills via the skills.sh CLI) when that CLI is installed, and more managers as they're added. It's available headlessly (`--json`) and as a TUI view (a summary pane on the dashboard plus a scrollable detail screen). It is **informational and read-only**: it reports what's upgradable — including packages myplace doesn't manage — and never changes the machine.

## Motivation

The machine has software from several sources: mise (dev tools/runtimes), Homebrew (CLI formulae + casks the owner installed by hand), and eventually apt/npm/cargo. `status` already flags outdated *mise* tools as drift because `update` can fix them — but it deliberately says nothing about the dozens of brew formulae behind their latest version, because myplace never upgrades those ([ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md)). There was no single "what's upgradable on this box?" view. This adds one, without polluting the drift verdict (see [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md)).

## Scope

### In scope

- A cross-manager inventory of outdated packages, grouped by source.
- Sources: **mise** (reuses `mise outdated`), **Homebrew** (`brew outdated --json=v2`, formulae + casks), **Shelly** (Arch/CachyOS — `shelly check-updates -a -l --json`, aggregating native Arch repos + AUR + Flatpak into one row; the `-a`/`-l` flags opt the AUR and Flatpak channels into the otherwise repo-only check, and secondary-channel packages are name-prefixed, e.g. `flatpak:org.gimp.GIMP`. AppImage has no `check-updates` flag, so it's out of scope), and **skills** (third-party AI agent skills — parses `skills check -g`, the skills.sh / `npx skills` CLI). brew, shelly, and skills are each included only when their CLI resolves (so shelly shows up on CachyOS, brew on Macs, skills wherever the `skills` CLI or `npx skills` is available). A user's *own* authored skills are chezmoi-managed dotfiles and don't appear here — only the third-party "stable" installed via the CLI ([ADR-0023](../adrs/0023-managing-ai-skills.md)).
- Packages myplace does **not** manage (most brew formulae) are shown — this is inventory, not just managed drift.
- Headless `myplace outdated --json` and a TUI view + a dashboard summary pane.
- A pluggable source interface so new managers (apt/dnf, npm, pipx, cargo) are one adapter each.

### Out of scope

- **Upgrading anything.** This feature only reads. brew in particular is never upgraded (ADR-0008/0009). Converging *mise* tools to their pinned versions remains `myplace update`'s job.
- **Affecting the `status`/drift verdict or its exit codes.** Outdated inventory is informational; the status verdict stays mise-only. See [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md).
- **Running `brew update`** to refresh brew's view (mutating/slow). Freshness reflects the user's last `brew update`.
- **Running `shelly sync`/`update`/`upgrade`** — same read-only stance as brew: `outdated` only reads Shelly's local update list (`check-updates`), never refreshes its databases or installs anything. Freshness reflects the user's last sync.
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
- `sources[]` — one entry per source, in a stable display order (mise, then brew, then shelly, then skills, then future managers).
- `sources[].available` — `false` when the manager isn't on PATH; its `packages` is then `[]`. In practice brew is `available` only on Macs, shelly only on CachyOS, and skills only where the skills.sh CLI resolves.
- `sources[].error` — present (string) only when that source was available but failed; other sources are unaffected.
- `packages[].current` / `latest` — installed version and the newer one offered. For mise, `latest` is the version mise would converge to; for brew it's `current_version` from `brew outdated`; for shelly it's the new version from `shelly check-updates`; for skills they're the current/latest refs `skills check` prints (git-based, so possibly a commit ref rather than semver, and either may be empty when the CLI prints no delta).
- For **shelly**, `packages[].name` from a secondary channel is prefixed with that channel (`aur:`, `flatpak:`) so the single row stays legible; native repo packages keep their bare name. Flatpak entries carry no installed version, so their `current` is empty (the row still signals an update is available).
- **skills is a text-parsed source, not JSON.** Unlike the others, `skills check` emits ANSI-colored *human* output and ignores `--json` (verified against skills v1.5.14 — only `skills list --json` is structured, and it carries no version info). So `internal/skills` strips ANSI and parses `check -g`'s text, the way `internal/chezmoi` line-parses `chezmoi status`. The all-clear ("✓ All global skills are up to date" → `packages: []`) and unavailable cases are verified; the *has-updates* line shape was not observable on an up-to-date machine, so that parse is best-effort and tolerant, isolated to `ParseCheck` for a one-line fix once a real outdated skill is seen (see the acceptance criteria).

### TUI

- **Dashboard home** gains an **"Updates available"** pane next to Dotfiles and Tools, showing per-source counts (`mise: N`, `brew: M`, `shelly: K`, `skills: J`, or `n/a` when a source is absent — so shelly reads `n/a` on Macs and skills reads `n/a` where the CLI isn't installed). The pane, its count chip, and the `o` detail table all iterate the inventory's `sources`, so a new source (like skills) appears with **no dashboard code change** — only a new adapter in the source slice. It carries a `press o for details` hint. It loads asynchronously alongside the status report; until it lands the pane shows `checking…`. It does **not** change the verdict badge.
- **`o`** opens a dedicated, scrollable outdated view (a `bubbles` viewport) rendering every outdated package as a bordered `lipgloss/table` (PACKAGE · CURRENT · LATEST · SOURCE). It carries a **count summary** (`N outdated across M sources`, and `X of N shown` when filtered), a **sort** toggle (`s` cycles by source / by name; by-source keeps the grouped layout, by-name flattens into one alphabetical list annotated with each package's source), and a **filter** (`/` focuses a text input for a case-insensitive substring match on the package name; `esc` clears it). `↑/↓`/`pgup`/`pgdn` scroll; `esc`/`q` returns to the dashboard; `ctrl+c` quits. Sort and filter are pure presentation over the already-collected inventory — no recompute, no extra command runs.

## Acceptance criteria

- [x] `myplace outdated --json | jq .` succeeds; exactly one document on stdout; contains `schema`, `machine`, `checked_at`, `sources[]`.
- [x] On a Mac with brew present, the `brew` source is `available: true` and lists outdated formulae and casks; on a machine without brew it's `available: false` with empty packages and is not an error.
- [x] Exit code is `1` when anything is outdated, `0` when nothing is, `3` when no source could be queried.
- [x] `myplace status --json` verdict and exit code are **unchanged** by brew/unmanaged packages being outdated (proves the informational separation).
- [x] `myplace help --json`/`--llm` lists `outdated` with its exit codes and this doc as its output schema.
- [x] Dashboard shows the "Updates available" pane with per-source counts; `o` opens a scrollable detail view; `esc` returns.
- [ ] The `o` view shows a count summary and supports `s` (sort by source / name) and `/` (filter by name); both are presentation-only and run no extra command.
- [x] Nothing in this feature ever installs or upgrades a package.
- [x] The `skills` source reports `available: true, packages: []` on a machine whose global skills are current (verified end-to-end against skills v1.5.14 via `npx skills`), and `available: false` where the CLI can't be resolved.
- [ ] Confirm `skills check -g`'s **has-updates** output shape against a machine with an outdated skill, and tighten `ParseCheck` (+ fixtures) to match — currently a documented best-effort parse.
- [ ] Wire the managed setup: a `home/dot_claude/skills/` chezmoi tree for authored skills, and a profile-gated `skills add -g` source-list block in the provision script for the third-party stable (self-managed until [vercel-labs/skills#683](https://github.com/vercel-labs/skills/issues/683) enables a global lockfile restore — [ADR-0023](../adrs/0023-managing-ai-skills.md)).

## Open questions

- The dashboard runs `mise outdated` twice on open (once via `drift.Compute`, once via the inventory). Accepted for v1; a shared result is a future optimization. ([ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md) Consequences.)
- "All sources unavailable → exit 3" treats "couldn't check anything" as an error (consistent with status's `unknown`/error stance). Revisit if a real machine legitimately has neither mise nor brew.

## Related

- [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md) — the decision: informational, pluggable, read-only, separate from drift
- [Headless CLI + JSON](headless-cli-and-json-output.md) — the envelope/stream/exit-code contract this follows
- [TUI dashboard layout](tui-dashboard.md) — the home pane + the `o` detail view
- [Review outdated packages workflow](../workflows/review-outdated-packages.md)
- [ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md) / [ADR-0009](../adrs/0009-homebrew-casks-macos.md) — why brew is read-only/informational here
- [ADR-0023](../adrs/0023-managing-ai-skills.md) — the skills source: own skills via chezmoi, third-party via the skills.sh CLI, surfaced here informationally
