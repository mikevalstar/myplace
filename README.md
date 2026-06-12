# myplace

A TUI for getting machines from zero to fully-configured, and keeping them that way.

`myplace` wraps [chezmoi](https://www.chezmoi.io/) (dotfiles) and [mise-en-place](https://mise.jdx.dev/) (tools, runtimes, tasks) in a single terminal UI that handles:

- **Bootstrap** — bring a brand-new machine (Mac or server) up to a known-good state
- **Update** — pull dotfile changes, upgrade tools, re-apply configuration
- **Status** — see at a glance whether a machine is in sync, what's drifted, and what's pending

Target machines: personal Macs, a work Mac, and assorted Linux servers — one common setup, many hosts.

## Roadmap

| Phase | Scope |
|-------|-------|
| 1 | Local TUI: bootstrap, update, and status for the current machine via chezmoi + mise |
| 2 | Fleet awareness: machines ping a central server so you can track status of every system from one place |

## Documentation

This is a **documentation-first** project. Design decisions, feature specs, and workflows are written down in [docs/](docs/README.md) before (or alongside) the code that implements them.

- [docs/adrs](docs/adrs/) — architecture decision records
- [docs/features](docs/features/) — feature specs
- [docs/workflows](docs/workflows/) — user-facing workflows the TUI supports
- [docs/guides](docs/guides/) — developer guides for this repo and the libraries it uses

## Status

🚧 Early — currently in the design/documentation phase. No code yet.
