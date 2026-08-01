---
title: Share HTML plans
status: active
created: 2026-08-01
updated: 2026-08-01
tags: [scripts, plans, html, sharing, agents]
phase: 1
---

# Share HTML plans

## Summary

`shareplan` publishes a raw HTML plan to `share.valstar.dev` from a file or
standard input. It can create a new share, replace an existing share by slug,
and save the API key locally through a hidden interactive prompt.

## Motivation

AI agents often produce plans that are easier to review and share as rendered
HTML than as terminal text. Publishing should be one command, preserve the HTML
byte-for-byte, and leave the caller with both the public URL and an explicit
command for updating that same plan later.

## Scope

### In scope

- `shareplan auth` prompts without echoing and stores the key at
  `${XDG_CONFIG_HOME:-$HOME/.config}/shareplan/key` with owner-only permissions.
- `SHAREIT_PLAN_KEY` overrides the stored key for ephemeral and automated use.
- `shareplan FILE` and `cat FILE | shareplan` create a plan with `POST /api/plans`.
- `shareplan help` and `shareplan --help` expose the complete usage contract.
- `shareplan --update SLUG FILE` and its stdin equivalent replace a plan with
  `PUT /api/plans/<slug>.html`. The canonical API update URL returned by create
  is also accepted in place of `SLUG`.
- Uploads use `Content-Type: text/html; charset=utf-8`, send the key in
  `X-Share-Key`, and pass the body to curl with `--data-binary`.
- Successful output includes the public URL, slug, and a ready-to-copy update
  command. API errors include the service's `error` message when available.

### Out of scope

- Editing, rendering, validating, listing, or deleting plans.
- Managing the sharing service or synchronizing the key through chezmoi.
- Accepting arbitrary update hosts or paths, which could disclose the key.

## Behavior

Run `shareplan auth` once on a machine and paste the shared key into the hidden
prompt. The key is machine-local and must never be committed or printed. An
existing `SHAREIT_PLAN_KEY` environment variable takes precedence without
changing the stored key.

Create from a file or pipeline:

```sh
shareplan plan.html
cat plan.html | shareplan
```

Update using the slug printed by the create command:

```sh
shareplan --update Ab3def4Gh5jk plan.html
cat plan.html | shareplan --update Ab3def4Gh5jk
```

The command fails before making a request when no key or non-empty HTML input is
available, a file is unreadable, or an update target is not a simple slug or a
canonical `https://share.valstar.dev/api/plans/<slug>.html` URL. In particular,
an agent shell whose standard input is a non-TTY but has no bytes must not create
an empty plan when the agent runs bare `shareplan` while discovering the tool.

## Acceptance criteria

- [x] The helper is deployed as executable `~/.mvscripts/shareplan` and appears
  in `mv_scripts` discovery.
- [x] Authentication input is hidden, stored outside chezmoi-managed files, and
  protected with directory mode `0700` and file mode `0600`.
- [x] File and stdin creates send unchanged raw HTML and recognize HTTP 201.
- [x] File and stdin updates send unchanged raw HTML and recognize HTTP 200.
- [x] Empty non-TTY stdin is rejected before any API request.
- [x] The secret is neither hardcoded nor included in normal output.
- [x] Success output teaches a human or AI agent how to update the share.
- [x] `shareplan --help` documents commands, environment, storage, and exits.
- [x] The managed inventory and README mention the installed helper and key.

## Open questions

- None.

## Related

- [ADR-0014 — Managed scripts deployed via chezmoi](../adrs/0014-managed-scripts-and-bun-runner.md)
- [Extending the managed setup](../guides/managed-setup.md)
