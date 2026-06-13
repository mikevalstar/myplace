# myplace

A TUI for getting machines from zero to fully-configured, and keeping them that way.

`myplace` wraps [chezmoi](https://www.chezmoi.io/) (dotfiles) and [mise-en-place](https://mise.jdx.dev/) (tools, runtimes, tasks) in a single terminal UI — plus a headless `--json` mode so the same commands work in scripts, cron, and SSH loops:

- **Bootstrap** — bring a brand-new machine (Mac or server) up to a known-good state
- **Update** — pull dotfile changes, upgrade tools, capture and push local tweaks
- **Status** — see what's drifted, in either direction, on screen or as JSON

Target machines: personal Macs, a work Mac, and assorted Linux servers — one common setup, many hosts.

This repo is a monorepo: the Go app, the chezmoi dotfiles (under [home/](home/), via `.chezmoiroot`), and the machines' mise config all live here — one clone carries everything ([ADR-0003](docs/adrs/0003-monorepo-app-dotfiles-mise.md)).

> 🚧 **v0.** Bootstrap, status (TUI + `--json`), update with interactive capture of local edits, and `self-update` all work. Not yet built: per-file diff review before apply, `push: false` profile policy, and the phase-2 server.

## Install

One static binary, no dependencies — works on a stock Mac or a bare server image:

```sh
curl -fsSL https://raw.githubusercontent.com/mikevalstar/myplace/main/install.sh | sh
```

Or build from source: `go build -o ~/.local/bin/myplace ./cmd/myplace` (or `mise run build`). Later, `myplace self-update` swaps in the newest release.

## Boot a new machine

On a fresh machine, after the install one-liner:

```sh
myplace
```

With no existing setup detected, this opens the **bootstrap wizard**: it installs chezmoi and mise if missing, asks for the config repo (defaults to this one) and this machine's profile (`personal-mac` / `work-mac` / `server`), applies the dotfiles, installs the tools, and finishes with a status report.

For servers, skip the wizard entirely:

```sh
myplace bootstrap --profile server --yes
```

Details and failure handling: [bootstrap workflow](docs/workflows/bootstrap-new-machine.md).

## Everyday use

```sh
myplace              # TUI dashboard: drift in both directions, r refresh / u update / q quit
myplace update       # capture local edits (keep/discard/skip per file), pull + apply, upgrade tools
myplace status       # quick plain-text summary, no TUI
myplace self-update  # swap this binary for the latest release
```

"In sync" is bidirectional: repo changes you haven't applied **and** local edits you haven't pushed both count as drift. Updating always shows you the diff before touching anything.

## Scripting and JSON

Every data-producing command takes `--json`: stdout is exactly one JSON document (logs go to stderr), and exit codes tell you the verdict without parsing — `0` in sync, `1` drifted, `2` unknown (e.g. offline), `3` error.

```sh
# Check one server
ssh web1 myplace status --json | jq .verdict

# Sweep the fleet
for h in web1 web2 db1; do
  ssh "$h" myplace status --json >/dev/null && echo "$h: ok" || echo "$h: drifted"
done
```

Contract details (schema versioning, non-interactive rules): [headless CLI spec](docs/features/headless-cli-and-json-output.md).

## Roadmap

| Phase | Scope |
|-------|-------|
| 1 | Local TUI + headless CLI: bootstrap, update, and status for the current machine via chezmoi + mise |
| 2 | Fleet awareness: machines report status (the same JSON) to a central server so every system is visible in one place |

## Documentation

This is a **documentation-first** project. Design decisions, feature specs, and workflows are written down in [docs/](docs/README.md) before (or alongside) the code that implements them.

- [docs/adrs](docs/adrs/) — architecture decision records
- [docs/features](docs/features/) — feature specs
- [docs/workflows](docs/workflows/) — end-to-end flows the TUI supports
- [docs/guides](docs/guides/) — developer guides for this repo and the libraries it uses

Built with Go and the [Charm](https://charm.sh) libraries ([ADR-0002](docs/adrs/0002-go-and-charm-for-the-tui.md)).
