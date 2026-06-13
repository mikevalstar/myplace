---
title: Agent-friendly shell init
status: active
created: 2026-06-13
updated: 2026-06-13
tags: [shell, zsh, agents, dotfiles]
audience: both
---

# Agent-friendly shell init

## Purpose

Keep the managed zsh dotfiles quiet and correct when a coding agent (Claude
Code, aider, Cursor, Codex, …) sources them, without changing the experience
in a real interactive terminal.

## Background

Coding agents run shell commands through a **non-interactive** shell, and most
of them — Claude Code included — source the user's `~/.zshrc` once at startup to
capture `PATH` and environment. That means every `eval "$(<tool> init zsh)"` in
our dotfiles also runs for the agent.

Two classes of problem follow:

- **Noise** — init banners and per-`cd` messages get mixed into the stdout the
  agent captures from a command. The biggest offenders here are fnm's
  `--use-on-cd` hook ("Using Node vX.Y.Z" on every `cd`) and our own
  `echo "... not found, and not setup"` fallbacks firing on hosts where a tool
  isn't installed.
- **Correctness** — zoxide's `--cmd cd` replaces `cd` with frecency jumping. An
  agent that runs `cd src` when there is no `./src` silently lands in some
  *remembered* `src` elsewhere and never knows it moved.

## The guide

Agents almost always run a **non-interactive** shell, while a human terminal is
interactive. That single signal — plus explicit per-agent env vars for the
agents that *do* run an interactive shell — gates everything that should be
human-only.

Compute the flag once in `.zshrc`, before sourcing the rest, and export it so
both `.zshrc` and `.mvdotfiles.zsh` share it:

```zsh
if [[ -o interactive && -z "$CLAUDECODE$AI_AGENT$CURSOR_AGENT$CODEX_SANDBOX$OPENCODE" ]]; then
  MYPLACE_INTERACTIVE_SHELL=1
else
  MYPLACE_INTERACTIVE_SHELL=0
fi
export MYPLACE_INTERACTIVE_SHELL
```

Then gate the prompt, shell-history, `cd`-hook and keybinding tooling on it;
leave `PATH` and runtime resolution **un**gated, because agents legitimately
want mise / fnm / cargo tools resolved.

```zsh
if [[ "${MYPLACE_INTERACTIVE_SHELL:-1}" == 1 ]]; then   # :-1 → never strip a human shell's tooling
    eval "$(starship init zsh)"
    eval "$(zoxide init --cmd cd zsh)"
    eval "$(atuin init zsh)"
    source <(fzf --zsh)
fi
```

For fnm, keep Node on `PATH` for everyone but only add the per-`cd` hook for
humans:

```zsh
if command -v fnm >/dev/null 2>&1; then
  if [[ "$MYPLACE_INTERACTIVE_SHELL" == 1 ]]; then
    eval "$(fnm env --use-on-cd --shell zsh)"   # human: auto-switch on cd
  else
    eval "$(fnm env --shell zsh)"               # agent: PATH only, no hook/prints
  fi
fi
```

Send the "not found" fallbacks to **stderr** so they never land in captured
stdout: `echo "zoxide not found, and not setup" >&2`.

### The guard, decoded

The concatenation `"$CLAUDECODE$AI_AGENT$CURSOR_AGENT$CODEX_SANDBOX$OPENCODE"`
is empty only when *all* of them are unset, so `-z` on it means "no known agent
var present."

- `[[ -o interactive ]]` — the portable, agent-agnostic signal. Works for any
  agent that runs a non-interactive shell. Codex (`codex exec`) and opencode
  (non-TTY) are caught here with no env var needed.
- `CLAUDECODE` / `AI_AGENT` — Claude Code sets both (`CLAUDECODE=1`,
  `AI_AGENT=claude-code_<ver>_agent`).
- `CURSOR_AGENT` — Cursor sets this; Cursor's own docs recommend keying your rc
  off it to skip prompts/themes. **Required**, because Cursor's terminal loads
  rc/themes (it isn't purely non-interactive), so the interactivity gate alone
  won't catch it.
- `CODEX_SANDBOX` — Codex exports this (`=seatbelt` on macOS, the default
  sandbox) to child processes. Backup for the rare interactive-Codex case.
- `OPENCODE` — opencode runs non-TTY so the interactivity gate covers it today;
  this var is requested upstream ([sst/opencode#1775]) but **not yet shipped** —
  included forward-compat so it just works if/when it lands.

[sst/opencode#1775]: https://github.com/sst/opencode/issues/1775

Live implementation: [home/dot_zshrc](../../home/dot_zshrc) and
[home/dot_mvdotfiles.zsh](../../home/dot_mvdotfiles.zsh).

## Gotchas

- **The rc runs even though `$-` has no `i`.** Don't assume a non-interactive
  shell skips `.zshrc` — Claude Code sources it explicitly to snapshot the env.
  `[[ -o interactive ]]` is what actually distinguishes the two, not "was the rc
  sourced at all."
- **Don't gate PATH/runtime setup.** If you wrap mise/fnm/cargo PATH resolution
  in the interactive guard, agents lose access to those tools entirely. Only the
  prompt/history/cd-hook/keybinding layer goes inside the guard.
- **There is no universal "disable my shell" env var.** Each agent has its own
  (`CLAUDECODE`/`AI_AGENT`, `CURSOR_AGENT`, `CODEX_SANDBOX`) or none yet
  (`OPENCODE`). Interactivity is the only signal that generalizes, so it must be
  the primary gate; the env vars only matter for agents that run an
  *interactive* shell (Cursor today). Verify any new agent two ways: run
  `env | grep -i <agent>` inside it, and check `[[ -o interactive ]]`.
- **zoxide is a correctness bug for agents, not just noise** — it can change
  which directory a command runs in. Treat disabling its `cd` override under an
  agent as mandatory, not cosmetic.

## References

- [docs/guides/managed-setup.md](managed-setup.md) — how tools/dotfiles are added.
- [ADR-0006](../adrs/0006-agent-runnable-commands.md) — every command is
  agent-runnable; a quiet shell is the same principle applied to the rc.
- zoxide: <https://github.com/ajeetdsouza/zoxide>
- fnm: <https://github.com/Schniz/fnm>
