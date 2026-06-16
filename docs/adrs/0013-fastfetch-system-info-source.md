---
title: ADR-0013 — fastfetch as the system-information data source
status: accepted
created: 2026-06-16
updated: 2026-06-16
tags: [sysinfo, fastfetch, mise, tui, json]
supersedes: null
superseded-by: null
---

# ADR-0013: fastfetch as the system-information data source

## Context

myplace should be able to answer "what *is* this machine?" — OS and version, plus the base hardware specs (host model, CPU, GPU, memory, disk) and a couple of fleet-relevant extras (battery, local IP). This is the kind of at-a-glance fact a `status` dashboard wants in its header and that the future phase-2 server will want to collect per host.

The fleet is mixed: personal Macs, a work Mac, and Linux servers. Gathering this natively means two very different code paths — `system_profiler` / `sysctl` / IOKit on macOS, `/proc`, `/sys`, `lsblk`, `lspci` on Linux — each with its own parsing and quirks, all of which we'd own and keep working across OS versions. That's a lot of surface for what is fundamentally a read-only display.

The same constraints as the rest of the tool apply: orchestrate existing tools rather than reimplement them (the project rule for chezmoi/mise), keep the logic in a TUI-free package behind the `run.Runner` choke point, and ship a headless `--json` path from day one (ADR-0006).

## Options considered

### Option A — Native per-OS collection in Go

Read `system_profiler`/`sysctl` on macOS and `/proc`+`/sys` on Linux ourselves (or via a Go sysinfo library). No extra runtime dependency. But it's the most code to write and maintain, the cross-platform field coverage is uneven, and it duplicates exactly what mature tools already do — against the project's "orchestrate, don't reimplement" grain.

### Option B — fastfetch with JSON output (chosen)

[fastfetch](https://github.com/fastfetch-cli/fastfetch) is a fast, actively-maintained C system-info tool that runs on macOS and Linux and emits structured `--format json`: an array of `{type, result}` modules (`OS`, `Host`, `Kernel`, `CPU`, `GPU`, `Memory`, `Disk`, `Battery`, `LocalIp`, …). It is **already in the mise registry** (`aqua:fastfetch-cli/fastfetch`), so it installs the same way as our other registry tools (ADR-0007) on every machine — no new provisioning mechanism. We shell out through `run.Runner` and parse the JSON, picking only the curated fields we display.

### Option C — neofetch / screenfetch

The familiar predecessors, but neofetch is archived/unmaintained and neither offers first-class structured JSON — we'd be scraping human-formatted text. fastfetch is the maintained successor with JSON built in.

## Decision

Option B. A new TUI-free `internal/sysinfo` package runs `fastfetch --format json` via the runner and parses the modules into a curated `Info` struct; the TUI renders a compact header band from it and `myplace sysinfo [--json]` exposes it headlessly. fastfetch is added to the mise baseline (`home/dot_config/mise/config.toml.tmpl`) so it's present on every managed machine.

fastfetch is an **optional read dependency**, treated like brew in `outdated` (ADR-0008/0010): if it isn't on PATH, the command fails fast with exit `3` naming the dependency, and the TUI band degrades to a one-line "unavailable" note rather than crashing. We never *require* it for bootstrap and never mutate anything with it.

## Consequences

- One small cross-platform code path instead of two native ones; field coverage and OS-version resilience are fastfetch's job, not ours.
- A new tool in the mise baseline. Because it's a registry tool, that's a one-line config addition installed by the normal `update` loop — not a new provisioning path (contrast ADR-0008's brew fallback).
- We depend on fastfetch's JSON module names/shape. We decode defensively: unknown modules are ignored and absent ones (a server with no `Battery`/`GPU`/`Display`) decode to zero values, never errors. A future fastfetch schema change is contained to `internal/sysinfo`.
- Informational only, like `outdated`: `sysinfo` has its own exit-code contract (`0`/`3`) and never touches the drift verdict.
- Phase-2 server reporting can collect the same `sysinfo --json` envelope per host with no extra work.

## Related

- [ADR-0007](0007-provisioning-mechanism.md) — fastfetch is a registry tool, installed via the mise config
- [ADR-0006](0006-agent-runnable-commands.md) — the headless `sysinfo --json` / fail-fast contract
- [ADR-0010](0010-cross-package-manager-outdated-inventory.md) — the informational, read-if-present, separate-from-drift pattern this mirrors
- [System information feature](../features/system-information.md) — what it surfaces and the JSON schema
- [managed-setup guide](../guides/managed-setup.md) — adding a registry tool to the mise baseline
