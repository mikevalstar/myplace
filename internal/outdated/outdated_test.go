package outdated

import (
	"context"
	"errors"
	"testing"
)

// fakeSource implements Source without touching any CLI.
type fakeSource struct {
	name      string
	available bool
	pkgs      []Package
	err       error
}

func (f fakeSource) Name() string                                { return f.name }
func (f fakeSource) Available(context.Context) bool              { return f.available }
func (f fakeSource) Outdated(context.Context) ([]Package, error) { return f.pkgs, f.err }

func TestCollectDegradesGracefully(t *testing.T) {
	ok := fakeSource{name: "mise", available: true, pkgs: []Package{
		{Name: "node", Current: "22.1.0", Latest: "22.3.0"},
		{Name: "jq", Current: "1.7.0", Latest: "1.8.0"},
	}}
	absent := fakeSource{name: "brew", available: false}
	broken := fakeSource{name: "apt", available: true, err: errors.New("boom")}

	inv := Collect(context.Background(), ok, absent, broken)

	if inv.Schema != Schema {
		t.Errorf("schema = %d, want %d", inv.Schema, Schema)
	}
	if len(inv.Sources) != 3 {
		t.Fatalf("want 3 source results in order, got %d", len(inv.Sources))
	}
	// Order is preserved.
	if inv.Sources[0].Name != "mise" || inv.Sources[1].Name != "brew" || inv.Sources[2].Name != "apt" {
		t.Errorf("source order not preserved: %v", []string{inv.Sources[0].Name, inv.Sources[1].Name, inv.Sources[2].Name})
	}
	// OK source: available, packages, no error.
	if s := inv.Sources[0]; !s.Available || s.Error != "" || len(s.Packages) != 2 {
		t.Errorf("ok source wrong: %+v", s)
	}
	// Unavailable source: not available, empty packages (never nil), no error.
	if s := inv.Sources[1]; s.Available || s.Error != "" || s.Packages == nil || len(s.Packages) != 0 {
		t.Errorf("absent source wrong: %+v", s)
	}
	// Broken source: available but error captured, no packages.
	if s := inv.Sources[2]; !s.Available || s.Error == "" || len(s.Packages) != 0 {
		t.Errorf("broken source wrong: %+v", s)
	}
}

func TestExitCode(t *testing.T) {
	pkg := []Package{{Name: "x", Current: "1", Latest: "2"}}
	cases := []struct {
		name string
		inv  Inventory
		want int
	}{
		{"something outdated", Inventory{Sources: []SourceResult{
			{Name: "mise", Available: true, Packages: pkg},
		}}, 1},
		{"all current, usable", Inventory{Sources: []SourceResult{
			{Name: "mise", Available: true, Packages: []Package{}},
			{Name: "brew", Available: false},
		}}, 0},
		{"all unavailable", Inventory{Sources: []SourceResult{
			{Name: "mise", Available: false},
			{Name: "brew", Available: false},
		}}, 3},
		{"errored + unavailable (no usable result)", Inventory{Sources: []SourceResult{
			{Name: "mise", Available: true, Error: "boom"},
			{Name: "brew", Available: false},
		}}, 3},
		{"errored + ok-empty (a usable result exists)", Inventory{Sources: []SourceResult{
			{Name: "mise", Available: true, Error: "boom"},
			{Name: "brew", Available: true, Packages: []Package{}},
		}}, 0},
		{"errored + ok-with-packages", Inventory{Sources: []SourceResult{
			{Name: "mise", Available: true, Error: "boom"},
			{Name: "brew", Available: true, Packages: pkg},
		}}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCode(tc.inv); got != tc.want {
				t.Errorf("ExitCode = %d, want %d", got, tc.want)
			}
		})
	}
}
