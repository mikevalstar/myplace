# TODO

Lightweight running list of work that isn't yet an ADR/feature doc. Design still
lands in [docs/](docs/) — this is just the backlog and the "what are we blocked on"
board. Promote an item to a `docs/features/*` spec or `docs/adrs/*` ADR when it's
ready to build; check it off (or delete it) when done.

## ⏳ Waiting on others (external, blocked)

- [ ] **Migrate global skills to a tool-managed lockfile.** Today the third-party
  global skill stable is self-managed as a `skills add -g` source list in the
  provision script, because the skills.sh CLI can't restore a *global* stable from
  its lock (`experimental_install` is project-only). Switch to a committed
  `~/.agents/.skill-lock.json` (minus its volatile timestamp/UI fields) driven by a
  real restore command once it exists.
  Blocked on **[vercel-labs/skills#683](https://github.com/vercel-labs/skills/issues/683)**.
  See [ADR-0023](docs/adrs/0023-managing-ai-skills.md).

## 🔎 Investigate / spikes

- [ ] **@mike: Computer-use setup on the home Mac** — get computer use working on
  the home Mac, and figure out whether it means anything for this project (e.g. a
  managed-setup dependency, a profile concern, or a myplace capability). Report back
  before deciding if it needs an ADR.

## 🛠️ Planned features

- [ ] **Skill reviewer in myplace** — a command/TUI view to add, remove, and manage
  **global** AI skills (list installed, add from a repo, remove, and drive updates)
  on top of the skills.sh CLI — the interactive/management counterpart to the
  read-only `outdated` skills source shipped in [ADR-0023](docs/adrs/0023-managing-ai-skills.md).
  Write a feature spec (and confirm the CLI's non-interactive `add`/`remove -g`
  surface) before building.
- [ ] **`update --on-local-edits=keep|discard|skip`** — a headless resolution for
  local edits, the flag pattern [ADR-0006](docs/adrs/0006-agent-runnable-commands.md)
  itself names. Today local-edit drift can only be resolved at a TTY; an agent
  driving `update --yes` can report the edits but never act on them.
- [ ] **Doctor check for the age key** — on non-`server` profiles: key file
  present, non-empty, and actually decrypts a probe target ([ADR-0022](docs/adrs/0022-age-encrypted-dotfiles.md)).
  Turns a missing/broken key into a named remedy ("run `myplace update` with
  `op` signed in") instead of a cryptic decrypt failure on status/apply.
- [ ] **`update --dry-run`** — print the steps that would run and the incoming
  per-file diff without touching anything, reusing the existing review machinery.
- [ ] **`myplace log`** — `--tail N` / `--follow` / `--json` over the state log
  (`$XDG_STATE_HOME/myplace/myplace.log`), so "what happened on this box" doesn't
  require remembering the path ([logging spec](docs/features/logging.md)).
