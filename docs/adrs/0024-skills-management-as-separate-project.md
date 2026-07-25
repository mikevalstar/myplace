---
title: ADR-0024 — Skills management is a separate project (skilloom), orchestrated by myplace
status: accepted
created: 2026-07-11
updated: 2026-07-11
tags: [skills, ai, claude-code, agents, skilloom, scope, orchestration]
supersedes: "0023"
superseded-by: null
---

# ADR-0024: Skills management is a separate project (skilloom)

## Context

[ADR-0023](0023-managing-ai-skills.md) tried to fit AI-skill management *inside* myplace — orchestrate the skills.sh CLI for a global stable, keep authored skills as chezmoi dotfiles, surface currency informationally. In use that proved too thin: no per-project story, no bidirectional reconcile/diff, and a dependence on a CLI whose global store can't be restored ([#683](https://github.com/vercel-labs/skills/issues/683)).

Designing something robust enough — the shape the requirements actually call for — makes the size of the thing obvious:

- **Multi-scope**: skills applied globally *and* vendored into individual project repos.
- **Multi-agent**: one canonical `.agents/skills/<name>/` symlinked into each vendor dir (`.claude/skills/`, `.cursor/skills/`, …).
- **Bidirectional reconcile**: skills get edited locally *and* upstream; the tool must classify in-sync / locally-changed / upstream-changed / changed-on-both / source-gone and show a diff.
- **Its own state and its own TUI**: a machine-local record of what's installed where, plus an interactive reconcile UI.

That is **a whole application in its own right** — comparable in weight to chezmoi or mise. And chezmoi and mise are exactly the kind of tool myplace **orchestrates rather than contains** (ADR-0003). Building a skills engine *into* myplace would bloat its scope and break the identity that makes it coherent: myplace is a thin orchestration + visibility layer over dedicated tools, not the place those tools live.

## Options considered

### Option A — build a native skills engine inside myplace

A `myplace skills` subcommand owning git fetch, state, symlinks, and reconcile. Rejected: it's not a subcommand-sized capability, it's a peer to chezmoi/mise. Embedding it inverts myplace's orchestrate-don't-reimplement stance (ADR-0003) and couples a large, fast-moving domain to myplace's release cycle.

### Option B — keep orchestrating the skills.sh CLI (ADR-0023)

Rejected for the reasons in the Context (no per-project model, no reconcile, blocked global restore). The whole point of revisiting was that this isn't enough.

### Option C — spin skills management out into its own project, orchestrated by myplace (chosen)

A dedicated tool, **skilloom**, implements the engine. myplace stays out of the skills business except to **manage skilloom as one more external tool** — install it via the setup and (optionally) surface its status informationally, the same relationship it has with chezmoi, mise, and fastfetch.

## Decision

**Skills management is spun out into a separate project, `skilloom`.** myplace does **not** implement a skills engine.

- **skilloom owns the design and the code.** The engine shape explored here — native git fetch of skill sources, its own machine-local store/state, **per-project vendoring** (real committed files under `<project>/.agents/skills/`), **2-way "track latest" reconcile** (no pinned base, no auto-merge — classify and pick-a-side via a diff), a canonical `.agents/skills/` symlinked into each agent's vendor dir, and a **TUI-mutates / read-only-`--json`-status** split — is recorded here as **design intent handed to skilloom**, to be specified properly in skilloom's own ADRs and feature docs. It is not myplace work.
- **myplace will orchestrate skilloom as an external tool, later.** Once skilloom exists, add it to the managed setup like any other tool (mise baseline or the profile-gated provision block, ADR-0007 — desktop-only, servers get no AI skills per ADR-0017) and, if useful, surface its status informationally the way `outdated`/`sysinfo` do. This is **deferred until skilloom is written** — no myplace code or setup change lands now.
- **ADR-0023's shipped code stays as a stopgap.** `internal/skills` (the skills.sh CLI wrapper) and its `outdated.SkillsSource` adapter keep working as the interim informational signal until skilloom lands, at which point they're revisited (skilloom likely provides the currency signal, or the skills.sh source is dropped). Nothing is ripped out today.

## Consequences

**Easier**

- myplace's scope stays lean and true to ADR-0003: it orchestrates tools, it doesn't grow into one.
- skilloom can move at its own pace, with its own release cadence, docs, and versioning, without touching myplace's.
- When skilloom ships it slots into the existing managed-tool machinery (install via setup, informational status) with no new pattern to invent.

**Harder / committed to**

- **Two repos to keep coherent.** The myplace↔skilloom seam (how myplace installs it, how/whether it surfaces its status) has to be designed when skilloom is integrated — captured as future follow-up, not now.
- **The detailed design must be re-homed.** The design intent summarized above needs porting into skilloom's own ADRs/feature specs when that project is scaffolded, so the thinking isn't lost.
- **A dangling stopgap.** The ADR-0023 skills.sh `outdated` source lives on until skilloom replaces it; until then "manage AI skills" is only partially served on myplace's side (read-only currency, no per-project/reconcile).

**Follow-up (deferred until skilloom exists)**

- Scaffold the `skilloom` project and port the design intent into its own ADRs/feature docs.
- Add skilloom to myplace's managed setup (INVENTORY, mise baseline or provision, README) and decide how myplace surfaces skilloom status; update `myplace help --json/--llm` then.
- Retire or re-point the ADR-0023 `internal/skills` / `outdated.SkillsSource` wiring once skilloom is the source of truth; update `outdated-packages.md` and the TODO accordingly.

## Related

- [ADR-0023](0023-managing-ai-skills.md) — **superseded by this ADR**; the in-myplace CLI-orchestration approach this replaces
- [ADR-0003](0003-monorepo-app-dotfiles-mise.md) — orchestrate-don't-reimplement; the identity that makes skilloom a peer tool, not a subcommand
- [ADR-0007](0007-provisioning-mechanism.md) / [ADR-0017](0017-linux-desktop-profile.md) — how a future managed tool is installed, and the desktop-vs-server gate (servers get no AI skills)
- [ADR-0010](0010-cross-package-manager-outdated-inventory.md) — the informational, separate-from-drift pattern myplace would reuse to surface skilloom status
- [Outdated packages feature](../features/outdated-packages.md) — where the ADR-0023 skills.sh stopgap currently surfaces
