---
title: ADR-0009 — Homebrew casks for macOS GUI/font assets
status: accepted
created: 2026-06-13
updated: 2026-06-13
tags: [provisioning, homebrew, macos, fonts, casks]
supersedes: null
superseded-by: null
---

# ADR-0009: Homebrew casks for macOS GUI/font assets

## Context

[ADR-0007](0007-provisioning-mechanism.md) split provisioning into mise (registry CLI tools) and the chezmoi provision script (everything else); [ADR-0008](0008-opportunistic-homebrew-macos.md) added opportunistic Homebrew on macOS for non-registry CLI tools installed with `brew install` (formulae). Now we want **fonts** on the machines — Monaspace Nerd Font plus a few Nerd Fonts (symbols-only, JetBrains Mono, FiraCode) for the terminal glyphs that starship/eza rely on.

Fonts don't fit any existing slot: they're not CLI binaries, not in mise's registry, and not `brew install` formulae — they're Homebrew **casks** (`brew install --cask`). Two more facts shape the decision: casks are a macOS-only concept, and in this fleet fonts only matter on the desktop Macs — the Linux machines are headless servers where fonts are pointless. The setup previously had no GUI/cask/font mechanism at all.

## Options considered

### Option A — chezmoi externals (cross-platform font files)

Use chezmoi's external-archive feature to download font releases and extract them into the per-OS font dir (`~/Library/Fonts` on macOS, `~/.local/share/fonts` + `fc-cache` on Linux). Portable and chezmoi-native, but heavy (Nerd Font archives are large), needs OS-templated paths and a Linux cache-refresh step, and would install fonts on headless servers — the only non-mac machines in the fleet — for no benefit.

### Option B — Vendor font files into the chezmoi source tree

Commit the font binaries to the repo and place them as managed files. Bloats the repo with large binaries and makes updates a manual re-vendor. Rejected.

### Option C — Homebrew casks on macOS, brew-if-present (chosen)

Extend the ADR-0008 pattern from formulae to casks with an `ensure_cask` helper: macOS-only, install via `brew install --cask` when Homebrew is present, note otherwise. Mac-only by nature, which exactly matches where fonts are wanted. Matches how these fonts are already installed by hand on the Macs.

## Decision

Option C. Add `ensure_cask NAME` to the provision script: it returns immediately off macOS, installs via `brew install --cask NAME` when brew is present (logs a note when it isn't), and guards on `brew list --cask` so re-runs are no-ops. Same brew-if-present, idempotent, failure-tolerant contract as `ensure_tool`. The first casks are `font-monaspace-nf`, `font-symbols-only-nerd-font`, `font-jetbrains-mono-nerd-font`, and `font-fira-code-nerd-font`.

This **extends** ADR-0008 (formulae → casks) and inherits its guarantee: bootstrap never installs or requires Homebrew, so the no-brew bootstrap rule still holds — casks install only when brew is already there. It also sets a scoping principle: **GUI and font assets are macOS-only in this setup**, because the Linux fleet is servers.

## Consequences

- Fonts land automatically on Macs that have Homebrew (all current ones); Linux servers correctly skip them; a brew-less Mac degrades to a logged note.
- Homebrew casks are now a recognized *optional* macOS mechanism alongside formulae. We only ever call `brew install --cask <name>`; we don't manage taps or Homebrew itself, and nothing gates bootstrap on it.
- Keep the cask set intentional — casks are heavier than CLI tools and macOS-only. Triage stays: mise registry → installer script → `ensure_tool` (CLI) / `ensure_cask` (macOS GUI/font).
- If a Linux *desktop* ever joins the fleet and needs fonts, this ADR doesn't cover it — revisit with Option A (chezmoi externals) scoped to a desktop profile. Deliberately deferred (the current non-mac machines are all headless).

## Related

- [ADR-0008](0008-opportunistic-homebrew-macos.md) — opportunistic Homebrew for CLI formulae; this extends the same idea to casks
- [ADR-0007](0007-provisioning-mechanism.md) — the provisioning split
- [managed-setup guide](../guides/managed-setup.md) — how to add a font/cask via `ensure_cask`
