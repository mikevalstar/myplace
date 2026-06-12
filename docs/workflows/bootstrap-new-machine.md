---
title: Bootstrap a new machine
status: draft
created: 2026-06-12
updated: 2026-06-12
tags: [bootstrap, chezmoi, mise, wizard]
actors: [user, tui, chezmoi, mise]
---

# Bootstrap a new machine

## Goal

Take a brand-new Mac or Linux server from "nothing installed" to fully configured: dotfiles applied, tools installed, and reporting in-sync — in one guided session.

## Preconditions

- Network access.
- A shell and `curl` (present on stock macOS and virtually all server images).
- The dotfiles repo exists and is reachable (see failure modes for the SSH-key chicken-and-egg).
- `myplace` itself installed via the installer one-liner: `curl -fsSL <install-url> | sh` (static binary into `~/.local/bin`, no dependencies — this is why ADR-0002 requires a static binary).

Git is *not* a hard precondition: chezmoi bundles a built-in git (`--use-builtin-git on`) sufficient for cloning; real git arrives later via mise/system packages.

## Steps

1. **User runs `myplace`.** No chezmoi state is found (`chezmoi source-path` fails), so the TUI offers the bootstrap wizard instead of the dashboard. (`myplace bootstrap` jumps straight here.)
2. **Detect the environment.** OS, arch, hostname, and which of chezmoi/mise/git are already present and at what versions. Display as a checklist.
3. **Install missing prerequisites.** With user confirmation, install via the official installers into `~/.local/bin`:
   - chezmoi: `sh -c "$(curl -fsLS get.chezmoi.io)" -- -b ~/.local/bin`
   - mise: `curl https://mise.run | sh`
   - No Homebrew dependency: servers won't have it, and the wizard must work the same everywhere. Brew-managed apps belong to the dotfiles' own run-once scripts, not to myplace.
4. **Collect machine identity** (huh form): dotfiles repo URL, machine profile (`personal-mac` / `work-mac` / `server`), and any prompts the dotfiles templates need. Profile answers are written to chezmoi's data (`~/.config/chezmoi/chezmoi.toml`) so templates can branch on them — profiles share by default; differences are the exception.
5. **Apply dotfiles:** `chezmoi init --apply <repo-url>`. Run via `tea.ExecProcess` (it may prompt or invoke askpass). The TUI shows `chezmoi diff` output for review before the apply when running interactively.
6. **Install tools:** `mise trust` on the now-present global config, then `mise install`. Stream progress; tool count and failures surface in the UI.
7. **Run the dotfiles' own setup scripts** — chezmoi `run_once_` scripts fire during step 5 automatically; the TUI surfaces their output rather than hiding it.
8. **Verify:** run the [status workflow](check-machine-status.md) and show the resulting dashboard. Offer "register this machine" hook here in phase 2.

Branch: if chezmoi state already exists, the wizard short-circuits to the [update workflow](update-machine.md) with a "this machine is already set up" notice.

## Outcome

Dotfiles applied, tools installed at pinned versions, machine profile recorded in chezmoi data, and a status screen showing in-sync. The user's next shell has everything on PATH.

## Failure modes

| What can go wrong | How the user finds out | Recovery |
|-------------------|------------------------|----------|
| No network / installer URL unreachable | step fails immediately with the curl error | retry; wizard resumes at the failed step |
| Repo is SSH-only but no SSH keys exist yet (chicken-and-egg) | clone fails with auth error | wizard offers HTTPS clone instead; `chezmoi init` can re-point origin to SSH later once keys are applied |
| Private repo over HTTPS needs a token | git credential prompt | `tea.ExecProcess` hands the terminal over so the prompt works |
| `chezmoi apply` would overwrite existing local files | chezmoi reports conflict | show diff, let user choose keep/overwrite per file |
| A `mise install` target fails to build/download | per-tool failure list at end of step 6 | continue with the rest; failed tools listed in status as missing |
| Wizard interrupted partway | next `myplace` run re-detects state | every step is idempotent re-runnable; wizard resumes from detection, not a saved cursor |

## Related

- [Check machine status](check-machine-status.md) — the verification step
- [Update a machine](update-machine.md) — where already-bootstrapped machines are routed
- [ADR-0002](../adrs/0002-go-and-charm-for-the-tui.md) — static-binary constraint that shapes the installer
