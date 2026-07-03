---
title: ADR-0022 — Age-encrypted dotfiles with a 1Password-held key
status: accepted
created: 2026-07-03
updated: 2026-07-03
tags: [chezmoi, secrets, age, encryption, 1password, ssh, dotfiles]
supersedes: "0016"
superseded-by: null
---

# ADR-0022: Age-encrypted dotfiles with a 1Password-held key

## Context

ADR-0016 keeps secret-bearing dotfile content (the motivating case: the SSH
host list with server IPs) in a 1Password Document, pulled by chezmoi's
`onepasswordDocument` template function. That deliberately traded a soft
runtime dependency for a perfectly clean repo: chezmoi evaluates
`onepassword*` functions during *every* target-state computation, so every
`status`/`diff`/`apply` on a desktop shells out to `op`.

Living with that trade for a few weeks showed the cost was underpriced:

- `myplace status` — meant to be cheap, read-only, run-anytime — blocks on an
  unlocked 1Password. Whenever the app has re-locked (reboot, lock timeout),
  status means a password/biometric prompt, or exit 3 if nobody's there to
  answer it. It's the loudest wart in daily use.
- Phase 2 makes it fatal, not annoying: an unattended, cron-driven status
  report can't answer a biometric prompt. A status that requires an
  interactive unlock can never be the thing machines ping a server with.
- ADR-0016 itself noted the escape hatch: age "remains the better tool if we
  ever need a secret to converge fully offline."

What must not regress: bidirectional drift detection on the secret file
(settled design point), servers never needing a secrets session (they have
none), and no plaintext secret in the public repo — the whole point of 0016.

## Options considered

### Option A — keep 1Password pulls; tune the session

Enable the desktop app's CLI integration, stretch auto-lock. Zero repo
changes, and prompts get rarer — but never zero, and unattended status still
can't unlock a vault. Doesn't solve phase 2; only postpones the annoyance.

### Option B — move the `op` pull out of the hot path (apply-time script)

Write `~/.ssh/config` from a `run_onchange_` script so only `apply` touches
1Password. Frees `status`/`diff`, but takes the file out of chezmoi's managed
state — bidirectional drift detection on it is lost, which breaks a settled
design point. And converging still needs an unlocked session.

### Option C — age-encrypt the content in the repo; distribute the key once via 1Password

chezmoi has native age support: an `encrypted_` source file is committed as
ciphertext and decrypted locally at evaluation time with an identity file on
the machine. `status`/`diff`/`apply` become offline and promptless; the file
stays fully managed (drift detection intact). 1Password's role shrinks to
distributing the identity — once per machine, at first apply. Costs: the repo
now publishes ciphertext (0016 explicitly preferred not even that); the
identity becomes a new crown-jewel secret with a rotation story to own; and
bootstrap gains a key-fetch step.

### Option D — dissolve the host list (mesh VPN, e.g. Tailscale + MagicDNS)

Make hostnames non-secret instead of hiding them. Attractive long-term (and
pre-builds phase-2 connectivity), but it's an infrastructure decision about
every machine's network, not a dotfiles-mechanism decision — and some SSH
config would still exist and want managing. Out of scope here; can be adopted
later independently of this ADR.

## Decision

**Option C.** The concrete shape:

- **One fleet-wide age identity**, generated once. The *public* half (the
  recipient) is committed in `.chezmoi.toml.tmpl`'s `[age]` section — not a
  secret. The *private* half lives at `~/.config/chezmoi/key.txt` (0600) on
  every desktop, and canonically as the **`chezmoi age key` Document**
  (`Private` vault, personal account) in 1Password.
- **`run_once_before_fetch-age-key.sh.tmpl`** pulls the key from 1Password at
  a machine's first apply — the single remaining `op` touchpoint at converge
  time, once per machine lifetime. Profile-gated: servers render it a no-op.
- **The SSH host list moves into the repo as ciphertext**:
  `home/private_dot_ssh/private_config.d/encrypted_private_hosts.age` →
  `~/.ssh/config.d/hosts`, `Include`d by the (public, template-rendered)
  `~/.ssh/config` on non-`server` profiles. `.chezmoiignore` takes
  `.ssh/config.d` out of scope on the `server` profile — chezmoi never
  decrypts an ignored entry, which is precisely what lets servers skip the
  key (and `op`, and 1Password) entirely.
- **Rollout is carried by `init`**: config-template changes only reach a
  machine's `chezmoi.toml` on `chezmoi init`, so `myplace update` now
  re-inits — the TUI path via `chezmoi update --init`, the CLI path via an
  explicit `chezmoi init` step between pull and apply. `prompt*Once` answers
  carry over; it never re-prompts.

On moving 0016's "no ciphertext in the repo" bar: the protected content is a
host list — names and IPs, worth keeping unindexed, not catastrophic if a key
ever leaks alongside the ciphertext. age's X25519 is modern, boring, strong
crypto. Against that stands a concrete, permanent operational cost (daily
prompts today, an impossible phase 2 tomorrow). The design point 0016 guarded
— bidirectional status — comes out *stronger*: drift is computable with no
session to unlock.

## Consequences

- **`status`/`diff`/`apply` are 1Password-free and fully offline** once a
  machine holds the key. Exit codes stop depending on the 1Password lock
  state; unattended/cron status (phase 2) becomes possible.
- **`op` stays installed on desktops** but is needed exactly once at converge
  time (the key fetch on first apply) and thereafter only for editing
  1Password-stored secrets. 1Password remains the root of trust — it
  *distributes* the key rather than serving every read.
- **The age identity is the crown-jewel secret.** Anyone holding it plus the
  public repo history can read every committed version of every encrypted
  file. If it leaks: generate a new identity, re-encrypt all `encrypted_`
  files to the new recipient, replace the 1Password Document, delete + re-fetch
  `key.txt` on each machine — and treat the old *contents* as burned (already-
  published ciphertext stays readable by the old key forever), rotating the
  underlying hosts/IPs if warranted.
- **Editing the host list changes shape**: decrypt → edit → re-encrypt in a
  checkout, then commit + push + `myplace update` — it's now an ordinary
  (encrypted) dotfile change that rolls out like any other, instead of a
  1Password edit that machines silently pick up. The rewritten
  [edit-ssh-config workflow](../workflows/edit-ssh-config.md) has the recipe.
  The old `ssh config` Document stays in 1Password as a dormant backup; the
  repo file is now the source of truth.
- **A fresh desktop still needs a signed-in `op` before first apply** — same
  as under 0016 (where the very first apply needed it too); the standing
  practice of installing 1Password first on interactive machines is unchanged.
- **`run_once` means lost keys aren't self-healing**: if `key.txt` is deleted,
  the fetch script won't re-fire; re-fetch by hand (documented in the script
  header and the workflow).
- **Generalizes**: future secret-bearing dotfiles are `encrypted_` files under
  the same recipient — no new 1Password Documents, no new mechanism. And
  because decryption is offline, a future *server-side* secret is now possible
  by simply not ignoring it there (plus fetching the key some server-safe way)
  — the mechanism no longer assumes an interactive session exists.
