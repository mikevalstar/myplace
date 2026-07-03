// Package shelly wraps the Shelly CLI (CachyOS's Rust-based all-in-one package
// helper) for read-only inventory. Like internal/brew it is present-if-installed:
// callers check Installed first and skip Shelly entirely when it is not on PATH,
// so this package is safe to wire up on every OS (it self-reports unavailable on
// macOS/servers). It never installs, upgrades, syncs, or otherwise mutates
// anything — it only reads the available-updates list across every channel Shelly
// manages (native Arch repos, AUR, Flatpak, AppImage). The adapter that turns
// this into an outdated.Source lives in internal/outdated, so this package stays
// a thin CLI wrapper with no cross-imports (ADR-0010).
package shelly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mikevalstar/myplace/internal/run"
)

type Client struct {
	r run.Runner
}

func New(r run.Runner) *Client {
	return &Client{r: r}
}

// Installed reports whether shelly is on PATH. Cheap and side-effect-free.
func (c *Client) Installed(ctx context.Context) bool {
	_, err := c.r.Run(ctx, "", "shelly", "--version")
	return err == nil
}

// Package is one outdated package. Field names match outdated.Package so the
// adapter in internal/outdated is a trivial conversion.
type Package struct {
	Name    string
	Current string
	Latest  string
}

// ParseOutdated parses `shelly check-updates --json`, the all-channel update
// check (native repos + AUR + Flatpak + AppImage).
//
// NOTE: Shelly post-dates this code's authoring and can't run on a Mac, so the
// exact JSON shape below is inferred from the documented CLI, not captured from
// the tool. It MUST be confirmed against real output on a CachyOS machine — see
// docs/features/outdated-packages.md. To that end the parser is deliberately
// tolerant: it accepts either a bare array of entries or an object wrapping one
// under `updates`/`packages`/`results`, and reads several field-name aliases for
// the package name and its current/new versions. If the real shape differs, the
// fix is confined to the key lists here (and this package's test fixtures).
//
// Packages from a secondary channel (AUR, Flatpak, AppImage) are prefixed with
// that channel (e.g. `flatpak:org.gimp.GIMP`) so they stay distinguishable once
// every channel is flattened into the single "shelly" source row; native repo
// packages keep their bare name (as pacman shows them). Entries with no name or
// no current version are skipped (mirrors brew/mise's empty-current skip).
func ParseOutdated(out []byte) ([]Package, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var entries []map[string]any
	switch trimmed[0] {
	case '[':
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return nil, err
		}
	case '{':
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &wrapper); err != nil {
			return nil, err
		}
		for _, key := range []string{"updates", "packages", "results"} {
			if raw, ok := wrapper[key]; ok {
				if err := json.Unmarshal(raw, &entries); err != nil {
					return nil, err
				}
				break
			}
		}
		// No known array key (e.g. an object carrying only a count) → no packages.
	default:
		return nil, fmt.Errorf("shelly: unexpected JSON (want object or array), got %q", trimmed[0])
	}

	var pkgs []Package
	for _, e := range entries {
		name := firstString(e, "name", "pkg", "package")
		cur := firstString(e, "current_version", "current", "installed_version", "installed", "old_version")
		latest := firstString(e, "new_version", "new", "latest", "available_version", "available")
		if name == "" || cur == "" {
			continue
		}
		switch channel := strings.ToLower(firstString(e, "source", "channel", "type", "repository", "repo")); channel {
		case "aur", "flatpak", "appimage":
			name = channel + ":" + name
		}
		pkgs = append(pkgs, Package{Name: name, Current: cur, Latest: latest})
	}
	return pkgs, nil
}

// firstString returns the first non-empty string value among the given keys.
func firstString(e map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := e[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// Outdated returns every package with a newer version available, across all
// channels Shelly manages. It reads only — it never runs `shelly sync`/`update`/
// `upgrade`, so freshness reflects the user's last sync (same read-only stance as
// brew.Outdated, which never runs `brew update`). Shelly may exit non-zero while
// still emitting valid JSON, so a non-empty body is trusted even on error (same
// tolerance as brew/mise).
func (c *Client) Outdated(ctx context.Context) ([]Package, error) {
	out, err := c.r.Run(ctx, "", "shelly", "check-updates", "--json")
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return ParseOutdated(out)
}
