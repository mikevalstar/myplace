// Package outdated aggregates "a newer version is available" inventories across
// package managers into one schema-versioned envelope. It is INFORMATIONAL:
// nothing here feeds the drift verdict or status exit codes (that stays in
// internal/drift, mise-only). It is also strictly read-only — it never installs
// or upgrades anything (ADR-0010).
//
// The Source adapters that wrap the mise/brew CLI clients live here, so those
// packages stay thin CLI wrappers with no knowledge of each other or of this
// package (the same import direction as internal/drift → internal/mise).
package outdated

import (
	"context"
	"os"
	"time"

	"github.com/mikevalstar/myplace/internal/brew"
	"github.com/mikevalstar/myplace/internal/cargo"
	"github.com/mikevalstar/myplace/internal/mise"
	"github.com/mikevalstar/myplace/internal/shelly"
	"github.com/mikevalstar/myplace/internal/skills"
)

// Schema is bumped only on breaking changes to the JSON shape (mirrors
// drift.Schema).
const Schema = 1

// Package is one outdated package, source-agnostic.
type Package struct {
	Name    string `json:"name"`
	Current string `json:"current"`
	Latest  string `json:"latest"`
}

// Source is a pluggable package manager. Adding apt/npm/etc. later is one new
// adapter implementing this interface, registered in the source slice.
type Source interface {
	Name() string
	Available(ctx context.Context) bool
	Outdated(ctx context.Context) ([]Package, error)
}

// SourceResult is one source's contribution. Available is false when the
// manager isn't on PATH (not an error); Error is set only when an available
// source failed — other sources are unaffected.
type SourceResult struct {
	Name      string    `json:"name"`
	Available bool      `json:"available"`
	Error     string    `json:"error,omitempty"`
	Packages  []Package `json:"packages"`
}

// Inventory is the whole `myplace outdated --json` document.
type Inventory struct {
	Schema    int            `json:"schema"`
	Machine   string         `json:"machine"`
	CheckedAt time.Time      `json:"checked_at"`
	Sources   []SourceResult `json:"sources"`
}

// Collect queries every source in order, degrading gracefully: an unavailable
// source is recorded with no packages; a failing source captures its error and
// never aborts the others. The overall verdict is computed by ExitCode.
func Collect(ctx context.Context, sources ...Source) Inventory {
	inv := Inventory{Schema: Schema, CheckedAt: time.Now().UTC(), Sources: []SourceResult{}}
	if h, err := os.Hostname(); err == nil {
		inv.Machine = h
	}
	for _, s := range sources {
		res := SourceResult{Name: s.Name(), Packages: []Package{}}
		if !s.Available(ctx) {
			inv.Sources = append(inv.Sources, res) // Available stays false
			continue
		}
		res.Available = true
		if pkgs, err := s.Outdated(ctx); err != nil {
			res.Error = err.Error()
		} else {
			res.Packages = pkgs
		}
		inv.Sources = append(inv.Sources, res)
	}
	return inv
}

// ExitCode is the CLI contract for `myplace outdated`:
//
//	0 = all current   — every usable source produced an empty list
//	1 = updates available — some usable source reported ≥1 package
//	3 = error         — no source could produce a result (all unavailable or errored)
//
// There is no 2/unknown: a partial failure where at least one source still
// produced a result resolves to 0/1, with the failure captured per-source.
func ExitCode(inv Inventory) int {
	anyOutdated, anyUsable := false, false
	for _, s := range inv.Sources {
		if !s.Available || s.Error != "" {
			continue
		}
		anyUsable = true
		if len(s.Packages) > 0 {
			anyOutdated = true
		}
	}
	switch {
	case anyOutdated:
		return 1
	case anyUsable:
		return 0
	default:
		return 3
	}
}

// --- adapters: the only place internal/mise and internal/brew meet ---

type miseSource struct{ c *mise.Client }

// MiseSource adapts a mise client, reusing mise.Client.Outdated.
func MiseSource(c *mise.Client) Source { return miseSource{c} }

func (s miseSource) Name() string                       { return "mise" }
func (s miseSource) Available(ctx context.Context) bool { return s.c.Installed(ctx) }
func (s miseSource) Outdated(ctx context.Context) ([]Package, error) {
	o, err := s.c.Outdated(ctx)
	if err != nil {
		return nil, err
	}
	pkgs := make([]Package, 0, len(o))
	for _, e := range o {
		// mise's "Wanted" is the version it would converge to — the newer one.
		pkgs = append(pkgs, Package{Name: e.Name, Current: e.Current, Latest: e.Wanted})
	}
	return pkgs, nil
}

type brewSource struct{ c *brew.Client }

// BrewSource adapts a brew client. Available() gates the shell-out, so brew is
// silently skipped when it isn't on PATH (brew-if-present, ADR-0008/0009).
func BrewSource(c *brew.Client) Source { return brewSource{c} }

func (s brewSource) Name() string                       { return "brew" }
func (s brewSource) Available(ctx context.Context) bool { return s.c.Installed(ctx) }
func (s brewSource) Outdated(ctx context.Context) ([]Package, error) {
	p, err := s.c.Outdated(ctx)
	if err != nil {
		return nil, err
	}
	pkgs := make([]Package, 0, len(p))
	for _, e := range p {
		pkgs = append(pkgs, Package{Name: e.Name, Current: e.Current, Latest: e.Latest})
	}
	return pkgs, nil
}

type shellySource struct{ c *shelly.Client }

// ShellySource adapts a Shelly client (CachyOS). Like brew it's present-if-
// installed: Available() gates the shell-out, so Shelly is silently skipped when
// it isn't on PATH (i.e. everywhere but an Arch/CachyOS box). One row aggregates
// the channels `shelly check-updates -a -l` reports — native repos, AUR, Flatpak
// (ADR-0010).
func ShellySource(c *shelly.Client) Source { return shellySource{c} }

func (s shellySource) Name() string                       { return "shelly" }
func (s shellySource) Available(ctx context.Context) bool { return s.c.Installed(ctx) }
func (s shellySource) Outdated(ctx context.Context) ([]Package, error) {
	p, err := s.c.Outdated(ctx)
	if err != nil {
		return nil, err
	}
	pkgs := make([]Package, 0, len(p))
	for _, e := range p {
		pkgs = append(pkgs, Package{Name: e.Name, Current: e.Current, Latest: e.Latest})
	}
	return pkgs, nil
}

type skillsSource struct{ c *skills.Client }

// SkillsSource adapts a skills client (skills.sh / `npx skills`). Like brew and
// shelly it's present-if-installed: Available() gates the shell-out, so skills is
// silently skipped when the CLI can't be resolved (a headless server with no
// Node, or a box that never installed a skill). Reports third-party AI skills
// with a newer version upstream; a user's own skills are chezmoi-managed dotfiles
// and don't appear here (ADR-0023). Informational only — never mutates.
func SkillsSource(c *skills.Client) Source { return skillsSource{c} }

func (s skillsSource) Name() string                       { return "skills" }
func (s skillsSource) Available(ctx context.Context) bool { return s.c.Installed(ctx) }
func (s skillsSource) Outdated(ctx context.Context) ([]Package, error) {
	p, err := s.c.Outdated(ctx)
	if err != nil {
		return nil, err
	}
	pkgs := make([]Package, 0, len(p))
	for _, e := range p {
		pkgs = append(pkgs, Package{Name: e.Name, Current: e.Current, Latest: e.Latest})
	}
	return pkgs, nil
}

type cargoSource struct{ c *cargo.Client }

// CargoSource adapts a cargo-update client. Present-if-installed like brew,
// shelly, and skills: Available() probes `cargo install-update --version`, so a
// machine without rustup (or without cargo-update built yet) silently reports
// unavailable. Covers the Rust binaries in ~/.cargo/bin — invisible to every
// other source, since Rust is rustup's and not mise's (ADR-0007). Read-only: it
// runs `--list`, never `-a`. This is the one source that needs the network (it
// polls crates.io); a slow or offline run lands in this source's Error and
// leaves the others intact.
func CargoSource(c *cargo.Client) Source { return cargoSource{c} }

func (s cargoSource) Name() string                       { return "cargo" }
func (s cargoSource) Available(ctx context.Context) bool { return s.c.Installed(ctx) }
func (s cargoSource) Outdated(ctx context.Context) ([]Package, error) {
	p, err := s.c.Outdated(ctx)
	if err != nil {
		return nil, err
	}
	pkgs := make([]Package, 0, len(p))
	for _, e := range p {
		pkgs = append(pkgs, Package{Name: e.Name, Current: e.Current, Latest: e.Latest})
	}
	return pkgs, nil
}
