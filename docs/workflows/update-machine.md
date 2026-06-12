---
title: Update a machine
status: draft
created: 2026-06-12
updated: 2026-06-12
tags: [update, drift, chezmoi, mise]
actors: [user, tui, chezmoi, mise]
---

# Update a machine

## Goal

Resolve the drift found by [status](check-machine-status.md), in both directions: apply what the repo has that this machine lacks, and capture/push what this machine has that the repo lacks. Everything is reviewed before it's applied — the TUI never silently rewrites files or force-pushes config.

## Preconditions

- Machine is bootstrapped.
- A fresh status report (the update screen *is* the status report, with actions attached).
- Network access for pull/push and tool downloads (offline mode can still re-apply already-fetched source state and install cached tools).

## Steps

The update screen presents drift grouped into the four buckets below; the user can run them all ("update everything") or drill into each. Order matters — incoming dotfiles first, because they may change the mise config that step 3 reads.

1. **Pull incoming dotfiles.** `chezmoi git -- pull --rebase` (rebase keeps any local source-dir commits on top), then show `chezmoi diff` in a viewport. On confirm: `chezmoi apply`. Per-file skip is available — skipping records nothing; the file just stays in next status as drift.
2. **Capture outgoing dotfiles.** For each locally-modified managed file: show the reverse diff, then per file choose:
   - **keep & share** → `chezmoi re-add <file>` (machine wins, repo updated)
   - **discard** → `chezmoi apply <file>` (repo wins, local edit overwritten)
   - **skip** → decide later
   Then commit and push captured changes: `chezmoi git -- add -A`, `chezmoi git -- commit` (message prompt with sensible default), `chezmoi git -- push`.
3. **Update tools.** `mise install` (anything missing), then `mise upgrade` for tools outdated against the (possibly just-updated) config. `mise up --bump`-style config-version bumps are a deliberate, separate action — updating a machine should converge it on the shared config, not mutate the shared config as a side effect.
4. **Update myplace itself.** If a newer release exists, download the new binary and swap in place (`myplace self-update`); prompt to restart the TUI.
5. **Re-run status** and show the closing dashboard — ideally all green.

Decision points worth noting:

- **Conflicts on pull** (rebase fails): drop to `tea.ExecProcess` with a shell in the source dir and clear instructions; myplace does not attempt automatic conflict resolution.
- **Work-mac caution:** profile data can mark a machine `push: false` (e.g. work Mac policy) — outgoing capture then stops at local commit and the dashboard shows "unpushed by policy" instead of treating it as drift to fix.

## Outcome

Machine matches the shared config, local improvements are committed and pushed (or deliberately parked), tools are at the pinned versions, and the closing status report is clean. Other machines will see the pushed changes on their next update.

## Failure modes

| What can go wrong | How the user finds out | Recovery |
|-------------------|------------------------|----------|
| Pull rebase conflict | step 1 halts with git's conflict output | hand-off shell via `tea.ExecProcess`; re-run update after resolving |
| `chezmoi apply` conflicts with unexpected local file | chezmoi prompts / errors per file | per-file keep/overwrite choice, same as bootstrap |
| Push rejected (non-fast-forward) | step 2 push error shown | TUI offers pull-rebase-retry once; otherwise hand-off shell |
| Tool download/build fails | per-tool failure list in step 3 | continue others; failures remain visible in status |
| Self-update swap fails (permissions) | step 4 error with the path | instructs re-run of installer one-liner |
| Interrupted mid-update | next status re-detects remaining drift | all steps are idempotent against a fresh status report; no saved state needed |

## Related

- [Check machine status](check-machine-status.md) — produces the drift report this workflow consumes
- [Bootstrap a new machine](bootstrap-new-machine.md) — first-time path; update is the every-other-time path
- CLAUDE.md "settled design points" — bidirectional sync definition; profiles share by default
