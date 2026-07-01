---
title: ADR-0018 — A repo-managed herdr plugin for the terminal title
status: accepted
created: 2026-07-01
updated: 2026-07-01
tags: [herdr, provisioning, desktop, terminal, plugins]
supersedes: null
superseded-by: null
---

# ADR-0018: A repo-managed herdr plugin for the terminal title

## Context

[herdr](https://herdr.dev) — the terminal multiplexer used on desktops (it's a
mise-managed tool, `home/dot_config/mise/config.toml.tmpl`) — sets the outer
terminal's title to a static `herdr`. With the same setup applied across a fleet
of Macs plus the `personal-linux` desktop (ADR-0017), every window/tab reads
`herdr`, so a taskbar or window switcher can't tell one machine's session from
another's.

We want the title to identify the machine — `herdr@<hostname> · <workspace>` —
and, per the fleet philosophy, to arrive automatically on every desktop rather
than be configured by hand on each one.

Constraints discovered while scoping this:

- **herdr's own config can't do it.** `config.toml` has no title/hostname key,
  and Ghostty's dynamic title means herdr's escape wins over any static
  terminal-side title. The only lever herdr exposes is a *plugin*: a manifest of
  event hooks whose commands run on focus/pane events and can call
  `herdr terminal title set`.
- **No config-declared or auto-discovered plugins.** herdr has no `plugins = […]`
  config key and no drop-in directory it scans. Registration is only ever an
  explicit `herdr plugin link <path>` / `herdr plugin install <owner/repo>`,
  persisted to `~/.config/herdr/plugins.json`.
- **Provisioning ordering.** The one provision script runs during `chezmoi apply`,
  *before* `mise install` (ADR-0007), so herdr (a mise tool) may be absent when
  provisioning runs on a brand-new machine.

## Options considered

### Option A — `herdr plugin install rjyo/herdr-window-title-sync`

Use the existing community plugin. It works, but it's ~100 lines of Bun that
composes titles from agent names and *reads local Claude/Codex session JSONL
files to put prompt text in the title* — far more than we want, an extra Bun
dependency at title-set time, and an out-of-repo dependency we don't control for
a fleet-wide default.

### Option B — Ghostty-side static title

Set `title = {{ "{{ .chezmoi.hostname }}" }}` in the managed Ghostty config. Simple
and needs no plugin, but it's a global switch that kills per-tab dynamic titles
everywhere (not just for herdr), and it's terminal-specific — it does nothing for
a Mac using a different terminal.

### Option C — a first-party, repo-managed plugin (chosen)

Ship our own minimal plugin as two managed files (`herdr-plugin.toml` +
`set-title.sh`, plain POSIX sh + `jq`, both already on the fleet), applied by
chezmoi to `~/.config/herdr/machine-title-plugin/`, and registered by a guarded,
idempotent `herdr plugin link` in the provision script.

## Decision

Option C. The title is just `herdr@<host> · <focused-workspace>`, so there's no
agent/prompt mining and no state/dedup logic — the script re-asserts a constant
title on herdr's focus events. It lives in-repo (versioned, fleet-wide, no
third-party trust), reuses tools we already manage (`sh`, `jq`), and terminal-
agnostically fixes the title for every desktop terminal, not just Ghostty.

Registration details that make it "auto-install":

- `herdr plugin link` is **idempotent** (keyed by plugin id), so the provision
  step can re-run it on every apply harmlessly.
- The step is **guarded** on `command -v herdr` and on the manifest existing, and
  **non-server gated** (`ne .profile "server"`) — servers are headless and never
  need an outer title.
- The provision script's onchange hash includes the plugin manifest, so a plugin
  change re-triggers the link.

## Consequences

- Every desktop's terminal title becomes `herdr@<host> · <workspace>` after
  `myplace update`, with no per-machine setup.
- **Cold-start lag (accepted):** on a brand-new machine, provisioning runs before
  mise installs herdr, so the guard skips the link on first bootstrap; it links on
  the next `myplace update`. Existing machines (herdr already installed) link on
  the first apply that carries this change.
- We now own a herdr plugin. It's tiny and pinned to the documented plugin API
  (`min_herdr_version = "0.7.0"`); a breaking herdr plugin-API change is the main
  future maintenance risk.
- `jq` becomes a soft runtime dependency of the title script; it already ships to
  the whole fleet via mise, and the script falls back to `herdr@<host>` if it's
  missing.
