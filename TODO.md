# TODO

Lightweight running list of work that isn't yet an ADR/feature doc. Design still
lands in [docs/](docs/) — this is just the backlog and the "what are we blocked on"
board. Promote an item to a `docs/features/*` spec or `docs/adrs/*` ADR when it's
ready to build; check it off (or delete it) when done.

## ⏳ Waiting on others (external, blocked)

- [ ] **Integrate `skilloom` once it's written.** Skills management has been spun
  out of myplace into a **separate project, skilloom** ([ADR-0024](docs/adrs/0024-skills-management-as-separate-project.md),
  supersedes ADR-0023) — myplace will orchestrate it as an external tool (install
  via the setup, optionally surface its status informationally), not build a skills
  engine itself. Blocked on skilloom being written. When it lands: add it to the
  managed setup (INVENTORY, mise baseline / provision, README), decide how myplace
  surfaces its status, and retire/re-point the interim skills.sh `outdated` source.
  The old skills.sh-CLI lockfile-migration plan (blocked on
  [vercel-labs/skills#683](https://github.com/vercel-labs/skills/issues/683)) is
  **superseded** by this — skilloom owns the reproducibility story now.

## 🔎 Investigate / spikes

- [ ] **@mike: Computer-use setup on the home Mac** — get computer use working on
  the home Mac, and figure out whether it means anything for this project (e.g. a
  managed-setup dependency, a profile concern, or a myplace capability). Report back
  before deciding if it needs an ADR.

## 🛠️ Planned features

- [ ] ~~**Skill reviewer in myplace**~~ — **moved out of myplace.** Interactive
  skill management (add/remove/reconcile, global + per-project, multi-agent) is now
  its own project, **skilloom** ([ADR-0024](docs/adrs/0024-skills-management-as-separate-project.md)).
  myplace's only future skills work is *orchestrating* skilloom (see the waiting item
  above), not building the reviewer itself.
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
