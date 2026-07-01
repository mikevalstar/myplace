---
title: ADR-0017 — A personal-linux desktop profile
status: accepted
created: 2026-07-01
updated: 2026-07-01
tags: [profiles, provisioning, linux, desktop, fonts, 1password, ssh]
supersedes: null
superseded-by: null
---

# ADR-0017: A personal-linux desktop profile

## Context

Until now the fleet was three profiles — `personal-mac`, `work-mac`, `server` — where "desktop" and "Mac" were effectively synonyms and "server" and "Linux" were too. Two behaviors leaned on that coincidence:

- **SSH host list from 1Password.** The `~/.ssh/config` template pulls the host list (with server IPs) from a 1Password Document on any non-`server` profile ([ADR-0016](0016-secrets-in-dotfiles-via-1password.md)). In practice that meant "the Macs," and `op` (the 1Password CLI) was installed only on macOS, via a Homebrew cask.
- **Fonts / GUI assets are macOS-only** ([ADR-0009](0009-homebrew-casks-macos.md)), installed as Homebrew casks, because the only non-Mac machines were headless servers. ADR-0009 explicitly deferred the Linux-desktop case: *"If a Linux desktop ever joins the fleet and needs fonts, this ADR doesn't cover it — revisit."*

A personal Linux desktop is now being set up. It is a **desktop** (wants the SSH hosts, wants terminal fonts) but runs **Linux** (no Homebrew, no casks). It breaks the desktop==Mac and Linux==server coincidences, so it needs its own profile and a provisioning path for the two desktop assets that were previously Mac-only.

The OS axis was already handled: templates branch on `.chezmoi.os` (mise's `btop`, the provision script's neovim/git/zsh installs). What was missing was a *desktop-but-Linux* profile and the two assets gated to it.

## Options considered

### Option A — Reuse `server` or a `work-mac`-like profile

Don't add a profile; pick an existing one. Rejected: `server` skips the SSH-host pull and the push policy is consume-only, neither of which fits a personal desktop; the `*-mac` profiles would (wrongly) drive macOS-only template branches. Profiles are the designed axis for exactly this kind of per-machine variation.

### Option B — Add `personal-linux`, gate the desktop assets on OS only

Add the profile but install `op` and fonts on *all* Linux. Rejected: that would put `op` and Nerd Fonts on the headless Linux **servers** too — pointless there and, for `op`, actively wrong (servers deliberately never touch 1Password). The gate has to be the *profile* (desktop vs server), not the OS.

### Option C — chezmoi externals for the fonts

Install the Nerd Fonts via chezmoi's external-archive feature (ADR-0009's Option A). Rejected for the same reason it was before: externals aren't profile-aware without extra gymnastics and would tend to land on every machine. The provision script already fetches non-registry binaries this way (fnm, pay-respects, neovim), and — once templated — it *is* profile-aware. Fonts fit there.

### Option D — Add `personal-linux`; install `op` + fonts in a profile-gated provision block (chosen)

Add the profile and template the provision script so a single block, gated to non-`server` Linux, installs both desktop assets straight into `$HOME`.

## Decision

Option D.

1. **New profile `personal-linux`**, added to the bootstrap choices (`cmd/myplace/bootstrap.go`) and the init prompt's allowed list (`home/.chezmoi.toml.tmpl`). It is a non-`server` profile, so it inherits the existing defaults automatically: `push = true` (a desktop may originate shared config) and the 1Password SSH-host pull. No new template branches were needed for those — they already keyed on `ne .profile "server"`.

2. **Provision script becomes a template** (`run_onchange_provision.sh` → `run_onchange_provision.sh.tmpl`) so it can branch on `.profile` — the pattern the managed-setup guide already prescribes for per-machine variation. Servers render the script without the desktop block, exactly as before.

3. **A Linux-desktop block**, gated `{{ if and (eq .chezmoi.os "linux") (ne .profile "server") }}`, installs into `$HOME` (no root, distro-agnostic — the target is Arch/CachyOS, but the code assumes only a Linux kernel):
   - **1Password CLI** — the official static `op` binary into `~/.local/bin` (pinned version, bumped in-place). Required because the non-`server` SSH template shells out to `op` on every `apply`/`status`/`diff`; without it those commands would exit 3 on the desktop. Sign-in is via the 1Password desktop app's CLI integration, same as macOS.
   - **Nerd Fonts** — the same families the Macs get as casks (JetBrainsMono, FiraCode, Monaspace, and the symbols-only overlay), downloaded from the `ryanoasis/nerd-fonts` releases into `~/.local/share/fonts` with an `fc-cache` refresh. Idempotent per family; failure-tolerant like the rest of the script.

This establishes the general principle that **"desktop vs server" is a profile distinction, independent of OS** — the `*-mac` and `personal-linux` profiles are desktops; `server` is not. Desktop-only assets (SSH hosts, `op`, fonts) gate on `ne .profile "server"`; OS-specific *mechanisms* (brew cask vs `$HOME` binary) still gate on `.chezmoi.os`.

## Consequences

- A personal Linux desktop bootstraps to parity with the Macs: shared CLI toolset, SSH hosts from 1Password, and terminal fonts. `myplace bootstrap --profile personal-linux` (or the wizard) is all it takes.
- The provision script is now a chezmoi **template**. It has no template syntax outside the one gated block, but future edits must stay valid Go-template *and* POSIX shell. (Verified: renders cleanly for both `personal-linux` and `server`, and the rendered script passes `sh -n`.)
- `op` is now a soft dependency on Linux desktops, not just Macs — `myplace status` shells out to it whenever it recomputes the SSH target. A locked/absent `op` makes `status` exit 3 there, same failure mode as the Macs (ADR-0016).
- The `op` version is pinned in the script (1Password publishes no stable "latest" URL for the raw binary); bump it on upgrades. The fonts resolve "latest" from the GitHub release redirect, like neovim.
- Fonts are no longer strictly macOS-only: ADR-0009's deferred Linux-desktop case is now handled here, via the provision script rather than casks or externals. ADR-0009's macOS cask path is unchanged.
- Not covered: GUI *applications* on Linux (beyond fonts + `op`), and a work-Linux profile. Add them when a machine needs them.

## Related

- [ADR-0009](0009-homebrew-casks-macos.md) — macOS fonts via casks; this handles the Linux-desktop case it deferred
- [ADR-0016](0016-secrets-in-dotfiles-via-1password.md) — the 1Password SSH-host pull that makes `op` a desktop dependency
- [ADR-0007](0007-provisioning-mechanism.md) — the provisioning split the desktop block extends
- [bootstrap workflow](../workflows/bootstrap-new-machine.md) · [managed-setup guide](../guides/managed-setup.md)
