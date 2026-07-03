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
