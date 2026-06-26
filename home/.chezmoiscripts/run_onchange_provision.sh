#!/bin/sh
# Provision the shell framework and toolchains that mise can't manage (ADR-0007):
# oh-my-zsh + external plugins, rustup, and fnm. Idempotent and failure-tolerant —
# every step is guarded, and a failed install is logged rather than aborting the
# apply. run_onchange_: re-runs whenever this file's content changes.
#
# Runs during `chezmoi apply`, BEFORE `mise install`, so it must not rely on any
# mise-managed tool being present.
set -u

log() { printf 'myplace provision: %s\n' "$1" >&2; }

# pm_install PKG — install a system package via the detected Linux package
# manager. Best-effort: returns non-zero if no known manager is present or the
# install fails, so callers can log and continue. (macOS has no system package
# manager here by rule — callers handle Darwin separately.)
pm_install() {
	pkg="$1"
	sudo=""
	[ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1 && sudo="sudo"
	if command -v apt-get >/dev/null 2>&1; then
		$sudo apt-get update -qq && $sudo apt-get install -y "$pkg"
	elif command -v dnf >/dev/null 2>&1; then
		$sudo dnf install -y "$pkg"
	elif command -v yum >/dev/null 2>&1; then
		$sudo yum install -y "$pkg"
	elif command -v pacman >/dev/null 2>&1; then
		$sudo pacman -S --noconfirm "$pkg"
	elif command -v apk >/dev/null 2>&1; then
		$sudo apk add "$pkg"
	else
		return 1
	fi
}

# ensure_tool CMD PKG — install a non-registry CLI tool when CMD is missing.
# Linux: the system package manager (pm_install). macOS: Homebrew *if present*
# (ADR-0008) — never required, so a brew-less Mac just gets a logged note and the
# bootstrap stays brew-free. Idempotent: a tool already on PATH is a no-op.
ensure_tool() {
	cmd="$1"
	pkg="$2"
	command -v "$cmd" >/dev/null 2>&1 && return 0
	if [ "$(uname -s)" = "Darwin" ]; then
		if command -v brew >/dev/null 2>&1; then
			log "installing $pkg (brew)"
			brew install "$pkg" || log "$pkg brew install failed"
		else
			log "$pkg missing and no Homebrew on this Mac — run 'brew install $pkg' or install manually"
		fi
	else
		log "installing $pkg"
		pm_install "$pkg" || log "$pkg install failed (no known package manager?)"
	fi
}

# ensure_cask NAME — install a macOS Homebrew cask (GUI app or font) when it's
# missing. macOS-only by nature (no cask concept elsewhere); brew-if-present like
# ensure_tool — ADR-0009 extends ADR-0008's brew story from formulae to casks.
# Idempotent and failure-tolerant: skips off macOS and when the cask is present.
ensure_cask() {
	name="$1"
	[ "$(uname -s)" = "Darwin" ] || return 0
	if ! command -v brew >/dev/null 2>&1; then
		log "$name: no Homebrew on this Mac — run 'brew install --cask $name' or install manually"
		return 0
	fi
	brew list --cask "$name" >/dev/null 2>&1 && return 0
	log "installing $name (brew cask)"
	brew install --cask "$name" || log "$name cask install failed"
}

# --- git (prerequisite for chezmoi and the clones below; not a mise tool) ---
# chezmoi's built-in git can clone the source repo on a machine with no system
# git, so this can run first and install real git before it's needed below.
if ! command -v git >/dev/null 2>&1; then
	if [ "$(uname -s)" = "Darwin" ]; then
		log "git not found — install the Command Line Tools: xcode-select --install"
	else
		log "installing git"
		pm_install git || log "git install failed (no known package manager?)"
	fi
fi

# --- zsh (the shell our whole setup targets; not a mise tool) ---
# macOS ships zsh as the default shell, but a stock Ubuntu/Debian server has only
# bash — so without this the oh-my-zsh install below is skipped and the managed
# ~/.zshrc never runs. ensure_tool no-ops on machines that already have zsh.
# Setting zsh as the *login* shell (chsh) is left to the provisioning driver
# (mvserver-init) so this apply never has to prompt for a password.
ensure_tool zsh zsh

# --- oh-my-zsh (keep our chezmoi-managed .zshrc) ---
if [ ! -d "$HOME/.oh-my-zsh" ]; then
	if command -v zsh >/dev/null 2>&1; then
		log "installing oh-my-zsh"
		RUNZSH=no CHSH=no KEEP_ZSHRC=yes \
			sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended \
			|| log "oh-my-zsh install failed"
	else
		log "zsh not found; skipping oh-my-zsh"
	fi
fi

# --- external zsh plugins ---
ZSH_CUSTOM="${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}"
if [ -d "$HOME/.oh-my-zsh" ]; then
	for spec in \
		"zsh-autosuggestions https://github.com/zsh-users/zsh-autosuggestions" \
		"zsh-syntax-highlighting https://github.com/zsh-users/zsh-syntax-highlighting"; do
		name=${spec%% *}
		url=${spec#* }
		dir="$ZSH_CUSTOM/plugins/$name"
		if [ ! -d "$dir" ]; then
			log "cloning $name"
			git clone --depth=1 "$url" "$dir" || log "$name clone failed"
		fi
	done
fi

# --- rustup (stable toolchain; the owner wants rustup, not a mise-pinned rust) ---
if ! command -v rustup >/dev/null 2>&1 && [ ! -x "$HOME/.cargo/bin/rustup" ]; then
	log "installing rustup (stable)"
	curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path \
		|| log "rustup install failed"
fi

# --- tokei (cargo-only upstream; built with rustup's cargo, off mise — ADR-0007) ---
# tokei stopped publishing prebuilt binaries after v12, so mise's only backend is
# `cargo:` (source build). Rather than let mise drive cargo, build it here with the
# rustup toolchain installed just above. One-time and slow; idempotent on PATH.
if ! command -v tokei >/dev/null 2>&1; then
	# rustup's installer ran with --no-modify-path, so cargo isn't on PATH yet
	# this run — source its env (no-op if rustup install was skipped/failed).
	# shellcheck disable=SC1091  # generated by rustup at apply time
	[ -f "$HOME/.cargo/env" ] && . "$HOME/.cargo/env"
	if command -v cargo >/dev/null 2>&1; then
		# A cargo build needs a C linker (`cc`). The base server image has none
		# until a later apt package drags in build-essential, and this block runs
		# before those — so ensure the toolchain here (Linux; macOS uses Xcode CLT).
		if [ "$(uname -s)" != "Darwin" ] && ! command -v cc >/dev/null 2>&1; then
			log "installing build-essential (C toolchain for the tokei build)"
			pm_install build-essential || log "build-essential install failed"
		fi
		log "installing tokei (cargo build — one-time, slow)"
		cargo install tokei --locked || log "tokei cargo install failed"
	else
		log "cargo not found; skipping tokei (install rustup, then re-run apply)"
	fi
fi

# --- fnm (Node version manager; not in mise's registry) ---
if ! command -v fnm >/dev/null 2>&1 && [ ! -x "$HOME/.local/bin/fnm" ]; then
	# fnm's official installer is a bash script (arrays, [[ ]]), so /bin/sh
	# can't run it. Minimal images (e.g. Alpine) ship only sh — ensure bash
	# first so Node provisioning isn't silently skipped there.
	if ! command -v bash >/dev/null 2>&1; then
		log "bash not found (fnm's installer requires it) — installing bash"
		pm_install bash || log "bash install failed"
	fi
	# fnm's installer unzips its release archive; a stock server image has no
	# unzip, and without it the installer bails ("missing dependencies") and Node
	# is silently skipped. macOS ships unzip; this is a no-op there.
	if ! command -v unzip >/dev/null 2>&1; then
		log "installing unzip (fnm's installer needs it)"
		pm_install unzip || log "unzip install failed"
	fi
	if command -v bash >/dev/null 2>&1; then
		log "installing fnm"
		curl -fsSL https://fnm.vercel.app/install | bash -s -- --install-dir "$HOME/.local/bin" --skip-shell \
			|| log "fnm install failed"
	else
		log "bash unavailable; skipping fnm (install bash, then re-run apply)"
	fi
fi

# --- pay-respects (the Rust `thefuck` replacement; not in mise's registry) ---
# Its official installer drops a prebuilt binary into ~/.local/bin (already first
# on PATH), so no cargo build and no brew dependency. The eval that wires up the
# `f` alias lives in dot_mvdotfiles.zsh (interactive-only). Idempotent on PATH.
if ! command -v pay-respects >/dev/null 2>&1 && [ ! -x "$HOME/.local/bin/pay-respects" ]; then
	# Its installer extracts a .tar.zst or .zip; a stock server image may have
	# neither zstd nor unzip. The fnm block above already ensures unzip, which the
	# installer falls back to, so Node and pay-respects share that prerequisite.
	log "installing pay-respects"
	curl -sSfL https://raw.githubusercontent.com/iffse/pay-respects/main/install.sh \
		| sh -s -- --bin-dir "$HOME/.local/bin" \
		|| log "pay-respects install failed"
fi

# --- non-registry CLI tools (not in mise's registry; brew-if-present on macOS, ADR-0008) ---
ensure_tool http httpie
ensure_tool mosh mosh
# neovim: the default $EDITOR (dot_mvdotfiles.zsh), configured with LazyVim via
# ~/.config/nvim (dot_config/nvim/**). LazyVim needs a recent nvim (>= 0.10), but
# Linux distro repos lag badly (Ubuntu 24.04 ships 0.9.5 and the neovim-ppa has no
# 24.04 build), so on Linux we install neovim's official static build into
# /usr/local — a global editor for every user, current, and distro-agnostic.
# macOS gets a current nvim from brew-if-present. Not in mise: we want it global,
# not shimmed onto one user's PATH.
if [ "$(uname -s)" = "Darwin" ]; then
	ensure_tool nvim neovim
else
	case "$(uname -m)" in
		x86_64|amd64) nvim_asset="nvim-linux-x86_64" ;;
		aarch64|arm64) nvim_asset="nvim-linux-arm64" ;;
		*) nvim_asset="" ;;
	esac
	if [ -z "$nvim_asset" ]; then
		log "neovim: unsupported arch $(uname -m); skipping"
	else
		# Resolve the latest release tag from the redirect (no API token needed).
		nvim_latest=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
			https://github.com/neovim/neovim/releases/latest 2>/dev/null | sed 's#.*/tag/##')
		nvim_cur=$(nvim --version 2>/dev/null | head -1 | awk '{print $2}')
		if [ -z "$nvim_latest" ]; then
			log "neovim: couldn't resolve latest release (offline?); leaving current install"
		elif [ "$nvim_cur" = "$nvim_latest" ]; then
			: # already current
		else
			nvim_sudo=""
			[ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1 && nvim_sudo="sudo"
			log "installing neovim $nvim_latest (global, /usr/local/bin)"
			nvim_tmp=$(mktemp -d)
			if curl -fsSL "https://github.com/neovim/neovim/releases/download/$nvim_latest/$nvim_asset.tar.gz" \
				-o "$nvim_tmp/nvim.tar.gz"; then
				if $nvim_sudo rm -rf "/opt/$nvim_asset" \
					&& $nvim_sudo tar -xzf "$nvim_tmp/nvim.tar.gz" -C /opt \
					&& $nvim_sudo ln -sf "/opt/$nvim_asset/bin/nvim" /usr/local/bin/nvim; then
					: # installed
				else
					log "neovim install failed"
				fi
			else
				log "neovim download failed"
			fi
			rm -rf "$nvim_tmp"
		fi
	fi
fi

# nano: macOS ships `/usr/bin/nano` as a symlink to pico, which has no syntax
# highlighting — so `command -v nano` is misleading and ensure_tool would skip
# it. Install real GNU nano from brew-if-present explicitly (idempotent via
# `brew list`). On Linux `nano` is already GNU nano; ensure it's present for
# minimal/headless boxes. Highlighting is wired up by ~/.nanorc (dot_nanorc.tmpl).
if [ "$(uname -s)" = "Darwin" ]; then
	if command -v brew >/dev/null 2>&1; then
		if ! brew list nano >/dev/null 2>&1; then
			log "installing nano (brew; macOS /usr/bin/nano is pico, no highlighting)"
			brew install nano || log "nano brew install failed"
		fi
	else
		log "nano: no Homebrew on this Mac — run 'brew install nano' for GNU nano (system nano is pico)"
	fi
else
	ensure_tool nano nano
fi

# btop's mise (aqua) package is linux-only, so mise installs it on the Linux
# fleet; on macOS it comes from brew-if-present here instead (ADR-0008).
[ "$(uname -s)" = "Darwin" ] && ensure_tool btop btop

# --- 1Password CLI (macOS-only cask; the Linux servers don't get the secret SSH
# config that needs it — ADR-0016). `op` is what chezmoi's onepasswordDocument
# shells out to when rendering ~/.ssh/config on desktops; enable the app's
# "Integrate with 1Password CLI" setting once so it unlocks without a manual signin. ---
ensure_cask 1password-cli

# --- fonts (macOS-only Homebrew casks; the Linux fleet is headless servers — ADR-0009) ---
ensure_cask font-monaspace-nf
ensure_cask font-symbols-only-nerd-font
ensure_cask font-jetbrains-mono-nerd-font
ensure_cask font-fira-code-nerd-font
