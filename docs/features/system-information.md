---
title: System information (fastfetch-backed)
status: accepted
created: 2026-06-16
updated: 2026-06-16
tags: [sysinfo, fastfetch, hardware, os, json, tui]
phase: 1
---

# System information (fastfetch-backed)

## Summary

`myplace sysinfo` reports what the machine *is* — OS and version plus the base hardware specs (host model, CPU, GPU, memory, disk) and a couple of fleet-relevant extras (battery, local IP). It's available headlessly (`--json`) and as a compact **header band** on the TUI dashboard. It is **informational and read-only**: it shells out to [fastfetch](https://github.com/fastfetch-cli/fastfetch) ([ADR-0013](../adrs/0013-fastfetch-system-info-source.md)), parses its JSON, and changes nothing.

## Motivation

The dashboard tells you whether a machine is *in sync*, but not *which machine* you're looking at — and across a fleet of personal Macs, a work Mac, and Linux servers, "what is this box and what's in it" is a basic question. Bootstrapping a new machine and confirming "yes, this really is the M4 Pro with 24 GB" or "this server's root disk is nearly full" wants an at-a-glance specs view. fastfetch already computes all of this cross-platform; this feature curates the useful fields and surfaces them in the two places myplace already speaks — the TUI and `--json` — so phase-2 fleet reporting can collect them later.

## Scope

### In scope

- A curated system-info snapshot: **OS** (name, version, build), **Host** (model, vendor), **Kernel** (arch), **CPU** (model, cores), **GPU**, **Memory** (used/total/free), **Disk** (per mountpoint), **Swap** (used/total/free), **Battery**, **Network** (local IPv4), **load averages**, **Uptime**.
- Headless `myplace sysinfo` (plain text) and `myplace sysinfo --json`.
- A passive header band on the TUI dashboard (no new keybinding).
- Graceful degradation: absent fastfetch modules (e.g. a server with no battery/GPU) are simply omitted, not errors.

### Out of scope

- **Cosmetic / environment fields** — shell, terminal, terminal font, theme, icons, cursor, window manager, desktop environment, locale, wallpaper, colors. Deliberately excluded; this is about the machine, not the desktop.
- **Mutating anything.** Read-only, like `outdated`.
- **Affecting the drift verdict or its exit codes.** `sysinfo` has its own contract (see below) and never feeds `status`.
- **History / trend / fleet aggregation.** A point-in-time snapshot only; phase-2 builds on the `--json` envelope.

## Behavior

### Command

`myplace sysinfo` prints a readable multi-line block; `myplace sysinfo --json` emits one JSON document on stdout (logs to stderr, per the [headless contract](headless-cli-and-json-output.md)). It never prompts and never mutates, so it's fully agent-runnable off a TTY.

fastfetch is a read **dependency** installed via the mise baseline (ADR-0013/0007). If it isn't on PATH or fails, the command fails fast naming the dependency (ADR-0006). Load averages aren't a fastfetch module, so they're read separately from `uptime` (best-effort: if `uptime` fails, `load` is simply omitted).

### Exit codes

Its own contract, distinct from the drift codes:

| Code | Meaning |
|------|---------|
| 0 | success — system info collected and emitted |
| 3 | error — fastfetch is unavailable (not on PATH) or failed |

There is no `1`/`2`: this command makes no in-sync/outdated judgement, it just reports.

### JSON envelope

```json
{
  "schema": 1,
  "machine": "hostname",
  "checked_at": "2026-06-16T18:00:00Z",
  "os": { "name": "macOS", "pretty": "macOS Tahoe 26.5.1 (25F80)", "version": "26.5.1", "codename": "Tahoe", "build": "25F80", "id": "macos" },
  "host": { "name": "MacBook Pro (14-inch, 2024, Three Thunderbolt 5 ports)", "family": "Mac16,8", "vendor": "Apple Inc." },
  "kernel": { "name": "Darwin", "release": "25.5.0", "arch": "arm64" },
  "cpu": { "name": "Apple M4 Pro", "vendor": "Apple", "cores_physical": 12, "cores_logical": 12 },
  "gpus": [ { "name": "Apple M4 Pro", "type": "Integrated", "vendor": "Apple" } ],
  "memory": { "total_bytes": 25769803776, "used_bytes": 23112105984 },
  "swap": [ { "name": "Encrypted", "total_bytes": 11811160064, "used_bytes": 11044782080 } ],
  "disks": [ { "name": "Macintosh HD", "mountpoint": "/", "total_bytes": 494384795648, "used_bytes": 465891303424 } ],
  "battery": { "capacity": 80.0, "cycle_count": 92, "status": ["AC Connected"] },
  "network": [ { "interface": "en0", "ipv4": "192.168.1.20" } ],
  "load": [2.34, 2.10, 1.98],
  "uptime": { "boot_time": "2026-06-08T08:14:09-04:00" }
}
```

- `schema` — bumped only on breaking changes (mirrors the drift / outdated envelopes).
- `machine`, `checked_at` — host identity and capture time, consistent with the other envelopes.
- Absent fastfetch modules are omitted (a Linux server with no `battery`/`gpus` simply has those keys empty/null) — never an error.
- Byte fields are raw bytes; the text and TUI renderers humanize them (e.g. `24 GiB`).

### TUI

The dashboard gains a passive **system-info header band** between the title header and the panes — three compact lines, each truncated to the terminal width, drawn from the same `sysinfo` snapshot (loaded asynchronously alongside the status report; shows `system: loading…` until it lands):

1. **Identity** — OS pretty name (leading, so it survives truncation) · host model · kernel arch
2. **Compute** — CPU · cores · GPU · memory (used/total/free) · root-disk used/total
3. **Runtime** — load averages · swap (used/total/free) · battery (% · cycles · status) · local IPv4 · uptime

It is informational and has no keybinding; it does not change the verdict badge. If fastfetch is unavailable the band shows a single `system: fastfetch unavailable` line (padded so the layout never shifts). Narrow and small-terminal fallbacks keep a condensed line.

## Acceptance criteria

- [x] `myplace sysinfo --json | jq .` succeeds; exactly one document on stdout; contains `schema`, `machine`, `checked_at`, and the curated sections.
- [x] `myplace sysinfo` (no flag) prints a readable block including OS+version, host model, CPU, GPU, memory, and **disk** usage.
- [x] Cosmetic fields (shell, terminal, terminal font, theme, icons, locale) are **not** present in either output.
- [x] Absent modules degrade gracefully (a machine without a battery/GPU still succeeds, exit `0`).
- [x] With fastfetch not on PATH, the command exits `3` with a message naming fastfetch; `status`/drift are unaffected.
- [x] `myplace help --json`/`--llm` lists `sysinfo` with its exit codes and this doc as its output schema.
- [x] The dashboard shows the three-line system-info band; the layout stays intact while it loads, when fastfetch is unavailable, and across wide/narrow/small terminals (a regression test asserts it fills the height exactly).
- [x] fastfetch is in the mise baseline so a freshly-`update`d machine has it.

## Open questions

- Which disks to show in the band when there are several mounts — v1 shows root `/` (all disks remain in `--json`). Revisit if servers with many volumes want more.
- fastfetch's JSON module names are an external contract; decoding is defensive (unknown ignored, absent → zero). A schema change is contained to `internal/sysinfo`.

## Related

- [ADR-0013](../adrs/0013-fastfetch-system-info-source.md) — the decision to use fastfetch
- [ADR-0006](../adrs/0006-agent-runnable-commands.md) — the headless / fail-fast contract
- [ADR-0007](../adrs/0007-provisioning-mechanism.md) — fastfetch installed via the mise baseline
- [Headless CLI + JSON](headless-cli-and-json-output.md) — the envelope/stream/exit-code contract this follows
- [TUI dashboard layout](tui-dashboard.md) — where the header band lives
- [managed-setup guide](../guides/managed-setup.md) — adding the tool to the mise baseline
