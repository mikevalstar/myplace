---
title: Outdated packages (cross–package-manager inventory)
status: accepted
created: 2026-06-13
updated: 2026-10-06
tags: [outdated, packages, mise, homebrew, shelly, cachyos, pacman, arch, omarchy, skills, ai, cargo, rust, toolchain, chezmoi, go, cli, json, tui]
phase: 1
---

# Outdated packages (cross–package-manager inventory)

## Summary

`myplace outdated` lists packages with a newer version available, grouped by package manager — **mise** today, **Homebrew** when it's present, **Shelly** on CachyOS, **pacman** on Arch-family boxes (repos via `checkupdates`, AUR via `yay`), **skills** (AI agent skills via the skills.sh CLI) when that CLI is installed, **cargo** (binaries installed with `cargo install`, via cargo-update) wherever rustup is present, **toolchain** (the core tools the setup itself stands on — mise, chezmoi, rustup, Rust, Go, fnm), and more managers as they're added. It's available headlessly (`--json`) and as a TUI view (a summary pane on the dashboard plus a scrollable detail screen). It is **informational and read-only**: it reports what's upgradable — including packages myplace doesn't manage — and never changes the machine.

## Motivation

The machine has software from several sources: mise (dev tools/runtimes), Homebrew (CLI formulae + casks the owner installed by hand), and eventually apt/npm/cargo. `status` already flags outdated *mise* tools as drift because `update` can fix them — but it deliberately says nothing about the dozens of brew formulae behind their latest version, because myplace never upgrades those ([ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md)). There was no single "what's upgradable on this box?" view. This adds one, without polluting the drift verdict (see [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md)).

A later gap closed the same way: the tools myplace is *built on* were watched by nobody. mise, chezmoi, rustup, Rust, Go, and fnm are each self-updating and appear in no other source's list — `mise outdated` reports the tools mise *manages*, not mise. A two-month-stale mise silently broke a tool install on 2026-08-31 (a since-fixed upstream attestation bug) while `doctor` read green, because doctor checks version *floors*, not currency. The `toolchain` source is that check — see [ADR-0027](../adrs/0027-core-toolchain-currency-checks.md).

## Scope

### In scope

- A cross-manager inventory of outdated packages, grouped by source.
- Sources: **mise** (reuses `mise outdated`), **Homebrew** (`brew outdated --json=v2`, formulae + casks), **Shelly** (Arch/CachyOS — `shelly check-updates -a -l --json`, aggregating native Arch repos + AUR + Flatpak into one row; the `-a`/`-l` flags opt the AUR and Flatpak channels into the otherwise repo-only check, and secondary-channel packages are name-prefixed, e.g. `flatpak:org.gimp.GIMP`. AppImage has no `check-updates` flag, so it's out of scope), **pacman** (Arch-family boxes such as Omarchy — `checkupdates --nocolor` from pacman-contrib for the sync repos, plus `yay -Qua` for the AUR when yay is installed, AUR rows name-prefixed `aur:`; read-only — `checkupdates` only refreshes a *temporary* copy of the sync database, and `pacman -Syu` is never run: on Omarchy that's `omarchy update`'s job — [ADR-0026](../adrs/0026-omarchy-as-os-variant.md)), and **skills** (third-party AI agent skills — parses `skills check -g`, the skills.sh / `npx skills` CLI). **cargo** (Rust binaries installed with `cargo install` — parses `cargo install-update --list`, the [cargo-update](https://github.com/nabijaczleweli/cargo-update) subcommand, which myplace installs on every machine as part of the managed setup). **toolchain** (the core self-updating tools nothing else watches: `mise`, `chezmoi`, `rustup` + the Rust stable toolchain, `go`, and `fnm`; current versions come from each binary, latest from GitHub releases / `rustup check` / go.dev — [ADR-0027](../adrs/0027-core-toolchain-currency-checks.md)). brew, shelly, pacman, skills, and cargo are each included only when their CLI resolves (so shelly shows up on CachyOS, pacman wherever `checkupdates` is installed, brew on Macs, skills wherever the `skills` CLI or `npx skills` is available, cargo wherever rustup + cargo-update are installed). A user's *own* authored skills are chezmoi-managed dotfiles and don't appear here — only the third-party "stable" installed via the CLI ([ADR-0023](../adrs/0023-managing-ai-skills.md)).
- Packages myplace does **not** manage (most brew formulae) are shown — this is inventory, not just managed drift.
- Headless `myplace outdated --json` and a TUI view + a dashboard summary pane.
- A pluggable source interface so new managers (apt/dnf, npm, pipx, cargo) are one adapter each.

### Out of scope

- **Upgrading anything.** This feature only reads. brew in particular is never upgraded (ADR-0008/0009). Converging *mise* tools to their pinned versions remains `myplace update`'s job.
- **Affecting the `status`/drift verdict or its exit codes.** Outdated inventory is informational; the status verdict stays mise-only. See [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md).
- **Running `brew update`** to refresh brew's view (mutating/slow). Freshness reflects the user's last `brew update`.
- **Running `pacman -Syu`** (or `yay -Syu`) — the pacman source only reads; system upgrades stay with the distro's own flow (`omarchy update` on Omarchy). A CachyOS box that has both shelly and `checkupdates` shows both rows; they overlap, which is accepted (both informational).
- **Running `shelly sync`/`update`/`upgrade`** — same read-only stance as brew: `outdated` only reads Shelly's local update list (`check-updates`), never refreshes its databases or installs anything. Freshness reflects the user's last sync.
- **Non-mac Homebrew install** — brew is read-if-present, never installed here.
- **Running `cargo install-update -a`** (or any `cargo install`) — same read-only stance: myplace inventories cargo-installed binaries, it never rebuilds them. Upgrading them stays a manual `cargo install-update -a`.
- **Git-sourced cargo packages** — `--list` without `-g` covers registry (crates.io) packages only; `-g` clones every git-origin package's repo to compare, which is too expensive for a dashboard pane.
- **Cargo *project* dependencies** (`cargo update --dry-run`, `cargo outdated` in a crate) — this inventory is about the machine's installed software, not any one repo's `Cargo.lock`.
- **Upgrading the toolchain.** The `toolchain` source never runs `mise self-update`, `rustup update`, or `chezmoi upgrade` — same read-only stance as brew and cargo. It reports; the human upgrades.
- **node, oh-my-zsh, and myplace itself** are deliberately *not* toolchain members: Node is brew's or fnm's per machine and its version is a per-project call; oh-my-zsh is a git checkout, not a released artifact; and myplace's own binary is already a first-class field in the `status` report and the dashboard's binary-update pane. Reasoning in [ADR-0027](../adrs/0027-core-toolchain-currency-checks.md).
- **Repo-scoped mise tools.** myplace queries mise from `$HOME`, so a *project's* `mise.toml` pins (this repo's root config pins `go` for development) don't appear — the inventory answers a machine-wide question. `go` is covered by the `toolchain` source instead, from whatever `go` is on PATH.

## Behavior

### Command

`myplace outdated` prints a per-source summary in plain text; `myplace outdated --json` emits one JSON document (logs/progress to stderr, per the [headless contract](headless-cli-and-json-output.md)). The command never prompts and never mutates, so it's fully agent-runnable off a TTY.

Each source is queried independently and degrades gracefully: a source that isn't installed is reported as unavailable (not an error); a source that errors captures its message and doesn't stop the others.

### Exit codes

Distinct from the drift codes — this is its own contract:

| Code | Meaning |
|------|---------|
| 0 | all current — every available source produced a result, nothing outdated |
| 1 | updates available — at least one source reports ≥1 outdated package |
| 3 | error — no source could produce a result (e.g. none installed, or all errored) |

So `myplace outdated --json; echo $?` tells an agent "is anything upgradable here?" in `$?` before parsing the body. (There's no `2`/unknown: a partial failure where some source still produced a result resolves to `0`/`1` with the failure captured per-source in the JSON.)

### JSON envelope

```json
{
  "schema": 1,
  "machine": "hostname",
  "checked_at": "2026-06-13T20:00:00Z",
  "sources": [
    {
      "name": "mise",
      "available": true,
      "packages": [
        { "name": "node", "current": "22.1.0", "latest": "22.3.0" }
      ]
    },
    {
      "name": "brew",
      "available": true,
      "packages": [
        { "name": "htop", "current": "3.5.0", "latest": "3.5.1" },
        { "name": "gnupg", "current": "2.5.19", "latest": "2.5.20" }
      ]
    },
    {
      "name": "toolchain",
      "available": true,
      "packages": [
        { "name": "chezmoi", "current": "2.70.5", "latest": "2.72.1" },
        { "name": "go", "current": "1.26.4", "latest": "1.27.0" }
      ]
    }
  ]
}
```

- `schema` — bumped only on breaking changes (mirrors the drift envelope).
- `sources[]` — one entry per source, in a stable display order (mise, then brew, then shelly, then pacman, then skills, then cargo, then toolchain, then future managers).
- `sources[].available` — `false` when the manager isn't on PATH; its `packages` is then `[]`. In practice brew is `available` only on Macs, shelly only on CachyOS, pacman only where `checkupdates` is installed (Omarchy ships it), skills only where the skills.sh CLI resolves, and cargo only where rustup + cargo-update are installed. **toolchain** is available on any myplace-managed box (mise and chezmoi are always there); individual members are omitted rather than reported when their binary isn't on PATH, so a server with no Rust simply has no `rustup`/`rust` rows.
- `sources[].error` — present (string) only when that source was available but failed; other sources are unaffected. A source may set **both** `error` and a non-empty `packages`: a **partial result**, which the `toolchain` source produces when some members compared cleanly and another's upstream lookup failed. Both halves are kept deliberately — dropping the rows would hide real staleness, and dropping the error would make a partial answer look complete, which is the failure mode [ADR-0027](../adrs/0027-core-toolchain-currency-checks.md) exists to prevent. Such a source's rows still count toward exit `1`; an error with *no* rows is still "no result" (exit `3` if nothing else produced one).
- `packages[].current` / `latest` — installed version and the newer one offered. For mise, `latest` is the version mise would converge to; for brew it's `current_version` from `brew outdated`; for shelly it's the new version from `shelly check-updates`; for pacman they're the `old -> new` pair `checkupdates`/`yay -Qua` print per line; for skills they're the current/latest refs `skills check` prints (git-based, so possibly a commit ref rather than semver, and either may be empty when the CLI prints no delta); for cargo they're the `Installed`/`Latest` columns of `cargo install-update --list`, with that tool's leading `v` trimmed so versions read the same across sources (a git-origin package's commit ref is passed through as-is).
- For **shelly**, `packages[].name` from a secondary channel is prefixed with that channel (`aur:`, `flatpak:`) so the single row stays legible; native repo packages keep their bare name. Flatpak entries carry no installed version, so their `current` is empty (the row still signals an update is available).
- **skills is a text-parsed source, not JSON.** Unlike the others, `skills check` emits ANSI-colored *human* output and ignores `--json` (verified against skills v1.5.14 — only `skills list --json` is structured, and it carries no version info). So `internal/skills` strips ANSI and parses `check -g`'s text, the way `internal/chezmoi` line-parses `chezmoi status`. The all-clear ("✓ All global skills are up to date" → `packages: []`) and unavailable cases are verified; the *has-updates* line shape was not observable on an up-to-date machine, so that parse is best-effort and tolerant, isolated to `ParseCheck` for a one-line fix once a real outdated skill is seen (see the acceptance criteria).
- **pacman is text-parsed and uses exit codes as status.** `checkupdates` exits 2 (and `yay -Qua` exits 1) with no output when nothing is outdated; `internal/pacman` treats that silent status exit as an empty result and anything else non-zero (or anything on stderr) as that source's error. The AUR half is best-effort — a missing or failing yay never hides the repo result. Both halves need the network (a temporary database sync; the AUR RPC).
- **cargo is also text-parsed, and — with pacman — one of the sources that needs the network.** cargo-update has no JSON mode, so `internal/cargo` parses its fixed-width table (`Package / Installed / Latest / Needs update`, split on runs of 2+ spaces) and keeps only the `Yes` rows; the `Polling registry '…'` preamble and progress dots are skipped. It polls crates.io on every run — a couple of seconds, and it fails when offline. That's deliberately not special-cased: it shares the caller's context like every other source, and a timeout or an offline box just lands in that source's `error` field, leaving the others intact.
- **toolchain is networked too**, and the only one that mixes probes: `mise`/`chezmoi`/`fnm` compare a locally-reported version against GitHub `releases/latest` (`jdx/mise`, `twpayne/chezmoi`, `Schniz/fnm`); `go` compares `go version` against `https://go.dev/VERSION?m=text`; `rustup` and `rust` come from a single `rustup check`, which reports both sides itself and costs us no HTTP. The four lookups run concurrently under short timeouts and honor `GH_TOKEN`/`GITHUB_TOKEN` when set (unauthenticated GitHub allows 60 requests/hour per IP — a state-dir cache is the tracked follow-up in [ADR-0027](../adrs/0027-core-toolchain-currency-checks.md)).
- **`go` reflects the `go` resolvable in the invocation's environment**, which is cwd-dependent under mise: run inside a repo whose `mise.toml` pins Go, that's the version compared. On a machine where Go is only ever repo-scoped (as on the primary Mac, where the root `mise.toml` pins it for developing myplace) a `go` row appears when run from the repo and no row at all from `$HOME`, where the shim doesn't resolve for a child process. Reporting "the Go you'd actually get here" is the useful answer; the alternative — pinning the probe to `$HOME` — would mean never checking Go on this fleet at all.
- **Rows where `current == latest` are dropped**, which matters concretely: `rustup check` can report `stable-aarch64-apple-darwin - update available: 1.98.0 (…) -> 1.98.0 (…)` — an "update" to the same version. Passed through verbatim that would be a permanent phantom "rust is outdated" row. `fnm` is additionally skipped when its binary resolves under the Homebrew prefix, since the `brew` source already reports it there (the provision script only installs fnm to `~/.local/bin` when it isn't already on PATH, so provenance varies per machine).

### TUI

- **Dashboard home** gains an **"Updates available"** pane next to Dotfiles and Tools, showing per-source counts (`mise: N`, `brew: M`, `shelly: K`, `pacman: P`, `skills: J`, `cargo: C`, `toolchain: T`, or `n/a` when a source is absent — so shelly reads `n/a` on Macs, and skills/cargo read `n/a` where their CLI isn't installed). The pane, its count chip, and the `o` detail table all iterate the inventory's `sources`, so a new source (like skills) appears with **no dashboard code change** — only a new adapter in the source slice. It carries a `press o for details` hint. It loads asynchronously alongside the status report; until it lands the pane shows `checking…`. It does **not** change the verdict badge.
- **`o`** opens a dedicated, scrollable outdated view (a `bubbles` viewport) rendering every outdated package as a bordered `lipgloss/table` (PACKAGE · CURRENT · LATEST · SOURCE). It carries a **count summary** (`N outdated across M sources`, and `X of N shown` when filtered), a **sort** toggle (`s` cycles by source / by name; by-source keeps the grouped layout, by-name flattens into one alphabetical list annotated with each package's source), and a **filter** (`/` focuses a text input for a case-insensitive substring match on the package name; `esc` clears it). `↑/↓`/`pgup`/`pgdn` scroll; `esc`/`q` returns to the dashboard; `ctrl+c` quits. Sort and filter are pure presentation over the already-collected inventory — no recompute, no extra command runs.

## Acceptance criteria

- [x] `myplace outdated --json | jq .` succeeds; exactly one document on stdout; contains `schema`, `machine`, `checked_at`, `sources[]`.
- [x] On a Mac with brew present, the `brew` source is `available: true` and lists outdated formulae and casks; on a machine without brew it's `available: false` with empty packages and is not an error.
- [x] Exit code is `1` when anything is outdated, `0` when nothing is, `3` when no source could be queried.
- [x] `myplace status --json` verdict and exit code are **unchanged** by brew/unmanaged packages being outdated (proves the informational separation).
- [x] `myplace help --json`/`--llm` lists `outdated` with its exit codes and this doc as its output schema.
- [x] Dashboard shows the "Updates available" pane with per-source counts; `o` opens a scrollable detail view; `esc` returns.
- [ ] The `o` view shows a count summary and supports `s` (sort by source / name) and `/` (filter by name); both are presentation-only and run no extra command.
- [x] Nothing in this feature ever installs or upgrades a package.
- [x] The `pacman` source reports `available: true` on an Omarchy box (verified against pacman-contrib 1.13.1 + yay, nothing outdated at the time: exit 2 / exit 1 handled as empty) and `available: false` where `checkupdates` isn't on PATH.
- [ ] Confirm the has-updates line shape of `checkupdates`/`yay -Qua` on a real box with pending updates (the parser follows the documented `name old -> new` format; fixtures in `internal/pacman`).
- [ ] The `toolchain` source reports a row for a deliberately stale core tool (verified on this Mac: `chezmoi` 2.70.5 → 2.72.1 and `go` 1.26.4 → 1.27.0), and no row for a current one (`fnm` 1.39.0, `rustup` 1.29.0).
- [ ] `rustup check`'s same-version "update available" line produces **no** row (the phantom-row regression test).
- [ ] The `toolchain` source never runs `mise self-update`, `rustup update`, or `chezmoi upgrade` — asserted against the fake runner's recorded command list.
- [ ] A member whose binary is absent is omitted, not reported as an error; the source stays `available` as long as one member resolves.
- [ ] With the network unreachable, `toolchain` lands an `error` and the other sources still produce results (exit stays `0`/`1`, not `3`).
- [ ] A partial toolchain result (one member compared, another's lookup failed) reports the rows **and** the error, and still exits `1`.
- [x] The `cargo` source reports `available: true` with the `Needs update: Yes` rows on a machine with cargo-update installed (verified against cargo-update 22.1.1 on this Mac — 3 outdated), and `available: false` where `cargo install-update` isn't on PATH.
- [ ] `cargo-update` is installed by the provision script on every machine (see [managed-setup](../guides/managed-setup.md)) and appears in [INVENTORY.md](../../INVENTORY.md).
- [x] The `skills` source reports `available: true, packages: []` on a machine whose global skills are current (verified end-to-end against skills v1.5.14 via `npx skills`), and `available: false` where the CLI can't be resolved.
- [ ] Confirm `skills check -g`'s **has-updates** output shape against a machine with an outdated skill, and tighten `ParseCheck` (+ fixtures) to match — currently a documented best-effort parse.
- [ ] Wire the managed setup: a `home/dot_claude/skills/` chezmoi tree for authored skills, and a profile-gated `skills add -g` source-list block in the provision script for the third-party stable (self-managed until [vercel-labs/skills#683](https://github.com/vercel-labs/skills/issues/683) enables a global lockfile restore — [ADR-0023](../adrs/0023-managing-ai-skills.md)).

## Open questions

- The dashboard runs `mise outdated` twice on open (once via `drift.Compute`, once via the inventory). Accepted for v1; a shared result is a future optimization. ([ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md) Consequences.)
- "All sources unavailable → exit 3" treats "couldn't check anything" as an error (consistent with status's `unknown`/error stance). Revisit if a real machine legitimately has neither mise nor brew.

## Related

- [ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md) — the decision: informational, pluggable, read-only, separate from drift
- [Headless CLI + JSON](headless-cli-and-json-output.md) — the envelope/stream/exit-code contract this follows
- [TUI dashboard layout](tui-dashboard.md) — the home pane + the `o` detail view
- [Review outdated packages workflow](../workflows/review-outdated-packages.md)
- [ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md) / [ADR-0009](../adrs/0009-homebrew-casks-macos.md) — why brew is read-only/informational here
- [ADR-0023](../adrs/0023-managing-ai-skills.md) — the skills source: own skills via chezmoi, third-party via the skills.sh CLI, surfaced here informationally
- [ADR-0007](../adrs/0007-provisioning-mechanism.md) — why cargo-update is a provision-script install rather than a mise tool (Rust and its cargo binaries are rustup's)
- [ADR-0027](../adrs/0027-core-toolchain-currency-checks.md) — the `toolchain` source: why currency belongs here and version *floors* stay in `doctor`
- [Doctor preflight diagnostics](doctor-preflight-diagnostics.md) — the floors that answer "new enough to drive?", the complement to this doc's "is it current?"
