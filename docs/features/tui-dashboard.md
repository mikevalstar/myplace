---
title: TUI dashboard layout
status: accepted
created: 2026-06-12
updated: 2026-06-12
tags: [tui, bubbletea, lipgloss, layout]
phase: 1
---

# TUI dashboard layout

## Summary

The bare `myplace` dashboard is a full-screen, paneled terminal UI: a header bar, side-by-side Dotfiles and Tools panes, a full-width Activity feed tailing the log, and a footer of key hints. It fills the terminal and reflows on resize.

## Motivation

The first cut was a few top-anchored lines that ignored most of the screen. A status tool you glance at across many machines benefits from a real layout — drift, tools, and recent activity visible at once, using the whole terminal.

## Scope

### In scope

- A responsive full-screen layout (Lip Gloss panes sized from `tea.WindowSizeMsg`).
- Panes: header bar, Dotfiles, Tools, Activity (log tail), footer.
- A live-refreshing Activity feed (cheap log tail on a timer; no network).
- Graceful degradation on small terminals.

### Out of scope

- Scrolling/selecting within panes, mouse, tabs (could come later via `bubbles` viewport/list).
- In-TUI interactive capture (keep/discard) — still deferred to the CLI per [ADR-0006](../adrs/0006-agent-runnable-commands.md) and the update workflow.

## Behavior

### Layout

```
┌ myplace <ver>   [VERDICT]   host (profile)            checked HH:MM:SS ┐  header bar (2 rows: title + rule)
├───────────────────────────────┬───────────────────────────────────────┤
│ Dotfiles                      │ Tools (mise)                          │  top row: two panes,
│  behind origin / to apply /   │  missing / outdated, with lists       │  ~55% of body height,
│  modified / uncommitted /     │                                       │  split left/right
│  unpushed, with file lists    │                                       │
├───────────────────────────────┴───────────────────────────────────────┤
│ Activity                                                               │  full-width, remaining
│  notices (update available, errors) then recent log lines             │  body height
└────────────────────────────────────────────────────────────────────────┘
 r refresh • u update • q quit                          myplace 0.1→0.2 ↑  footer
```

- Pane sizes derive from `width`/`height`: header 2 rows, footer 1, body split ~55/45 between the top row and Activity; the top row splits width in half. Borders/padding are subtracted so panes tile exactly to the terminal edges.
- Content is truncated to pane width (ellipsis) and clipped to pane height (with a "+N more" line) so nothing wraps or overflows the frame.

### Live activity

A 1-second `tea.Tick` reloads the tail of `myplace.log` into the Activity pane — so commands the dashboard runs (and anything else writing the log) scroll by in near-real-time. The tick only re-reads the file tail; it never triggers a status recompute or network call.

### States

- **Loading / updating**: a spinner centered in the full screen (`lipgloss.Place`).
- **Small terminal** (below a minimum width/height): fall back to a simple stacked, unframed rendering rather than drawing broken boxes.

### Keys

`r` refresh (recompute status), `u` update (converge), `q`/`ctrl+c` quit — unchanged.

## Acceptance criteria

- [ ] Dashboard fills the terminal and re-tiles on resize without overflow or wrapped borders.
- [ ] Dotfiles and Tools panes sit side by side; Activity spans full width below.
- [ ] Activity shows recent `myplace.log` lines and updates roughly every second.
- [ ] Long file paths/tool lines are ellipsized; overlong lists show "+N more".
- [ ] Tiny terminals render a readable fallback, not garbage.
- [ ] No network/status recompute happens on the activity tick.

## Related

- [charm-tui-stack guide](../guides/charm-tui-stack.md) — layout APIs and the WindowSizeMsg/measure gotchas
- [Logging](logging.md) — the Activity pane tails this
- [ADR-0002](../adrs/0002-go-and-charm-for-the-tui.md) — the Charm stack
