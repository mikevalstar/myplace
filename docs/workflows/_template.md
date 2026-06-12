---
title: Workflow name (verb phrase, e.g. "Bootstrap a new machine")
status: draft # draft | active | deprecated
created: 2026-01-01
updated: 2026-01-01
tags: []
actors: [user] # who/what participates: user, tui, chezmoi, mise, server
---

# Workflow name

## Goal

What the user is trying to accomplish, in one sentence.

## Preconditions

What must be true before this workflow starts (e.g. "git and curl are installed", "machine has network access", "chezmoi repo exists").

## Steps

1. Each step from the user's point of view, noting what the TUI does underneath.
2. Include the actual commands orchestrated (e.g. `chezmoi init --apply <repo>`, `mise install`).
3. Note decision points and branches ("if dotfiles repo not configured, prompt for URL").

## Outcome

The end state when the workflow succeeds — what changed on the machine, what the user sees.

## Failure modes

| What can go wrong | How the user finds out | Recovery |
|-------------------|------------------------|----------|
| e.g. network unreachable | error panel with retry option | retry / skip step |

## Related

- Features that implement this workflow, relevant ADRs.
