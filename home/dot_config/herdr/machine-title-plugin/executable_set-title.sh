#!/bin/sh
# herdr plugin: pin the outer terminal title to "herdr@<host> · <workspace>".
#
# herdr sets the terminal title to "herdr" on its own; this runs on herdr's
# focus/pane events (see herdr-plugin.toml) and re-asserts a machine-identifying
# title instead, so multiplexed sessions across the fleet are told apart at a
# glance. herdr passes its binary path in HERDR_BIN_PATH; jq is a mise-managed
# tool and reachable from the herdr server's environment (both are mise tools).
set -eu

herdr="${HERDR_BIN_PATH:-herdr}"
host="$(hostname -s 2>/dev/null || hostname)"

# The focused workspace's label (usually the repo name). Best-effort: if the
# query or jq fails, fall back to just the host rather than blocking the title.
ws=""
if command -v jq >/dev/null 2>&1; then
	ws="$("$herdr" workspace list 2>/dev/null \
		| jq -r 'first(.result.workspaces[] | select(.focused) | .label) // empty' 2>/dev/null)"
fi

if [ -n "$ws" ]; then
	title="herdr@$host · $ws"
else
	title="herdr@$host"
fi

exec "$herdr" terminal title set "$title"
