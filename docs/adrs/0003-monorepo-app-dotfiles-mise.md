---
title: ADR-0003 — One repo for the app, the dotfiles, and the mise config
status: accepted
created: 2026-06-12
updated: 2026-06-12
tags: [repo-layout, chezmoi, mise, monorepo]
supersedes: null
superseded-by: null
---

# ADR-0003: One repo for the app, the dotfiles, and the mise config

## Context

Earlier docs assumed myplace points at a *separate* dotfiles repo. The owner's actual intent: **this repo is also the chezmoi source repo and the mise config repo.** One clone carries the tool, the configuration it applies, and the docs — machines pull a single thing.

chezmoi normally treats the entire repo root as its source state, which would make it try to manage `cmd/`, `docs/`, `go.mod`, etc. as dotfiles. chezmoi's supported answer is a `.chezmoiroot` file at the repo root naming a subdirectory to use as the source state root.

mise plays two distinct roles that must not be conflated:

1. **Dev tooling for this repo** (the Go toolchain) — a normal `mise.toml` at the repo root, active only inside the project directory.
2. **The machines' global tool set** — `~/.config/mise/config.toml` on every machine, which is itself a dotfile and therefore *managed by chezmoi* from this repo.

## Options considered

### Option A — Separate repos (app vs dotfiles)

Conventional; keeps the app releasable independently. But it doubles the number of things to clone, version, and keep in sync, for a single-user project where the app and the config evolve together.

### Option B — Monorepo with `.chezmoiroot`

One repo: Go app at the root, dotfiles under `home/` (selected by `.chezmoiroot`), machine tool set inside `home/` as the managed mise config. `chezmoi init --apply <this-repo>` works unchanged; chezmoi only sees `home/`.

## Decision

Option B. Layout:

```
.chezmoiroot                          # contains "home" — chezmoi's source root
home/                                 # chezmoi source state (the dotfiles)
  .chezmoi.toml.tmpl                  # machine config template: prompts for profile on init
  dot_config/mise/config.toml.tmpl    # the GLOBAL tool set every machine gets (templated by profile)
mise.toml                             # dev tooling for working on this repo (Go, lint)
cmd/, internal/                       # the Go app
docs/                                 # unchanged
```

The default repo URL in bootstrap is this repo; the `--repo` flag remains for forks/tests.

## Consequences

- One `git clone`/`chezmoi init` gets a machine everything; app releases and config changes share history. Tags/releases version the binary; config is always "latest of main" via `chezmoi update`.
- `home/` starts deliberately tiny (mise config + machine identity) and grows as real dotfiles migrate in — migrating existing dotfiles is follow-up work, done carefully since `chezmoi apply` overwrites live files.
- The work-mac question sharpens: this repo must contain nothing private, or the work Mac needs a restricted profile. Secrets stay in a secret manager (chezmoi supports several) — never in `home/`.
- Bootstrap docs/README updated to reflect the default-repo behavior (done alongside this ADR).
