---
title: TUI dashboard layout
status: accepted
created: 2026-06-12
updated: 2026-06-13
tags: [tui, bubbletea, lipgloss, layout, navigation, catppuccin]
phase: 1
---

# TUI dashboard layout

## Summary

The bare `myplace` dashboard is a full-screen, **interactive** terminal UI: a header bar, a passive system-info band (OS + base specs), a row of Dotfiles / Tools / Updates panes, a full-width Activity feed tailing the log, and a footer of key hints. Focus moves between panes; selecting an item opens a **detail panel** — the `chezmoi diff` of a drifted dotfile, or the version delta of a tool/package. On a wide terminal the detail panel sits to the right (master-detail); on a narrow one it replaces the body. `?` opens a help overlay, `u` shows per-step update progress, and `o` opens a sortable/filterable list of every outdated package. The whole thing is themed **Catppuccin** (Mocha dark / Latte light) and reflows on resize. It is **read-only** beyond the existing `u` converge ([ADR-0012](../adrs/0012-interactive-tui-navigation.md)).

## Motivation

The first cut was a static display — three read-only panes that ignored most of a large screen, with no way to answer "*what* changed in `.zshrc`?" without dropping to a shell. A status tool you glance at across many machines benefits from a real, navigable layout: focus a pane, arrow through items, read a file's diff side-by-side, all without leaving the TUI or mutating anything. The colors should also match the rest of the user's Catppuccin tooling rather than rolling terminal-dependent ANSI indices.

## Scope

### In scope

- A responsive full-screen layout (Lip Gloss panes sized from `tea.WindowSizeMsg`), themed with Catppuccin via a semantic theme abstraction ([ADR-0011](../adrs/0011-catppuccin-theming.md)).
- Panes: header bar, system-info band, Dotfiles, Tools, Updates available, Activity (log tail), footer.
- **Focus + selection**: move focus between panes; select an actionable item within the focused pane (drifted dotfiles, missing/outdated tools, outdated packages).
- **Master-detail panel**: the selected item's detail — a drifted dotfile's `chezmoi diff` (a read, fetched lazily and cached), or a tool/package version delta (rendered from already-loaded data). Wide terminals show it beside the panes; narrow ones swap the body for it.
- A **help overlay** (`?`) listing all keys.
- **Per-step update progress** for `u` (chezmoi apply → mise install → mise upgrade) instead of a bare spinner.
- An informational "Updates available" summary pane and a **sortable/filterable** `o` detail view of all outdated packages, loaded asynchronously — see [Outdated packages](outdated-packages.md).
- A live-refreshing Activity feed (cheap log tail on a timer; no network).
- Graceful degradation on small terminals.

### Out of scope

- **In-TUI mutation beyond the existing `u` converge.** No interactive keep/discard capture, no package upgrades from the TUI — those stay in the CLI per [ADR-0006](../adrs/0006-agent-runnable-commands.md) and [ADR-0012](../adrs/0012-interactive-tui-navigation.md). The detail panel only *reads* (`chezmoi diff`).
- **Editing a file from the diff view.** The diff is read-only; resolving drift is still `myplace update` (interactive capture in a terminal).
- **Mouse.** Keyboard-only.
- Upgrading packages from the dashboard — the Updates pane/`o` view is informational and never mutates ([ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md)).

## Behavior

### Layout

**Wide** (terminal ≥ ~120 cols) — master-detail; the detail panel takes the right ~40%:

```
┌ myplace <ver>  [VERDICT]  host (profile)        checked HH:MM:SS ┐  header (title + rule)
├─ Dotfiles ──────┬─ Tools ──────┬─ Updates ──┐┌─ Detail: .zshrc ──────────┐
│ ▸ .zshrc      ↑ │  fzf       + │  mise: 1   ││ modified locally          │  detail panel:
│   config.toml ↓ │  node 22→22… │  brew: 2   ││ diff vs source:           │  diff of the
│                 │              │  press o   ││  - alias gs='gits'        │  selected item
├─ Activity ──────┴──────────────┴────────────┤│  + alias gs='git status'  │  (right ~40%)
│ notices, then recent log lines              ││                           │
└─────────────────────────────────────────────┘└───────────────────────────┘
 tab focus • ↑↓ select • enter detail • r u o • ? help • q quit                footer
```

**Narrow** (~60–120 cols) — no side panel; the three panes tile across the top with Activity below (as before). `enter` on a selected item swaps the body for a full-width detail view; `esc` returns.

**Tiny** (below ~60×16) — a plain stacked, unframed rendering rather than broken boxes.

- Pane sizes derive from `width`/`height`: header 2 rows, footer 1; on wide the body splits ~60/40 (panes / detail), on narrow the body splits ~55/45 (top row / Activity) and the top row splits into thirds. Borders/padding are subtracted so panes tile exactly to the edges.
- The **focused** pane is drawn with a distinct border (focus ring); the **selected** row within it is highlighted. Content is truncated to pane width (ellipsis) and clipped to pane height (with a "+N more" line) so nothing wraps or overflows — including the highlight and the focus ring.
- The "Updates available" pane is informational only — per-source outdated counts, never changing the verdict badge. It loads asynchronously and shows `checking…` until ready, so the rest of the dashboard isn't blocked on it.

### Focus, selection, and detail

- **Focus** moves between the Dotfiles / Tools / Updates panes with `tab`/`shift+tab` or `h`/`l` (`←`/`→`).
- **Selection** moves within the focused pane with `j`/`k` (`↑`/`↓`) over that pane's *actionable* items (drifted dotfiles; missing/outdated tools; outdated packages) — not the static count lines.
- **Detail** for the selected item:
  - **Dotfile** → its `chezmoi diff` (what `apply` would change; for a locally-modified file, the change that would undo the local edit). Fetched lazily via a `tea.Cmd` when the file is selected, **cached per file**, shown with `+`/`-`/hunk coloring. Read-only.
  - **Tool / package** → name, `current → wanted`/`latest`, and source — rendered from the already-loaded `drift.Report` / `outdated.Inventory`; no new command runs.
- On a **wide** terminal the panel is always visible and tracks the selection. On a **narrow** one, `enter` opens it full-screen and `esc` closes it.

### Per-step update progress

`u` runs the converge-only update (same as headless `myplace update --yes`): chezmoi apply (skipped, with a notice, when local edits are present — see [ADR-0006](../adrs/0006-agent-runnable-commands.md)), then mise install, then mise upgrade. It's shown as a **floating modal window** centered over a dimmed (disabled) copy of the dashboard, containing a progress bar and a per-step checklist — completed steps `✓`, the active step a live spinner, pending steps `·` — advancing as each step finishes, then it reloads status. The backdrop stays visible (greyed) so context isn't lost; input is inert until the converge completes. It stays converge-only and mutates nothing else. (On a terminal too small to composite, it falls back to the modal centered on a blank screen.)

### Help overlay

`?` toggles a centered overlay listing every key and what it does (built from the keymap). `?`/`esc` closes it. The footer shows the short key hints; the overlay shows the full set.

### Live activity

A 1-second `tea.Tick` reloads the tail of `myplace.log` into the Activity pane — commands the dashboard runs (and anything else writing the log) scroll by in near-real-time. The tick only re-reads the file tail; it never triggers a status recompute or network call.

### System-info band

A passive, three-line band sits between the header and the panes (fixed height, so the layout never shifts): **identity** (OS · host model · arch — OS leads so it survives truncation), **compute** (CPU · cores · GPU · RAM used/total/free · root-disk used/total), and **runtime** (load averages · swap used/total/free · battery · local IPv4 · uptime). Colored labels (mauve) with a teal OS headline, dimmed values. It loads asynchronously from fastfetch alongside the status report — `system: loading…` until it lands — and degrades to a single `system: fastfetch unavailable` line if fastfetch isn't installed. It has no keybinding and never affects the verdict badge. See [System information](system-information.md).

### Outdated detail (`o`)

`o` opens a full-screen scrollable view of every outdated package as a bordered table (`lipgloss/table`: PACKAGE · CURRENT · LATEST · SOURCE), with a **count summary** (`N outdated across M sources`), **sort** (`s` cycles by source / by name), and **filter** (`/` for a case-insensitive name substring; `esc` clears). `↑/↓`/`pgup`/`pgdn` scroll; `esc`/`q` returns to the dashboard. See [Outdated packages](outdated-packages.md).

### States

- **Loading**: a spinner centered in the full screen (`lipgloss.Place`). The full-screen spinner gates on the status/update load only — the outdated inventory loads independently, so the dashboard appears as soon as status is ready and the Updates pane fills in a beat later.
- **Updating**: the per-step progress checklist (above).
- **Detail / outdated**: as described above.
- **Small terminal**: a simple stacked, unframed rendering rather than drawing broken boxes.

### Theme

All colors come from a semantic theme ([ADR-0011](../adrs/0011-catppuccin-theming.md)): Catppuccin **Mocha** by default, **Latte** when a light terminal background is detected. Roles include pane border, focus ring, pane title, selected-row highlight, add/del (and diff +/-), error, notice, subtle, and the verdict badges.

Each dashboard card carries its own **accent** (Dotfiles / Tools / Updates), shown as a colored leading bar on its title and a right-aligned **count chip** — an accent-colored badge with the number of items (drifted dotfiles, missing/outdated tools, outdated packages) or a green ✓ when the card is clean.

### Keys

| Key | Action | Context |
|-----|--------|---------|
| `tab` / `shift+tab`, `h`/`l`, `←`/`→` | move focus between panes | dashboard |
| `j`/`k`, `↑`/`↓` | select item in focused pane / scroll detail | dashboard, detail, `o` |
| `enter` | open detail for the selected item | dashboard (narrow) |
| `r` | refresh (recompute status + reload inventory) | dashboard |
| `u` | update (converge) | dashboard |
| `o` | open the outdated detail view | dashboard |
| `s` / `/` | cycle sort / filter | `o` view |
| `?` | toggle the help overlay | global |
| `esc` | close overlay / clear filter / back from detail or `o` | contextual |
| `q` / `ctrl+c` | quit | global |

## Acceptance criteria

- [ ] Dashboard fills the terminal and re-tiles on resize without overflow or wrapped borders, at widths from 60 up through 240 columns.
- [ ] On a wide terminal (≥ ~120 cols) the detail panel sits beside the panes; the focused pane shows a focus ring and the selected row is highlighted.
- [ ] `tab`/`hjkl`/arrows move focus and selection; selecting a drifted dotfile shows its `chezmoi diff` in the detail panel (read-only), with +/- coloring; the diff is fetched once and cached.
- [ ] Selecting a tool/package shows its version detail rendered from loaded data, with no extra command run.
- [ ] `u` shows the update as a floating modal (per-step checklist + bar) over a dimmed dashboard, preserves the local-edits skip, stays converge-only, and reloads status when done.
- [ ] `?` opens a help overlay listing all keys; `?`/`esc` closes it.
- [ ] `o` opens the outdated view with a count summary, `s` sort, and `/` filter; `esc` returns.
- [ ] The Updates pane shows per-source counts after the inventory loads (and `checking…` before), without changing the verdict badge.
- [ ] Activity shows recent `myplace.log` lines and updates roughly every second; no network/status recompute happens on the activity tick.
- [ ] Long file paths/tool lines are ellipsized; overlong lists show "+N more".
- [ ] Tiny terminals render a readable fallback, not garbage.
- [ ] The TUI mutates nothing except the existing `u` converge (the diff panel only reads `chezmoi diff`).
- [ ] Colors come from the Catppuccin theme (Mocha default, Latte on a light terminal); no hardcoded ANSI palette indices remain in render code.

## Related

- [charm-tui-stack guide](../guides/charm-tui-stack.md) — layout APIs, the WindowSizeMsg/measure gotchas, the theme + keymap + progress patterns
- [Outdated packages](outdated-packages.md) — the Updates pane + the sortable/filterable `o` detail view
- [Logging](logging.md) — the Activity pane tails this
- [ADR-0002](../adrs/0002-go-and-charm-for-the-tui.md) — the Charm stack
- [ADR-0006](../adrs/0006-agent-runnable-commands.md) — why mutation/capture stays in the CLI, not the TUI
- [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md) — the outdated inventory rendered here
- [ADR-0011](../adrs/0011-catppuccin-theming.md) — the theme
- [ADR-0012](../adrs/0012-interactive-tui-navigation.md) — the interactive, read-only navigation model
