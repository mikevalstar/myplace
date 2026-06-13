---
title: Update a machine
status: active
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

The update screen presents drift grouped into the four buckets below; the user can run them all ("update everything") or drill into each. Order matters — **capture outgoing edits before applying anything**, because `chezmoi apply` overwrites locally-modified managed files; capturing first turns a silent clobber into an ordinary git merge that step 2 surfaces properly.

1. **Capture outgoing dotfiles** (interactive only — never in headless/cron runs). For each locally-modified managed file: show the diff, then per file choose:
   - **keep & share** → `chezmoi re-add <file>` (machine wins, repo updated)
   - **discard** → `chezmoi apply --force <file>` (repo wins, local edit overwritten)
   - **skip** → decide later (the file stays visible as drift)
   Then commit and push captured changes: `chezmoi git -- add -A`, `chezmoi git -- commit` (message prompt with sensible default), `chezmoi git -- push`.

   **Known limitation — templated files:** `chezmoi re-add` cannot reverse a rendered file back into a `.tmpl` source, so for a templated managed file (e.g. the mise config) it silently leaves the template unchanged and the edit is *not* captured. The capture flow detects this (the file is still modified after re-add) and tells the user to edit the source template directly (`chezmoi edit <file>`) and commit in the source repo. Doing so is the correct way to change a templated dotfile and preserves its conditionals. Smarter handling (offering `chezmoi edit`, or diffing into the template) is future work.
2. **Pull incoming dotfiles.** `chezmoi update` (pull --rebase + apply; rebase keeps just-captured local commits on top). Before applying, the step checks for managed files that still have local edits (any the user skipped, or — in headless runs — all of them, since capture never runs there). If any remain, the apply is **skipped, not attempted**: a bare `chezmoi apply` would prompt to overwrite them and, with no TTY, abort with a cryptic error. Instead the step reports `not applied — local edits to <files>; <how to resolve>` and the rest of the update (tools) proceeds. The files stay as drift for an interactive `myplace update` to keep or discard.
3. **Update tools.** `mise install` (anything missing), then `mise upgrade` for tools outdated against the just-updated config. `mise up --bump`-style config-version bumps are a deliberate, separate action — updating a machine should converge it on the shared config, not mutate the shared config as a side effect.
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
