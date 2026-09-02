---
title: Omarchy support plan
status: accepted
created: 2026-09-02
updated: 2026-09-02
tags: [omarchy, arch, linux, desktop, planning, review]
---

# Omarchy support plan

Findings from surveying the repo against a fresh Omarchy 4.0.2 install (this
laptop, `ID=omarchy`, `ID_LIKE=arch`). Reviewed 2026-09-02; the ticked items
and the "Clarified" notes are what was implemented — the decision record is
[ADR-0026](../adrs/0026-omarchy-as-os-variant.md).

**How to use this file:** tick `[x]` on the items you want done, leave `[ ]`
on the ones you don't, and write anything else in the `Comments` block under
each section. Items marked **(recommended)** are what I'd do by default.
Once reviewed, the ticked items become the work list; the ADR gets written
first (documentation-first), then the rest.

## Bottom line

Bootstrap would technically run today: Arch is one of the package managers the
provision script knows, and Omarchy already ships mise, `op` (correctly
root:onepassword-cli setgid), a Nerd Font, `wl-clipboard`, and git. But a
first apply would overwrite config that Omarchy owns and re-edits on theme
switches and updates, and it collides with three Omarchy conventions:

1. **bash** is the shell; all Omarchy aliases/functions/env live in bash rc files.
2. **mise** owns runtimes (`omarchy install dev-env` does `mise use --global node`; `omarchy update` runs `mise up`).
3. **Theming is generated**: terminal, nvim, btop, claude, etc. configs import from `~/.local/state/omarchy/current/theme/`.

The fix is mostly gating, not new machinery.

## 0. Decisions that shape everything else

- [x] **Detection axis (recommended):** gate on `.chezmoi.osRelease.id == "omarchy"` in templates. No new profile; this machine is `personal-linux`. Omarchy is an OS-variant axis like darwin vs linux (ADR-0017 logic).
- [x] **Ownership split (recommended):** Omarchy owns the desktop layer (terminal configs, nvim, herdr config, starship, hypr, fonts-via-`omarchy font set`). myplace owns shell tooling, secrets, `~/.mvscripts`, git config, and the mise CLI baseline.
- [x] **Write the ADR** (`docs/adrs/0026-omarchy-as-os-variant.md`) capturing the two points above before touching `home/`.
- [x] **Profile answer for this machine:** `personal-linux` (confirm).

Comments:

```

```

## 1. Files chezmoi would clobber that Omarchy owns

Proposed mechanism for all of these: `.chezmoiignore` entries gated on the
Omarchy os-release id, so the source tree is unchanged for every other machine.

- [x] **Terminal configs** (`.config/alacritty/alacritty.toml`, `.config/ghostty/config`) — ignore on Omarchy **(recommended)**. Omarchy's versions import the live theme file and `omarchy font set` rewrites the font line with sed; ours hardcode Catppuccin Mocha + FiraCode and would break theme switching, then show drift after every font change.
  - Alternative: keep managing them but make the templates Omarchy-aware (import the theme file, drop the color block). More work, still fights `omarchy font set`.
- [x] **Neovim** (`.config/nvim/**`) — ignore on Omarchy **(recommended)**. Omarchy ships its own LazyVim tree via the `omarchy-nvim` package with `lua/plugins/theme.lua` symlinked to the current theme plus a hot-reload plugin; shipped migrations repair that symlink. Our `colorscheme.lua` would replace it and future migrations would fight chezmoi.
  - Alternative: manage only our `keymaps.lua`/`options.lua` on top of Omarchy's tree. Risky; their config files have the same names.
- [ ] **herdr config** (`.config/herdr/config.toml`) — ignore on Omarchy **(recommended)**; keep managing the `machine-title-plugin/` dir and the plugin link step. Omarchy seeds a tmux-mirroring config (ctrl+space prefix); ours has different keys plus the command-palette binding.
  - [ ] Optionally port the `prefix+p` command-palette binding into a note/how-to for Omarchy machines, since it lives in the ignored config.
- [x] **starship** — ignore `.config/starship/**` on Omarchy (Omarchy uses `~/.config/starship.toml`). See also the fleet-wide bug in section 7.
- [a] **bat** (`.config/bat/config`) — decide. Omarchy exports `BAT_THEME=ansi` so bat follows the terminal theme; our config forces Catppuccin. Options: (a) ignore on Omarchy, (b) keep ours (a config file `--theme` beats the env var, so ours would win), (c) template `--theme` out on Omarchy. My pick: (c).
- [x] **git** — no change needed. Omarchy writes `~/.config/git/config` (aliases, `defaultBranch = master`); our `~/.gitconfig` loads later and wins on conflicts. Only duplicated identity. Leave it.
- [x] **`~/.claude/CLAUDE.md`** — no conflict. Omarchy's theme sync writes `~/.claude/themes/` and `settings.json`, which we don't manage.

Comments: make things omarchy aware; and keep omarcheys theme switching abilities, lets keep my herdr config, but backup the omarchy one i'll compare them later for things i want

```

Clarified 2026-09-02:
- herdr: keep managing our config.toml on Omarchy. Omarchy's seeded config backed
  up to ~/.config/herdr/config.toml.omarchy (done by hand before first apply).
- bat: [a] = ignore .config/bat/config on Omarchy.
```

## 2. The shell: bash vs zsh

Omarchy's user-facing shell layer is entirely bash: aliases (`a`, `cx`, `cy`,
`t`, `h`, `n`, `g*`), the worktree and `try` functions, inputrc, completions,
and `default/bash/envs` (sets `EDITOR=omarchy-launch-editor --inline`,
`BROWSER`, `BAT_THEME`, `MANPAGER`, locale fixups). Our provision script installs
zsh + oh-my-zsh and expects a later `chsh`.

Verified: zsh still works on Omarchy. `OMARCHY_PATH` reaches zsh via
`/etc/profile.d/omarchy.sh` (sourced by Arch's `/etc/zsh/zprofile`) and the
uwsm session env, so every `omarchy …` command runs. You only lose the bash
aliases/functions.

Pick one:

- [x] **Option A — zsh everywhere (recommended):** keep zsh as the interactive shell for fleet consistency. `.zshrc` sources Omarchy's `env-bootstrap` and `envs` (both POSIX) when `OMARCHY_PATH` is set. Port the handful of Omarchy aliases you actually want into `dot_mvdotfiles.zsh`. One-time manual `chsh -s /usr/bin/zsh` (provision never prompts for a password, same rule as servers).
  - [x] Which Omarchy aliases/functions to port? (`cx`, `t`, `h`, `n`, worktree fns, `try`, `open`…) — list here: (port call but limit to omarchy installs in chezmoi)
- [ ] **Option B — bash on Omarchy:** don't install zsh/oh-my-zsh there; don't manage `.zshrc`. Manage a `~/.mvdotfiles.bash` drop-in and source it from Omarchy's `.bashrc` (its comment invites local additions). Loses atuin/pay-respects/omz wiring and the agent-friendly gating unless re-implemented for bash.
- [ ] **`EDITOR`:** ours exports `nvim`; Omarchy's `omarchy-launch-editor --inline` respects the editor chosen in Omarchy's defaults. Keep ours (simplest) or defer to Omarchy's on that OS? Default: keep ours.

Comments: For the editor on omarchy defer to omarchy setup

```

Clarified 2026-09-02:
- Port ALL Omarchy aliases/functions, by sourcing Omarchy's own
  default/bash/aliases + fns/* from .zshrc when on Omarchy (not copying), so they
  track Omarchy updates. Bash-only bits (inputrc, completions) are skipped.
- EDITOR: not exported by us on Omarchy; Omarchy's envs sets it.
```

## 3. Provision script (`run_onchange_provision.sh.tmpl`) on Arch

- [x] **Package-name mapping:** `pm_install build-essential` fails on pacman (Arch name is `base-devel`, already installed here so the `cc` guard skips it, but the mapping should exist) **(recommended)**.
- [x] **Skip the neovim static build** when the distro nvim already clears the 0.10 floor **(recommended)**. Today it installs to `/opt` + `/usr/local/bin/nvim`, shadowing Omarchy's packaged 0.12.5 which Omarchy keeps current, and re-runs `sudo` whenever versions differ.
- [x] **Prefer `omarchy pkg add` when present** instead of raw `sudo pacman`. It wraps yay/pacman and handles AUR-only packages (fnm is AUR-only; rustup is in `extra`). Optional; raw pacman also works.
- [x] **sudo prompts:** no passwordless sudo on this box, so zsh/nano/httpie/mosh installs would grab the terminal during `myplace update`. Options: install those once by hand before first apply, or accept the prompt on desktops. Not Omarchy-specific; same on any password-sudo machine.
- [x] **rustup via pacman** (`rustup` package) instead of the curl installer on Arch? Optional. The curl path works too.
- [x] **fnm:** AUR-only on Arch; the curl installer into `~/.local/bin` works as-is. Only matters if you switch Node to mise (section 4).
- [x] **Fonts:** Omarchy ships `ttf-jetbrains-mono-nerd-basic`; our FiraCode/Monaspace/Symbols download into `~/.local/share/fonts` is harmless and works. If you want FiraCode in the terminal on Omarchy, the sanctioned way is `omarchy font set "FiraCode Nerd Font Mono"` (one-time, manual), not a managed terminal config.
- [x] No change needed: `op`, `wl-clipboard`, git, unzip, `cc` are all present. The `op` block no-ops (already root:onepassword-cli setgid at `/usr/bin/op`). **Manual step before first apply:** turn on "Integrate with 1Password CLI" in the 1Password app so the age-key fetch works.

Comments: we should skip any nvim stuff on omarchy

```

Clarified 2026-09-02:
- sudo prompts are acceptable on this box (interactive bootstrap); no pre-install
  step required.
- Use `omarchy pkg add` for package installs when present (rustup included).
- Skip everything nvim-related on Omarchy (static build, config).
```

## 4. mise config and tool duplication

Pacman already provides: bat, btop, eza, fastfetch, fd, fzf, gum, jq, lazygit,
ripgrep, starship, tldr, zoxide, wl-clipboard, herdr, mise itself, neovim.
Our mise baseline would install a second copy of most of those. Under our zsh
(`mise activate`) the mise copy shadows `/usr/bin`; under Omarchy's bash the
pacman copy wins (shims are appended). Two shells, two versions.

- [x] **herdr: gate out of the mise config on Omarchy (recommended, required).** A shipped migration explicitly uninstalls any mise herdr so it can't shadow the packaged `/usr/bin/herdr` with an older wire protocol. Ours re-installs it every update.
- [x] **Everything else: accept the duplication (recommended).** Simplest and keeps the fleet baseline identical. `omarchy update` runs `mise up` so they stay current anyway.
  - Alternative: gate the pacman-provided tools out of the mise config on Omarchy. Fewer duplicates, more template branches, and the bash/zsh version split goes away.
- [a] **Node policy:** ADR-0007 says Node is fnm's, never mise. Omarchy's `dev-env` installer does `mise use --global node`, and the local config on this box already has `node = "26.8.1"`. Pick: (a) keep fnm fleet-wide and just don't run Omarchy's dev-env installer here, (b) allow mise-managed Node on Omarchy only (ADR supersession), (c) move the fleet to mise Node (bigger ADR). Default: (a).
- [ ] **Local mise entries that would be lost** on first apply: `claude`, `codex`, `node`. How is Claude Code installed on the rest of the fleet? If it's not managed, decide whether to add `claude`/`codex` to the baseline (desktop-gated) or re-add them by hand after apply.
- [x] **`tldr`:** Omarchy ships the python `tldr`; ours installs `tlrc` (same binary name). Harmless shadowing; leave it.

Comments: claude and codex can stay unmanaged; omarchy pre-installs htem which i'm fine with, i manually install them on otehrsw as needed

```

Clarified 2026-09-02:
- claude/codex are NOT "pre-installed": Omarchy's ~/.local/bin/<tool> wrappers run
  `mise use -g <tool>` on every invocation, writing into ~/.config/mise/config.toml.
  Managing that file on Omarchy would drift forever.
- Decision: on Omarchy, render the baseline to ~/.config/mise/conf.d/myplace.toml
  and leave config.toml unmanaged (ignored) for Omarchy's wrappers. mise merges
  both (verified with `mise config ls` in an isolated XDG_CONFIG_HOME); config.toml
  loads last so an Omarchy pin wins on conflict. Other machines unchanged.
- Node stays on fnm (a); the local mise `node` entry is Omarchy's to keep in config.toml.
```

## 5. Go app changes

- [x] **`outdated`: add a pacman source (recommended).** The shelly source is CachyOS-only. On Omarchy the read-only equivalents are `checkupdates` (pacman-contrib, shipped) for repos and `yay -Qua` for AUR. Present-if-installed like brew/shelly, informational only, never mutates. **Never run `pacman -Syu` from myplace**: `omarchy update` owns that (migrations, keyring, mise up, firmware).
- [x] **Doc updates that go with it:** ADR-0010/0023 (sources), `docs/features/outdated-packages.md`, README, cobra annotations on `outdated`.
- [x] **bootstrap:** no change needed. mise is already installed (`mise-bin` from pacman) so it's skipped; chezmoi installs to `~/.local/bin` via get.chezmoi.io. Optional: prefer `omarchy pkg add chezmoi` on Omarchy (chezmoi is in `extra`).
- [x] **doctor:** passes as-is (`~/.local/bin` is appended to PATH by Omarchy's env-bootstrap). Optional: a doctor line that reports the Omarchy version when detected.
- [x] **sysinfo:** fastfetch present; nothing to do unless Omarchy's `/etc/fastfetch` config changes the output shape we parse (untested).

Comments:

```

Clarified 2026-09-02:
- Optionals left to judgment: skip the doctor Omarchy-version line and the
  `omarchy pkg add chezmoi` bootstrap path (get.chezmoi.io works). Verify sysinfo
  parses Omarchy's fastfetch output.
```

## 6. Spec/docs to update in the same change

- [x] `docs/adrs/0026-omarchy-as-os-variant.md` (new)
- [x] `docs/guides/managed-setup.md` — the Omarchy gate and what's ignored there
- [x] `INVENTORY.md` — Omarchy column/notes for what's intentionally omitted
- [x] `README.md` — Omarchy first-apply notes (1Password CLI integration toggle, `chsh`, `omarchy font set`)
- [x] `docs/workflows/bootstrap-new-machine.md` — Omarchy branch of the flow

Comments:

```

```

## 7. Side findings (not Omarchy-specific)

- [ ] **starship config is probably never loaded anywhere.** The repo manages `~/.config/starship/starship.toml`, but starship only reads `~/.config/starship.toml` unless `STARSHIP_CONFIG` is set, and nothing in `home/` sets it. Check on a Mac (`starship config` or `echo $STARSHIP_CONFIG`). Fix is either moving the file to `dot_config/starship.toml` or exporting `STARSHIP_CONFIG` in `.zshrc`.
- [ ] `alacritty.toml.tmpl` has `title = "Alacritty@CachyOS"` hardcoded. Cosmetic.

Comments: lets update the title on the cachyos setting, and we can ignore the starship issue for now

```

Clarified 2026-09-02:
- alacritty title becomes Alacritty@<hostname> via the template. starship left as is.
```

## Manual steps on this machine (after the above lands)

Not automatable by design; listed so nothing is forgotten.

- [ ] Enable "Integrate with 1Password CLI" in the 1Password app, sign in once.
- [ ] `chsh -s /usr/bin/zsh` (if Option A in section 2). Needs zsh installed first: `sudo pacman -S zsh` (or let the provision script install it).
- [ ] `omarchy font set "FiraCode Nerd Font Mono"` (optional, after the fonts land).
- [x] Re-add `claude`/`codex`/`node` to mise by hand — not needed: config.toml stays Omarchy's on Omarchy (section 4).
- [x] Back up Omarchy's herdr config before first apply → `~/.config/herdr/config.toml.omarchy` (done 2026-09-02).
- [ ] `myplace bootstrap` with profile `personal-linux`, then `myplace status`.

## General comments

```

```
