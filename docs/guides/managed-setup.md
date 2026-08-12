---
title: Extending the managed setup (tools & dotfiles)
status: active
created: 2026-06-13
updated: 2026-08-12
tags: [chezmoi, mise, dotfiles, provisioning, skills, how-to]
audience: both
---

# Extending the managed setup (tools & dotfiles)

## Purpose

Where things live and how to add a new tool, dotfile, or provisioning step so it lands on every machine. The mechanism rationale is [ADR-0007](../adrs/0007-provisioning-mechanism.md); this is the how-to.

## The layout (under `home/`, chezmoi's source root)

| Path | What it is |
|------|------------|
| `dot_config/mise/config.toml.tmpl` | The mise tool set — every machine's CLI tools/runtimes from mise's registry |
| `dot_mvscripts/executable_*` | Helper scripts deployed to a dedicated `~/.mvscripts` (on `PATH`), runnable by name on every machine; `mv_scripts` lists them ([ADR-0014](../adrs/0014-managed-scripts-and-bun-runner.md)) |
| `.chezmoiscripts/run_onchange_provision.sh.tmpl` | Idempotent installer for the things mise can't own — git, zsh, oh-my-zsh + plugins, rustup, fnm (+ unzip), tokei (cargo build), a current neovim (official static build → `/usr/local` on Linux, brew on macOS), plain OS/brew packages via `ensure_tool` (httpie, mosh, nano), and macOS fonts/GUI casks via `ensure_cask`. A chezmoi template (`.tmpl`) so it can branch on `.profile`: a final block gated to non-`server` Linux installs the Linux **desktop** extras — the `op` CLI, `wl-clipboard` (Wayland copy/paste), and Nerd Fonts into `$HOME` ([ADR-0017](../adrs/0017-linux-desktop-profile.md)) |
| `dot_zshrc` | The managed `~/.zshrc` — oh-my-zsh setup, mise activation, tool env wiring |
| `dot_gitconfig.tmpl` | `~/.gitconfig` — identity (name/email from `.gitName`/`.gitEmail`), modern defaults, and SSH commit signing auto-enabled when a key exists ([ADR-0015](../adrs/0015-git-defaults-and-ssh-commit-signing.md)) |
| `dot_config/git/allowed_signers.tmpl` | `~/.config/git/allowed_signers` — generated `<email> <pubkey>` so local signature verification works; empty (and signing off) on a keyless machine |
| `dot_mvdotfiles.zsh` | Personal shell config (`~/.mvdotfiles.zsh`) sourced by `.zshrc`: tool inits, aliases, functions |
| `dot_claude/CLAUDE.md` | `~/.claude/CLAUDE.md` — fleet-wide Claude Code preferences (coding style, documentation-first practice, tooling, agent posture). Only this one file is managed; chezmoi leaves the rest of `~/.claude` (settings, sessions, plugins, caches) machine-local |
| `dot_nanorc.tmpl` | The managed `~/.nanorc` — GNU nano syntax highlighting (includes the bundled syntax files, path templated per OS/arch) + editor niceties |
| `private_dot_ssh/private_config.tmpl` | `~/.ssh/config` — non-secret global `Host *` defaults for every machine; on non-`server` profiles (every desktop, Mac or `personal-linux`) it also `Include`s `~/.ssh/config.d/hosts`, the full host list (with IPs) decrypted at apply time from the age ciphertext committed at `private_dot_ssh/private_config.d/encrypted_private_hosts.age` — so the secrets never appear in this public repo in plaintext ([ADR-0022](../adrs/0022-age-encrypted-dotfiles.md), [ADR-0017](../adrs/0017-linux-desktop-profile.md)) |
| `.chezmoi.toml.tmpl` | Init prompts → chezmoi data: `profile`, `push`, plus `gitName`/`gitEmail` (answered at install, pre-fillable with `--promptString`). Optional `signingKey` (no prompt; `dig`-defaulted) overrides the commit-signing key path |

`dot_` becomes a leading `.` in the target; a `.tmpl` suffix means chezmoi templates it.

## How to add…

### A CLI tool that's in mise's registry

1. Check it exists: `mise registry | grep <name>`.
2. Add a line under `[tools]` in `dot_config/mise/config.toml.tmpl`:
   ```toml
   ripgrep = "latest"
   ```
3. Commit & push. On each machine, `myplace update` (or `mise install`) picks it up.

### A tool mise doesn't carry

Both cases live in `.chezmoiscripts/run_onchange_provision.sh.tmpl`. It's `run_onchange_`, so editing it re-runs on the next apply; keep every step guarded and failure-tolerant (`|| log …`) so re-runs and network blips are harmless. It's a chezmoi template (`.tmpl`), so it can branch on `.profile` — but keep template syntax to the few gated blocks that need it (the rest is plain POSIX shell), and make sure it stays valid both as a template and as a script.

**A plain package** that the OS package managers / Homebrew carry (e.g. `httpie`, `mosh`) — add one `ensure_tool` line. It installs via the system package manager on Linux and via Homebrew *if it's already present* on macOS, logging a note otherwise (bootstrap never requires brew — [ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md)):
```sh
ensure_tool http httpie   # ensure_tool <command-to-check> <package-name>
ensure_tool mosh mosh
```

**An installer or framework** with its own install script (rustup, fnm, oh-my-zsh — not a packaged binary) — add a guarded block:
```sh
if ! command -v <tool> >/dev/null 2>&1 && [ ! -x "$HOME/.local/bin/<tool>" ]; then
  log "installing <tool>"
  curl -fsSL <installer> | sh -s -- <non-interactive-flags> || log "<tool> install failed"
fi
```

### A font or GUI app (desktops only)

Fonts and GUI apps land on **desktops** (`personal-mac`/`work-mac`/`personal-linux`), never on the headless servers. The install mechanism differs by OS:

**On macOS** they're Homebrew *casks*. Add an `ensure_cask` line to the provision script; it installs via `brew install --cask` when Homebrew is present, skips off macOS, and logs a note on a brew-less Mac ([ADR-0009](../adrs/0009-homebrew-casks-macos.md)):
```sh
ensure_cask font-monaspace-nf
ensure_cask font-jetbrains-mono-nerd-font
```
Find the exact name with `brew search /<name>/`. Nerd Fonts are `font-<family>-nerd-font`; the icon-only overlay is `font-symbols-only-nerd-font`.

**On the Linux desktop** there's no brew, so the fonts are downloaded from the `ryanoasis/nerd-fonts` releases into `~/.local/share/fonts` (then `fc-cache`). That lives in the `{{ if and (eq .chezmoi.os "linux") (ne .profile "server") }}` block at the end of the provision script — profile-gated so servers skip it ([ADR-0017](../adrs/0017-linux-desktop-profile.md)). Add a font family by extending the `for family in …` list there. The same block installs the `op` CLI (a desktop needs it once, at first apply, to fetch the age key that decrypts the SSH host list — [ADR-0022](../adrs/0022-age-encrypted-dotfiles.md)) and `wl-clipboard` (a Wayland desktop needs `wl-copy`/`wl-paste` for CLI copy/paste — e.g. Claude Code's image paste and `/copy`).

### A new dotfile

- Bring an existing file under management: `chezmoi add ~/.foorc` (creates `home/dot_foorc` in the source clone), then commit/push from the source repo — **or**, when working in the dev checkout, drop the file at `home/dot_foorc` directly.
- Make paths portable: use `$HOME`, never `/Users/<you>`. Servers and other usernames must work.
- Needs per-machine variation? Rename to `…​.tmpl` and branch on `.profile` (e.g. `{{ if ne .profile "server" }}…{{ end }}`).

### A helper script on every machine

For a standalone utility you want on `PATH` on every box (not dev tooling for this
repo, and not a provisioning step) — drop it under `home/dot_mvscripts/` with the
`executable_` prefix so chezmoi marks it `+x`. It deploys to `~/.mvscripts/<name>`
(a dedicated dir prepended to `PATH` in `dot_zshrc`, kept separate from `~/.local/bin`
so our scripts don't mingle with mise/installer binaries) through the normal
`myplace update` flow and is invoked by name
(`home/dot_mvscripts/executable_ai_installed` → `ai_installed`).

- **Default to plain shell**; reach for `bun` (a managed tool — `core:bun` in mise) only
  when a script needs TypeScript, real arg parsing, or JSON it'd be painful to emit from
  bash. Rationale and the shell-vs-bun split: [ADR-0014](../adrs/0014-managed-scripts-and-bun-runner.md).
- **Make it discoverable:** add a `# mv_scripts: <one-line description>` comment to the
  script body (avoid `|` in the text). `mv_scripts` scans `~/.mvscripts` for that marker
  and lists every marked script with its description in a table — so a new helper shows
  up automatically, no list to maintain.
- You own portability: use `$HOME` not `/Users/<you>`, guard against missing deps, and
  keep shell scripts POSIX-safe enough for the headless Linux servers.

### A herdr plugin (all machines)

herdr (the terminal multiplexer, a mise tool) has no config-declared or
auto-discovered plugins — a plugin is only ever registered by an explicit
`herdr plugin link <path>`, persisted to `~/.config/herdr/plugins.json`. To ship
one to the fleet, track its files as managed dotfiles and link them from the
provision script. The worked example is the machine-title plugin
([ADR-0019](../adrs/0019-herdr-plugin-fleet-wide-reliable-relink.md), which
supersedes ADR-0018), which sets the terminal title to
`herdr@<host> · <workspace>` on every machine — servers included, since an
SSH-attached server's herdr sets the title you see locally:

1. **The plugin files** live at `home/dot_config/herdr/machine-title-plugin/`
   (`herdr-plugin.toml` + `executable_set-title.sh`), applied by chezmoi to
   `~/.config/herdr/machine-title-plugin/`. Keep the script POSIX `sh`; it may use
   managed tools like `jq` (they're on `PATH` for the herdr server) but should
   fall back if one is missing.
2. **Register it in the provision script**, on every profile (no server gate — the
   title distinguishes servers too). `herdr plugin link` is idempotent (keyed by
   plugin id), so re-running it every apply is safe; guard on `command -v herdr`
   because provision runs before `mise install` (herdr is a mise tool, so absent on
   a brand-new machine's first apply). **The catch:** a `run_onchange_` script only
   re-runs when its content changes, so a `command -v herdr` skip is *not* retried
   on later applies on its own. Fold herdr's presence into the onchange hash as a
   1/0 boolean (`# herdr installed: {{ "{{ if lookPath \"herdr\" }}1{{ else }}0{{ end }}" }}`)
   so the script re-fires the first update after mise installs herdr and the
   skipped link is retried. Use the boolean, not lookPath's raw path — the path is
   env-dependent (install dir vs shims) and would leave `chezmoi status` dirty.
   Also add
   the manifest to the hash
   (`# … {{ "{{ include \"dot_config/herdr/machine-title-plugin/herdr-plugin.toml\" | sha256sum }}" }}`)
   so a plugin change re-triggers the link.

A **third-party** plugin (one you don't vendor into the repo) is registered with
`herdr plugin install <owner>/<repo>` instead of `link`, and the semantics differ:
`install` fetches from GitHub, so it's **not** safe to re-run every apply — guard
on `herdr plugin list` not already containing the plugin id and install once, and
pass `--yes` (apply is non-interactive). The worked example is the fzf command
palette ([ADR-0020](../adrs/0020-herdr-command-palette-plugin.md), `jt.command-palette`,
installed from `JanTvrdik/herdr-command-palette`); it sits in the same
`command -v herdr` block as the machine-title `link`, so ADR-0019's onchange-hash
cold-start machinery retries it once herdr appears. Trade-off: install-once means
it doesn't auto-track upstream — re-install by hand to bump. If the plugin binds
behaviour to a key, note that **herdr 0.7 does not bind keys from a plugin
manifest** — add a `[[keys.command]]` with `type = "plugin_action"` to the managed
`home/dot_config/herdr/config.toml` (that's where the palette's `prefix+p` lives).

### An AI agent skill

Skills (a `SKILL.md` + optional supporting files that Claude Code and other agents discover) split into two kinds, managed two different ways ([ADR-0023](../adrs/0023-managing-ai-skills.md)) — the same dotfile-vs-tool split as everything else:

**Your own authored skills → chezmoi**, like any dotfile. They live under a `home/dot_claude/skills/<name>/` tree → `~/.claude/skills/<name>/`; commit/push and `myplace update` applies them, with drift and diff review for free. Manage only `skills/` (and specific files like `settings.json`) — **not** all of `~/.claude`: `plugins/cache/`, logs, and history are machine-local, so `.chezmoiignore` them. Do **not** manage `~/.agents/skills/` either — that's the skills.sh CLI's own store (it symlinks from there into `~/.claude/skills/`), so managing it fights the CLI, the way adding node to mise fights fnm.

**Third-party "stable" skills → the skills.sh CLI** (`skills`, aka `npx skills`), which is the AI-skill equivalent of a package manager. myplace treats it present-if-installed and *informational*: `myplace outdated` (and the dashboard's Updates pane) run `skills check -g` and list any skill behind upstream, but never upgrade — it's read-only, and it never touches the drift verdict ([ADR-0010](../adrs/0010-cross-package-manager-outdated-inventory.md)).

Making the **global** set reproducible is self-managed for now, because the CLI can't yet restore a global stable from its lock (`experimental_install` is project-only; `~/.agents/.skill-lock.json` also carries volatile timestamp/UI state — [ADR-0023](../adrs/0023-managing-ai-skills.md), [vercel-labs/skills#683](https://github.com/vercel-labs/skills/issues/683)). So the declaration is a **source-repo list**, not a committed lockfile (not yet wired — the ADR's follow-up):

- Add a profile-gated block to `run_onchange_provision.sh.tmpl` — `{{ if ne .profile "server" }}` (servers get no AI skills) — with a `for repo in … ; do npx --yes skills add "$repo" -g -s '*' -y ; done` loop, guarded on `command -v npx` (Node is fnm's, present on desktops per ADR-0007). That `for repo in …` list *is* the fleet's global stable; edit it to add/remove a skill. Fold `npx`'s presence into the onchange hash (a 1/0 boolean, like the herdr blocks) so it re-fires once Node lands.
- **Migrate to a committed global lock** (`~/.agents/.skill-lock.json`, minus its volatile fields) driving a real `skills` restore command once #683 ships — 👍 that issue to move it along.

Don't wire skill auto-updates via a Claude Code `SessionStart` hook (`npx skills update`) — that hides update state from myplace, which is where updates should surface and be driven.

### A dotfile that carries secrets (age-encrypted)

When a managed file holds something that must not land in the public repo in
plaintext (server IPs, tokens), commit it **age-encrypted** — never the
`private_` prefix alone (that only sets 0600 perms; the content is still
committed in plaintext). Rationale and trade-offs:
[ADR-0022](../adrs/0022-age-encrypted-dotfiles.md), which supersedes the
1Password-Document mechanism of ADR-0016.

The worked example is the SSH host list
(`private_dot_ssh/private_config.d/encrypted_private_hosts.age` →
`~/.ssh/config.d/hosts`, `Include`d by `~/.ssh/config` on desktops):

1. **Reuse the fleet key — don't mint per-file keys.** One age identity lives
   at `~/.config/chezmoi/key.txt` on every desktop (fetched from the
   `chezmoi age key` 1Password Document by
   `.chezmoiscripts/run_before_fetch-age-key.sh.tmpl` whenever it's missing or
   empty — normally just once, at first apply); its public half is
   the `recipient` committed in `.chezmoi.toml.tmpl`'s `[age]` section.
2. **Encrypt the content into the repo.** From a checkout, on any machine that
   has the key:
   ```sh
   chezmoi encrypt <plaintext-file> > home/<path>/encrypted_private_<name>.age
   ```
   The output is armored ASCII — safe to commit. The `encrypted_` attribute is
   what makes chezmoi decrypt at apply time; keep `private_` too so the target
   lands 0600. The plaintext must never touch a tracked file — pipe it, or use
   a 0600 temp file you delete (the
   [edit-ssh-config workflow](../workflows/edit-ssh-config.md) shows the full
   decrypt → edit → re-encrypt loop).
3. **Gate who gets it in `.chezmoiignore`, not the template.** List the target
   under `{{ if eq .profile "server" }}`. chezmoi never decrypts an ignored
   entry — that's what lets servers skip the key (and `op`, and 1Password)
   entirely.
4. **Gate OS-specific keywords on `.chezmoi.os`, not profile** (e.g. `UseKeychain`
   is Apple-openssh-only; Linux `ssh` errors on it). Profile ≠ OS: gate *who
   gets the secret* on `.profile` (desktop vs server) and *OS quirks* on
   `.chezmoi.os` — the two axes are independent
   ([ADR-0017](../adrs/0017-linux-desktop-profile.md)).
5. **To change the content later**: decrypt → edit → re-encrypt → commit +
   push, then `myplace update` converges each machine — the recipe lives in the
   [edit-ssh-config workflow](../workflows/edit-ssh-config.md). Non-secret
   parts (the `Host *` defaults) stay in the plain template; edit those
   normally.

### Shell tool wiring

Tool init (`eval "$(x init zsh)"`, PATH additions) goes in `dot_mvdotfiles.zsh`, guarded with `command -v x` so a missing tool is silent. mise activation and the fnm/cargo env lines live in `dot_zshrc`.

## Gotchas

- **Node is fnm's, Rust is rustup's — not mise's.** Don't add `node`/`rust` to the mise config; they're installed by the provision script and managed by fnm/rustup (ADR-0007). Adding them to mise creates two managers fighting over the same binary.
- **Watch for tools whose only mise backend is `cargo:`/source.** Most tools resolve to a prebuilt-binary backend (`aqua:`/`github:`), but some default to `cargo:`, which runs `cargo install` — needing a Rust toolchain on `PATH` (which a fresh bootstrap doesn't have) and meaning mise drives cargo (which it must not — Rust is rustup's, ADR-0007). Check with `mise registry <tool>`. If a prebuilt backend exists, pin it (`"aqua:Owner/Repo" = "latest"`). If the tool is genuinely source-only (e.g. `tokei`, which dropped prebuilt binaries after v12), keep it **out** of the mise config and build it in `run_onchange_provision.sh` with rustup's cargo — sourcing `~/.cargo/env` first so cargo is on `PATH` that run.
- **The provision script runs before `mise install`** (during `chezmoi apply`), so it can't use any mise tool.
- **A stock Linux server has no zsh.** macOS defaults to zsh, but Ubuntu/Debian server images ship only bash — so the provision script installs zsh (`ensure_tool zsh zsh`) *before* the oh-my-zsh step, which is itself gated on `command -v zsh`. Without it, oh-my-zsh and the managed `.zshrc` would silently never run. Provision installs zsh but does **not** change the login shell; the remote bootstrap helper (`mvserver-init`) runs `chsh` so an apply never has to prompt for a password.
- **oh-my-zsh install must keep our `.zshrc`** — the script passes `KEEP_ZSHRC=yes`. Don't drop it or the managed `.zshrc` gets overwritten with OMZ's template.
- **Bootstrapping a fresh Linux server:** use the managed `mvserver-init` script (`~/.mvscripts/mvserver-init`, source at `home/dot_mvscripts/executable_mvserver-init`) rather than running the steps by hand. It SSHes in (`-i` key, `-j` jump host), creates the `mikevalstar` user with sudo + seeded keys, installs and runs `myplace bootstrap --profile server`, sets the login shell to zsh, and prints `status`. It's interactive — plain `sudo` (no NOPASSWD), so it prompts for passwords as it goes.
- **Editing the managed `.zshrc` on a machine** shows as drift (it's managed now); change it in the repo and `myplace update`, or use the capture flow.
- **Homebrew on macOS is opportunistic, never required.** `ensure_tool` uses brew when it's present and logs a note when it isn't, so a brew-less Mac still bootstraps; anything in mise's registry still belongs in mise, not here ([ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md)).
- **macOS `nano` is pico, not GNU nano.** `/usr/bin/nano` is a symlink to pico, which can't do syntax highlighting, so `command -v nano` is misleading and `ensure_tool nano nano` would no-op. The provision script installs real GNU nano via brew explicitly (idempotent on `brew list`); `~/.nanorc` (`dot_nanorc.tmpl`) wires up highlighting. On Linux `nano` is already GNU nano.
- **Fonts and GUI apps are desktop-only, and the mechanism is per-OS.** On Macs they install as Homebrew casks via `ensure_cask`; on the `personal-linux` desktop the Nerd Fonts are downloaded into `~/.local/share/fonts` from the profile-gated block at the end of the provision script; the headless Linux **servers** get neither by design ([ADR-0009](../adrs/0009-homebrew-casks-macos.md), [ADR-0017](../adrs/0017-linux-desktop-profile.md)).
- **Apple's `container` is Apple-Silicon + macOS 26 only.** The `container` runtime (brew *formula*, `ensure_tool container container`) and its GUI **Orchard** (brew *cask*, `ensure_cask orchard`) are a daemon-free Docker/Podman alternative. The formula line carries an explicit `[ "$(uname -m)" = "arm64" ]` gate so Intel Macs skip it; both are further self-limited by brew's own macOS-26 requirement and the usual brew-if-present tolerance, so an older/Intel Mac, a brew-less Mac, and all of Linux just log and move on. `container` doesn't start its VM service on install — run `container system start` (or `brew services start container`) once per machine; Orchard drives it from the GUI thereafter ([ADR-0008](../adrs/0008-opportunistic-homebrew-macos.md), [ADR-0009](../adrs/0009-homebrew-casks-macos.md)).
- **Encrypted dotfiles decrypt locally — 1Password is needed exactly once.**
  chezmoi decrypts `encrypted_` files with `~/.config/chezmoi/key.txt` during
  every target-state computation, entirely offline: `myplace status`/`diff`/
  `apply` never touch `op` (the always-on soft dependency ADR-0016 accepted;
  removed by [ADR-0022](../adrs/0022-age-encrypted-dotfiles.md)). The key
  arrives via `run_before_fetch-age-key.sh.tmpl`, which fetches from 1Password
  whenever the key file is missing **or empty** — normally just the machine's
  first apply, the one moment a signed-in `op` is required; while a non-empty
  key is in place the script renders empty and chezmoi skips it. A machine
  missing the key errors on `status`/`diff` (exit 3) until the next
  apply/update re-fetches it — the fix is `myplace update` with `op` signed
  in, not a hand-run `op document get`. Two more traps: age output is
  randomized, so re-encrypting unchanged content makes a spurious *git* diff
  (machines see no drift — chezmoi compares plaintext — but don't commit it);
  and servers never need the key — their encrypted targets are ignored in
  `.chezmoiignore`, so don't "fix" a server by copying the key onto it.
- **Commit signing auto-enables only when a key is present.** `dot_gitconfig.tmpl` turns on SSH signing when `~/.ssh/id_ed25519.pub` (or the `signingKey` data override) exists, so a keyless machine signs nothing and never fails a commit. After a machine starts signing, upload the **public** key to GitHub as a *signing* key (separate from an auth key) once for the Verified badge: `gh ssh-key add ~/.ssh/id_ed25519.pub --type signing --title "$(hostname)"` ([ADR-0015](../adrs/0015-git-defaults-and-ssh-commit-signing.md)).

## References

- [ADR-0007](../adrs/0007-provisioning-mechanism.md), [ADR-0003](../adrs/0003-monorepo-app-dotfiles-mise.md)
- chezmoi scripts: https://www.chezmoi.io/user-guide/use-scripts-to-perform-actions/
