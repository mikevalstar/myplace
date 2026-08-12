---
title: Managed machine inventory
status: active
created: 2026-07-09
updated: 2026-08-12
tags: [inventory, provisioning, dotfiles, mise, chezmoi]
---

# Managed machine inventory

This is the human-readable inventory of what `myplace` installs and configures on managed machines. It summarizes the declarations under `home/`; those files remain the executable source of truth. The root `mise.toml` is development tooling for this repository and is not part of the machine inventory.

For implementation details and instructions for extending the setup, see the [managed setup guide](docs/guides/managed-setup.md).

## Profiles and management layers

The available profiles are `personal-mac`, `work-mac`, `personal-linux`, and `server`. The setup is shared by default; desktop-only assets are excluded from `server`, while OS-specific installation mechanisms are selected independently.

- **myplace** orchestrates bootstrap, updates, drift reporting, diagnostics, and self-update.
- **chezmoi** applies the files and scripts under `home/`.
- **mise** installs registry-backed CLI tools from `home/dot_config/mise/config.toml.tmpl`.
- **The provision script** installs system packages, frameworks, and tools that mise should not own.

On a new machine, `install.sh` installs the `myplace` binary into `~/.local/bin`. The bootstrap flow then installs chezmoi and mise there when missing, initializes and applies this repository, and runs `mise install`. The [bootstrap workflow](docs/workflows/bootstrap-new-machine.md) is the detailed source of truth.

## Tools installed on every machine

### Managed by mise

| Area | Tools |
|---|---|
| Search and navigation | `ripgrep`, `fd`, `fzf`, `zoxide`, `eza` |
| Data and text | `jq`, `yq`, `miller`, `sd`, `fx`, `bat`, `glow`, `tlrc` (`tldr`), `okq` |
| Shell and prompts | `starship`, `atuin`, `gum` |
| Git and change review | `gh`, `delta`, `hunk`, `jj`, `lazygit`, `git-lfs`, `actionlint` |
| Development and automation | `pnpm`, `bun`, `d2`, `ast-grep`, `hyperfine` |
| System and terminal | `herdr`, `fastfetch` |

Linux also gets `btop` from mise. macOS gets `btop` through Homebrew when Homebrew is available. Desktop profiles also get `duckdb`; servers omit it. `okq` is not in mise's registry and is installed from its GitHub releases through mise's `github:` backend.

### Managed outside mise

The idempotent provision script at `home/.chezmoiscripts/run_onchange_provision.sh.tmpl` installs or ensures:

- `git` and `zsh`
- oh-my-zsh with `zsh-autosuggestions` and `zsh-syntax-highlighting`
- rustup with the stable Rust toolchain
- `tokei`, built with rustup's Cargo because its current releases are source-only
- fnm for Node.js version management; Node is not managed by mise
- `pay-respects`
- `httpie`, `mosh`, GNU nano, and a current Neovim
- platform prerequisites when needed, including `bash`, `unzip`, and a C build toolchain
- the managed herdr machine-title plugin and the third-party herdr command-palette plugin

On macOS, these installs use Homebrew only when it is already present; Homebrew itself is not required or installed. On supported Apple Silicon Macs, the script also attempts to install Apple's `container` CLI.

## Desktop-only additions

These apply to the current desktop profiles: `personal-mac`, `work-mac`, and `personal-linux`. The Linux block explicitly excludes `server`; the macOS entries are OS-gated because the current Mac profiles are desktop profiles.

| Capability | macOS | Linux desktop |
|---|---|---|
| 1Password CLI | Homebrew cask | Pinned binary in `/usr/local/bin`, configured for desktop-app integration |
| Nerd Fonts | Monaspace, Symbols Only, JetBrains Mono, and Fira Code Homebrew casks | The same families under `~/.local/share/fonts/NerdFonts` |
| Clipboard integration | Provided by macOS | `wl-clipboard` |
| Container GUI | Orchard cask where supported | Not installed |
| Encrypted SSH host list | Decrypted locally with the fleet age key | Decrypted locally with the fleet age key |

Desktop machines need 1Password only to fetch a missing or empty age identity during apply. Once the key exists, normal status, diff, and apply operations decrypt locally and work offline.

## Managed configuration

| Target | What is managed |
|---|---|
| `~/.zshrc` and `~/.mvdotfiles.zsh` | oh-my-zsh plugins, PATHs, mise/fnm/rustup activation, agent-safe shell behavior, Starship, zoxide, fzf, Atuin, aliases, functions, and editor defaults |
| `~/.gitconfig` | Per-machine identity, delta pager, modern pull/push/fetch/rebase defaults, rerere, Git LFS filters, and automatic SSH signing when a public key exists |
| `~/.config/git/allowed_signers` and `ignore` | Local SSH signature verification and global ignore rules |
| `~/.ssh/config` | Shared secure defaults, OS-specific options, and the colima VM include on macOS |
| `~/.ssh/config.d/hosts` | Age-encrypted fleet host list on desktop profiles only |
| `~/.ssh/authorized_keys` | Shared authorized public keys |
| `~/.config/nvim` | LazyVim-based Neovim configuration, keymaps, options, autocmds, and colorscheme |
| `~/.nanorc` | GNU nano behavior and syntax highlighting |
| `~/.config/alacritty/alacritty.toml` | Alacritty terminal configuration |
| `~/.config/ghostty/config` | Ghostty terminal configuration |
| `~/.config/zed/settings.json` | Zed editor settings |
| `~/.config/flashspace/settings.json` | FlashSpace settings |
| `~/.config/starship/starship.toml` | Starship prompt |
| `~/.config/atuin` | Atuin behavior and Catppuccin theme |
| `~/.config/bat/config` | bat display and theme settings |
| `~/.config/eza/theme.yml` | eza colors and icons |
| `~/.config/hunk/config.toml` | hunk diff-viewer theme and behavior |
| `~/.config/herdr` | Multiplexer settings, command-palette binding, and the fleet-aware terminal-title plugin |
| `~/.config/tlrc/config.toml` | `tldr` client settings |
| `~/.config/gh/config.yml` | GitHub CLI settings |
| `~/.config/mise/config.toml` | The global fleet tool declaration |
| `~/.claude/CLAUDE.md` | Fleet-wide Claude Code preferences: coding and TypeScript conventions, documentation-first practice, tooling defaults, and how the agent should engage. Only this file is managed; the rest of `~/.claude` (settings, sessions, caches, plugins) stays machine-local |

Machine identity and behavior are recorded as chezmoi data during bootstrap: profile, Git name, Git email, and whether the machine normally pushes shared changes.

## Managed helper commands

The following scripts are deployed to `~/.mvscripts` and placed on `PATH`. Run `mv_scripts` on a managed machine for the live list.

| Command | Purpose |
|---|---|
| `ai_installed` | Report installed AI CLI tools |
| `backup` | Create, list, and restore timestamped file backups |
| `env-diff` | Compare dotenv keys against an example without reading values |
| `gclean` | Confirm and remove stale local Git branches |
| `killport` | Kill the process listening on a TCP port |
| `mv_scripts` | List managed helper commands |
| `mvserver-init` | Bootstrap a remote Linux server with myplace over SSH |
| `portwhat` | Identify listeners on TCP ports |
| `pubip` | Print the machine's public IP |
| `repos` | Summarize repositories under `~/projects` |
| `serve-this` | Serve the current directory locally |
| `shareplan` | Publish or update raw HTML plans on `share.valstar.dev` |
| `whichver` | Display installed toolchain versions |

## Secrets and intentional machine-local state

- Secret-bearing files are committed as age ciphertext. The current encrypted asset is the desktop SSH host list.
- The fleet age identity lives at `~/.config/chezmoi/key.txt`; it is fetched from 1Password when missing or empty and is never committed.
- The `shareplan` API key lives at `${XDG_CONFIG_HOME:-$HOME/.config}/shareplan/key`, is created with owner-only permissions by `shareplan auth`, and is never managed by chezmoi.
- Servers ignore `.ssh/config.d`, so they do not decrypt the host list or need the age key or 1Password CLI.
- Neovim's `lazy-lock.json`, herdr's plugin registry, caches, logs, histories, and application-generated state remain machine-local.
- myplace logs live under `$XDG_STATE_HOME/myplace`, not in the managed config tree.

## Present only when installed separately

Shell PATH wiring recognizes `opencode`, Google Cloud SDK, MiniMax Code, Antigravity CLI, and the Windsurf CLI, but myplace does not install them. Homebrew itself is also never installed. Node versions remain fnm-managed, and Neovim plugins are fetched by LazyVim on first use.

The `home/dot_claude/` tree currently carries only `CLAUDE.md`, the fleet-wide Claude Code preference file. Managed Claude skills and a reproducible third-party AI-skill set remain documented future work; there is no skills installation loop to inventory yet.

## Keeping this inventory current

Update this file in the same change whenever bootstrap, `home/dot_config/mise/config.toml.tmpl`, `home/.chezmoiscripts/`, profile gates, managed dotfiles, or managed helper scripts change what a machine installs, configures, or deliberately leaves local.
