---
title: ADR-0011 — Catppuccin theming behind a semantic Theme abstraction
status: accepted
created: 2026-06-13
updated: 2026-06-13
tags: [tui, theming, lipgloss, catppuccin]
supersedes: null
superseded-by: null
---

# ADR-0011: Catppuccin theming behind a semantic Theme abstraction

## Context

The TUI's colors are hardcoded ANSI palette indices scattered through package-level `lipgloss` vars in `internal/tui/dashboard.go` (`lipgloss.Color("240")` for borders, `"63"` for titles, `"9"`/`"11"`/`"42"`/`"214"` for error/notice/add/del, etc.). Two problems:

1. **No coherent palette.** ANSI 0–15 render against whatever 16-color scheme the user's terminal ships; 240/214-style indices assume a 256-color terminal and clash with anything else. The result looks different on every machine — exactly the machines this tool is meant to feel consistent across.
2. **Not swappable.** As the TUI grows an interactive layer ([ADR-0012](0012-interactive-tui-navigation.md)) it needs new roles (a focus ring, a selected-row highlight, diff +/- coloring) and there's no single place to define them or to flip dark/light.

The rest of the user's tooling is already themed **Catppuccin** (Mocha dark / Latte light) via the dotfiles. The TUI should match. `github.com/catppuccin/go` is already in `go.mod` (pulled in indirectly) — its `Flavor` interface exposes the full named palette (`Mauve()`, `Surface1()`, `Lavender()`, …) as `Color{Hex, RGB, HSL}`.

A constraint from [ADR-0003](0003-monorepo-app-dotfiles-mise.md): only `internal/tui` may import `lipgloss`. A theme that hands out `lipgloss.Style`/`lipgloss.Color` must therefore live inside `internal/tui`.

## Options considered

### Option A — Keep the ANSI indices, just centralize them

Pull the existing color vars into one struct so they're defined once. Cheap, but it keeps the incoherent palette and the terminal-dependent rendering. We'd still be picking 256-color indices by eye. Solves swappability, not consistency.

### Option B — A raw-hex theme package outside `internal/tui`

Put a `theme` package elsewhere returning hex strings, so non-TUI code could theme too. But nothing outside the TUI emits color — the CLI commands print plain text, and the headless `--json` path has no styling. It'd be an abstraction with exactly one consumer, and it couldn't return `lipgloss.Style` (the layering rule), so the TUI would re-wrap every hex anyway. Rejected as premature.

### Option C — Catppuccin behind a semantic `Theme` in `internal/tui` (chosen)

A `Theme` struct of resolved `lipgloss.Style` values keyed by **semantic role** (PaneStyle, PaneFocused, PaneTitle, Header, Rule, Help, Subtle, Err, Notice, Add, Del, Selected, OK/Warn/Error badges) — not by Catppuccin color name. `NewTheme(flavor)` builds those styles from a `catppuccingo.Flavor`; `DefaultTheme()` picks Mocha or Latte from `lipgloss.HasDarkBackground()`. Render code reads `m.theme.X`, never a raw color. Swapping flavor (or adding a `--theme` flag / `MYPLACE_THEME` env later) is one constructor call; adding a role is one field.

## Decision

Option C. Specifically:

- **Semantic roles, not color names.** Render code never references `Mauve` or `Surface1` directly — it asks the theme for `PaneTitle`, `Selected`, etc. The Catppuccin→role mapping lives in exactly one place (`NewTheme`), so re-theming is a palette swap, not a find-and-replace.
- **Catppuccin is now a direct dependency.** `github.com/catppuccin/go` moves from indirect to a direct `require`. Mocha is the default flavor; Latte is used when the terminal reports a light background.
- **Dark/light by detection, with a safe default.** `DefaultTheme()` uses `lipgloss.HasDarkBackground()`; when detection is ambiguous it returns Mocha (dark), which is the common case across these machines and never renders unreadably on a dark terminal.
- **Lives in `internal/tui`.** It returns `lipgloss.Style`, so the layering rule (only the TUI imports Charm UI libs) forces it here. That's fine — it has no other consumer.

## Consequences

- One coherent palette across every machine, matching the rest of the user's Catppuccin tooling, instead of terminal-roulette ANSI indices.
- The interactive layer ([ADR-0012](0012-interactive-tui-navigation.md)) gets the roles it needs (focus ring = Lavender border, selected row = Surface0 background, diff +/- = Green/Peach) defined alongside the rest.
- A future `--theme`/`MYPLACE_THEME` selector (Frappé/Macchiato, or a forced flavor) is trivial — `catppuccingo.Variant(name)` → `NewTheme`. Not built now; the seam is there.
- `HasDarkBackground()` queries the terminal and can be wrong or slow over some multiplexers/SSH; we accept Mocha as the fallback and read it once at model construction, not per frame (see the [charm-tui-stack guide](../guides/charm-tui-stack.md)).
- Tests that build a `Model` directly get a real theme via `New`, so styled output is exercised by the existing render tests (no separate theme wiring needed).

## Related

- [ADR-0002](0002-go-and-charm-for-the-tui.md) — the Charm/lipgloss stack this themes
- [ADR-0003](0003-monorepo-app-dotfiles-mise.md) — the layering rule that pins the theme inside `internal/tui`
- [ADR-0012](0012-interactive-tui-navigation.md) — the interactive layer that consumes the new roles
- [TUI dashboard layout](../features/tui-dashboard.md) — where the roles render
- [charm-tui-stack guide](../guides/charm-tui-stack.md) — the theme pattern and the `HasDarkBackground` gotcha
