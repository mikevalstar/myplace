---
title: ADR-0016 — Secret-bearing dotfiles via 1Password
status: accepted
created: 2026-06-16
updated: 2026-06-16
tags: [chezmoi, secrets, 1password, ssh, dotfiles]
supersedes: null
superseded-by: null
---

# ADR-0016: Secret-bearing dotfiles via 1Password

## Context

The dotfiles source (`home/`) lives in a **public** git repo (ADR-0003). Most
managed files are safe to publish, but some carry information we don't want
indexed by the world. The motivating case is `~/.ssh/config`: it's worth
sharing across machines, but its `HostName` lines expose server IPs and the
shape of the fleet.

`chezmoi`'s `private_` prefix only sets file *permissions* (0600) — the content
is still committed in plaintext, so it does nothing for a public repo. We need a
way to keep the *content* of a managed file out of the repo while still letting
chezmoi manage the file as tracked state (so bidirectional `status` still works).

Constraints that shaped the choice:

- The repo stays public; secrets must never land in it, even encrypted blobs we'd
  rather not advertise.
- Machines come in profiles (ADR-0003): `personal-mac`, `work-mac`, `server`.
  The SSH host list is wanted on the macs but **not** on servers.
- Every command must stay agent-runnable (ADR-0006): the converge path can't hang
  waiting for an interactive unlock on a machine that legitimately has the secret.
- 1Password is already installed first on every interactive machine (operator's
  standing practice).

## Options considered

### Option A — `Include ~/.ssh/config.local`, secret part unmanaged

Commit a public config that `Include`s a local file holding the IPs; leave the
local file out of chezmoi entirely. Zero crypto, but the sensitive part isn't
versioned or shared — it's recreated by hand on every machine.

### Option B — chezmoi `[data]` in the machine-local `chezmoi.toml`

Template the config against IPs stored in `~/.config/chezmoi/chezmoi.toml` (never
committed). Fits the existing pattern, but every machine must have the full host
list typed in at init — no shared source of truth.

### Option C — age encryption (`encrypted_` attribute)

Commit the real config encrypted with an age key. Shared + versioned + offline,
but adds an age keypair to the bootstrap story (the private key becomes a new
out-of-band secret) and publishes ciphertext we'd rather not even show.

### Option D — 1Password via chezmoi's native `onepassword*` template funcs

Store the file as a 1Password **Document**; the template pulls it at apply time
with `onepasswordDocument`. chezmoi already integrates with the `op` CLI, so
there's no custom script (keeps to the "orchestrate, don't reimplement" rule).
Secrets live in 1Password (central, rotatable, already deployed); the repo holds
only the template and the non-secret global defaults. Cost: `apply`/`status`/`diff`
shell out to `op`, so an authenticated `op` session is a soft dependency on
machines that consume the secret.

## Decision

**Option D — 1Password documents via chezmoi's `onepassword*` template
functions.** It keeps the public repo clean (no ciphertext, no plaintext),
reuses tooling already on every machine, and needs no bespoke apply script.

The profile split rides on the template, not on separate files: the secret pull
is gated behind `{{ if ne .profile "server" }}`. Because Go templates only
evaluate the branch they take, **servers never invoke `op` at all** — they render
only the non-secret global `Host *` defaults. This neatly sidesteps the headless-
server auth problem: the machines that can't easily run an interactive `op`
unlock are exactly the ones that never call it.

OS-specific keywords are gated on `.chezmoi.os`, not profile (e.g. `UseKeychain`
is Apple-openssh-only and makes Linux `ssh` error), so the same template is safe
on every OS.

## Consequences

- **Secrets stay out of the public repo** while the file remains chezmoi-managed,
  so bidirectional `status` (ADR's settled design point) still reports drift.
- **`op` becomes a soft dependency** on machines that consume the secret (the
  macs). chezmoi evaluates `onepasswordDocument` during *every* target-state
  computation — `apply`, `status`, **and** `diff` — so `myplace status` on a mac
  shells out to `op`. With the 1Password desktop app's CLI integration this is a
  cached, no-prompt session in practice; if `op` is locked/absent, chezmoi errors
  and `status` exits 3. Servers are unaffected (they never hit that branch).
- **A manual step exists per secret file**: the 1Password Document must be created
  once (it can't be bootstrapped from the repo, by design). Documented in the
  managed-setup guide.
- **Generalizes beyond SSH**: any future secret-bearing dotfile (tokens, private
  configs) follows the same `onepasswordRead`/`onepasswordDocument` pattern rather
  than inventing a new mechanism. age (Option C) remains the better tool if we
  ever need a secret to converge fully offline on a server.
