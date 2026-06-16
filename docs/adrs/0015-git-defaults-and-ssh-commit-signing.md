---
title: ADR-0015 — Modern git defaults and SSH commit signing
status: accepted
created: 2026-06-16
updated: 2026-06-16
tags: [chezmoi, git, signing, dotfiles]
supersedes: null
superseded-by: null
---

# ADR-0015: Modern git defaults and SSH commit signing

## Context

`home/dot_gitconfig.tmpl` carried only identity (`user.name`/`user.email`, from
install-time chezmoi data) and the delta pager wiring. Everything else fell back
to git's stock defaults, which are conservative for backward compatibility — so
every machine repeated the same first-day fixups by hand (`push.autoSetupRemote`,
`pull.rebase`, `fetch.prune`, …). These are exactly the "share by default" knobs
this repo exists to centralize.

Separately, the owner wants **signed commits** across the fleet so work pushed
from any machine is verifiable. Two design constraints make this non-trivial:

1. **The fleet is heterogeneous.** Personal Macs and the work Mac have an SSH key
   and should sign; the Linux servers (profile `server`) generally don't have a
   user signing key and shouldn't. Turning on `commit.gpgsign` unconditionally
   would make **every** `git commit` fail on a keyless box — a footgun that also
   breaks any automation that commits.
2. **Signing keys are per-machine, identity is shared.** `user.name`/`user.email`
   are one identity collected once; the signing key is whatever key lives on that
   particular machine. The config has to bridge a shared identity to a local key.

The toolchain favours this: git supports **SSH signing** (`gpg.format = ssh`)
since 2.34, so an existing `~/.ssh/id_ed25519.pub` doubles as the signing key —
no GPG keyring to provision. The fleet's git is well past that floor (2.50 here).

## Options considered

### Option A — Defaults only, signing left manual

Ship the modern defaults; leave signing to each machine. Simple, but punts the
thing actually asked for and leaves signing inconsistent across the fleet.

### Option B — Unconditional `commit.gpgsign = true`

One static block, signing always on. Breaks every commit on a keyless server and
on any Mac whose key isn't where we assumed. Rejected — violates the "a machine
without the prerequisite degrades quietly" expectation the rest of the setup holds
to (cf. brew-if-present, ADR-0008).

### Option C — Signing auto-enabled when a key is present (chosen)

Template the signing block behind `stat` of the signing key. The key path defaults
to `~/.ssh/id_ed25519.pub` and is overridable via a `signingKey` chezmoi data key
(same `dig`-with-default pattern as identity). When the key exists the block emits
`gpg.format = ssh`, `user.signingkey`, `commit.gpgsign`/`tag.gpgsign`, and points
`gpg.ssh.allowedSignersFile` at a companion `~/.config/git/allowed_signers` that we
generate from the same public key + committer email. When the key is absent the
whole block — and the allowed-signers entry — is omitted, so the machine simply
doesn't sign.

## Decision

Option C, plus the modern defaults.

- **Modern defaults (always on):** `init.defaultBranch = main`;
  `push.autoSetupRemote = true`, `push.followTags = true`; `pull.rebase = true`;
  `fetch.prune = true`, `fetch.pruneTags = true`; `rebase.autoStash = true`,
  `rebase.updateRefs = true`; `rerere.enabled = true`; `branch.sort =
  -committerdate`, `tag.sort = -version:refname`; `column.ui = auto`;
  `commit.verbose = true`; `help.autocorrect = prompt`; and `diff.algorithm =
  histogram`, `diff.mnemonicPrefix = true`, `diff.renames = true` folded into the
  existing `[diff]`. These are behaviour-shaping but non-destructive and match
  what a modern git user sets by hand anyway.
- **SSH signing, conditional on key presence.** Auto-on when the signing key
  exists; silently off otherwise. Key path defaults to `~/.ssh/id_ed25519.pub`,
  overridable per machine via the `signingKey` data key.
- **`allowed_signers` is generated, not hand-maintained.** A companion templated
  file (`home/dot_config/git/allowed_signers.tmpl`) writes `<email> <pubkey>` by
  reading the public key at apply time, so local `git log --show-signature` /
  `git verify-commit` validate. It is gated on the same `stat`, so a keyless
  machine gets no stray entry.
- **One manual step remains, by design.** GitHub's "Verified" badge needs the
  public key uploaded as a *signing* key (distinct from an auth key):
  `gh ssh-key add ~/.ssh/id_ed25519.pub --type signing --title "$(hostname)"`.
  We don't automate this — it needs a GitHub token scope and is a one-time,
  per-machine action — but it's documented in the managed-setup guide.

## Consequences

- New machines inherit sane git behaviour with no first-day fixups, and Macs sign
  commits the moment a key is present — both flowing through the normal
  `myplace update` path.
- Servers and any keyless machine keep working unchanged: no signing, no failed
  commits, no empty config noise.
- A new optional chezmoi data key, `signingKey`, exists for the exception case
  (non-default key name/path). It's `dig`-defaulted, so it never needs answering
  at bootstrap and existing `chezmoi.toml` files keep working untouched.
- `allowed_signers` is regenerated from the live public key on every apply via a
  template `output "cat"`; rotating the key updates it automatically.
- The defaults are opinionated. They're non-destructive (no history rewriting, no
  data loss) and reversible per machine via a local `~/.gitconfig.local`-style
  include if ever needed — not built now, noted as the escape hatch.

## Related

- [ADR-0003](0003-monorepo-app-dotfiles-mise.md) — monorepo as the chezmoi source; where `dot_gitconfig.tmpl` lives
- [ADR-0008](0008-opportunistic-homebrew-macos.md) — the same "degrade quietly when a prerequisite is absent" stance
- [Extending the managed setup](../guides/managed-setup.md) — how-to, including the one-time GitHub signing-key upload
