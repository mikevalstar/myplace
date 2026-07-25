---
title: ADR-0023 — Managing AI agent skills (own via chezmoi, third-party via skills.sh)
status: superseded
created: 2026-07-03
updated: 2026-07-11
tags: [skills, ai, claude-code, chezmoi, outdated, provisioning, cli]
supersedes: null
superseded-by: "0024"
---

# ADR-0023: Managing AI agent skills

> **Superseded by [ADR-0024](0024-skills-management-as-separate-project.md).** In practice the orchestrate-the-skills.sh-CLI + chezmoi + informational-only approach below wasn't robust enough — it has no per-project story, no bidirectional reconcile/diff, and leaned on a CLI whose global store can't be restored (#683). A robust design turned out to be a whole application in its own right, so skills management is being spun out into a **separate project, skilloom**, that myplace will orchestrate as an external tool (like chezmoi/mise) rather than contain. The `internal/skills` + `outdated.SkillsSource` code from this ADR stays as an interim stopgap until skilloom lands. This record is kept for the history and the tool facts it documents.

## Context

The machines myplace manages increasingly carry **AI agent skills** — folders of markdown (a `SKILL.md` plus optional supporting files/scripts) that Claude Code, Codex, Cursor and other agents discover and invoke. Two kinds coexist:

1. **Skills the owner authors** — written and edited locally, part of their personal environment.
2. **A "stable" of third-party skills** — pulled from public git repos and kept roughly current, installed with the **skills.sh CLI** (the Vercel-Labs `skills` npm package, run as `npx skills`).

We want the same thing myplace already gives dotfiles and tools: a machine bootstraps with the expected skills, and `status`/`outdated` make it visible when something is behind. The question is *which mechanism* — myplace's rule is to **orchestrate** existing tools, not reimplement them (ADR-0003), and to keep new capabilities headless-first and informational unless they genuinely belong in the drift verdict (ADR-0006, ADR-0010).

Facts that shaped the decision (verified against the real tools, not inferred):

- **Claude Code has no skill registry.** Skills are discovered from the filesystem: personal `~/.claude/skills/`, project `.claude/skills/`, and plugin-provided. Loose skills have no auto-update.
- **The skills.sh CLI (`skills` v1.5.14) is "npm for skills".** `add`/`list`/`update`/`remove`/`find`, `experimental_install`, and a `check` that reports what's behind upstream. Its **canonical store is `~/.agents/skills/`**, which it symlinks into each agent's dir (`~/.claude/skills/`, `~/.cursor/skills/`, …). `-g` selects that global scope.
- **There are two lockfiles, and the global one can't be restored (yet).** Global lock: `~/.agents/.skill-lock.json`; project lock: `./skills-lock.json`. `check`/`update` read the **global** lock; `experimental_install` restores only from the **project** lock into the project store, and refuses global outright ("No project skills found in skills-lock.json"). So there is no first-class way to rehydrate the global stable on a new machine — tracked upstream in [vercel-labs/skills#683](https://github.com/vercel-labs/skills/issues/683). The global lock also mixes in volatile UI/timestamp state (`updatedAt`, `installedAt`, `lastSelectedAgents`, `dismissed`), so committing it verbatim as a managed dotfile would churn `chezmoi` drift on every skill update.
- **`skills check` emits ANSI human text and ignores `--json`.** Only `skills list --json` is structured, and it carries no version info (name/path/scope/agents only). So an "outdated skills" check has to parse `check`'s text.
- There is also a competing native path — Claude Code **plugins/marketplaces** (`/plugin`, `marketplace.json`) — that bundles skills + commands + subagents + MCP and has its own auto-update.

## Options considered

### Distribution mechanism for the third-party stable

**Option A — orchestrate the skills.sh CLI (chosen).** Commit `skills-lock.json`; restore on new machines; surface "what's outdated" by shelling out to the CLI. It's skills-specific, has a lockfile, and maps directly onto the existing mise-style wrapper + `outdated.Source` pattern. Cost: a Node/`npx` dependency (already present on desktops via fnm, ADR-0007) and `check`'s non-JSON output.

**Option B — native Claude Code plugins/marketplaces.** Manage marketplace/enabled-plugin entries in `~/.claude/settings.json` via chezmoi and let Claude Code's own auto-update refresh them. No extra dependency, but little for myplace to actively drive or report, it bundles far more than skills, and it doesn't give a clean committed-manifest → restore story myplace can run headlessly.

**Option C — support both.** Most coverage, most surface area and docs to maintain; premature.

### Does skill status affect the drift verdict?

**Informational only (chosen)** — skills flow through the existing `outdated.Source` interface, exactly like brew/shelly, and never touch `internal/drift` or the exit codes. **Vs. part of the verdict** — a missing/outdated skill would count as drift (exit 1), requiring changes to `internal/drift` (Report field, `Decide`, `Compute`) and a schema bump.

## Decision

1. **The owner's own skills are chezmoi-managed dotfiles.** They live under a `home/dot_claude/skills/` tree → `~/.claude/skills/`, versioned in this repo like any dotfile; drift, diff review, and push policy come free from `update`/`status`. chezmoi manages only `skills/` (and specific files like `settings.json`), **not** all of `~/.claude` — `plugins/cache/`, logs, and history are machine-local and `.chezmoiignore`d. It also does **not** manage `~/.agents/skills/`: that's the skills.sh CLI's own store (same stance as never managing mise's install dir).

2. **The third-party stable is orchestrated through the skills.sh CLI, self-managed for now.** Because the CLI can't yet restore a *global* stable from its lock (see Context / #683), the declarative install set is a **hand-maintained list of source repos** in a profile-gated provision block (`ne .profile "server"` — servers get no AI skills) that runs `skills add <repo> -g -s '*' -y` per repo — that list *is* the fleet's global "lockfile" until the tool provides one. Ongoing visibility comes from a new `internal/skills` wrapper that shells out to `skills check -g`. **Migration intent: switch to a tool-driven global restore from `~/.agents/.skill-lock.json` as soon as [vercel-labs/skills#683](https://github.com/vercel-labs/skills/issues/683) ships** — at which point the committed source list is replaced by a committed global lock the CLI rehydrates directly.

3. **Skill status is informational, surfaced via `outdated` only.** `internal/skills` is adapted into an `outdated.Source` (`SkillsSource`) and added to the source slice in `main.go`. Because the TUI renders whatever is in that slice (the "Updates available" pane, the `o` detail table, and the count chip all iterate `inventory.Sources`), skills appear alongside mise/brew/shelly with **no dashboard code change**. It never affects the drift verdict or exit codes (ADR-0010). We do **not** adopt the common `SessionStart` hook that runs `npx skills update` — that hides update state from the very tool meant to show it; updates are driven through `myplace`.

## Consequences

**Easier**

- No third mechanism: authored skills reuse chezmoi, third-party skills reuse the mise/`outdated` pattern. The seam is one wrapper + one adapter + one slice entry.
- `myplace outdated` / `outdated --json` and the dashboard now answer "are my skills behind?" on any box with the CLI; the source self-reports `available: false` where it isn't (servers, no Node), so it's safe everywhere.
- The lockfile makes the skill set reproducible across the fleet.

**Harder / committed to**

- **`skills check` output is human text, not JSON.** `internal/skills` parses `check -g`'s ANSI-stripped text (the way `internal/chezmoi` line-parses `chezmoi status`). The all-clear and unavailable cases are verified; the *has-updates* line shape was not observable on an up-to-date machine, so that branch is best-effort and tolerant, isolated to `ParseCheck` (+ its tests) for a one-line fix once a real outdated skill is seen. Confirmation is tracked in the feature doc's acceptance criteria.
- We depend on the CLI's resolution (`skills` on PATH, else `npx --no-install skills` — offline, never auto-downloads) and on `check`/`-g` semantics staying stable; `check` isn't in the CLI's `--help` list, so a future rename is a risk, again confined to the wrapper.
- **Self-managed global stable is manual.** The source-repo list is hand-maintained; adding/removing a global skill means editing the provision block (and, until #683, there's no committed lock capturing exact resolved versions — `check -g` reports drift, but reproduction re-resolves "latest"). Accepted as interim.
- **Migration on [vercel-labs/skills#683](https://github.com/vercel-labs/skills/issues/683):** once the CLI can restore/relink the global store from `~/.agents/.skill-lock.json`, replace the source list with a committed global lock the CLI rehydrates — likely filtering the lock's volatile UI/timestamp fields so it doesn't churn `chezmoi` drift. This ADR is not superseded by that; it's a mechanism swap noted here.
- **Follow-up not done here (needs the owner's real skills + a push to `main`, per the dogfooding rule):** the `home/dot_claude/skills/` tree for authored skills and the profile-gated `skills add -g` source-list block in `run_onchange_provision.sh.tmpl`. This ADR + the wrapper land the app-side visibility; wiring the managed setup is a separate change.

## Related

- [ADR-0010](0010-cross-package-manager-outdated-inventory.md) — informational, pluggable, read-only, separate from drift (the pattern this reuses)
- [ADR-0007](0007-provisioning-mechanism.md) / [ADR-0017](0017-linux-desktop-profile.md) — the provision split and the desktop-vs-server profile gate
- [ADR-0003](0003-monorepo-app-dotfiles-mise.md) / [ADR-0006](0006-agent-runnable-commands.md) — orchestrate-don't-reimplement; headless-first
- [Outdated packages feature](../features/outdated-packages.md) — the inventory/schema/TUI this extends
- [Managed setup guide](../guides/managed-setup.md) — how the chezmoi tree + provision block will be wired
- [vercel-labs/skills#683](https://github.com/vercel-labs/skills/issues/683) — upstream request for global restore/relink from `~/.agents/.skill-lock.json`; the migration target that would replace the self-managed source list
