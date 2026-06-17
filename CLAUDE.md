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
- **Secret-bearing dotfiles use 1Password, never the repo (ADR-0016).** The `private_` prefix only sets 0600 perms — content is still committed in plaintext, so it's useless for secrets in this public repo. Pull secret content at apply time with chezmoi's `onepasswordDocument`/`onepasswordRead` instead. The live case is `~/.ssh/config` (`home/private_dot_ssh/private_config.tmpl`): the **host list with server IPs lives in a 1Password Document** (`ssh config`, `Private` vault), pulled only on non-`server` profiles; the repo template holds just the non-secret global `Host *` defaults. **To change hosts/IPs, edit the 1Password Document directly — NEVER put a server IP or host block in this repo.** Claude can and should do this edit itself via the `op` CLI, so the secret only ever lives in 1Password: read with `op document get "ssh config" --vault Private --account my.1password.com`, then replace from stdin with `op document edit "ssh config" - --vault Private --account my.1password.com` (pipe the full new config in). The `--account` (and the matching third arg `"my.1password.com"` on the template's `onepasswordDocument` call) is **required** on machines signed into more than one 1Password account — without it `op` errors `multiple accounts found` and chezmoi's apply fails; drop it on single-account machines. The IPs pass through `op` and your context, never a tracked file. To change the shared defaults instead (not secret), edit the `Host *` block in the template. OS-specific keywords (e.g. `UseKeychain`) are gated on `.chezmoi.os`, secret pulls on `.profile`. Note: this makes `myplace status` on a mac shell out to `op` (chezmoi runs `onepassword*` on every status/diff/apply), so an unlocked 1Password is a soft dependency there; servers never call it. How-to: [managed-setup guide](docs/guides/managed-setup.md).

## Dogfooding myplace on setup changes

When the request is to change the **machine setup** — a mise tool, a dotfile, an alias, a provision step under `home/**` — rather than the app code, drive and verify it through `myplace`, not by hand-running `chezmoi apply` / `mise install`. Running the tool on its own config is the point, and it's the quickest way to catch a change that breaks `apply`.

- **Learn the command surface from the tool, not memory:** `myplace help --llm` (or `--json`) is generated from the command tree — authoritative, lists every command/flag/exit code. Don't restate it here. All data commands take `--json`; exit codes: `0` in sync · `1` drifted · `2` unknown · `3` error. Off a TTY, bare `myplace` prints the status summary instead of launching the TUI — but prefer `myplace status --json`.
- **Inspect anytime (read-only, safe):** `myplace status` / `myplace status --json` shows drift in both directions.
- **myplace applies from the source clone, not this checkout.** chezmoi/myplace read `~/.local/share/chezmoi/` — a *separate* clone of this repo — so an edit under `home/**` here is invisible to `status`/`update` until it's committed and pushed to `main` (the branch every machine's `update` pulls; setup edits land on `main`, not a feature branch). The loop to land a change on this machine:
  1. Edit the file under `home/` — where things go: [managed-setup guide](docs/guides/managed-setup.md).
  2. Commit + push to `origin/main` (the push is what makes it take effect — confirm before pushing per the usual rules).
  3. `myplace update` (headless: `myplace update --yes --json`) — pulls origin into the source clone, applies dotfiles, upgrades tools. `--yes` is converge-only: it won't capture local edits made directly in `$HOME`.
  4. `myplace status` — confirm it's back to in sync (exit `0`).
- **Don't run `update`/`bootstrap` just to test the app** — they mutate this real machine. Exercise app behavior with `status`, `help`, and the Go tests instead.

## Cutting a release

Releases are **tag-driven** — pushing a `vX.Y.Z` tag *is* the release. The `release.yml` Action runs goreleaser, which cross-compiles the darwin/linux × amd64/arm64 matrix, builds the archives + `checksums.txt`, and publishes the GitHub Release. **Never build or upload artifacts by hand** — the tag is the trigger and CI does the rest (pipeline internals and the version-less archive URL that `install.sh`/`self-update` depend on: [ADR-0004](docs/adrs/0004-release-pipeline-goreleaser.md)).

The version is **injected from the tag** via ldflags; `internal/version/version.go` only holds an `X.Y.Z-dev` fallback for local/non-release builds, so the released binary reports the clean tag (`myplace version`). Bump semver-style: new capability → minor, fix → patch, breaking change → major.

The ritual (matches the existing tag history — e.g. `git log -- internal/version/version.go`):

1. Land the change on `main` with **green CI** (`ci.yml` runs `go test`/`vet`/`gofmt` on every push — a release tag should never be the first time code builds in CI).
2. Bump `internal/version/version.go` to the version you're about to tag with the `-dev` suffix, in its own commit: `Bump dev version to X.Y.Z-dev for the vX.Y.Z release`.
3. Push `main`.
4. Create an **annotated** tag — `git tag -a vX.Y.Z -m "vX.Y.Z — <headline>"` — then `git push origin vX.Y.Z`. The tag push is the outward step, so confirm first per the usual rules.
5. Confirm the release run is green (`gh run watch`) and goreleaser published the release + assets. `install.sh` and `self-update` then serve the new tag automatically; `status` flags machines on an older binary.

## Project state

v0 implemented and verified end-to-end: `bootstrap` (wizard + headless), `status` (TUI dashboard + `--json`, spec'd exit codes, includes outdated-binary check), `doctor` (read-only preflight: chezmoi/mise installed + version floors, PATH, chezmoi initialized, dotfiles-remote + GitHub reachability, state-dir writability, TTY; per-check pass/warn/fail with remedies + `--json`; exit 0 ready / 1 failed / 2 incomplete-offline / 3 error; spec `docs/features/doctor-preflight-diagnostics.md`), `outdated` (cross–package-manager inventory — mise + brew-if-present — TUI pane + `o` detail view + `--json`; informational, never affects the drift verdict, ADR-0010), `update` (interactive: per-file capture of local edits, per-file incoming diff review, profile push policy, then converge; headless: converge-only), `sysinfo` (OS + hardware specs via fastfetch — TUI header band + `--json`; informational, read-only, ADR-0013), `self-update` (GitHub releases), `help --llm`/`--json` (self-describing agent manifest + brief generated from the cobra tree; per-command facts live in cobra `Annotations`, enforced by `cmd/myplace/help_test.go`). Persistent debug log to `$XDG_STATE_HOME/myplace/myplace.log` (ADR-0005, logging feature spec). Releases: tag `v*` → goreleaser via Actions (ADR-0004); `install.sh` at repo root is the installer. Not yet built: phase-2 server.
