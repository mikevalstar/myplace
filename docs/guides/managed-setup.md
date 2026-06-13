---
title: Extending the managed setup (tools & dotfiles)
status: active
created: 2026-06-13
updated: 2026-06-13
tags: [chezmoi, mise, dotfiles, provisioning, how-to]
audience: both
---

# Extending the managed setup (tools & dotfiles)

## Purpose

Where things live and how to add a new tool, dotfile, or provisioning step so it lands on every machine. The mechanism rationale is [ADR-0007](../adrs/0007-provisioning-mechanism.md); this is the how-to.

## The layout (under `home/`, chezmoi's source root)

| Path | What it is |
|------|------------|
| `dot_config/mise/config.toml.tmpl` | The mise tool set — every machine's CLI tools/runtimes from mise's registry |
| `.chezmoiscripts/run_onchange_provision.sh` | Idempotent installer for the things mise can't own — git, oh-my-zsh + plugins, rustup, fnm, plain OS/brew packages via `ensure_tool` (httpie, mosh), and macOS-only fonts/GUI casks via `ensure_cask` |
| `dot_zshrc` | The managed `~/.zshrc` — oh-my-zsh setup, mise activation, tool env wiring |
| `dot_gitconfig.tmpl` | Git identity (`~/.gitconfig`) — name/email, rendered from install-time data (`.gitName`/`.gitEmail`) |
| `dot_mvdotfiles.zsh` | Personal shell config (`~/.mvdotfiles.zsh`) sourced by `.zshrc`: tool inits, aliases, functions |
| `.chezmoi.toml.tmpl` | Init prompts → chezmoi data: `profile`, plus `gitName`/`gitEmail` (answered at install, pre-fillable with `--promptString`) |

`dot_` becomes a leading `.` in the target; a `.tmpl` suffix means chezmoi templates it.

## How to add…

### A CLI tool that's in mise's registry

1. Check it exists: `mise registry | grep <name>`.
2. Add a line under `[tools]` in `dot_config/mise/config.toml.tmpl`:
   ```toml
   ripgrep = "latest"
   ```
3. Commit & push. On each machine, `myplace update` (or `mise install`) picks it up.

### A tool mise doesn't carry

Both cases live in `.chezmoiscripts/run_onchange_provision.sh`. It's `run_onchange_`, so editing it re-runs on the next apply; keep every step guarded and failure-tolerant (`|| log …`) so re-runs and network blips are harmless.

**A plain package** that the OS package managers / Homebrew carry (e.g. `httpie`, `mosh`) — add one `ensure_tool` line. It installs via the system package manager on Linux and via Homebrew *if it's already present* on macOS, logging a note otherwise (bootstrap never requires brew — [ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md)):
```sh
ensure_tool http httpie   # ensure_tool <command-to-check> <package-name>
ensure_tool mosh mosh
```

**An installer or framework** with its own install script (rustup, fnm, oh-my-zsh — not a packaged binary) — add a guarded block:
```sh
if ! command -v <tool> >/dev/null 2>&1 && [ ! -x "$HOME/.local/bin/<tool>" ]; then
  log "installing <tool>"
  curl -fsSL <installer> | sh -s -- <non-interactive-flags> || log "<tool> install failed"
fi
```

### A macOS font or GUI app (Homebrew cask)

Fonts and GUI apps are Homebrew *casks*, and in this fleet they're macOS-only (the Linux machines are headless servers). Add an `ensure_cask` line to the provision script; it installs via `brew install --cask` when Homebrew is present, skips off macOS, and logs a note on a brew-less Mac ([ADR-0009](../adrs/0009-homebrew-casks-macos.md)):
```sh
ensure_cask font-monaspace-nf
ensure_cask font-jetbrains-mono-nerd-font
```
Find the exact name with `brew search /<name>/`. Nerd Fonts are `font-<family>-nerd-font`; the icon-only overlay is `font-symbols-only-nerd-font`.

### A new dotfile

- Bring an existing file under management: `chezmoi add ~/.foorc` (creates `home/dot_foorc` in the source clone), then commit/push from the source repo — **or**, when working in the dev checkout, drop the file at `home/dot_foorc` directly.
- Make paths portable: use `$HOME`, never `/Users/<you>`. Servers and other usernames must work.
- Needs per-machine variation? Rename to `…​.tmpl` and branch on `.profile` (e.g. `{{ if ne .profile "server" }}…{{ end }}`).

### Shell tool wiring

Tool init (`eval "$(x init zsh)"`, PATH additions) goes in `dot_mvdotfiles.zsh`, guarded with `command -v x` so a missing tool is silent. mise activation and the fnm/cargo env lines live in `dot_zshrc`.

## Gotchas

- **Node is fnm's, Rust is rustup's — not mise's.** Don't add `node`/`rust` to the mise config; they're installed by the provision script and managed by fnm/rustup (ADR-0007). Adding them to mise creates two managers fighting over the same binary.
- **The provision script runs before `mise install`** (during `chezmoi apply`), so it can't use any mise tool.
- **oh-my-zsh install must keep our `.zshrc`** — the script passes `KEEP_ZSHRC=yes`. Don't drop it or the managed `.zshrc` gets overwritten with OMZ's template.
- **Editing the managed `.zshrc` on a machine** shows as drift (it's managed now); change it in the repo and `myplace update`, or use the capture flow.
- **Homebrew on macOS is opportunistic, never required.** `ensure_tool` uses brew when it's present and logs a note when it isn't, so a brew-less Mac still bootstraps; anything in mise's registry still belongs in mise, not here ([ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md)).
- **Fonts and GUI apps are macOS-only.** They install as Homebrew casks via `ensure_cask`; the Linux fleet is headless servers, so casks are skipped there by design. A Linux desktop would need a different path (chezmoi externals) — not built yet ([ADR-0009](../adrs/0009-homebrew-casks-macos.md)).

## References

- [ADR-0007](../adrs/0007-provisioning-mechanism.md), [ADR-0003](../adrs/0003-monorepo-app-dotfiles-mise.md)
- chezmoi scripts: https://www.chezmoi.io/user-guide/use-scripts-to-perform-actions/
