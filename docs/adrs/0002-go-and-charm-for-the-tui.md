---
title: ADR-0002 — Go with the Charm stack for the TUI
status: accepted
created: 2026-06-12
updated: 2026-06-12
tags: [tui, go, charm, bubbletea]
supersedes: null
superseded-by: null
---

# ADR-0002: Go with the Charm stack for the TUI

## Context

The TUI must run on a brand-new machine *before* anything else is installed — no Homebrew, no mise, no language runtime. That makes "single static binary installable via `curl | sh`" a hard requirement, not a preference. Targets are macOS (Apple Silicon and Intel) and Linux servers (amd64, some arm64).

Two further constraints from the project's design:

- Every capability must also work headlessly (`myplace status --json`), because phase 2's server reporting is built on the same output. The core logic therefore can't be welded to the TUI layer.
- The TUI orchestrates chezmoi and mise by shelling out, so first-class `os/exec`-style subprocess handling matters more than raw rendering performance.

## Options considered

### Option A — Go + Charm libraries (Bubble Tea, Bubbles, Lip Gloss, Huh)

Static cross-compiled binaries with zero runtime deps. The Charm ecosystem is the most complete TUI toolkit available: framework, prebuilt components, styling, and form/wizard support. chezmoi itself is written in Go, leaving the door open to importing parts of it as a library later. Trade-off: garbage-collected runtime and Go's verbosity, neither of which matters for this workload.

### Option B — Rust + ratatui

Also static binaries, excellent performance. But ratatui is lower-level (immediate-mode, bring-your-own architecture), there's no equivalent of Bubbles/Huh for components and forms, and iteration speed is slower. Performance headroom buys nothing for a tool that mostly waits on subprocesses.

### Option C — TypeScript + Ink

Fastest iteration, but requires Node at runtime — fails the bootstrap constraint outright. Eliminated.

## Decision

Option A: **Go**, with the **Charm** stack — and it's also the author's stated preference:

- **bubbletea** — TUI framework (Elm architecture)
- **bubbles** — stock components (spinner, table, list, viewport, progress, help)
- **lipgloss** — styling and layout
- **huh** — forms and prompts (bootstrap wizard, confirmations)
- **log** — logging
- **cobra** (+ Charm's **fang** for polish) — CLI command structure, so `myplace status --json` and friends exist independently of the TUI

Core logic (invoking chezmoi/mise, parsing output, computing drift) lives in TUI-free internal packages; the cobra commands and the Bubble Tea app are both thin layers over that core.

## Consequences

- Release builds cross-compile a matrix: `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`; distribution starts as GitHub releases + a `curl | sh` installer script.
- Contributors (human or AI) need Go and familiarity with the Elm architecture — captured in the companion guide [charm-tui-stack.md](../guides/charm-tui-stack.md).
- The headless `--json` requirement is enforced structurally: TUI packages must not be imported by core packages, ever.
- Follow-ups created: scaffold the Go module, pick a Bubble Tea major version (v1 vs v2 — see guide gotchas), write the installer script.
