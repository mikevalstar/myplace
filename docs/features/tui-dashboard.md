---
title: TUI dashboard layout
status: accepted
created: 2026-06-12
updated: 2026-06-13
tags: [tui, bubbletea, lipgloss, layout]
phase: 1
---

# TUI dashboard layout

## Summary

The bare `myplace` dashboard is a full-screen, paneled terminal UI: a header bar, a top row of Dotfiles / Tools / Updates-available panes, a full-width Activity feed tailing the log, and a footer of key hints. It fills the terminal and reflows on resize. Pressing `o` opens a scrollable detail view of all outdated packages.

## Motivation

The first cut was a few top-anchored lines that ignored most of the screen. A status tool you glance at across many machines benefits from a real layout — drift, tools, and recent activity visible at once, using the whole terminal.

## Scope

### In scope

- A responsive full-screen layout (Lip Gloss panes sized from `tea.WindowSizeMsg`).
- Panes: header bar, Dotfiles, Tools, Updates available, Activity (log tail), footer.
- An informational "Updates available" summary pane (per-source outdated counts) and a scrollable `o` detail view, loaded asynchronously — see [Outdated packages](outdated-packages.md).
- A live-refreshing Activity feed (cheap log tail on a timer; no network).
- Graceful degradation on small terminals.

### Out of scope

- Selecting within panes, mouse, tabs (could come later via `bubbles` list). The outdated detail view (`o`) is the first scrollable surface (a `bubbles` viewport); the dashboard panes themselves still ellipsize rather than scroll.
- In-TUI interactive capture (keep/discard) — still deferred to the CLI per [ADR-0006](../adrs/0006-agent-runnable-commands.md) and the update workflow.
- Upgrading packages from the dashboard — the Updates pane is read-only/informational and never mutates ([ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md)).

## Behavior

### Layout

```
┌ myplace <ver>   [VERDICT]   host (profile)            checked HH:MM:SS ┐  header bar (2 rows: title + rule)
├──────────────────────┬──────────────────────┬─────────────────────────┤
│ Dotfiles             │ Tools (mise)         │ Updates available       │  top row: three panes,
│  behind / to apply / │  missing / outdated, │  mise: N                │  ~55% of body height,
│  modified / uncommit │  with lists          │  brew: M                │  split into thirds
│  / unpushed, w/ lists │                      │  press o for details    │
├──────────────────────┴──────────────────────┴─────────────────────────┤
│ Activity                                                               │  full-width, remaining
│  notices (update available, errors) then recent log lines             │  body height
└────────────────────────────────────────────────────────────────────────┘
 r refresh • u update • o outdated • q quit              myplace 0.1→0.2 ↑  footer
```

Pressing `o` replaces the dashboard with a full-screen scrollable list of outdated packages grouped by source; `esc`/`q` returns. See [Outdated packages](outdated-packages.md).

- Pane sizes derive from `width`/`height`: header 2 rows, footer 1, body split ~55/45 between the top row and Activity; the top row splits width into thirds. Borders/padding are subtracted so panes tile exactly to the terminal edges.
- Content is truncated to pane width (ellipsis) and clipped to pane height (with a "+N more" line) so nothing wraps or overflows the frame.
- The "Updates available" pane is informational only — it shows per-source outdated counts and never changes the verdict badge. It loads asynchronously and shows `checking…` until ready, so the rest of the dashboard isn't blocked on it.

### Live activity

A 1-second `tea.Tick` reloads the tail of `myplace.log` into the Activity pane — so commands the dashboard runs (and anything else writing the log) scroll by in near-real-time. The tick only re-reads the file tail; it never triggers a status recompute or network call.

### States

- **Loading / updating**: a spinner centered in the full screen (`lipgloss.Place`). The full-screen spinner gates on the status/update load only — the outdated inventory loads independently, so the dashboard appears as soon as status is ready and the Updates pane fills in a beat later.
- **Outdated detail (`o`)**: a full-screen scrollable viewport listing outdated packages grouped by source; `esc`/`q` returns to the dashboard.
- **Small terminal** (below a minimum width/height): fall back to a simple stacked, unframed rendering rather than drawing broken boxes.

### Keys

`r` refresh (recompute status + reload the outdated inventory), `u` update (converge), `o` open the outdated detail view, `q`/`ctrl+c` quit. In the outdated view: `↑/↓`/`pgup`/`pgdn` scroll, `esc`/`q` back, `ctrl+c` quit.

## Acceptance criteria

- [ ] Dashboard fills the terminal and re-tiles on resize without overflow or wrapped borders.
- [ ] Dotfiles, Tools, and Updates-available panes sit side by side; Activity spans full width below.
- [ ] The Updates pane shows per-source counts after the inventory loads (and `checking…` before), without changing the verdict badge.
- [ ] `o` opens a scrollable outdated detail view grouped by source; `esc`/`q` returns to the dashboard.
- [ ] Activity shows recent `myplace.log` lines and updates roughly every second.
- [ ] Long file paths/tool lines are ellipsized; overlong lists show "+N more".
- [ ] Tiny terminals render a readable fallback, not garbage.
- [ ] No network/status recompute happens on the activity tick.

## Related

- [charm-tui-stack guide](../guides/charm-tui-stack.md) — layout APIs and the WindowSizeMsg/measure gotchas
- [Outdated packages](outdated-packages.md) — the Updates pane + the `o` detail view
- [Logging](logging.md) — the Activity pane tails this
- [ADR-0002](../adrs/0002-go-and-charm-for-the-tui.md) — the Charm stack
- [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md) — the outdated inventory rendered here
