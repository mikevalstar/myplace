---
title: ADR-0010 — Cross–package-manager outdated inventory (informational, not drift)
status: accepted
created: 2026-06-13
updated: 2026-06-13
tags: [outdated, packages, mise, homebrew, status, tui]
supersedes: null
superseded-by: null
---

# ADR-0010: Cross–package-manager outdated inventory (informational, not drift)

## Context

`myplace status` already reports outdated **mise** tools: `drift.Compute` calls `mise outdated` and a tool behind its config pin counts as drift (verdict `drifted`, exit 1), because `myplace update` can fix it by converging to the requested version. That framing is correct *for mise* — it's the one manager myplace fully owns.

We want a broader view: show outdated packages across *all* the package managers on a machine — starting with **Homebrew**, and built so apt/dnf, npm, pipx, cargo, etc. can be added as the fleet grows — and explicitly **including packages myplace does not manage**.

That last point collides with the drift model. [ADR-0008](0008-opportunistic-homebrew-macos.md) and [ADR-0009](0009-homebrew-casks-macos.md) are emphatic: myplace only ever runs `brew install <x>` for a tiny, intentional set (httpie, mosh, a few fonts) and **never** `brew upgrade`, never manages brew or its taps. So `brew outdated` is dominated by formulae the user installed by hand (20 are outdated on the primary Mac as of this writing — ca-certificates, gnupg, htop, …). None of those are drift myplace can fix.

If we folded `brew outdated` into the status verdict the way `mise outdated` is folded in, every machine would read `drifted` / exit 1 essentially forever, and `myplace update` would resolve none of it. That guts the status verdict as a CI/agent signal (the whole point of the headless contract in [ADR-0006](0006-agent-runnable-commands.md)). ADR-0008 (Consequences) already anticipated this seam: *"Future status/doctor work can check whether brew-provisioned tools are present… without changing this contract."* This ADR is that read-side work.

## Options considered

### Option A — Extend `drift`/`status` to include brew

Add brew alongside mise in `drift.Tools` and the status dashboard. Least new code, one command. But it conflates "inventory across managers (incl. unmanaged)" with "managed drift I can fix": the verdict goes permanently `drifted`, the `status` JSON schema gains fields it can't act on, and there's no clean headless answer to the narrower question "what's upgradable here?" Rejected — it breaks the meaning of the status verdict.

### Option B — TUI-only outdated view

Show the cross-manager list only in the dashboard. Violates the day-one rule that every capability is agent-runnable with `--json` ([ADR-0006](0006-agent-runnable-commands.md), the `--json` spec). Rejected.

### Option C — Separate informational inventory behind a pluggable source interface (chosen)

A new `internal/outdated` package defines a small `Source` interface (`Name`/`Available`/`Outdated`) and an `Inventory` envelope; adapters wrap the existing mise client and a new brew client. A new `myplace outdated` command (`--json`) and a dedicated TUI view render it, plus a summary pane on the dashboard home. It is **informational**: it never feeds the drift verdict or `status` exit codes, and it never mutates the machine (it only ever *reads* `mise outdated` / `brew outdated --json=v2`). It is the natural home for "expand to other managers later" — adding apt/npm/cargo is one new adapter.

## Decision

Option C. Specifically:

- **Separate from drift.** `internal/drift` is unchanged; the status verdict stays mise-only. Outdated inventory lives in `internal/outdated` and has its **own** exit contract: `0` = all current, `1` = updates available, `3` = error (no source could produce a result).
- **Pluggable `Source` interface.** `Name() string`, `Available(ctx) bool`, `Outdated(ctx) ([]Package, error)`. `Collect` queries each source and degrades gracefully — an unavailable source is recorded (not an error), a failing source captures its error and never aborts the others.
- **Read-only, always.** This feature never installs or upgrades anything. brew especially is never upgraded, consistent with ADR-0008/0009.
- **brew-if-present.** The brew source self-reports `Available() == false` when brew isn't on PATH, so it's silently skipped on Linux servers and brew-less Macs — same posture as the provisioning ADRs, applied to reads.
- **Layering preserved (ADR-0003).** `internal/mise` and the new `internal/brew` stay thin CLI wrappers with no knowledge of each other or of `internal/outdated`; the adapters that turn them into `Source`s live in `internal/outdated` (same import direction as `drift`→`mise`). Nothing here imports a TUI package.

## Consequences

- A clean conceptual split: **`status`/drift** answers "is the setup myplace manages in sync?" (and stays exit 0 when it is); **`outdated`** answers "what's upgradable on this box, across managers, including things I manage by hand?" mise outdated appears in both — as fixable drift in `status`, as one source among several in `outdated`.
- Adding a package manager is one adapter + one line in the source slice in `newRootCmd`; the command, JSON envelope, and TUI view absorb it automatically.
- The dashboard runs `mise outdated` **twice** on open — once inside `drift.Compute`, once inside `outdated.Collect`. Accepted for v1 (both are cheap-ish and run concurrently behind the spinner); a future optimization can share the result. Noted so it isn't mistaken for a bug.
- brew freshness depends on the user's last `brew update`: `brew outdated` reads the local formula DB and we deliberately don't run `brew update` (mutating, slow, and not ours to run). The inventory reflects what brew already knows — fast and offline-safe.
- Phase 2's reporting agent can upload the same `Inventory` envelope, exactly as it will the drift report.

## Related

- [ADR-0008](0008-opportunistic-homebrew-macos.md) — opportunistic Homebrew; its Consequences foreshadow this read-side work, and its "never `brew upgrade`" stance is why brew is informational here
- [ADR-0009](0009-homebrew-casks-macos.md) — brew casks; the inventory reads casks too (`brew outdated --json=v2`)
- [ADR-0006](0006-agent-runnable-commands.md) — every command is agent-runnable; `outdated --json` honors it
- [ADR-0003](0003-monorepo-app-dotfiles-mise.md) — the package layering the `Source` adapters respect
- [Outdated packages feature](../features/outdated-packages.md) — the user-facing spec and JSON envelope
- [Review outdated packages workflow](../workflows/review-outdated-packages.md)
