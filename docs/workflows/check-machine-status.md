---
title: Check machine status
status: draft
created: 2026-06-12
updated: 2026-06-12
tags: [status, drift, chezmoi, mise, json]
actors: [user, tui, chezmoi, mise]
---

# Check machine status

## Goal

Answer "is this machine in sync?" — in **both directions** (changes here that aren't in the repo, and repo/tool updates not yet applied here) — without changing anything. This is the heart of the product: the dashboard renders it, the update workflow runs on it, and phase 2 ships it to the server.

## Preconditions

- Machine has been bootstrapped (chezmoi initialized, mise installed). If not, the TUI routes to [bootstrap](bootstrap-new-machine.md).
- Network access is optional: without it, the remote-comparison checks are reported as `unknown` rather than failing the whole status.

## Steps

All checks are read-only on the machine (the only network op is a git fetch into the chezmoi source repo) and run concurrently as `tea.Cmd`s:

1. **Incoming dotfile drift** — what would change if we applied:
   - `chezmoi git -- fetch` then compare `chezmoi git -- rev-list --count HEAD..@{upstream}` → commits behind origin.
   - `chezmoi status` / `chezmoi diff` → files that differ between source state and the machine.
2. **Outgoing dotfile drift** — what the machine has that the repo doesn't:
   - Locally modified managed files (from the same `chezmoi status` output, direction-aware).
   - Uncommitted changes in the source dir: `chezmoi git -- status --porcelain`.
   - Committed-but-unpushed: `chezmoi git -- rev-list --count @{upstream}..HEAD`.
3. **Tool drift** (mise):
   - Missing: `mise ls --json` entries where `installed == false`.
   - Outdated vs config: `mise outdated --json`.
   - mise itself outdated: `mise version` self-check.
4. **myplace itself** — compare running version against latest release (skipped when offline).
5. **Aggregate** into a single report with an overall verdict: `in_sync` | `drifted` | `unknown` (some checks couldn't run) | `error`.

### Output modes

- **TUI dashboard:** one pane per section with counts and drill-down (viewport showing the actual diff / tool list).
- **Headless:** `myplace status --json` prints the report and exits non-zero when drifted — usable from cron, SSH loops, and CI. Sketch of the shape (the precise schema gets its own feature spec before phase 2 freezes it):

```json
{
  "machine": "hostname",
  "profile": "server",
  "checked_at": "2026-06-12T20:00:00Z",
  "verdict": "drifted",
  "dotfiles": {
    "behind_origin": 2,
    "to_apply": ["dot_zshrc"],
    "local_modified": [],
    "unpushed_commits": 0
  },
  "tools": {
    "missing": [],
    "outdated": [{"name": "node", "current": "22.1.0", "wanted": "22.3.0"}]
  },
  "myplace": {"current": "0.3.0", "latest": "0.4.0"}
}
```

## Outcome

Nothing on the machine has changed. The user (or a script) knows exactly what is out of sync, in which direction, and what the [update workflow](update-machine.md) would do about it.

## Failure modes

| What can go wrong | How the user finds out | Recovery |
|-------------------|------------------------|----------|
| Offline / fetch fails | remote-dependent fields show `unknown`, verdict degrades to `unknown` not `error` | rest of the report still renders; retry when online |
| chezmoi or mise binary missing/broken | section-level error with the exec error | suggests bootstrap repair step |
| `mise` JSON output shape changes across versions | parse error surfaced with raw output attached | guide gotcha: pin minimum mise version, tolerate unknown fields |
| Source repo has no upstream configured | behind/ahead counts `unknown` | status hints how to set upstream |

## Related

- [Update a machine](update-machine.md) — acts on this report
- [Bootstrap a new machine](bootstrap-new-machine.md) — runs this as its final verification
- CLAUDE.md "settled design points" — bidirectional definition of in-sync; `--json` from day one
