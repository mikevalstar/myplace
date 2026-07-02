---
title: ADR-0021 — Add jj (Jujutsu) to the fleet baseline
status: accepted
created: 2026-07-02
updated: 2026-07-02
tags: [tooling, mise, vcs]
supersedes: null
superseded-by: null
---

# ADR-0021: Add jj (Jujutsu) to the fleet baseline

## Context

Every machine already has git plus git-adjacent tooling in the baseline (`gh`,
`delta`, `git-lfs`, `lazygit`). [Jujutsu](https://jj-vcs.github.io/) (`jj`) is a
Git-compatible VCS that operates on the same repositories git does, so it can be
adopted incrementally without converting any repo or changing the git workflow
for others. Adding it fleet-wide means it's present the moment it's wanted on any
host — the same "common base, drift is the exception" posture the rest of the
baseline follows.

`jj` is in mise's registry with an **aqua** backend (`aqua:jj-vcs/jj`), which
ships prebuilt binaries for the darwin/linux × amd64/arm64 matrix. That matters:
the fallback backend is `cargo:jj-cli` (source build), and mise must never drive
cargo in this project (Rust is rustup's — ADR-0007). Because aqua wins, `jj`
installs as a plain prebuilt binary on every profile, servers included, with no
Rust toolchain dependency.

## Options considered

### Option A — Add `jj` to the mise `[tools]` baseline (chosen)

One line in `home/dot_config/mise/config.toml.tmpl`, installed everywhere via the
aqua backend. Consistent with how the rest of the CLI baseline is managed; no new
mechanism, no profile gating, no provision-script change.

### Option B — Provision-script install / rustup-cargo build

Only needed if mise couldn't supply a prebuilt binary. It can (aqua), so this
would add a source-build dependency and complexity for no benefit. Rejected.

### Option C — Desktop-profile only

`jj` is a CLI with no GUI/desktop dependency and is just as useful on a server as
on a desktop, so gating it would be arbitrary. Kept it in the unconditional
baseline alongside git/`gh`/`delta`.

## Decision

Add `jj = "latest"` to the unconditional `[tools]` block in the mise config
template, managed by mise via its aqua backend on every profile.

**TUI companion deferred.** A jj TUI (the leading one is
[jjui](https://github.com/idursun/jjui), in mise's registry as
`aqua:idursun/jjui` under the alias `jujutsu-ui`) is intentionally *not* added
now. `jj`'s built-in `jj log`/revset commands cover most day-to-day needs, and a
TUI is desktop-shaped. If we later want one, add `jjui` gated to
`ne .profile "server"`, matching the ADR-0017 desktop-extras pattern — that's a
follow-up ADR, not this one.

## Consequences

- Every managed machine gains `jj` on the next `myplace update`; no config or
  profile changes required to start using it on a given repo.
- No Rust/cargo dependency is introduced — aqua backend ships prebuilt binaries.
- `jj` global config (`~/.config/jj/config.toml`) is **not** managed yet. If/when
  we want shared jj defaults (user identity, UI settings, aliases), that's a
  chezmoi dotfile addition and should reference ADR-0015's git-defaults reasoning
  for the identity/signing overlap. Deferred until there's a concrete default worth
  sharing.
- Leaves the door open for `jjui` as a desktop-profile follow-up without committing
  to it now.
