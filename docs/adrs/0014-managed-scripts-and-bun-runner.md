---
title: ADR-0014 — Managed scripts deployed via chezmoi, with bun as an optional runner
status: accepted
created: 2026-06-16
updated: 2026-06-16
tags: [chezmoi, scripts, bun, provisioning, dotfiles]
supersedes: null
superseded-by: null
---

# ADR-0014: Managed scripts deployed via chezmoi, with bun as an optional runner

## Context

We want a **folder of helpful scripts that grows over time** — small utilities that ride
along with the fleet and should exist on every managed machine, not just in this repo's
checkout. The first is `ai_installed`, a probe that reports which AI CLI tools are
present on the host; more will follow, and we want them to stay discoverable as the
collection grows (hence `mv_scripts`, below).

Two questions fall out of that:

1. **Where do these scripts live, and do they get deployed?** myplace already has two
   distinct "script" homes that must not be confused: `mise.toml` tasks at the repo
   root (dev tooling for *this app only*, ADR-0003) and `.chezmoiscripts/` (idempotent
   *provisioning* that runs during `chezmoi apply`). Neither fits a standalone,
   user-invokable helper that should be on `PATH` on every box.
2. **What do we write them in?** Some helpers are pure presence/string checks where
   POSIX shell is the right tool; others will want structured data, argument parsing,
   or JSON output where a real runtime pays off. We don't want to standardize on a
   heavyweight runtime for a one-line `command -v` loop, nor hand-roll JSON in bash
   when a script genuinely needs it.

## Options considered

### Option A — Repo-only `scripts/` folder

A top-level `scripts/` dir, like `mise.toml`, holding maintenance tooling for the
project. Simple, but it never reaches the machines — you'd have to clone the repo to
run anything. Wrong home for fleet-wide helpers; these aren't *this app's* dev tooling.

### Option B — Deploy scripts as chezmoi-managed files on `PATH`

Source them under `home/dot_local/bin/` (chezmoi target `~/.local/bin`, already on
`PATH` via `dot_zshrc`). Mark them executable with chezmoi's `executable_` prefix.
They land on every machine through the normal `myplace update` flow, show up in drift
status like any other managed file, and are runnable by name (`ai_installed`). This is
the same deploy mechanism every other dotfile already uses — no new machinery.

### Runtime — shell by default, bun when a script needs it

- **Shell (default):** presence checks, simple glue. No runtime dependency, works on a
  bare server, fastest path. `ai_installed` is exactly this.
- **bun (opt-in per script):** when a script wants TypeScript, real arg parsing, or
  clean JSON. bun is a single self-contained binary in mise's registry (`core:bun`),
  so adding it is one line and it's managed like every other CLI tool (ADR-0007). It is
  *not* Node — it doesn't go through fnm, and nothing here makes it Node's runtime.

## Decision

Helper scripts that belong on every machine are **chezmoi-managed files under
`home/dot_local/bin/`** (Option B), executable via the `executable_` prefix, invoked by
name off `PATH`. This keeps repo-dev tooling (`mise.toml` tasks), provisioning
(`.chezmoiscripts/`), and fleet helpers (`~/.local/bin`) as three clearly separate
things.

Write each script in **plain shell by default**; reach for **bun** only when a script
genuinely needs a runtime. bun is added to the global mise tool set so it's available
fleet-wide as a runner when wanted.

The collection stays self-documenting by **convention, not a manifest**: a script opts
into the index by carrying a `# mv_scripts: <one-line description>` comment in its body.
`mv_scripts` scans `~/.local/bin` for that marker and renders the name + description as a
table (via `gum`, already in the tool set; plain columns when `gum` is absent). Adding a
marked script makes it show up automatically — there's no list to keep in sync.

## Consequences

- Easier: a new helper is just a file under `home/dot_local/bin/`; it deploys, versions,
  and drift-checks through the existing flow with zero new tooling. bun is available
  everywhere for the scripts that want it.
- Harder / to watch: `~/.local/bin` helpers are *unmanaged by mise/brew* — they're our
  code, so we own their portability (`$HOME` not `/Users/...`, guard against missing
  deps). Shell scripts must stay POSIX-safe enough for the headless Linux servers.
- bun now installs on every machine including servers. It's a small single binary, but
  if a future server profile wants a leaner tool set, bun is a candidate to gate behind
  a profile check in the mise config.
- Follow-up: the [managed-setup guide](../guides/managed-setup.md) documents the
  `home/dot_local/bin/` path and the shell-vs-bun choice as the how-to.
