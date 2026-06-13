---
title: ADR-0008 — Opportunistic Homebrew for non-registry CLI tools on macOS
status: accepted
created: 2026-06-13
updated: 2026-06-13
tags: [provisioning, homebrew, macos, mise, chezmoi]
supersedes: null
superseded-by: null
---

# ADR-0008: Opportunistic Homebrew for non-registry CLI tools on macOS

## Context

[ADR-0007](0007-provisioning-mechanism.md) splits provisioning in two: registry binaries go in the mise config, and everything else (installers, frameworks, packages mise can't carry) goes in `home/.chezmoiscripts/run_onchange_provision.sh`. On Linux that script installs packages through one `pm_install` helper that dispatches across `apt`/`dnf`/`yum`/`pacman`/`apk`. A deliberate constraint of that ADR is the **no-brew bootstrap rule**: bootstrap must work on a bare machine without Homebrew, so nothing in the path may *require* brew.

That rule left a gap. Some wanted CLI tools are in neither mise's registry nor reachable by a clean cross-platform installer — e.g. **httpie** (a Python app; only `httpie-go`, a different tool, is in the registry) and **mosh** (a C++ app with no installer script). On Linux they install fine via `pm_install`. On macOS there is no system package manager *by the project's own rule*, so there was no path at all — and the fleet is mostly Macs, so "Linux only" leaves the primary machines uncovered.

## Options considered

### Option A — Linux only, macOS manual

`pm_install` on Linux; on macOS log an "install it yourself" note (the pattern already used for git). Dead simple and strictly preserves the no-brew rule — but the majority of the fleet (Macs) never gets these tools from a fresh setup, defeating the point of a managed setup.

### Option B — Require Homebrew on macOS

Make the macOS path `brew install` and treat a brew-less Mac as unsupported. Covers Macs, but turns Homebrew into a hard bootstrap dependency, directly breaking ADR-0007's no-brew guarantee.

### Option C — Force everything through another backend

E.g. install httpie via mise's `pipx:` backend. Works for httpie but drags Python into the global tool set (a toolchain the owner otherwise manages deliberately and narrowly), and still does nothing for mosh, which has no such backend.

### Option D — Opportunistic Homebrew on macOS (chosen)

The provision script installs non-registry packages via `pm_install` on Linux and via `brew` on macOS **only if brew is already installed**; if it isn't, it logs a note and moves on. Bootstrap never installs or requires brew, so the no-brew guarantee still holds for the bootstrap path — brew is used merely when the user already has it (true of every current Mac).

## Decision

Option D. A new `ensure_tool <command> <package>` helper in the provision script encapsulates it: skip if the command is already on PATH; on macOS use `brew` when present (note otherwise); on Linux use `pm_install`. Idempotent and failure-tolerant like the rest of the script. The first tools provisioned this way are **httpie** and **mosh**.

This **extends** ADR-0007's "what mise can't own" branch with a macOS path; it does not supersede it. The no-brew *bootstrap* rule stands — what changes is that Homebrew is now a recognized *optional* provisioner on macOS, not a forbidden one.

## Consequences

- Macs that have Homebrew (all current ones) pick up these tools automatically; Linux servers get them via `pm_install`; a brew-less Mac degrades to a logged note instead of a hard failure.
- Homebrew is an *optional* mechanism, not a dependency: we only ever call `brew install <formula>`. We do not install, update, or tap brew, and we never gate bootstrap on it.
- Keep the brew-provisioned set small. Anything in mise's registry still belongs in mise (ADR-0007); `ensure_tool`/brew is the fallback only for tools no other mechanism can install on macOS. Triage a new tool registry → installer-script → `ensure_tool`, in that order.
- Future `status`/doctor work can check whether brew-provisioned tools are present and surface the brew-less-Mac case, without changing this contract.

## Related

- [ADR-0007](0007-provisioning-mechanism.md) — the provisioning split this extends
- [ADR-0003](0003-monorepo-app-dotfiles-mise.md) — the mise config as a managed dotfile
- [managed-setup guide](../guides/managed-setup.md) — how to add a tool, including the `ensure_tool` path
