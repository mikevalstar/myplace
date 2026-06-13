---
title: ADR-0007 — How tools and frameworks get installed (mise vs chezmoi scripts)
status: accepted
created: 2026-06-13
updated: 2026-06-13
tags: [provisioning, mise, chezmoi, rustup, fnm, oh-my-zsh]
supersedes: null
superseded-by: null
---

# ADR-0007: How tools and frameworks get installed (mise vs chezmoi scripts)

## Context

"Saving the setup" means a machine ends up with a specific shell framework (oh-my-zsh + plugins), language toolchains (Rust, Node), and a set of CLI tools (fzf, bat, starship, …). These come from different worlds: some are simple binaries in mise's registry, others are installer scripts (rustup), git-cloned frameworks (oh-my-zsh), or tools mise doesn't carry (fnm). We need one clear rule for where each kind lives, so the setup is reproducible and the next person knows where to add things.

## Decision

Two mechanisms, chosen by what the thing *is*:

1. **mise — registry CLI tools.** Anything in mise's registry that's "just a binary" goes in `home/dot_config/mise/config.toml.tmpl`: `jq`, `ripgrep`, `fd`, `bat`, `fzf`, `eza`, `zoxide`, `starship`, `atuin`, `gum`, `pnpm`. Installed/updated by the normal `mise install` step.

2. **chezmoi scripts — installers, frameworks, and non-registry tools.** A single idempotent `home/.chezmoiscripts/run_onchange_provision.sh` installs the things mise can't cleanly own:
   - **git** — not in mise's registry, and a *prerequisite* (chezmoi and the plugin clones below need it). Ensured first via the OS package manager on Linux (`apt`/`dnf`/`yum`/`pacman`/`apk`); on macOS it ships with the Xcode Command Line Tools, so the script points there if it's somehow missing. chezmoi's built-in git can clone the source repo on a git-less machine, so installing git from inside the script is not too late for everything that follows. git identity (`~/.gitconfig`) is a managed dotfile (`home/dot_gitconfig.tmpl`), rendered from name/email collected at install time (the `.chezmoi.toml.tmpl` prompts, pre-fillable headlessly via `--promptString`/`bootstrap --git-email`) — so the work mac can carry a different commit email without a per-profile template branch.
   - **oh-my-zsh** + external plugins (`zsh-autosuggestions`, `zsh-syntax-highlighting`) — a framework that lives as a git checkout under `~/.oh-my-zsh`, installed with `KEEP_ZSHRC=yes` so our chezmoi-managed `.zshrc` is never clobbered.
   - **rustup** — installed via the official `sh.rustup.rs` script. The owner wants the rustup toolchain manager specifically (`rustup`/`cargo`/per-toolchain components), not a single mise-pinned `rust` binary.
   - **fnm** — Node version manager. Not in mise's registry, and the owner manages Node per-project via fnm (`fnm env --use-on-cd`). Installed via fnm's official installer into `~/.local/bin`.

   `run_onchange_` (not `run_once_`) so editing the script — e.g. adding a plugin — re-runs it; every step is guarded by an existence check, so re-runs are no-ops.

**Consequence for Node and Rust: they are deliberately NOT mise tools.** `node` was removed from the mise set — fnm owns Node. Rust is owned by rustup. mise manages everything else.

### Ordering

Bootstrap runs `chezmoi apply` (which runs the provision script) *before* `mise install`. So the provision script may not assume any mise tool is present — which is why oh-my-zsh, rustup, and fnm are installed by their own installers, not via mise.

## Consequences

- Clear home for every new thing: a registry binary → add a line to the mise config; an installer/framework/non-registry tool → add a guarded block to the provision script. Documented in the [managed-setup guide](../guides/managed-setup.md).
- The provision script is idempotent and failure-tolerant (a failed install logs and continues rather than aborting the whole apply), matching the bootstrap workflow's "tool failures don't abort" rule.
- Shell wiring lives in the managed `.zshrc`: it activates mise (`eval "$(mise activate zsh)"`) so mise tools are on PATH, then sources fnm and cargo env (guarded), so the two non-mise toolchains light up too.
- Cross-platform: every installer used works on macOS and Linux without Homebrew, preserving the no-brew bootstrap rule.

## Related

- [ADR-0003](0003-monorepo-app-dotfiles-mise.md) — the mise config is a managed dotfile; this ADR says what goes in it vs scripts
- [ADR-0002](0002-go-and-charm-for-the-tui.md), bootstrap & update workflows
- [managed-setup guide](../guides/managed-setup.md) — how to add a tool or dotfile
