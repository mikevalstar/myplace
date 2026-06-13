# MYPLACE_INTERACTIVE_SHELL is computed in .zshrc (1 = real terminal, 0 =
# agent/CI shell). The prompt, history, cd-hooks and keybindings below are
# interactive-only and their init chatter pollutes an agent's captured stdout,
# so gate them. Default to 1 if unset (e.g. this file sourced standalone) so a
# human shell never loses its tooling. See docs/guides/agent-friendly-shell.md.
if [[ "${MYPLACE_INTERACTIVE_SHELL:-1}" == 1 ]]; then

## Starship
if [[ -x "$(command -v starship)" ]]; then
    eval "$(starship init zsh)"
else
    echo "starship not found, and not setup" >&2
fi

## zoxide setup for better cd
if [[ -x "$(command -v zoxide)" ]]; then
    eval "$(zoxide init --cmd cd zsh)"
else
    echo "zoxide not found, and not setup" >&2
fi

## atuin
if [[ -x "$(command -v atuin)" ]]; then
    eval "$(atuin init zsh)"
else
    echo "atuin not found, and not setup" >&2
fi

## fzf
if [[ -x "$(command -v fzf)" ]]; then
    source <(fzf --zsh)
else
    echo "fzf not found, and not setup" >&2
fi

fi  # end interactive-only block

## add scripts folder to the path
export PATH="$PATH:$HOME/.config/scripts"

## FZF
# $HOME 
export FZF_COMPLETION_TRIGGER='~~'
export FZF_DEFAULT_COMMAND="fd --hidden --follow . "
export FZF_CTRL_T_COMMAND="$FZF_DEFAULT_COMMAND"
export FZF_CTRL_T_OPTS="--preview 'bat --color=always --style=header,grid --line-range :500 {}' --bind 'ctrl-/:change-preview-window(down|hidden|)'"
export FZF_ALT_C_COMMAND="fd --hidden --follow -t d . "

# Use fd (https://github.com/sharkdp/fd) for listing path candidates.
# - The first argument to the function ($1) is the base path to start traversal
# - See the source code (completion.{bash,zsh}) for the details.
_fzf_compgen_path() {
  fd --hidden --follow . "$1"
}

# Use fd to generate the list for directory completion
_fzf_compgen_dir() {
  fd --type d --hidden --follow . "$1"
}

## Terminal Alacritty
fpath+=${ZDOTDIR:-~}/.zsh_functions

## Random ENV Vars
export TEALDEER_CONFIG_DIR="$HOME/.config/tealdeer"

### Aliases / configs

cd_to_dir() {
    local selected_dir
    if [[ -z "$1" ]]; then
        selected_dir=$(fd -t d . $HOME | fzf +m --height 50% --preview 'tree -C {}')
    else
        selected_dir=$(fd -t d . $HOME "$1" | fzf +m --height 50% --preview 'tree -C {}')
    fi

    if [[ -n "$selected_dir" ]]; then
        # Change to the selected directory
        cd "$selected_dir" || return 1
    fi
}

### fix alacritty not in path 
if [[ ! -x "$(command -v alacritty)" ]]; then
    if [[ -d "/Applications/Alacritty.app/Contents/MacOS" ]]; then
        export PATH="$PATH:/Applications/Alacritty.app/Contents/MacOS"
    fi
fi

## BJourn
export BJOURN_USAGE="0"

# Quick helpful items
alias c="clear"
alias doit="sudo !!"
alias genpass="openssl rand -base64 20"
alias sha='shasum -a 256 '
alias pn="pnpm"
alias vim="nvim"
alias lvim="NVIM_APPNAME=LazyVim nvim"
alias cdd="cd_to_dir"
alias bj="bjourn"

# Next level of an ls 
# options :  --no-filesize --no-time --no-permissions --no-user --color=always --icons=always
export EZA_CONFIG_DIR="$HOME/.config/eza"
alias lss="eza --long " 

## Some ENV things
export EDITOR="nvim"