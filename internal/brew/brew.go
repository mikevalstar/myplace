// Package brew wraps the Homebrew CLI for read-only inventory. It is
// brew-if-present (ADR-0008/0009): callers check Installed first and skip brew
// entirely when it is not on PATH. This package never installs, upgrades, taps,
// or otherwise mutates anything — it only reads `brew outdated`. The adapter
// that turns this into an outdated.Source lives in internal/outdated, so this
// package stays a thin CLI wrapper with no cross-imports.
package brew

import (
	"context"
	"encoding/json"

	"github.com/mikevalstar/myplace/internal/run"
)

type Client struct {
	r run.Runner
}

func New(r run.Runner) *Client {
	return &Client{r: r}
}

// Installed reports whether brew is on PATH. Cheap and side-effect-free.
func (c *Client) Installed(ctx context.Context) bool {
	_, err := c.r.Run(ctx, "", "brew", "--version")
	return err == nil
}

// Package is one outdated formula or cask. Field names match outdated.Package
// so the adapter in internal/outdated is a trivial conversion.
type Package struct {
	Name    string
	Current string
	Latest  string
}

// brewOutdated mirrors `brew outdated --json=v2`. In v2 both formulae and casks
// carry installed_versions + current_version, so one entry type covers both.
type brewOutdated struct {
	Formulae []brewEntry `json:"formulae"`
	Casks    []brewEntry `json:"casks"`
}

type brewEntry struct {
	Name              string   `json:"name"`
	InstalledVersions []string `json:"installed_versions"`
	CurrentVersion    string   `json:"current_version"`
}

// ParseOutdated parses `brew outdated --json=v2`, merging formulae and casks:
// installed_versions[0] → Current, current_version → Latest. Entries with no
// installed version are skipped (mirrors mise.ParseOutdated's empty-current
// skip). Unknown fields are ignored.
func ParseOutdated(out []byte) ([]Package, error) {
	var raw brewOutdated
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	var pkgs []Package
	for _, group := range [][]brewEntry{raw.Formulae, raw.Casks} {
		for _, e := range group {
			cur := ""
			if len(e.InstalledVersions) > 0 {
				cur = e.InstalledVersions[0]
			}
			if cur == "" {
				continue
			}
			pkgs = append(pkgs, Package{Name: e.Name, Current: cur, Latest: e.CurrentVersion})
		}
	}
	return pkgs, nil
}

// Outdated returns the formulae and casks with newer versions available. brew
// can exit non-zero while still emitting valid JSON, so a non-empty body is
// trusted even on error (same tolerance as mise.Outdated).
func (c *Client) Outdated(ctx context.Context) ([]Package, error) {
	out, err := c.r.Run(ctx, "", "brew", "outdated", "--json=v2")
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return ParseOutdated(out)
}
