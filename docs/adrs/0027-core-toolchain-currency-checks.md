---
title: ADR-0027 — Core toolchain currency checks (mise, chezmoi, rustup/rust, go, fnm)
status: accepted
created: 2026-08-31
updated: 2026-10-06
tags: [outdated, toolchain, mise, chezmoi, rustup, rust, go, fnm, doctor, cli, json]
supersedes: null
superseded-by: null
---

# ADR-0027: Core toolchain currency checks (mise, chezmoi, rustup/rust, go, fnm)

## Context

myplace watched every layer of a machine's software *except the layer it stands on*.

The gap surfaced as a support incident on 2026-08-31: `mise upgrade` began failing to install
`herdr` with a Sigstore error (`TSA timestamp verification failed: no certificate matches issuer
and serial number`). The artifact was fine — the machine was running mise `2026.6.10` (2026-06-14)
and hit a client-side attestation-verification bug fixed upstream weeks earlier
([jdx/mise#10680](https://github.com/jdx/mise/discussions/10680)). Updating mise to `2026.8.15`
resolved it outright. The tool had been two months stale, and **nothing in myplace said so**:

- **`outdated`'s mise source** runs `mise outdated`, which lists the tools *mise manages*. mise is
  not one of them, so it is structurally invisible there. Same for chezmoi.
- **`doctor`** does check mise's version, but against a **floor** (`miseFloor = "2024.1.0"`).
  `2026.6.10` clears that by two years, so the check passed green.
- **The outdated-binary check** in [`internal/drift`](../../internal/drift/drift.go) tracks
  **myplace's own** release only (`release.Repo` is this repo), not its dependencies.
- **The `cargo` source** covers binaries installed by `cargo install`, not rustup or the Rust
  toolchain itself.

mise *was* reporting it — `mise WARN mise version 2026.8.15 available`, on stderr, on every single
invocation. [`run.Runner`](../../internal/run/run.go) logs stderr at DEBUG for successful commands,
so it landed in `myplace.log` and nowhere a human looks.

The tools whose upgrade path is "self-update, and nothing else watches them" are exactly the tools
the rest of the setup depends on. They need a currency check.

## Options considered

### Option A — Raise doctor's version floors and bump them per release

Cheapest change: `doctor` already reads mise's and chezmoi's versions, so move the floors up.
Rejected on two counts. It conflates fitness with currency: `doctor` is a **preflight gate** whose
`fail` means exit 1, and "a newer mise exists upstream" must never fail a preflight — it isn't a
reason not to run. And a hardcoded floor is a treadmill: it goes stale exactly as silently as the
tool did, and every fleet machine's verdict then depends on when myplace was last released.

### Option B — Surface mise's own stderr warning

mise (and chezmoi) already self-report a newer version on stderr. Promote that text out of the
DEBUG log into the dashboard. No network calls of our own and no new upstream coupling — but it
means scraping human-facing warning strings whose format is not a contract, it only covers tools
that happen to warn (rustup and go do not), and it inherits each tool's own check cadence and
cache. Rejected as too fragile for a fleet signal.

### Option C — A `toolchain` source in the existing outdated inventory (chosen)

[ADR-0010](0010-cross-package-manager-outdated-inventory.md) already built the right shape: a
pluggable `Source` interface (`Name`/`Available`/`Outdated`), an informational envelope that never
touches the drift verdict, and graceful per-source degradation. A core-toolchain check is one more
adapter. It reuses the `outdated` command, its `--json` envelope, its exit contract, and its TUI
pane and detail view, and it inherits the read-only guarantee for free.

## Decision

Option C. A new `internal/toolchain` package, adapted into an `outdated.Source` named
`toolchain`, reporting these members:

| Member | Current version from | Latest version from |
|---|---|---|
| `mise` | `mise version` | GitHub `jdx/mise` releases/latest |
| `chezmoi` | `chezmoi --version` | GitHub `twpayne/chezmoi` releases/latest |
| `rustup` | `rustup check` | `rustup check` (no network of ours) |
| `rust` (stable toolchain) | `rustup check` | `rustup check` |
| `go` | `go version` | `https://go.dev/VERSION?m=text` |
| `fnm` | `fnm --version` | GitHub `Schniz/fnm` releases/latest |

Rules that go with it:

- **`doctor` keeps its floors, unchanged.** The split is deliberate and load-bearing:
  **`doctor` answers "is this tool new enough to drive?"** (a gate — fail means stop);
  **`outdated` answers "is this tool current?"** (inventory — never a gate). ADR-0010's
  informational stance is what makes it safe to report a stale mise without turning every machine
  red.
- **Read-only, always.** This source never runs `mise self-update`, `rustup update`, `chezmoi
  upgrade`, or any installer. Upgrading the toolchain stays a human-initiated step, consistent with
  ADR-0010's stance on brew and cargo.
- **Present-if-installed, per member.** A member whose binary isn't on PATH is simply omitted (a
  server with no Rust reports no `rustup`/`rust` rows). The source as a whole is `Available` when at
  least one member resolves — mise and chezmoi always do on a myplace-managed box.
- **Drop rows where current == latest.** Not cosmetic: `rustup check` on the primary Mac emits
  `stable-aarch64-apple-darwin - update available: 1.98.0 (88d9e12ae 2026-08-18) -> 1.98.0
  (88d9e12ae 2026-08-18)` — an "update" to the identical version. Reported verbatim that would be a
  permanent phantom "rust is outdated" row on the dashboard.
- **Never double-report what another source owns.** `fnm` is skipped when its resolved binary lives
  under the Homebrew prefix, because the `brew` source already reports it there. (The provision
  script only installs fnm to `~/.local/bin` when it isn't already on PATH, so provenance genuinely
  varies per machine — on the primary Mac it is brew's.)
- **Network posture.** Four HTTP lookups (three GitHub + go.dev), run concurrently, sharing the
  caller's context. Unauthenticated GitHub allows 60 requests/hour per IP; `GH_TOKEN`/`GITHUB_TOKEN`
  is used when set. A failed or offline lookup lands in this source's `Error` and leaves every other
  source intact (ADR-0010).
- **Partial results report both halves.** When some members compare cleanly and another's lookup
  fails, the source returns the rows it has *and* an error naming what it couldn't check.
  `outdated.Collect` was widened to record both fields, and `ExitCode` counts such a source's rows.
  This is the one place this ADR changes shared code, and it's the direct lesson of the incident:
  dropping the rows would hide real staleness, and dropping the error would make a partial answer
  look complete. Both halves already had a field in the JSON envelope; only `Collect`'s either/or
  handling had to go.
- **`go` is compared as resolved in the invocation's environment**, which is cwd-dependent under
  mise. On a machine where Go is only repo-scoped, a `go` row appears when run from that repo and
  none from `$HOME`. Accepted: "the Go you'd actually get here" is the useful answer, and pinning the
  probe to `$HOME` would mean never checking Go on this fleet.

### Deliberately out of scope

- **node.** It is fnm's or brew's depending on the machine, and *which* Node version a box should
  run is a per-project decision, not machine staleness. brew reports it where brew owns it.
- **oh-my-zsh.** A git checkout, not a released, versioned artifact — "14 commits behind upstream"
  is a different question with a different answer shape.
- **myplace itself.** Already a first-class field in the `status`/drift report
  (`drift.Report.Myplace.Latest`) and the dashboard's binary-update pane. Repeating it in the
  outdated pane would show one fact twice on one screen and spend a second API call to do it.
- **Repo-scoped mise tools.** myplace queries mise from `$HOME`, so tools pinned by a *project's*
  `mise.toml` (this repo's root config pins `go` for development) do not appear in the machine-wide
  inventory. That is correct: the machine-wide inventory answers a machine-wide question. `go` is
  covered here because a `go` binary on PATH is a core toolchain member however it was installed.

## Consequences

- The blind spot that produced the herdr incident closes: a stale mise, chezmoi, rustup, Rust, Go,
  or fnm now shows up in `myplace outdated`, `myplace outdated --json`, and the dashboard pane —
  informationally, without ever flipping a machine's drift verdict.
- `outdated`'s network dependency grows from one source (cargo, crates.io) to two. Both are already
  isolated per-source, and `outdated` was never offline-clean, so the posture is unchanged in kind.
- **Follow-up: cache latest-version lookups in the state dir.** [ADR-0005](0005-machine-local-state-directory.md)
  anticipated caches under `$XDG_STATE_HOME/myplace` ("logs now, caches later"). With four more
  calls per run, an unauthenticated machine gets roughly a dozen dashboard opens per hour before
  GitHub throttles it. A short-TTL cache is the fix; deferred rather than built blind, because the
  right TTL is easier to pick once the pane has been in use.
- Adding a future core member (a new runtime manager) is one entry in the `toolchain` member table,
  not a new source.
- A second, unrelated gap this incident exposed and this ADR does **not** address: `mise upgrade`
  exited non-zero inside a TUI run and the failure reached only the log. Converge-step failures
  being quiet is a `update`/TUI concern, tracked separately.

## Related

- [ADR-0010](0010-cross-package-manager-outdated-inventory.md) — the informational inventory and
  `Source` interface this extends; its read-only and never-affects-drift rules apply unchanged
- [ADR-0007](0007-provisioning-mechanism.md) — why Rust is rustup's and Node is fnm's, i.e. why
  these tools are outside mise's reach in the first place
- [ADR-0005](0005-machine-local-state-directory.md) — the state dir the deferred cache would use
- [ADR-0006](0006-agent-runnable-commands.md) — the `--json` contract this inherits via `outdated`
- [Outdated packages feature](../features/outdated-packages.md) — the user-facing spec
- [Doctor preflight diagnostics](../features/doctor-preflight-diagnostics.md) — the version *floors*
  this ADR deliberately leaves alone
