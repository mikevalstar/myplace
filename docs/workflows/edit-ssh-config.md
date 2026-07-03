---
title: Edit the SSH config (hosts age-encrypted in the repo)
status: active
created: 2026-06-16
updated: 2026-07-03
tags: [ssh, age, chezmoi, dotfiles, secrets, how-to]
actors: [user, chezmoi]
---

# Edit the SSH config (hosts age-encrypted in the repo)

## Goal

Add or change an SSH host (e.g. a new server's IP) without ever putting that IP
into the public git repo **in plaintext**.

## Preconditions

- The machine's age key at `~/.config/chezmoi/key.txt` — any desktop that has
  applied this setup has it (fetched once from 1Password at first apply —
  [ADR-0022](../adrs/0022-age-encrypted-dotfiles.md)).
- A clone of this repo: host edits are commits under `home/`, rolled out by
  push + `myplace update` like any other dotfile change.

## Background — what is the source of truth

`~/.ssh/config` is **rendered**, not edited by hand. Two pieces make it up:

- **Host entries / IPs** → `~/.ssh/config.d/hosts`, decrypted at apply time
  from the age ciphertext committed at
  `home/private_dot_ssh/private_config.d/encrypted_private_hosts.age`. That
  encrypted file is the source of truth — edit it via the recipe below.
- **Shared defaults** (keepalives, `UseKeychain`, …) → the `Host *` block in
  `home/private_dot_ssh/private_config.tmpl` (not secret; edit normally).
- Editing `~/.ssh/config` or `~/.ssh/config.d/hosts` directly is pointless —
  both are overwritten on the next apply and show as drift until then.

The encrypted file holds only `Host …` blocks; do **not** put the global
`Host *` block in it (the template adds that, after the Include).

## Steps

From a checkout of this repo (`chezmoi encrypt`/`decrypt` use the machine's
own key + recipient from `~/.config/chezmoi/chezmoi.toml`, so there's nothing
to pass):

```sh
hosts=home/private_dot_ssh/private_config.d/encrypted_private_hosts.age

# 1. decrypt to a private temp file (0600 via umask)
(umask 077; chezmoi decrypt "$hosts" > "${TMPDIR:-/tmp}/hosts.edit")

# 2. edit — Host blocks only (no global Host * block)
"${EDITOR:-nano}" "${TMPDIR:-/tmp}/hosts.edit"

# 3. re-encrypt into the repo and remove the plaintext
chezmoi encrypt "${TMPDIR:-/tmp}/hosts.edit" > "$hosts"
rm -f "${TMPDIR:-/tmp}/hosts.edit"

# 4. commit + push (to main — that's what machines pull), then converge
git add "$hosts" && git commit -m "Update SSH host list" && git push
myplace update        # on this machine; others pick it up on their next update
```

## Outcome

Every desktop's `~/.ssh/config.d/hosts` regenerates with the new host(s) on its
next update; the repo carries only armored age ciphertext. Unlike the old
1Password-Document flow (ADR-0016), the change is an ordinary commit — visible
in history, rolled out by the normal update path.

## Failure modes

| What can go wrong | How you find out | Recovery |
|-------------------|------------------|----------|
| Age key missing on this machine | `chezmoi decrypt` errors (`no identity`) | `op document get "chezmoi age key" --vault Private --account my.1password.com > ~/.config/chezmoi/key.txt` then `chmod 600` it |
| Re-encrypted without actually changing anything | git shows a diff every time (age output is randomized) | `git checkout -- <file>` — only commit ciphertext when the plaintext changed |
| Plaintext committed by accident | IPs visible in the public repo | Treat them as leaked: rotate/replace the exposed hosts' addresses; force-pushing history out of a public repo doesn't un-leak it |
| Edited `~/.ssh/config` or `config.d/hosts` directly | shows as drift in `myplace status`; lost on next apply | Move the change into the encrypted file (hosts) or template (defaults) |

## Related

- [ADR-0022 — Age-encrypted dotfiles with a 1Password-held key](../adrs/0022-age-encrypted-dotfiles.md)
- [Extending the managed setup](../guides/managed-setup.md) (the "secret-bearing dotfile" section)
