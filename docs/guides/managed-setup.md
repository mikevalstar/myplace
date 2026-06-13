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
| `.chezmoiscripts/run_onchange_provision.sh` | Idempotent installer for oh-my-zsh + plugins, rustup, fnm (things mise can't own) |
| `dot_zshrc` | The managed `~/.zshrc` — oh-my-zsh setup, mise activation, tool env wiring |
| `dot_mvdotfiles.zsh` | Personal shell config (`~/.mvdotfiles.zsh`) sourced by `.zshrc`: tool inits, aliases, functions |
| `.chezmoi.toml.tmpl` | Machine identity (prompts for `profile` on init) |

`dot_` becomes a leading `.` in the target; a `.tmpl` suffix means chezmoi templates it.

## How to add…

### A CLI tool that's in mise's registry

1. Check it exists: `mise registry | grep <name>`.
2. Add a line under `[tools]` in `dot_config/mise/config.toml.tmpl`:
   ```toml
   ripgrep = "latest"
   ```
3. Commit & push. On each machine, `myplace update` (or `mise install`) picks it up.

### A tool mise doesn't carry, or an installer/framework

Add a guarded block to `.chezmoiscripts/run_onchange_provision.sh`. Always guard so re-runs are no-ops:
```sh
if ! command -v <tool> >/dev/null 2>&1 && [ ! -x "$HOME/.local/bin/<tool>" ]; then
  log "installing <tool>"
  curl -fsSL <installer> | sh -s -- <non-interactive-flags> || log "<tool> install failed"
fi
```
Because the file is `run_onchange_`, editing it re-runs it on the next apply. Keep it failure-tolerant (`|| log …`) — a network blip shouldn't abort the whole apply.

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

## References

- [ADR-0007](../adrs/0007-provisioning-mechanism.md), [ADR-0003](../adrs/0003-monorepo-app-dotfiles-mise.md)
- chezmoi scripts: https://www.chezmoi.io/user-guide/use-scripts-to-perform-actions/
