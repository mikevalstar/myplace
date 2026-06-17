---
title: Edit the SSH config (hosts in 1Password)
status: active
created: 2026-06-16
updated: 2026-06-16
tags: [ssh, 1password, chezmoi, dotfiles, how-to]
actors: [user, chezmoi]
---

# Edit the SSH config (hosts in 1Password)

## Goal

Add or change an SSH host (e.g. a new server's IP) without ever putting that IP
into the public git repo.

## Preconditions

- 1Password app installed with **Settings → Developer → "Integrate with 1Password
  CLI"** enabled, and the `op` CLI on PATH (provisioning installs it on macs).
- The `ssh config` Document exists in the `Private` vault (the source of truth for
  the host list). See [ADR-0016](../adrs/0016-secrets-in-dotfiles-via-1password.md).

## Background — what is the source of truth

`~/.ssh/config` is **rendered**, not edited by hand: chezmoi pulls your host
blocks from the 1Password Document and appends the non-secret global `Host *`
defaults from `home/private_dot_ssh/private_config.tmpl`. So:

- **Host entries / IPs** → live in the **1Password Document** (edit there).
- **Shared defaults** (keepalives, `UseKeychain`, …) → live in the **template**.
- Editing `~/.ssh/config` directly is pointless — it's overwritten on the next
  apply and shows as drift until then.

The Document holds only your `Host …` blocks; do **not** put the global `Host *`
block in it (the template adds that).

## Steps

### Option A — 1Password app (simplest, no temp file)

1. Open the **`ssh config`** Document in the 1Password app and edit it there.
2. Regenerate the local file: `chezmoi apply ~/.ssh/config`
   (or a full `myplace update` if you also want to pull/converge everything else).

### Option B — CLI (edit a local copy, push back, apply)

```sh
# 1. pull the current host list to a temp file
#    (--account disambiguates if more than one account is signed in; drop it otherwise)
op document get "ssh config" --vault Private --account my.1password.com > "$HOME/.sshconfig.edit"

# 2. edit it — add/adjust Host blocks only (not the global Host * block)
"${EDITOR:-nano}" "$HOME/.sshconfig.edit"

# 3. push the new contents back into 1Password
op document edit "ssh config" "$HOME/.sshconfig.edit" --vault Private --account my.1password.com

# 4. remove the local copy (it contains your IPs)
rm -f "$HOME/.sshconfig.edit"

# 5. regenerate ~/.ssh/config from the updated Document
chezmoi apply ~/.ssh/config        # or: myplace update
```

## Outcome

`~/.ssh/config` is regenerated with the new host(s) plus the global defaults; the
IPs live only in 1Password. The git repo is untouched — no commit or push needed,
because nothing in `home/` changed (only the Document did).

## Failure modes

| What can go wrong | How you find out | Recovery |
|-------------------|------------------|----------|
| `op` locked / not signed in | `chezmoi apply` errors with `op signin` | Unlock 1Password (or the app integration toggle), retry |
| Global `Host *` accidentally added to the Document | duplicate `Host *` in rendered file | Remove it from the Document; defaults belong in the template |
| Edited `~/.ssh/config` directly | shows as drift in `myplace status`; lost on next apply | Move the change into the Document (hosts) or template (defaults) |

## Related

- [ADR-0016 — Secret-bearing dotfiles via 1Password](../adrs/0016-secrets-in-dotfiles-via-1password.md)
- [Extending the managed setup](../guides/managed-setup.md) (the "secret-bearing dotfile" section)
