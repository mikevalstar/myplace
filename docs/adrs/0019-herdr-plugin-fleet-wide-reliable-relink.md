---
title: ADR-0019 — herdr machine-title plugin runs fleet-wide with a reliable relink
status: accepted
created: 2026-07-02
updated: 2026-07-02
tags: [herdr, provisioning, terminal, plugins, chezmoi]
supersedes: ADR-0018
superseded-by: null
---

# ADR-0019: herdr machine-title plugin runs fleet-wide with a reliable relink

## Context

ADR-0018 shipped a repo-managed herdr plugin that sets the outer terminal title
to `herdr@<hostname> · <workspace>`, registered by a guarded `herdr plugin link`
in the one provision script. Two of its decisions turned out wrong in practice.

1. **The link never actually happened on real machines.** ADR-0018 accepted a
   "cold-start lag": provisioning runs *before* `mise install` (ADR-0007), so on a
   fresh machine herdr is absent and the `command -v herdr` guard skips the link.
   It claimed the link would happen "on the next `myplace update`". It doesn't —
   the provision script is `run_onchange_`, which chezmoi only re-runs when the
   script's *content* changes. Once herdr is finally installed, nothing changes
   the script's hash, so the skipped link is never retried. Observed on a
   `work-mac`: the plugin *files* were applied, but `~/.config/herdr/plugins.json`
   was never written and `herdr plugin list` reported no plugins. Confirmed root
   cause by running `herdr plugin link` by hand — it linked immediately.

2. **Servers were gated out, but they're the case that benefits most.** ADR-0018
   reasoned "servers run headless and never need an outer-terminal title." But
   herdr is a fleet-wide mise tool (not desktop-only), and when you attach to a
   server's herdr over SSH the title is set by the *server's* herdr and shows in
   your **local** terminal. So `herdr@<server-host>` is exactly what distinguishes
   one attached box from another — the original fleet-legibility goal applies to
   servers more than anywhere.

We reconfirmed herdr still has no config-file route: `herdr --default-config` has
no plugin section, and the docs state a plugin is registered only via
`herdr plugin link` / `herdr plugin install` (persisted to `plugins.json`);
`config.toml` references plugins only in `type = "plugin_action"` keybindings.
So dropping the provision step in favour of config is not possible.

## Decision

Keep Option C (the repo-managed plugin, registered by `herdr plugin link`), but:

- **Remove the `ne .profile "server"` gate.** The link block runs on every
  profile. The plugin *files* were already applied everywhere (never in
  `.chezmoiignore`), so only the provision step changes.
- **Make the onchange script re-fire when herdr appears.** Add herdr's resolved
  path to the script's hash via a comment line — `# herdr on PATH: {{ lookPath "herdr" }}`.
  When herdr goes from absent → present on PATH, `lookPath`'s output changes, the
  script's content hash changes, and chezmoi re-runs it exactly once — the first
  `myplace update` after mise installs herdr — so the guard-skipped link is
  retried automatically. `lookPath` returns an empty string (not an error) when
  the binary is missing, so it's safe to evaluate on a machine without herdr.

## Options considered for the relink

- **Commit `plugins.json` as a chezmoi dotfile** (herdr auto-loads it on startup).
  Rejected: it's a herdr-owned state file, not config — it embeds absolute
  `$HOME` paths (wrong across users/OSes without templating), duplicates the whole
  manifest (goes stale when `herdr-plugin.toml` changes), and herdr rewrites it on
  link/enable/disable and may migrate its schema across versions, which would put
  chezmoi in a permanent drift fight with herdr.
- **`lookPath` in the onchange hash (chosen).** Minimal, keeps herdr owning
  `plugins.json`, self-heals the cold start with no manifest churn.

## Consequences

- Fresh machine: provisioning skips the link on first apply (herdr not yet
  installed), mise installs herdr, and the next `myplace update` re-runs the
  provision script (its hash now includes herdr's path) and links the plugin — no
  manual step, no stale cold-start gap.
- Servers now carry the plugin and get a `herdr@<host>` title when herdr runs
  there (directly or over an SSH attach).
- Existing machines that were silently missing the link (like the `work-mac` that
  surfaced this) pick it up on their next `myplace update`.
- Everything else from ADR-0018 stands: tiny in-repo plugin, `sh` + `jq` only,
  idempotent link keyed by plugin id, pinned to `min_herdr_version = "0.7.0"`.
