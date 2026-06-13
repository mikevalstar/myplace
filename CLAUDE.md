# CLAUDE.md

## What this project is

`myplace` is a TUI that wraps **chezmoi** (dotfiles) and **mise-en-place** (tool/runtime management) to bootstrap new machines, update existing ones, and report their status. It targets a mix of personal Macs, a work Mac, and Linux servers — the same setup applied across many hosts.

A future phase adds a central server that machines ping, so the status of every system can be tracked from one place. Keep that in mind when designing, but don't build for it prematurely.

## Documentation-first

This project is documentation-first. **Before implementing a feature or making an architectural choice, write (or update) the relevant doc:**

- New tech/library/architecture choice → ADR in `docs/adrs/`
- New user-visible capability → spec in `docs/features/`
- New end-to-end flow the TUI supports → `docs/workflows/`
- Knowledge a developer of this repo needs (library usage, conventions, gotchas) → `docs/guides/`

Each folder has a `_template.md` showing the expected format. **All docs use YAML frontmatter** (title, status, dates, tags) so they can be searched and filtered later — never omit it. See [docs/README.md](docs/README.md) for the structure and conventions.

When a decision changes, don't edit history: supersede the old ADR with a new one and update the old ADR's `status` field.

## Conventions

- ADRs are numbered sequentially: `0001-some-decision.md`, `0002-...`
- Other docs use kebab-case descriptive names: `bootstrap-new-machine.md`
- Doc `status` values: `draft` → `accepted`/`active` → `superseded`/`deprecated`
- Dates in frontmatter are ISO format: `2026-06-12`
- **The README is part of the spec**: its install/usage sections must be updated in the same change whenever the command surface, flags, or install story changes. Docs explain design; the README shows a user how to run it.
- **The agent help is part of the contract**: `myplace help --json`/`--llm` is the primary way AI agents and scripts discover this tool, and it's generated from the cobra command tree — treat it like the docs and README, something to keep current rather than an afterthought. Whenever you add or change a command or flag, set/update that command's cobra `Annotations` (canonical headless invocation, `required` flags off a TTY, exit-code meanings, output-schema doc path) in the same change so the manifest stays truthful. `cmd/myplace/help_test.go` walks the tree and fails if a command is missing what it owes, so this can't silently drift. See the [LLM-friendly help spec](docs/features/llm-friendly-help.md).

## Key external tools

- **chezmoi** — manages dotfiles from a git repo; supports templates, per-machine data, and scripts. The TUI shells out to it rather than reimplementing its logic.
- **mise-en-place (mise)** — manages dev tools, language runtimes, and tasks via `mise.toml`. Same rule: orchestrate it, don't replace it.

The TUI's job is orchestration and visibility on top of these tools, not duplication of them. Prefer invoking their CLIs and parsing their output (both support `--format json` style output for most commands) over re-implementing behavior.

## Settled design points

Decided but not all spec'd yet — write the feature/workflow doc before building on one of these:

- **Stack: Go + Charm libraries** (Bubble Tea, Bubbles, Lip Gloss, Huh) — see ADR-0002 and `docs/guides/charm-tui-stack.md`. Core logic lives in TUI-free packages; the TUI is a skin.
- **Headless `--json` from day one**: every capability works as a plain CLI command with `--json` output. Phase 2's server reporting builds on this, so never weld logic to the TUI layer.
- **Machine profiles share by default**: personal Macs, work Mac, and servers are profiles over one common setup; per-machine differences are the exception, handled via chezmoi templates/data.
- **Status is bidirectional**: "in sync" means no drift in *either* direction — local changes not pushed to the dotfiles repo, and repo/tool updates not yet applied locally both count as out of sync.
- **Every command is agent-runnable** (ADR-0006): each command must have a fully non-interactive path (flags + `--yes` + `--json`); off a TTY with a needed decision unsupplied, it fails fast (exit 3) naming the flag rather than prompting or hanging. No subprocess may grab the terminal. A new command isn't done until a script can drive it without a human.

## Monorepo layout (ADR-0003)

This repo is simultaneously the app, the chezmoi source repo, and the mise config:

- `home/` — chezmoi source state (selected by `.chezmoiroot`); the machines' global mise config lives at `home/dot_config/mise/config.toml.tmpl`
- `mise.toml` at the root is dev tooling for **this repo only** (Go toolchain, build/test tasks) — don't confuse the two
- `cmd/`, `internal/` — the Go app; `internal/{run,chezmoi,mise,drift}` must never import TUI packages
- Machine-local state (logs now, caches later) lives under `$XDG_STATE_HOME/myplace` (ADR-0005), **never** `~/.config` — that's chezmoi's tree. Every external command is logged via the `run.Runner` choke point.
- **Provisioning split (ADR-0007):** mise installs registry CLI tools (in `home/dot_config/mise/config.toml.tmpl`); `home/.chezmoiscripts/run_onchange_provision.sh` installs what mise can't — oh-my-zsh + plugins, rustup, fnm. **Node is fnm's and Rust is rustup's — never add them to mise.** Adding a tool/dotfile? See `docs/guides/managed-setup.md`.

## Project state

v0 implemented and verified end-to-end: `bootstrap` (wizard + headless), `status` (TUI dashboard + `--json`, spec'd exit codes, includes outdated-binary check), `update` (interactive: per-file capture of local edits then converge; headless: converge-only), `self-update` (GitHub releases), `help --llm`/`--json` (self-describing agent manifest + brief generated from the cobra tree; per-command facts live in cobra `Annotations`, enforced by `cmd/myplace/help_test.go`). Persistent debug log to `$XDG_STATE_HOME/myplace/myplace.log` (ADR-0005, logging feature spec). Releases: tag `v*` → goreleaser via Actions (ADR-0004); `install.sh` at repo root is the installer. Not yet built: per-file diff review before apply, `push: false` profile policy, phase-2 server.
