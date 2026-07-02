---
title: ADR-0020 — Adopt the third-party herdr command-palette plugin fleet-wide
status: accepted
created: 2026-07-02
updated: 2026-07-02
tags: [herdr, provisioning, terminal, plugins, chezmoi]
supersedes: null
superseded-by: null
---

# ADR-0020: Adopt the third-party herdr command-palette plugin fleet-wide

## Context

ADR-0018/0019 established a repo-managed herdr plugin (machine-title) registered
with `herdr plugin link`, and along the way ADR-0018's "Option A" explicitly
*rejected* using a third-party community plugin — for the title — because that
one was ~100 lines of Bun that mined Claude/Codex session files, i.e. far more
than we wanted and "an out-of-repo dependency we don't control for a fleet-wide
default."

We now want herdr's **fzf command palette** — `JanTvrdik/herdr-command-palette`
(plugin id `jt.command-palette`) — on every machine: a `prefix+p` fuzzy picker
over all plugin actions, so you don't have to memorise bindings. This adopts a
third-party plugin as a fleet default, so it's worth stating why that's the right
call here when ADR-0018 said no.

Facts that make this cheap and safe:

- **No build step.** Its `herdr-plugin.toml` (id `jt.command-palette`, v0.1.0,
  `min_herdr_version = "0.7.0"`, macos+linux, single action `open`) has no
  `[[build]]` section — it's pure shell. So the pre-`mise install` provisioning
  order (ADR-0007) doesn't bite: there's nothing to build at install time.
- **Its runtime deps are already fleet-wide.** It needs `fzf` and `jq`, both
  already mise-managed for the whole fleet (`home/dot_config/mise/config.toml.tmpl`),
  and only at *use* time (when you press `prefix+p`), never at install time.
- **herdr 0.7 does not bind keys from a plugin manifest.** The keybinding must
  live in `~/.config/herdr/config.toml` (a `type = "plugin_action"` key), so
  shipping the plugin isn't enough — the managed config carries the binding too.

## Decision

Adopt it fleet-wide, via two managed changes in the same place the machine-title
plugin is wired (ADR-0019):

1. **Install from GitHub in the provision script**, in the existing
   `command -v herdr` block:
   `herdr plugin install JanTvrdik/herdr-command-palette --yes`. `--yes` because
   `chezmoi apply` is non-interactive (ADR-0006).
2. **Install once, guarded on presence.** Unlike `herdr plugin link` (our own
   files, idempotent, keyed by id — safe to re-run every apply), `install`
   *fetches from GitHub*. Re-fetching on every apply is wasteful and fails
   offline, so guard on `herdr plugin list` not already containing
   `jt.command-palette` and install only when absent. Consequence: it does **not**
   auto-track upstream — bump it by re-installing by hand
   (`herdr plugin uninstall jt.command-palette && herdr plugin install …`).
3. **Bind the key in the managed config.** Add to
   `home/dot_config/herdr/config.toml`:
   ```toml
   [[keys.command]]
   key = "prefix+p"
   type = "plugin_action"
   command = "jt.command-palette.open"
   description = "Command palette"
   ```

The cold-start/relink machinery from ADR-0019 already covers this install too:
provisioning runs before `mise install`, so on a fresh machine herdr is absent
and the whole `command -v herdr` block is skipped; the `# herdr installed (1=yes)`
line in the script's onchange hash flips `0`→`1` when mise installs herdr, which
re-fires the script on the next `myplace update` and performs the install then.

## Why a third-party plugin is acceptable here (when ADR-0018 said no)

- **Different scope.** ADR-0018 rejected a heavyweight title plugin *we could
  trivially replace* with two files. A command palette is a real feature
  (fzf-driven action discovery) not worth reimplementing.
- **Well-scoped and low-trust-surface.** Pure shell, single `open` action, deps
  already on the fleet — no Bun runtime, no session-file mining.
- **Contained blast radius.** It's behind a keybinding, install-once (not
  auto-updating), and trivially removable (`herdr plugin uninstall` + drop the
  key). The maintenance risk is an upstream break or a herdr plugin-API change —
  the same bounded risk we already accepted for our own plugin.

## Options considered

- **Vendor it into the repo like the title plugin** (copy its files, `link` them).
  Rejected: it's actively developed upstream and larger than our title script;
  vendoring means manually tracking its changes and owning its code. `install`
  keeps it upstream-owned with a clean uninstall.
- **Pin a `--ref`** for reproducibility. Rejected for now: adds a version to keep
  bumped for little gain on a personal fleet; install-once already freezes each
  machine at whatever it first installed. Revisit if upstream churn causes drift.
- **Leave it as a manual per-machine `herdr plugin install`.** Rejected: defeats
  the fleet philosophy — a capability we want everywhere should arrive on
  `myplace update`, not by hand on each box.

## Consequences

- After `myplace update`, `prefix+p` opens the command palette on every machine
  (servers included — same reasoning as ADR-0019; herdr is fleet-wide and the
  palette works over an SSH attach).
- `fzf` and `jq` are now runtime dependencies of this feature; both are already
  managed, so no new mise entries.
- We carry a third-party fleet dependency we don't control, frozen per-machine at
  first install. Upstream fixes/features require a manual re-install; a breaking
  herdr plugin-API change is the standing risk.
- The managed `config.toml` gains its first keybinding; future herdr keybindings
  follow the same `[[keys.command]]` pattern there.
