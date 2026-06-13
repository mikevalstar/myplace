---
title: ADR-0005 — Machine-local state lives in the XDG state dir
status: accepted
created: 2026-06-12
updated: 2026-06-12
tags: [logging, state, xdg, filesystem]
supersedes: null
superseded-by: null
---

# ADR-0005: Machine-local state lives in the XDG state dir

## Context

myplace needs somewhere to write machine-local runtime data — starting with logs, and later a cached last-status report (for fast TUI startup and phase-2 server pings). This data is per-machine, regenerable, and must **never** end up in the dotfiles repo.

The obvious-but-wrong choice is `~/.config/myplace/`. `~/.config` is exactly the tree chezmoi manages from this repo; putting logs there invites them into `chezmoi status`/`add` and muddies the "is this machine in sync?" signal. Config and state are different things.

## Options considered

### Option A — `~/.config/myplace/`

What the user first reached for. Wrong: it's chezmoi's territory, and logs/caches are not configuration.

### Option B — XDG state dir: `$XDG_STATE_HOME/myplace` (default `~/.local/state/myplace`)

The XDG Base Directory spec's purpose-built location for "state data that should persist between restarts but isn't important enough for `~/.local/share`" — logs and history are its canonical examples. Cleanly separate from the chezmoi-managed config tree.

## Decision

Option B. All machine-local state goes under a single directory resolved as:

1. `$MYPLACE_STATE_DIR` if set (escape hatch / tests), else
2. `$XDG_STATE_HOME/myplace` if `XDG_STATE_HOME` is set, else
3. `~/.local/state/myplace`.

First use: `myplace.log`. Reserved for later: `last-status.json` (cache), and whatever phase 2 needs to track server check-ins.

## Consequences

- Logs and caches are structurally outside chezmoi's scope — no special ignore rules needed, no false drift.
- One resolver (`internal/logging.Dir`, to be generalized if more state types appear) owns the path logic; everything else asks it.
- Cross-platform note: on macOS there's no real XDG default, so `~/.local/state` is used directly rather than `~/Library/...` — consistent with how mise and other XDG-respecting CLIs behave, and keeps server/Mac parity.
- If a machine sets `XDG_STATE_HOME` to something exotic, logs follow it; documented in the logging feature spec.
