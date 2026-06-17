# myplace

A TUI for getting machines from zero to fully-configured, and keeping them that way.

`myplace` wraps [chezmoi](https://www.chezmoi.io/) (dotfiles) and [mise-en-place](https://mise.jdx.dev/) (tools, runtimes, tasks) in a single terminal UI — plus a headless `--json` mode so the same commands work in scripts, cron, and SSH loops:

- **Bootstrap** — bring a brand-new machine (Mac or server) up to a known-good state
- **Update** — pull dotfile changes, upgrade tools, capture and push local tweaks
- **Status** — see what's drifted, in either direction, on screen or as JSON

Target machines: personal Macs, a work Mac, and assorted Linux servers — one common setup, many hosts.

This repo is a monorepo: the Go app, the chezmoi dotfiles (under [home/](home/), via `.chezmoiroot`), and the machines' mise config all live here — one clone carries everything ([ADR-0003](docs/adrs/0003-monorepo-app-dotfiles-mise.md)).

> 🚧 **v0.** Bootstrap, status (TUI + `--json`), update with interactive capture of local edits and per-file incoming diff review, profile push policy, outdated-package inventory (mise + brew, TUI + `--json`), system info (`sysinfo`, TUI band + `--json`), preflight diagnostics (`doctor`, + `--json`), `self-update`, and self-describing help (`help --llm`/`--json`) all work. Not yet built: the phase-2 server.

## Install

One static binary, no dependencies — works on a stock Mac or a bare server image:

```sh
curl -fsSL https://raw.githubusercontent.com/mikevalstar/myplace/main/install.sh | sh
```

The installer (and `self-update`) verifies the downloaded archive against the release's `checksums.txt` before installing it.

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

`bootstrap` also takes `--repo <url>` (the dotfiles repo, defaults to this one) and `--git-name` / `--git-email` to seed this machine's git identity non-interactively — handy in an unattended image build.

Details and failure handling: [bootstrap workflow](docs/workflows/bootstrap-new-machine.md).

## Everyday use

```sh
myplace              # TUI dashboard: drift in both directions, r refresh / u update / o outdated / q quit
myplace update       # capture local edits (keep/discard/skip per file), pull + apply, upgrade tools
myplace status       # quick plain-text summary, no TUI
myplace doctor       # preflight: can this machine run myplace? names a remedy for anything wrong
myplace outdated     # what's upgradable across package managers (mise + brew), read-only
myplace sysinfo      # this machine's OS + hardware specs (via fastfetch), read-only
myplace self-update  # swap this binary for the latest release
myplace version      # print the version (add --json for a machine-readable document)
```

Narrow an update to one half with `myplace update --dotfiles` (pull + apply only) or `myplace update --tools` (install + upgrade only); bare `myplace update` does both.

"In sync" is bidirectional: repo changes you haven't applied **and** local edits you haven't pushed both count as drift. Updating always shows you the diff before touching anything.

`myplace outdated` is the cross-manager "what's upgradable here?" view — mise tools plus Homebrew packages when brew is present, **including software myplace doesn't manage**. It's informational and read-only: it never upgrades anything and never affects the `status` drift verdict. In the dashboard, the "Updates available" pane summarizes it and `o` opens the full list.

`myplace doctor` answers a different question than `status`: not "is this machine in sync" but "can myplace run here at all." It's a read-only preflight — chezmoi and mise installed and recent enough, `~/.local/bin` on `PATH`, the dotfiles repo and GitHub API reachable, the state dir writable — and every failure prints the exact remedy. Exit codes follow the headless contract (`0` ready, `1` a check failed, `2` checks incomplete e.g. offline, `3` error), so a provisioning script can gate on `myplace doctor --json` before attempting a bootstrap. Reachability checks degrade to a warning when offline rather than failing — being offline isn't broken.

`myplace sysinfo` reports what the machine *is* — OS and version plus base specs (host model, CPU, GPU, memory, disk) and a couple of extras (battery, local IP), via [fastfetch](https://github.com/fastfetch-cli/fastfetch) (installed as part of the managed tool set). It's informational and read-only, with `--json` for scripts; the dashboard shows the same facts in a compact header band.

## Where things live

After install + a bootstrap, this is what lands where. Paths honor the XDG base dirs and assume the defaults; everything is under `$HOME`, nothing requires root.

| Path | What it is |
|------|------------|
| `~/.local/bin/myplace` | The binary itself. Override the install dir with `MYPLACE_BIN_DIR`. |
| `~/.local/share/chezmoi/` | chezmoi's **source clone** of this repo — the copy `myplace update` does `git pull` + apply on. The dotfiles live under its `home/` subdir (selected by [`.chezmoiroot`](home/)). Edit + push the repo, not the applied files. |
| `~/.zshrc`, `~/.mvdotfiles.zsh`, `~/.gitconfig` | Dotfiles applied into `$HOME` from the source state. Editing these directly shows up as drift. |
| `~/.config/mise/config.toml` | This machine's global mise tool set, rendered from `home/dot_config/mise/config.toml.tmpl`. |
| `~/.ssh/config` | Rendered (macs only): host list pulled from a 1Password Document, plus non-secret global defaults from the template. Server IPs stay out of this public repo ([ADR-0016](docs/adrs/0016-secrets-in-dotfiles-via-1password.md)). Edit hosts in 1Password, not here — see below. |
| `~/.local/state/myplace/myplace.log` | Debug log (see below). Honors `XDG_STATE_HOME`; override with `MYPLACE_STATE_DIR`. Deliberately outside `~/.config` so it never lands in your dotfiles. |
| `~/.oh-my-zsh`, `~/.cargo` + `~/.rustup`, `~/.local/share/fnm` | Installed by the provision script — oh-my-zsh, rustup (Rust), and fnm (Node): the things mise can't own ([ADR-0007](docs/adrs/0007-provisioning-mechanism.md)). |
| `~/.local/share/mise/` | mise's own installed tools, shims, and runtimes. |

`chezmoi` and `mise` themselves are installed by `bootstrap` if missing; their location varies by platform (Homebrew on a Mac, `~/.local/bin` on a bare server). Treat `~/.config` as chezmoi's tree — machine-local state goes in the state dir above, never there ([ADR-0005](docs/adrs/0005-machine-local-state-directory.md)).

Adding a tool or dotfile to the managed set: [managed-setup guide](docs/guides/managed-setup.md).
Editing your SSH hosts (kept in 1Password, not the repo): [edit-ssh-config workflow](docs/workflows/edit-ssh-config.md).

## Logs

Every run appends a full debug trace — each chezmoi/mise command with its arguments, duration, and outcome, plus the run's verdict — to `~/.local/state/myplace/myplace.log` (see [Where things live](#where-things-live) for overrides). The file self-rotates past ~5 MB. Tail it when something misbehaves on a machine you'll next see in three weeks:

```sh
tail -f ~/.local/state/myplace/myplace.log
```

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

### Self-describing help, for agents and humans

The CLI describes itself so an agent (or a human filling an LLM context) can learn to drive every command from one read — no scraping a dozen `--help` screens:

```sh
myplace help --llm    # copy-paste brief: every command, exit codes, and workflow recipes (ANSI-free)
myplace help --json   # the same as a machine-readable manifest (flags, defaults, required-off-a-TTY, exit codes)
myplace help status --json   # scope either view to one command
```

Both views are generated from the live command tree, so they can't drift from the real flags. Plain `myplace help` (and `myplace --help`) opens with a pointer to them. Design: [LLM-friendly help spec](docs/features/llm-friendly-help.md).

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
