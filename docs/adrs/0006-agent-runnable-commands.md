---
title: ADR-0006 — Every command is agent-runnable (non-interactive contract)
status: accepted
created: 2026-06-12
updated: 2026-06-12
tags: [cli, automation, ai-agents, headless, json]
supersedes: null
superseded-by: null
---

# ADR-0006: Every command is agent-runnable (non-interactive contract)

## Context

myplace is increasingly driven by automation, not just a human at a keyboard: AI agents (like the one building it), cron jobs, SSH fan-out across a fleet, and phase 2's reporting all need to run commands with **no human in the loop**. A command that blocks on a prompt is invisible to an agent — it just hangs until timeout. We already hit this exactly: a `chezmoi` overwrite prompt grabbed the TTY and froze the TUI.

The [headless CLI feature spec](../features/headless-cli-and-json-output.md) established `--json` and exit codes pragmatically. This ADR promotes that into a **binding architectural rule** that every current and future command must satisfy — so "can an agent run this?" is answered at design time, not discovered when something hangs.

## Decision

**Every command MUST have an invocation that runs to completion with zero required human interaction.** Concretely, for all commands — including ones added later:

1. **A non-interactive path always exists.** Decisions a human would be prompted for are instead supplied up front via flags (`--yes`, `--repo`, `--profile`, and for future decision points `--on-local-edits=keep|discard|skip`-style flags). An agent can always pre-answer.
2. **Never block waiting for input that won't come.** When stdin/stdout isn't a TTY (detected, not assumed) and a required decision wasn't supplied by flag, the command **fails fast with exit code 3 and a message naming the exact flag** that resolves it — it never prompts and never hangs.
3. **No subprocess may capture the terminal.** Shelled-out tools run with `--no-tty`-style flags and a closed stdin so *their* prompts also fail fast rather than hang the parent. (This is why ADR-0006 exists as much as the flags do.)
4. **Structured output on demand.** Every data-producing command accepts `--json`: exactly one JSON document on stdout (logs/progress to stderr), with a `schema` field. Agents parse stdout; humans read stderr.
5. **The exit code carries the verdict.** Success/drift/unknown/error map to documented codes so an agent branches on `$?` without parsing the body.
6. **Safety is preserved via defaults, not prompts.** Where an unattended action would be dangerous (e.g. auto-pushing unreviewed local edits), the *default* non-interactive behavior is the safe subset (converge incoming + report what was skipped), and the riskier action requires an explicit flag. "Agent-runnable" means *never blocks*, not *does everything unattended by default*.

This rule is a precondition for merging any new command: if you can't drive it from a script without a TTY, it isn't done.

## Consequences

- Every command is testable by an agent and in CI without a pty harness; the interactive TUI/forms become a convenience layer over an always-present non-interactive core.
- Adding a new interactive decision point obligates a matching flag (or a documented fail-fast) in the same change — interactivity can't sneak in as the only path.
- Phase 2's server reporting and cross-fleet SSH loops are free: they're just the non-interactive paths plus transport.
- The contract is enforceable in review via a checklist (non-interactive path? fails fast off-TTY? `--json`? exit code? no subprocess TTY grab?).
- Slight cost: every command carries flag plumbing even when a human would rarely use it. Accepted — it's the price of being automatable end to end.

## Related

- [Headless CLI & JSON output](../features/headless-cli-and-json-output.md) — the concrete implementation of this contract
- [ADR-0002](0002-go-and-charm-for-the-tui.md) — core logic is TUI-free, which is what makes the non-interactive path possible
- [charm-tui-stack guide](../guides/charm-tui-stack.md) — the subprocess-TTY-hang gotcha this rule guards against
