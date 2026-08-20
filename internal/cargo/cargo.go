// Package cargo wraps the cargo-update subcommand (`cargo install-update`) for
// read-only inventory of Rust binaries installed with `cargo install`. Like
// internal/brew, internal/shelly, and internal/skills it is present-if-installed:
// callers check Installed first and skip cargo entirely when the subcommand
// can't be resolved, so this package is safe to wire up on every machine (a box
// with no rustup self-reports unavailable). It never installs, rebuilds, or
// otherwise mutates anything — it only reads `--list`, never `-a`.
//
// Why cargo at all: Rust is rustup's, not mise's (ADR-0007), so the binaries
// `cargo install` puts in ~/.cargo/bin are invisible to every other source here.
// cargo-update is itself installed by the provision script, alongside tokei.
//
// Unlike the other sources this one needs the NETWORK: `--list` polls crates.io
// for each installed package. That's not special-cased — it shares the caller's
// context, and an offline or slow machine lands in this source's error field
// without affecting the others (ADR-0010).
//
// The adapter that turns this into an outdated.Source lives in internal/outdated,
// so this package stays a thin CLI wrapper with no cross-imports.
package cargo

import (
	"context"
	"regexp"
	"strings"

	"github.com/mikevalstar/myplace/internal/run"
)

type Client struct {
	r run.Runner
}

func New(r run.Runner) *Client {
	return &Client{r: r}
}

// Installed reports whether the cargo-update subcommand is available (which
// implies cargo itself is). Cheap, offline, and side-effect-free — `--version`
// short-circuits before any registry poll.
func (c *Client) Installed(ctx context.Context) bool {
	_, err := c.r.Run(ctx, "", "cargo", "install-update", "--version")
	return err == nil
}

// Package is one cargo-installed binary with a newer version available. Field
// names match outdated.Package so the adapter is a trivial conversion.
type Package struct {
	Name    string
	Current string
	Latest  string
}

// columns splits a table row on runs of two or more spaces — cargo-update lays
// its table out with tabwriter, so single spaces can occur inside a cell but
// column gaps are always wider.
var columns = regexp.MustCompile(` {2,}`)

// ParseList extracts the outdated set from `cargo install-update --list`, whose
// output is a tabwriter table (there is no JSON mode):
//
//	  Polling registry 'https://index.crates.io/'........
//
//	Package         Installed  Latest   Needs update
//	checksums       v0.5.0     v0.5.2   Yes
//	cargo-outdated  v0.2.0     v0.2.0   No
//
// Only `Needs update: Yes` rows are returned. Everything before a header row is
// ignored (the polling preamble and its progress dots), and a second header
// starts a new table — `--list` prints a separate one for git-origin packages,
// which have the same shape. Rows that don't have the expected column count are
// skipped rather than erroring: a cosmetic upstream change should degrade to
// "nothing outdated", not to a failed source.
func ParseList(out []byte) ([]Package, error) {
	var pkgs []Package
	inTable := false
	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(strings.Trim(raw, "\r"))
		if line == "" {
			inTable = false // blank line ends a table
			continue
		}
		cells := columns.Split(line, -1)
		if len(cells) >= 4 && cells[0] == "Package" {
			inTable = true
			continue
		}
		if !inTable || len(cells) < 4 {
			continue
		}
		if !strings.EqualFold(cells[3], "yes") {
			continue
		}
		pkgs = append(pkgs, Package{
			Name:    cells[0],
			Current: trimV(cells[1]),
			Latest:  trimV(cells[2]),
		})
	}
	return pkgs, nil
}

// trimV drops cargo-update's leading "v" from a version so the inventory reads
// consistently across sources (mise/brew/shelly all report bare versions). Git
// packages list commit hashes instead, which are left alone.
func trimV(s string) string {
	if len(s) > 1 && s[0] == 'v' && s[1] >= '0' && s[1] <= '9' {
		return s[1:]
	}
	return s
}

// Outdated returns every cargo-installed binary with a newer version on
// crates.io. It runs `--list`, which by contract only reports; the upgrade
// (`cargo install-update -a`) stays a manual, human-initiated step. Git-origin
// packages are excluded: comparing them means cloning each repo (`-g`), far too
// expensive for a dashboard pane.
func (c *Client) Outdated(ctx context.Context) ([]Package, error) {
	out, err := c.r.Run(ctx, "", "cargo", "install-update", "--list")
	if err != nil && len(strings.TrimSpace(string(out))) == 0 {
		return nil, err
	}
	return ParseList(out)
}
