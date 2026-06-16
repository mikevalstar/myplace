---
title: Preflight diagnostics (myplace doctor)
status: active
created: 2026-06-13
updated: 2026-06-16
tags: [cli, diagnostics, troubleshooting, headless]
phase: 1
---

# Preflight diagnostics (myplace doctor)

## Summary

`myplace doctor` inspects the local environment and reports whether the machine is ready to run myplace's other commands — chezmoi and mise present and a supported version, `~/.local/bin` on `PATH`, the dotfiles repo and GitHub API reachable, the state directory writable — and, for anything wrong, names the exact remedy.

## Motivation

The defining constraint of this project is the machine you'll next touch in three weeks (see the README's logging section). When `status` or `update` misbehaves there, the cause is almost always environmental, not a real drift: mise isn't on `PATH` after a shell change, the dotfiles remote moved, the box is offline, the GitHub API is rate-limiting `self-update`, or the state dir isn't writable so the debug log silently dropped. Today the user discovers these one failed command at a time, by reading a stack of unrelated errors. `doctor` front-loads all of it into one pass with one verdict, the way `brew doctor` and `git fsck` do — so the first thing you run on a flaky machine tells you *what* is wrong before you start guessing.

It also gives the headless story a clean preflight: a provisioning script can run `myplace doctor --json` and refuse to proceed (or page someone) before attempting a bootstrap that would fail halfway.

## Scope

### In scope

- A read-only `myplace doctor` command: it diagnoses, it never mutates.
- A fixed set of checks (below), each producing pass / warn / fail with a one-line remedy.
- Human-readable output by default; `--json` for machines, following the [headless contract](headless-cli-and-json-output.md).
- An aggregate verdict and exit code so a script can gate on it.

### Out of scope

- **Fixing** anything. `doctor` only reports; remedies are printed as commands/links the user runs themselves. An opt-in `--fix` could come later but is not specified here.
- Diagnosing *drift* (that's `status`) — `doctor` answers "can myplace run at all," not "is this machine in sync."
- Network reachability to a phase-2 reporting server (no server exists yet).

## Behavior

`myplace doctor` runs every check, prints a section per check with a status glyph, and ends with an overall verdict. It never prompts and never writes, so it is safe to run anywhere, including non-interactively.

### Checks

| Check | Pass condition | On failure, remedy names |
|-------|----------------|--------------------------|
| `chezmoi` installed | binary found on `PATH` | how bootstrap installs it / the install command |
| `chezmoi` version | ≥ the minimum myplace relies on | `chezmoi upgrade` |
| `mise` installed | binary found on `PATH` | the mise install command |
| `mise` version | ≥ minimum | `mise self-update` |
| `PATH` sane | `~/.local/bin` (myplace) and mise shims resolve | the shell line to add |
| chezmoi initialized | source state present, profile resolved | `myplace bootstrap` |
| dotfiles remote reachable | `git ls-remote` against the configured repo succeeds | check network / credentials; names the remote URL |
| GitHub API reachable | releases API responds (gates `self-update`) | offline, or rate-limited — retry / token hint |
| state dir writable | `$XDG_STATE_HOME/myplace` (or override) is writable | the resolved path and why logging would be silent |
| TTY | reports whether stdout is a TTY (informational, never a failure) | — |

Each check is independent: one failure does not abort the rest, so a single run surfaces *all* problems. Reachability checks that can't complete (offline) are `warn`/`unknown`, not `fail` — being offline isn't broken.

### Output

Default (human):

```console
$ myplace doctor
myplace doctor — host: web1, profile: server

  ✓ chezmoi installed         2.52.0
  ✓ mise installed            2026.5.1
  ✗ PATH                      ~/.local/bin not on PATH
      → add 'export PATH="$HOME/.local/bin:$PATH"' to your shell profile
  ✓ chezmoi initialized       profile: server
  ⚠ dotfiles remote           could not reach git@github.com:mikevalstar/myplace (offline?)
  ✓ GitHub API reachable
  ✓ state dir writable        ~/.local/state/myplace

verdict: problems found (1 failed, 1 warning)
```

`--json` (machine), one document on stdout per the headless contract:

```console
$ myplace doctor --json
{
  "schema": 1,
  "machine": "web1",
  "profile": "server",
  "checked_at": "2026-06-13T01:52:00Z",
  "verdict": "fail",
  "checks": [
    { "id": "chezmoi_installed", "status": "pass", "detail": "2.52.0" },
    { "id": "path", "status": "fail", "detail": "~/.local/bin not on PATH",
      "remedy": "add 'export PATH=\"$HOME/.local/bin:$PATH\"' to your shell profile" },
    { "id": "dotfiles_remote", "status": "warn", "detail": "unreachable (offline?)" }
  ]
}
```

### Exit codes

Aligned with the [headless contract](headless-cli-and-json-output.md), interpreted for diagnostics:

| Code | Meaning |
|------|---------|
| 0 | all checks passed (warnings allowed) |
| 1 | at least one check failed (machine is not ready) |
| 2 | checks could not be completed (e.g. fully offline and that's all we learned) |
| 3 | doctor itself errored before producing a report |

So a provisioning script can `myplace doctor --json || exit` before attempting work.

## Acceptance criteria

- [ ] `myplace doctor` runs all checks and prints one section per check plus an overall verdict, with no prompts and no filesystem mutation.
- [ ] One failing check does not prevent the others from running; a single invocation reports every problem found.
- [ ] Every `fail` includes a concrete remedy (command or config line) naming the specific value (path, URL) involved.
- [ ] `myplace doctor --json | jq .` emits exactly one document; logs go to stderr.
- [ ] Exit code is 1 when any check fails; 2 when nothing failed but a check could not complete (a warning); 0 only when every check passed.
- [ ] Reachability checks degrade to `warn`/exit 2 when offline rather than reporting `fail` — being offline isn't broken, so it never yields exit 1.
- [ ] Lives in the core packages with no TUI imports (ADR-0002); reuses the `run.Runner` choke point so every probe is logged.

## Resolved

- **Version floors:** pinned explicit, low floors recorded next to the check in `internal/doctor` (`chezmoi 2.0.0`, `mise 2024.1.0`) — the versions below which myplace's own invocations break, not a "stay current" nudge (that's `status`/`outdated`). Below floor is `fail` with the upgrade remedy; installed-but-unparseable degrades to `warn`, never a false `fail`.
- **Exit semantics:** `fail` present → exit 1; otherwise any `warn` (a check that couldn't complete, e.g. offline) → exit 2; all pass → exit 0. Matches the tool-wide 0/1/2 convention. A gating script that wants to proceed offline should branch on `verdict != "fail"` rather than `|| exit`, since exit 2 is "couldn't verify everything," not "broken."

## Open questions

- Should bootstrap call `doctor` automatically at the end as its self-verification step, replacing the ad-hoc final status report? (Would unify "did bootstrap actually work" with this command.)
- A future `--fix` for the safe, unambiguous remedies (PATH line, `chezmoi upgrade`) — opt-in only; tracked separately if pursued.

## Related

- [Headless CLI with JSON output](headless-cli-and-json-output.md) — the envelope, stream discipline, and exit codes this reuses
- [ADR-0006](../adrs/0006-agent-runnable-commands.md) — the non-interactive contract `doctor` must honor
- [ADR-0005](../adrs/0005-machine-local-state-directory.md) — the state dir the writability check probes
- [Bootstrap a new machine](../workflows/bootstrap-new-machine.md) — the install steps `doctor`'s remedies point back to
- [Logging](logging.md) — why a non-writable state dir fails silently, which this check surfaces
