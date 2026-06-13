---
title: LLM-friendly help output (self-describing CLI)
status: draft
created: 2026-06-13
updated: 2026-06-13
tags: [cli, llm, agent, headless, discoverability]
phase: 1
---

# LLM-friendly help output (self-describing CLI)

## Summary

A single command — `myplace help --json` (machine manifest) and a concise `myplace help --llm` (prose brief) — emits the entire command surface in a form an LLM or agent can consume in one shot: every command, its flags and defaults, which flags are required for non-interactive use, the exit-code semantics, and a pointer to each command's JSON output shape. An agent learns to drive every myplace command from one read, instead of scraping a dozen `--help` screens.

## Motivation

ADR-0006 commits to *every command being agent-runnable* — a fully non-interactive path that fails fast (exit 3) naming the missing flag rather than hanging. But "runnable" assumes the agent already knows the flags. Today an agent has to scrape human `--help` text, which is written for people: prose, ANSI, no exit-code table, no statement of which flag unblocks a non-interactive run, no link to the JSON schema it'll get back. That's the discovery gap ADR-0006 left open.

This feature closes it from the tool's side. The CLI describes *itself* in a structured, schema-versioned document, generated from the actual command tree so it can never drift from the real flags. An agent (or a script, or a human piping into an LLM) fetches the manifest once and knows: here are the commands, here is the headless form of each, here is what `$?` will mean, here is the shape of the JSON I'll parse. It is the natural companion to the headless contract — that doc defines the conventions; this command advertises them per-command, machine-first.

It also helps humans: `myplace help --llm` produces a compact, ANSI-free, copy-pasteable brief of the whole tool — exactly what you'd drop into an LLM context window or a runbook.

## Scope

### In scope

- `myplace help --json`: one JSON document describing all commands, generated from the cobra command tree (single source of truth — no hand-maintained list).
- `myplace help --llm`: a concise, structured plain-text/markdown brief of the same information, ANSI-free, sized for an LLM context (the "llms.txt"-style flavor).
- Per-command coverage: name, one-line purpose, flags (name, shorthand, type, default, required), the canonical headless invocation, exit-code meanings, and a reference to the command's JSON output schema (the owning feature/workflow doc).
- Schema versioning (`"schema": 1`) and stream discipline matching the [headless contract](headless-cli-and-json-output.md).

### Out of scope

- Embedding the full JSON Schema of each command's *output* document inline. v1 references the owning doc by path; inlining schemas can come later if an agent genuinely needs to validate without fetching docs.
- An MCP server or any RPC transport. This is discovery over plain stdout; wrapping it in a protocol is a separate decision.
- Replacing cobra's normal `--help` for humans. The default `--help` stays as-is; `--json`/`--llm` are additive views.

## Behavior

`myplace help` with no flag behaves like cobra's normal help. The two new flags change the *format*, not the content source — both are rendered from the same in-memory command tree plus per-command annotations.

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
      "headless": "myplace bootstrap --repo <url> --profile <name> --yes",
      "interactive": true,
      "flags": [
        { "name": "repo", "type": "string", "required_for_headless": true,
          "description": "dotfiles git repo URL" },
        { "name": "profile", "type": "string", "required_for_headless": true,
          "description": "personal-mac | work-mac | server" },
        { "name": "yes", "type": "bool", "default": false,
          "description": "assume yes; required off a TTY" }
      ]
    }
  ]
}
```

The `required_for_headless` flag on a parameter is the machine-readable form of ADR-0006's promise: it tells an agent exactly which flags it must supply to avoid an exit-3 "needs a decision" failure.

### `myplace help --llm`

The same data as a compact brief — no ANSI, stable headings, examples inline — suitable to paste into an LLM prompt or a runbook:

```console
$ myplace help --llm
# myplace (v0.2.0-dev)
Bootstrap and keep machines in sync via chezmoi and mise.

Exit codes: 0 in-sync/success · 1 drifted/partial · 2 unknown · 3 error or missing-decision.
All data commands accept --json (one document on stdout, logs on stderr).

## status — report drift in both directions
  headless: myplace status --json
  output:   see docs/features/headless-cli-and-json-output.md
  exits:    0 in-sync · 1 drifted · 2 unknown · 3 error

## bootstrap — bring a fresh machine to known-good
  headless: myplace bootstrap --repo <url> --profile <name> --yes
  required off a TTY: --repo, --profile, --yes
...
```

### Generation, not maintenance

Both views are walked from cobra's command tree at runtime, so commands and flags are never listed twice. The pieces cobra doesn't know — per-command exit-code meaning, the output-schema doc path, and `required_for_headless` — live in each command's cobra `Annotations`, set right where the command is defined. A new command shows up in the manifest automatically; only its annotations need filling in, and a test enforces that they are.

## Acceptance criteria

- [ ] `myplace help --json | jq .` emits exactly one document with `"schema": 1`; logs go to stderr.
- [ ] Every command in the cobra tree appears in the manifest, with its flags, types, and defaults matching the actual flag definitions (verified by a test that walks the tree).
- [ ] Each command that has a non-interactive path lists its `required_for_headless` flags, matching ADR-0006's fail-fast behavior.
- [ ] `myplace help --llm` produces ANSI-free output even when stdout is a TTY.
- [ ] A test fails if a command is missing required annotations (exit-code meaning and, for data commands, an output-schema reference), so the manifest can't silently go stale.
- [ ] Lives in the CLI layer rendering from the command tree; introduces no TUI imports into core (ADR-0002).

## Open questions

- Should this be `myplace help --json` (a flag on help) or a dedicated `myplace describe` / top-level `--describe`? Leaning `help --json/--llm` so there's one obvious place to look.
- Do we inline each command's *output* JSON shape eventually, or keep referencing the owning doc? Start with references; inline only if an agent demonstrably needs offline validation.
- Worth shipping the `--llm` brief as a generated `llms.txt` artifact in releases too, for tools that fetch it from the repo without running the binary?

## Related

- [ADR-0006](../adrs/0006-agent-runnable-commands.md) — the agent-runnable contract this makes discoverable
- [Headless CLI with JSON output](headless-cli-and-json-output.md) — the envelope, exit codes, and stream rules this advertises per-command
- [Preflight diagnostics](doctor-preflight-diagnostics.md) — a sibling machine-first command an agent would discover here
