---
title: LLM-friendly help output (self-describing CLI)
status: active
created: 2026-06-13
updated: 2026-06-13
tags: [cli, llm, agent, headless, discoverability]
phase: 1
---

# LLM-friendly help output (self-describing CLI)

## Summary

Two views on the help command emit the entire command surface in a form an LLM or agent can consume in one shot: `myplace help --json` (a machine manifest) and `myplace help --llm` (a copy-paste brief). Between them they describe every command, its flags and defaults, which flags are required for non-interactive use, the exit-code semantics, a pointer to each command's JSON output shape, **and the end-to-end workflow recipes** (bootstrap a server, sweep status across a fleet, update headlessly). An agent learns to *drive* myplace — not just enumerate its flags — from one read, instead of scraping a dozen `--help` screens.

The design follows what `agent-browser` does well for agents: the ordinary, human `--help` is itself the discovery hook — it opens with a one-line "AI agents: start here" pointer to the two machine-first views, so an agent that reflexively runs `--help` is routed to the right surface immediately rather than parsing prose.

## Motivation

ADR-0006 commits to *every command being agent-runnable* — a fully non-interactive path that fails fast (exit 3) naming the missing flag rather than hanging. But "runnable" assumes the agent already knows the flags. Today an agent has to scrape human `--help` text, which is written for people: prose, ANSI, no exit-code table, no statement of which flag unblocks a non-interactive run, no link to the JSON schema it'll get back. That's the discovery gap ADR-0006 left open.

This feature closes it from the tool's side. The CLI describes *itself* in a structured, schema-versioned document, generated from the actual command tree so it can never drift from the real flags. An agent (or a script, or a human piping into an LLM) fetches the manifest once and knows: here are the commands, here is the headless form of each, here is what `$?` will mean, here is the shape of the JSON I'll parse. It is the natural companion to the headless contract — that doc defines the conventions; this command advertises them per-command, machine-first.

It also helps humans: `myplace help --llm` produces a compact, ANSI-free, copy-pasteable brief of the whole tool — exactly what you'd drop into an LLM context window or a runbook.

## Scope

### In scope

- `myplace help --json`: one JSON document describing all commands, generated from the cobra command tree (single source of truth — no hand-maintained list).
- `myplace help --llm`: a concise, structured plain-text/markdown brief of the same information, ANSI-free, sized for an LLM context (the "llms.txt"-style flavor), **including a short Recipes section** that shows the real end-to-end flows as copy-paste blocks.
- A one-line **"AI agents: start here" pointer** in the ordinary `myplace help` / `myplace --help` output, naming the two machine-first views so the surface is self-advertising.
- Per-command coverage: name, one-line purpose, flags (name, shorthand, type, default, required), the canonical headless invocation, exit-code meanings, and a reference to the command's JSON output schema (the owning feature/workflow doc).
- Also describing `myplace help` itself and bare `myplace`, so the manifest is complete (an agent sees the no-arg behavior, including the off-TTY status fallback).
- Schema versioning (`"schema": 1`) and stream discipline matching the [headless contract](headless-cli-and-json-output.md).

### Out of scope

- Embedding the full JSON Schema of each command's *output* document inline. v1 references the owning doc by path; inlining schemas can come later if an agent genuinely needs to validate without fetching docs.
- An MCP server or any RPC transport. This is discovery over plain stdout; wrapping it in a protocol is a separate decision.
- Replacing cobra's normal `--help` for humans. The default `--help` stays as-is; `--json`/`--llm` are additive views.

## Behavior

`myplace help` with no flag behaves like cobra's normal help, with one addition: the root command's long description opens with a single agent-pointer line —

```
AI agents: run `myplace help --llm` for a copy-paste brief of every command, or
`myplace help --json` for a machine-readable manifest.
```

so the human help screen (and `myplace --help`, and the usage shown on errors) is the discovery hook. The two new flags change the *format*, not the content source — both are rendered from the same in-memory command tree plus per-command annotations. `myplace help <command> --json|--llm` narrows either view to a single command.

### `myplace help --json`

One document on stdout, logs to stderr, `"schema": 1`:

```console
$ myplace help --json
{
  "schema": 1,
  "tool": "myplace",
  "version": "0.2.0-dev",
  "summary": "Bootstrap and keep machines in sync via chezmoi and mise.",
  "exit_codes": {
    "0": "in sync / success",
    "1": "drifted / completed with per-item failures",
    "2": "unknown (e.g. offline)",
    "3": "error, or a needed decision was not supplied non-interactively"
  },
  "commands": [
    {
      "name": "status",
      "summary": "Report drift in both directions for this machine.",
      "headless": "myplace status --json",
      "interactive": true,
      "flags": [
        { "name": "json", "type": "bool", "default": false,
          "description": "emit one JSON document on stdout" }
      ],
      "output_schema": "docs/features/headless-cli-and-json-output.md",
      "exit_codes": { "0": "in sync", "1": "drifted", "2": "unknown", "3": "error" }
    },
    {
      "name": "bootstrap",
      "summary": "Bring a fresh machine to a known-good state.",
      "headless": "myplace bootstrap --profile <name> --yes",
      "interactive": true,
      "required_for_headless": ["profile", "yes"],
      "flags": [
        { "name": "repo", "type": "string", "default": "https://github.com/mikevalstar/myplace.git",
          "description": "dotfiles repo (this repo, or a fork)" },
        { "name": "profile", "type": "string", "default": "", "required_for_headless": true,
          "description": "personal-mac | work-mac | server" },
        { "name": "yes", "type": "bool", "default": false, "required_for_headless": true,
          "description": "run without prompts (requires --profile)" }
      ]
    }
  ]
}
```

The `required_for_headless` flag on a parameter is the machine-readable form of ADR-0006's promise: it tells an agent exactly which flags it must supply to avoid an exit-3 "needs a decision" failure.

### `myplace help --llm`

The same data as a compact brief — no ANSI, stable headings, examples inline — suitable to paste into an LLM prompt or a runbook. It has three parts: a conventions header, the per-command reference, and a Recipes section that strings the headless forms into the real workflows:

```console
$ myplace help --llm
# myplace (v0.2.0-dev)
Bootstrap and keep machines in sync via chezmoi and mise.

Conventions
  Exit codes: 0 in-sync/success · 1 drifted/partial · 2 unknown · 3 error or missing-decision.
  All data commands accept --json: exactly one document on stdout, logs/progress on stderr.
  Off a TTY, a command needing an unsupplied decision fails fast (exit 3) naming the flag — it never prompts.
  Mutating commands (bootstrap, update, self-update) require --yes to run unattended.

## status — report drift in both directions (read-only)
  headless: myplace status --json
  output:   see docs/features/headless-cli-and-json-output.md
  exits:    0 in-sync · 1 drifted · 2 unknown · 3 error

## bootstrap — bring a fresh machine to known-good
  headless: myplace bootstrap --repo <url> --profile <name> --yes
  required off a TTY: --profile, --yes
  note: repo and git identity default to this setup's owner; --yes still needs --profile.
...

## Recipes
# Bootstrap a server unattended
myplace bootstrap --profile server --yes

# Check one host, then sweep a fleet (branch on exit code, not stdout)
myplace status --json | jq .verdict
for h in web1 web2 db1; do ssh "$h" myplace status --json >/dev/null && echo "$h ok" || echo "$h drifted"; done

# Update headlessly — converge-only; never captures/pushes local edits
myplace update --yes --json
```

The Recipes are editorial (which commands, in what order) but their invocations are the same headless forms the manifest emits; they encode the [workflows](../workflows/) an agent would otherwise have to reconstruct, with the agent-relevant gotchas inline (headless `update` is converge-only; bare `myplace` off a TTY prints status text and exits with the drift code rather than launching the TUI).

### Generation, not maintenance

Both views are walked from cobra's command tree at runtime, so commands and flags are never listed twice. The pieces cobra doesn't know — per-command exit-code meaning, the output-schema doc path, and `required_for_headless` — live in each command's cobra `Annotations`, set right where the command is defined. A new command shows up in the manifest automatically; only its annotations need filling in, and a test enforces that they are.

## Acceptance criteria

- [x] `myplace help --json | jq .` emits exactly one document with `"schema": 1`; logs go to stderr.
- [x] Every command in the cobra tree appears in the manifest, with its flags, types, and defaults matching the actual flag definitions (verified by a test that walks the tree).
- [x] Each command that has a non-interactive path lists its `required_for_headless` flags, matching ADR-0006's fail-fast behavior.
- [x] `myplace help --llm` produces ANSI-free output even when stdout is a TTY.
- [x] The ordinary `myplace help` / `myplace --help` output carries the one-line agent pointer to `--llm`/`--json`.
- [x] A test fails if a command is missing required annotations (exit-code meaning and, for data commands, an output-schema reference), so the manifest can't silently go stale.
- [x] Lives in the CLI layer rendering from the command tree; introduces no TUI imports into core (ADR-0002).

## Decisions made during implementation

- **Surface is `myplace help --json` / `--llm`, plus a pointer in plain help** — not a dedicated `describe`/`agent` verb. One obvious place to look, and the human help screen advertises it so discovery doesn't depend on already knowing the flag.
- **Annotations are the single source for the non-cobra bits.** Exit-code meanings, the `required_for_headless` set, the canonical headless invocation, the output-schema doc path, and an optional agent note live in each command's cobra `Annotations`, set at the definition site. A test walks the tree and fails if a command is missing the ones it owes, so the manifest can't drift from the code.
- **Output schemas are referenced, not inlined.** v1 points at the owning feature/workflow doc by path. Inlining each command's full output JSON Schema waits until an agent demonstrably needs offline validation.
- **The `--llm` Recipes section is editorial but invocation-faithful.** The narrative (which commands, in what order) is hand-written from the workflow docs; the commands themselves are the same headless forms the manifest generates, so they can't drift in their flags.

## Open questions

- Worth shipping the `--llm` brief as a generated `llms.txt` artifact in releases too, for tools that fetch it from the repo without running the binary? Deferred — revisit if a consumer wants it without invoking the binary.

## Related

- [ADR-0006](../adrs/0006-agent-runnable-commands.md) — the agent-runnable contract this makes discoverable
- [Headless CLI with JSON output](headless-cli-and-json-output.md) — the envelope, exit codes, and stream rules this advertises per-command
- [Preflight diagnostics](doctor-preflight-diagnostics.md) — a sibling machine-first command an agent would discover here
