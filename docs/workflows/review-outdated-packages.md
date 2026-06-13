---
title: Review outdated packages
status: active
created: 2026-06-13
updated: 2026-06-13
tags: [outdated, packages, mise, homebrew, status]
actors: [user, tui, mise, brew]
---

# Review outdated packages

## Goal

See what software is upgradable on this machine, across every package manager present — and understand which of it myplace will fix versus which the user upgrades themselves.

## Preconditions

- myplace is installed and the machine is set up (mise present; chezmoi initialized).
- Homebrew is optional: if it's on PATH its packages are included; if not, the brew source is skipped (no error).
- No network is required for brew (it reads its local DB); mise may reach the network to learn latest versions.

## Steps

1. **Glance (TUI).** Run `myplace` with a TTY. The dashboard's **"Updates available"** pane shows per-source counts (`mise: N`, `brew: M`). This is informational — it does **not** change the IN SYNC / DRIFTED badge.
2. **Drill in.** Press **`o`** to open the scrollable outdated view: every outdated package grouped by source, as `name current → latest`. `↑/↓`/`pgup`/`pgdn` scroll; `esc`/`q` returns to the dashboard.
3. **Headless / scripted.** Off a TTY (agent, cron, SSH), run `myplace outdated --json` — under the hood it runs `mise outdated --json` and, when brew is present, `brew outdated --json=v2` (formulae + casks). Branch on the exit code: `0` all current, `1` updates available, `3` couldn't check.
4. **Decide what to do (outside this command — it only reports):**
   - **mise tools** behind their pinned versions are *managed drift*; converge them with `myplace update` (or `myplace update --yes --json`). These also show in `myplace status` as drift.
   - **brew formulae/casks** are *not* managed by myplace and are shown for awareness only. Upgrade them yourself when you choose (`brew upgrade [<name>]`). myplace never runs `brew upgrade` ([ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md)).

## Outcome

The user knows exactly what's upgradable and from where, with a clear line between "`myplace update` fixes this" (mise) and "you upgrade this when you want" (brew/unmanaged). Nothing on the machine has changed — this workflow is read-only.

## Failure modes

| What can go wrong | How the user finds out | Recovery |
|-------------------|------------------------|----------|
| brew not installed | brew source shows `n/a` (TUI) / `available: false` (JSON) | expected on Linux/brew-less Macs; not an error |
| a source errors (e.g. `mise outdated` fails) | that source shows its error; others still report; exit stays 0/1 if any source produced a result | rerun; check `~/.local/state/myplace/myplace.log` |
| brew list looks stale | versions reflect the last `brew update` | run `brew update` yourself first — myplace deliberately doesn't |
| no manager available at all | exit `3` (error) | ensure mise is installed (`myplace bootstrap`) |

## Related

- [Outdated packages feature](../features/outdated-packages.md) — command, JSON envelope, TUI view
- [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md) — why this is informational and separate from drift
- [Check machine status](check-machine-status.md) — the drift verdict (where mise outdated counts, brew does not)
- [Update a machine](update-machine.md) — how mise drift actually gets fixed
