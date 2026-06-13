---
title: Logging
status: accepted
created: 2026-06-12
updated: 2026-06-12
tags: [logging, debugging, observability]
phase: 1
---

# Logging

## Summary

myplace writes a structured, append-only log of everything it does — every chezmoi/mise command it shells out, with arguments, duration, and outcome, plus per-run lifecycle events — to a file in the machine-local state dir. It's a debugging black box for a tool that runs unattended on machines you rarely sit at.

## Motivation

When a bootstrap half-fails on a server you'll next touch in three weeks, or an unattended `update` cron job exits non-zero, the terminal output is long gone. A persistent log of the exact commands run and what they returned turns "it didn't work" into a diagnosable trace. It also future-proofs phase 2: the server can ingest these logs later.

## Scope

### In scope

- A per-machine log file capturing command invocations and run lifecycle.
- Location, format, level control, and size bounding.
- Preserving the `--json` stdout contract (logs never pollute stdout).

### Out of scope

- Shipping logs to a server (phase 2).
- Per-command log files or structured query tooling — one rolling file is enough for now.
- Logging secrets: myplace doesn't handle secret values, but log writers must keep it that way (never log file *contents*, only paths and metadata).

## Behavior

### Location

`<state-dir>/myplace.log`, where `<state-dir>` is resolved per [ADR-0005](../adrs/0005-machine-local-state-directory.md): `$MYPLACE_STATE_DIR`, else `$XDG_STATE_HOME/myplace`, else `~/.local/state/myplace`. Deliberately **not** under `~/.config` (chezmoi's tree).

### What's logged

- **Every external command** (the `run.Runner` choke point): tool, args, working dir, duration, success/failure, and a trimmed tail of stderr on failure. This is the high-value data — it's the literal sequence of chezmoi/mise calls.
- **Run lifecycle**: process start with the subcommand and argv, and exit. Each line is tagged with the subcommand and pid so interleaved runs are separable.
- **Notable outcomes**: status verdict, bootstrap/update step results, errors.

Format is `charmbracelet/log`'s structured key/value output with RFC3339 timestamps — greppable and human-readable.

### Level

Defaults to `debug` (command-level detail) so the log is a complete trace. Override with `MYPLACE_LOG_LEVEL` (`debug`|`info`|`warn`|`error`). Command invocations log at `debug`; lifecycle at `info`; failures at `error`.

### Size bounding

Single backup rotation: when `myplace.log` exceeds ~5 MB at startup, it's renamed to `myplace.log.1` (overwriting any previous backup) and a fresh file is started. No unbounded growth, no cron dependency, at most ~10 MB on disk.

### Robustness

Logging must never break a run. If the state dir can't be created or the file can't be opened, myplace falls back to a no-op logger and proceeds silently — a machine with a read-only `$HOME` still bootstraps.

### Relationship to `--json`

Logs go to the file (and the file only); stdout stays reserved for the single JSON document, stderr for human progress. Turning logging on does not change either stream — it makes the `--json` contract *easier* to keep, since debug detail has somewhere to go besides stderr.

## Acceptance criteria

- [ ] After any command, `<state-dir>/myplace.log` exists and contains one line per chezmoi/mise invocation with duration and outcome.
- [ ] `myplace status --json | jq .` still succeeds with logging enabled (stdout unpolluted).
- [ ] `MYPLACE_STATE_DIR=/tmp/x myplace status` writes to `/tmp/x/myplace.log`.
- [ ] A failed subprocess produces an `error`-level line with stderr context.
- [ ] An unwritable state dir does not fail the command.
- [ ] The log file is bounded (rotation verified past the threshold).

## Open questions

- Should `myplace` expose the log path (e.g. `myplace version` footer or a `myplace logs --path` helper) so users don't have to know the XDG rule? Leaning: print it on bootstrap completion and in `--help`, add a `logs` subcommand only if asked for.

## Related

- [ADR-0005](../adrs/0005-machine-local-state-directory.md) — where state lives
- [ADR-0002](../adrs/0002-go-and-charm-for-the-tui.md) — charmbracelet/log was chosen here
- [charm-tui-stack guide](../guides/charm-tui-stack.md) — "don't print to stdout while the TUI runs" gotcha
