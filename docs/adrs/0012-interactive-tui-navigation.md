---
title: ADR-0012 — Interactive master-detail TUI navigation (read-only)
status: accepted
created: 2026-06-13
updated: 2026-06-13
tags: [tui, bubbletea, navigation, read-only]
supersedes: null
superseded-by: null
---

# ADR-0012: Interactive master-detail TUI navigation (read-only)

## Context

The first dashboard ([tui-dashboard spec](../features/tui-dashboard.md)) was deliberately a *display*: three read-only panes (Dotfiles / Tools / Updates) over a full-width Activity log, with a single scrollable `o` detail screen. No focus, no selection, no way to drill into a specific drifted file or outdated tool. On a large monitor most of the screen is unused, and the natural next question — "*what* changed in `.zshrc`?" — has no answer inside the TUI; you drop to a shell and run `chezmoi diff`.

We want the dashboard to be navigable: move focus between panes, select an item, and see detail for it (the diff of a drifted dotfile, the version delta of a tool/package) in a master-detail layout that actually uses the width.

This bumps against [ADR-0006](0006-agent-runnable-commands.md), the load-bearing rule that **every capability is agent-runnable** and the TUI owns no logic. ADR-0006 specifically defers *interactive keep/discard capture* (deciding per-file whether the machine or the repo wins) to the CLI `update` flow, because that's a mutation that must also work headlessly. The risk is that "make the TUI interactive" slides into "let the TUI mutate," forking behavior that the headless path can't reach.

## Options considered

### Option A — Stay display-only

Keep the panes static; richer layout but no selection. Safest, but leaves the core ask (drill into a file's diff, use the width) unmet, and the `o` view is the only scrollable surface. Rejected — it's the status quo the request is asking to move past.

### Option B — Interactive, including mutation (keep/discard, upgrade from the TUI)

Add selection *and* let the user resolve drift or upgrade packages right there. Most "app-like," but it forks decision-capture between an interactive TUI path and the headless CLI path — exactly what ADR-0006 forbids — and would make the TUI own logic. Rejected.

### Option C — Interactive navigation, strictly read-only (chosen)

Add focus/selection/detail as a pure *view* concern. Selecting a dotfile shows `chezmoi diff <file>` (a read); selecting a tool/package shows version detail already present in the loaded report/inventory (no new I/O). The only state mutation the TUI performs stays the existing `u` converge, which is the same converge-only behavior as headless `myplace update --yes` — no per-file capture, no upgrades. Everything new is selection indices and a detail panel rendered from data the TUI already has.

## Decision

Option C. The TUI gains an interactive navigation layer that is **read-only**:

- **Selection is view state, not logic.** Focus (which pane) and a selection index per pane live in the Bubble Tea `Model`. They change what's *rendered* (a focus ring, a highlighted row, a detail panel) and nothing else. Core packages (`internal/{drift,chezmoi,mise,outdated}`) are untouched and still never import the TUI.
- **Detail is a read.** A drifted dotfile's detail is `chezmoi.Client.Diff(ctx, target)` — already used read-only by the CLI capture flow — fetched in a `tea.Cmd`, cached per file, and rendered with +/- coloring. Tool/package detail is rendered from the already-loaded `drift.Report` / `outdated.Inventory`; no new command runs.
- **Mutation is unchanged.** The only thing the TUI mutates remains the existing `u` update, and it stays converge-only with the existing local-edits skip. **No keep/discard capture and no package upgrades happen in the TUI** — that stays in the CLI per [ADR-0006](0006-agent-runnable-commands.md). The Updates inventory stays informational ([ADR-0010](0010-cross-package-manager-outdated-inventory.md)).
- **The headless contract is the source of truth.** Anything the navigation surfaces (drift, diffs, outdated counts) is already available via `status --json` / `outdated --json`. The TUI adds no capability an agent can't reach; it's a nicer lens on the same reads.

## Consequences

- The dashboard becomes useful on a large monitor: focus a pane, arrow through items, read a file's diff or a tool's version delta side-by-side — without leaving the TUI or running anything that mutates.
- `internal/tui` grows real interaction state (focus enum, per-pane selection, a detail viewport, a per-file diff cache) and outgrows one file; it splits into `theme.go`/`keys.go`/`model.go`/`update.go`/`views.go`, still one package so the lipgloss-only rule holds.
- A clear, testable invariant for review: the TUI issues no command that writes, except the existing `u` converge. The diff fetch is read-only by construction (`chezmoi diff`).
- Per-step update progress (replacing the bare spinner) restructures the single update command into a sequence of step commands — still converge-only, still the same operations, just observable.
- We keep the door shut on in-TUI capture/upgrade. If that's ever wanted, it must come *through* a headless-first CLI mechanism (a new ADR), not by special-casing the TUI.

## Related

- [ADR-0006](0006-agent-runnable-commands.md) — every command agent-runnable; why mutation/capture stays in the CLI, not the TUI
- [ADR-0010](0010-cross-package-manager-outdated-inventory.md) — the Updates inventory the nav browses stays informational/read-only
- [ADR-0011](0011-catppuccin-theming.md) — the theme roles (focus ring, selected row, diff +/-) this layer consumes
- [ADR-0003](0003-monorepo-app-dotfiles-mise.md) — the layering this preserves (TUI imports core, never the reverse)
- [TUI dashboard layout](../features/tui-dashboard.md) — the user-facing spec of the interaction model
