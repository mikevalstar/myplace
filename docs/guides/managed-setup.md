---
title: Extending the managed setup (tools & dotfiles)
status: active
created: 2026-06-13
updated: 2026-06-16
tags: [chezmoi, mise, dotfiles, provisioning, how-to]
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
| `.chezmoiscripts/run_onchange_provision.sh` | Idempotent installer for the things mise can't own — git, zsh, oh-my-zsh + plugins, rustup, fnm, plain OS/brew packages via `ensure_tool` (httpie, mosh, neovim, nano), and macOS-only fonts/GUI casks via `ensure_cask` |
| `dot_zshrc` | The managed `~/.zshrc` — oh-my-zsh setup, mise activation, tool env wiring |
| `dot_gitconfig.tmpl` | `~/.gitconfig` — identity (name/email from `.gitName`/`.gitEmail`), modern defaults, and SSH commit signing auto-enabled when a key exists ([ADR-0015](../adrs/0015-git-defaults-and-ssh-commit-signing.md)) |
| `dot_config/git/allowed_signers.tmpl` | `~/.config/git/allowed_signers` — generated `<email> <pubkey>` so local signature verification works; empty (and signing off) on a keyless machine |
| `dot_mvdotfiles.zsh` | Personal shell config (`~/.mvdotfiles.zsh`) sourced by `.zshrc`: tool inits, aliases, functions |
| `dot_nanorc.tmpl` | The managed `~/.nanorc` — GNU nano syntax highlighting (includes the bundled syntax files, path templated per OS/arch) + editor niceties |
| `private_dot_ssh/private_config.tmpl` | `~/.ssh/config` — non-secret global `Host *` defaults for every machine; on non-`server` profiles it also pulls the full host list (with IPs) from a 1Password Document at apply time, so secrets stay out of this public repo ([ADR-0016](../adrs/0016-secrets-in-dotfiles-via-1password.md)) |
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

Both cases live in `.chezmoiscripts/run_onchange_provision.sh`. It's `run_onchange_`, so editing it re-runs on the next apply; keep every step guarded and failure-tolerant (`|| log …`) so re-runs and network blips are harmless.

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

### A macOS font or GUI app (Homebrew cask)

Fonts and GUI apps are Homebrew *casks*, and in this fleet they're macOS-only (the Linux machines are headless servers). Add an `ensure_cask` line to the provision script; it installs via `brew install --cask` when Homebrew is present, skips off macOS, and logs a note on a brew-less Mac ([ADR-0009](../adrs/0009-homebrew-casks-macos.md)):
```sh
ensure_cask font-monaspace-nf
ensure_cask font-jetbrains-mono-nerd-font
```
Find the exact name with `brew search /<name>/`. Nerd Fonts are `font-<family>-nerd-font`; the icon-only overlay is `font-symbols-only-nerd-font`.

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

### A dotfile that carries secrets (1Password)

When a managed file holds something that must not land in the public repo (server
IPs, tokens), keep the file chezmoi-managed but pull the secret content from
1Password at apply time — never the `private_` prefix alone (that only sets 0600
perms; the content is still committed in plaintext). Rationale and trade-offs:
[ADR-0016](../adrs/0016-secrets-in-dotfiles-via-1password.md).

The worked example is `~/.ssh/config` (`private_dot_ssh/private_config.tmpl`):

1. **Store the secret in 1Password.** For a whole file, create a **Document**
   item (here titled `ssh config` in the `Private` vault) holding the real config
   — the host blocks with their `HostName` IPs. For a single value, use a normal
   field instead.
2. **Reference it from the template**, not the repo:
   ```gotmpl
   {{ onepasswordDocument "ssh config" "Private" "my.1password.com" }} # whole file
   {{ onepasswordRead "op://Private/some-item/token" }}               # one field
   ```
   The third arg is the 1Password account (a sign-in address). It's **required**
   so `op` can disambiguate on a machine signed into more than one account —
   without it, chezmoi's apply fails with `multiple accounts found`. The Document
   lives in the personal account, which every non-server mac must be signed into
   to read it, so naming it explicitly is both correct and portable.
3. **Gate by profile so servers don't need the secret.** Wrap the pull in
   `{{ if ne .profile "server" }}…{{ end }}`. Go templates only evaluate the taken
   branch, so servers never call `op` — they render only the non-secret parts.
   This is what lets headless servers converge without a 1Password session.
4. **Gate OS-specific keywords on `.chezmoi.os`, not profile** (e.g. `UseKeychain`
   is Apple-openssh-only; Linux `ssh` errors on it). Both mac profiles are
   `darwin`, so OS is the correct axis for OS quirks.
5. **To change the host list later, edit the 1Password Document** — not this repo.
   Do it with the `op` CLI so a server IP never lands in a tracked file:
   ```sh
   op document get "ssh config" --vault Private --account my.1password.com  # read current
   printf '%s' "$NEW_CONFIG" | op document edit "ssh config" - --vault Private --account my.1password.com  # replace
   ```
   `--account` is needed when more than one account is signed in (same reason the
   template passes it); drop it if the machine has only the one account.
   To change the shared defaults every machine gets (not secret), edit the
   `Host *` block in the template and `myplace update`.

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
- **Fonts and GUI apps are macOS-only.** They install as Homebrew casks via `ensure_cask`; the Linux fleet is headless servers, so casks are skipped there by design. A Linux desktop would need a different path (chezmoi externals) — not built yet ([ADR-0009](../adrs/0009-homebrew-casks-macos.md)).
- **1Password-backed dotfiles make `status` shell out to `op`.** chezmoi evaluates
  `onepassword*` functions during *every* target-state computation — `apply`,
  `status`, **and** `diff` — so on a mac `myplace status` invokes the 1Password CLI.
  With the desktop app's CLI integration that's a cached, no-prompt session; if `op`
  is locked or absent, chezmoi errors and `status` exits 3. Servers never hit this
  (the secret pull is behind the non-`server` branch) ([ADR-0016](../adrs/0016-secrets-in-dotfiles-via-1password.md)).
- **Commit signing auto-enables only when a key is present.** `dot_gitconfig.tmpl` turns on SSH signing when `~/.ssh/id_ed25519.pub` (or the `signingKey` data override) exists, so a keyless machine signs nothing and never fails a commit. After a machine starts signing, upload the **public** key to GitHub as a *signing* key (separate from an auth key) once for the Verified badge: `gh ssh-key add ~/.ssh/id_ed25519.pub --type signing --title "$(hostname)"` ([ADR-0015](../adrs/0015-git-defaults-and-ssh-commit-signing.md)).

## References

- [ADR-0007](../adrs/0007-provisioning-mechanism.md), [ADR-0003](../adrs/0003-monorepo-app-dotfiles-mise.md)
- chezmoi scripts: https://www.chezmoi.io/user-guide/use-scripts-to-perform-actions/
