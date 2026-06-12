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

## Key external tools

- **chezmoi** — manages dotfiles from a git repo; supports templates, per-machine data, and scripts. The TUI shells out to it rather than reimplementing its logic.
- **mise-en-place (mise)** — manages dev tools, language runtimes, and tasks via `mise.toml`. Same rule: orchestrate it, don't replace it.

The TUI's job is orchestration and visibility on top of these tools, not duplication of them. Prefer invoking their CLIs and parsing their output (both support `--format json` style output for most commands) over re-implementing behavior.

## Project state

Design/documentation phase. The TUI language and framework are not yet chosen — that decision belongs in an ADR before any code is written.
