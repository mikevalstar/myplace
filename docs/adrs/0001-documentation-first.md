---
title: ADR-0001 — Documentation-first development
status: accepted
created: 2026-06-12
updated: 2026-06-12
tags: [process, docs]
supersedes: null
superseded-by: null
---

# ADR-0001: Documentation-first development

## Context

`myplace` will be developed incrementally, largely with AI-assisted tooling, and will run on machines the author touches infrequently (servers, work Mac). Decisions made early — tool choices, workflow shapes, what the TUI delegates to chezmoi/mise versus owns itself — need to stay legible months later, and AI agents working in the repo need durable context beyond the code.

## Options considered

### Option A — Code-first, document later

Fastest to start. In practice "later" rarely arrives, and rationale is lost; agents and future-me re-litigate settled decisions.

### Option B — Documentation-first

Write ADRs, feature specs, and workflows before or alongside implementation, in a structured `docs/` tree with frontmatter for searchability. Slower per feature, but decisions stay traceable and docs become the working spec.

## Decision

Option B. The repo carries a `docs/` tree with four sections — `adrs/`, `features/`, `workflows/`, `guides/` — each with a `_template.md`. All docs carry YAML frontmatter (`title`, `status`, `created`, `updated`, `tags`, plus type-specific fields). ADRs are numbered and immutable: changes are made by superseding, not editing.

## Consequences

- Every significant choice (e.g. TUI language/framework) gets an ADR before code exists.
- New features start as a spec in `features/`, usually paired with a workflow doc.
- Frontmatter makes the docs greppable/filterable, and could later drive tooling (e.g. a docs index or status dashboard).
- Small overhead per change; accepted as the cost of long-term legibility.
