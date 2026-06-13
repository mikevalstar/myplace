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

# --- git (prerequisite for chezmoi and the clones below; not a mise tool) ---
# chezmoi's built-in git can clone the source repo on a machine with no system
# git, so this can run first and install real git before it's needed below.
if ! command -v git >/dev/null 2>&1; then
	if [ "$(uname -s)" = "Darwin" ]; then
		log "git not found — install the Command Line Tools: xcode-select --install"
	else
		sudo=""
		[ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1 && sudo="sudo"
		log "installing git"
		if command -v apt-get >/dev/null 2>&1; then
			$sudo apt-get update -qq && $sudo apt-get install -y git || log "git install failed"
		elif command -v dnf >/dev/null 2>&1; then
			$sudo dnf install -y git || log "git install failed"
		elif command -v yum >/dev/null 2>&1; then
			$sudo yum install -y git || log "git install failed"
		elif command -v pacman >/dev/null 2>&1; then
			$sudo pacman -S --noconfirm git || log "git install failed"
		elif command -v apk >/dev/null 2>&1; then
			$sudo apk add git || log "git install failed"
		else
			log "no known package manager; install git manually"
		fi
	fi
fi

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

# --- fnm (Node version manager; not in mise's registry) ---
if ! command -v fnm >/dev/null 2>&1 && [ ! -x "$HOME/.local/bin/fnm" ]; then
	log "installing fnm"
	curl -fsSL https://fnm.vercel.app/install | bash -s -- --install-dir "$HOME/.local/bin" --skip-shell \
		|| log "fnm install failed"
fi
