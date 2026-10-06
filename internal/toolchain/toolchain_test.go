package toolchain

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRunner answers by binary name and records every command it was asked to
// run, so a test can assert this package never issues a mutating one.
type fakeRunner struct {
	out  map[string][]byte // keyed by binary name
	fail map[string]bool
	cmds []string
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	f.cmds = append(f.cmds, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	if f.fail[name] {
		return nil, errors.New("not installed")
	}
	out, ok := f.out[name]
	if !ok {
		return nil, errors.New("not installed")
	}
	return out, nil
}

// Real captured `rustup check` output from the primary Mac (2026-08-31). The
// stable line is the phantom case: rustup announces an "update available" whose
// two sides are the identical version.
const rustupPhantom = `stable-aarch64-apple-darwin - update available: 1.98.0 (88d9e12ae 2026-08-18) -> 1.98.0 (88d9e12ae 2026-08-18)
rustup - up to date : 1.29.0
`

func TestParseRustupCheckPhantomRow(t *testing.T) {
	// The regression this guards: passing the same-version "update available"
	// through would put a permanent "rust is outdated" row on the dashboard.
	if pkgs := ParseRustupCheck([]byte(rustupPhantom)); len(pkgs) != 0 {
		t.Fatalf("same-version update should yield no rows, got %d: %+v", len(pkgs), pkgs)
	}
}

func TestParseRustupCheckRealUpdate(t *testing.T) {
	out := `stable-aarch64-apple-darwin - update available: 1.98.0 (88d9e12ae 2026-08-18) -> 1.99.0 (abc123def 2026-09-29)
rustup - update available : 1.28.0 -> 1.29.0
`
	pkgs := ParseRustupCheck([]byte(out))
	if len(pkgs) != 2 {
		t.Fatalf("want 2 rows, got %d: %+v", len(pkgs), pkgs)
	}
	// The stable toolchain reads as plain "rust" — that's the Rust people mean.
	if p := pkgs[0]; p.Name != "rust" || p.Current != "1.98.0" || p.Latest != "1.99.0" {
		t.Errorf("toolchain row: got %+v, want rust 1.98.0 → 1.99.0", p)
	}
	if p := pkgs[1]; p.Name != "rustup" || p.Current != "1.28.0" || p.Latest != "1.29.0" {
		t.Errorf("rustup row: got %+v, want rustup 1.28.0 → 1.29.0", p)
	}
}

func TestParseRustupCheckChannelNames(t *testing.T) {
	out := `nightly-aarch64-apple-darwin - update available : 1.100.0 -> 1.101.0
beta-aarch64-apple-darwin - update available : 1.99.0 -> 1.99.1
`
	pkgs := ParseRustupCheck([]byte(out))
	if len(pkgs) != 2 {
		t.Fatalf("want 2 rows, got %d", len(pkgs))
	}
	if pkgs[0].Name != "rust (nightly)" || pkgs[1].Name != "rust (beta)" {
		t.Errorf("channels should stay distinguishable, got %q and %q", pkgs[0].Name, pkgs[1].Name)
	}
}

func TestParseRustupCheckGarbageIsSkipped(t *testing.T) {
	// A cosmetic upstream change should degrade to "nothing outdated", not to a
	// failed source.
	for _, in := range []string{"", "\n\n", "info: syncing channel updates\n", "weird - update available :\n"} {
		if pkgs := ParseRustupCheck([]byte(in)); len(pkgs) != 0 {
			t.Errorf("%q: want 0 rows, got %+v", in, pkgs)
		}
	}
}

func TestParseVersionRealBanners(t *testing.T) {
	// Every banner shape captured from the real tools on 2026-08-31.
	for _, tc := range []struct{ banner, want string }{
		{"2026.8.15 macos-arm64 (2026-08-30)\n", "2026.8.15"},
		{"chezmoi version v2.70.5, commit b81bd8daa23126bd5f1b2f787141b2b69439abee, built at 2026-06-03T21:59:37Z, built by goreleaser\n", "2.70.5"},
		{"go version go1.26.4 darwin/arm64\n", "1.26.4"},
		{"fnm 1.39.0\n", "1.39.0"},
		{"go1.27.0\ntime 2026-08-18T21:24:23Z\n", "1.27.0"},
		{"no version here", ""},
	} {
		if got := parseVersion([]byte(tc.banner)); got != tc.want {
			t.Errorf("parseVersion(%q) = %q, want %q", tc.banner, got, tc.want)
		}
	}
}

func TestLessVersion(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"2026.6.10", "2026.8.15", true},  // mise calver — the incident case
		{"2026.8.15", "2026.6.10", false}, // and not backwards
		{"2026.8.15", "2026.8.15", false}, // equal is not outdated
		{"2.70.5", "2.72.1", true},        // chezmoi semver
		{"1.26.4", "1.27.0", true},        // go
		{"1.98.0", "1.98.0", false},       // the rustup phantom pair
		{"1.9.0", "1.10.0", true},         // numeric, not lexical
		{"1.40.0", "1.39.0", false},       // a locally newer build is not "outdated"
		{"2.72", "2.72.1", true},          // missing segments count as 0
		{"2.72.1", "2.72", false},         //
		{"1.2.x", "1.2.3", true},          // non-numeric segment sorts as 0
	} {
		if got := lessVersion(tc.a, tc.b); got != tc.want {
			t.Errorf("lessVersion(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCompareOnlyReportsWhenUpstreamAhead(t *testing.T) {
	res := compare("mise", "2026.6.10", "2026.8.15")
	if !res.compared || len(res.pkgs) != 1 || res.pkgs[0].Latest != "2026.8.15" {
		t.Fatalf("stale tool should yield one row, got %+v", res)
	}
	res = compare("mise", "2026.8.15", "2026.8.15")
	if !res.compared || len(res.pkgs) != 0 {
		t.Fatalf("current tool should compare cleanly with no row, got %+v", res)
	}
	if res := compare("mise", "2026.8.15", ""); res.err == nil {
		t.Error("an empty upstream version should be an error, not silence")
	}
}

func TestInstalledFallsBackToChezmoi(t *testing.T) {
	ctx := context.Background()
	f := &fakeRunner{out: map[string][]byte{"chezmoi": []byte("chezmoi version v2.70.5")}}
	if !New(f).Installed(ctx) {
		t.Error("chezmoi alone should be enough to make the source available")
	}
	if got := New(&fakeRunner{}).Installed(ctx); got {
		t.Error("a box with neither mise nor chezmoi has no core toolchain to report")
	}
}

// cancelledCtx exercises the full Outdated path with the upstream lookups
// guaranteed to fail before any network I/O — the local probes still run and
// record, so this stays a hermetic test.
func cancelledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestOutdatedNeverMutates(t *testing.T) {
	f := &fakeRunner{out: map[string][]byte{
		"mise":    []byte("2026.6.10 macos-arm64 (2026-06-14)"),
		"chezmoi": []byte("chezmoi version v2.70.5, commit abc"),
		"go":      []byte("go version go1.26.4 darwin/arm64"),
		"rustup":  []byte(rustupPhantom),
		"fnm":     []byte("fnm 1.39.0"),
	}}
	// The upstream lookups can't run here, so an error naming them is expected —
	// what matters for this test is the command list.
	if _, err := New(f).Outdated(cancelledCtx()); err == nil {
		t.Fatal("failed upstream lookups must be reported, not swallowed")
	}
	// The whole point of ADR-0027's read-only rule: reporting staleness must
	// never be able to upgrade the machine out from under the user.
	readOnly := map[string]bool{
		"mise version":      true,
		"chezmoi --version": true,
		"go version":        true,
		"rustup check":      true,
		"fnm --version":     true,
	}
	for _, cmd := range f.cmds {
		if !readOnly[cmd] {
			t.Errorf("unexpected command %q — this package must only ever read", cmd)
		}
	}
	for _, banned := range []string{"self-update", "upgrade", "update", "install"} {
		for _, cmd := range f.cmds {
			// "rustup check" is fine; "rustup update" is not.
			if strings.Contains(cmd, " "+banned) {
				t.Errorf("mutating command issued: %q", cmd)
			}
		}
	}
}

func TestOutdatedErrorsWhenNothingCouldBeCompared(t *testing.T) {
	// No rustup on this box and every upstream lookup fails: the source must say
	// so rather than report a misleading "everything's current".
	f := &fakeRunner{out: map[string][]byte{
		"mise": []byte("2026.6.10 macos-arm64 (2026-06-14)"),
	}}
	pkgs, err := New(f).Outdated(cancelledCtx())
	if err == nil {
		t.Fatal("want an error when no member could be compared, got nil")
	}
	if len(pkgs) != 0 {
		t.Fatalf("nothing was comparable, so there should be no rows: %+v", pkgs)
	}
	if !strings.Contains(err.Error(), "mise") {
		t.Errorf("error should name the member that couldn't be checked, got %q", err)
	}
}

func TestOutdatedReportsPartialResultsAndTheFailure(t *testing.T) {
	// The regression that matters most here: when one member compares fine and
	// another's lookup fails, BOTH the row and the failure must survive. Dropping
	// the row hides staleness; dropping the error makes a partial answer look
	// complete — the failure mode that let a stale mise break a tool install.
	f := &fakeRunner{out: map[string][]byte{
		"rustup": []byte(`stable-aarch64-apple-darwin - update available : 1.98.0 -> 1.99.0`),
		"go":     []byte("go version go1.26.4 darwin/arm64"), // go.dev unreachable here
	}}
	pkgs, err := New(f).Outdated(cancelledCtx())
	if len(pkgs) != 1 || pkgs[0].Name != "rust" {
		t.Fatalf("the comparison that worked must survive, got %+v", pkgs)
	}
	if err == nil {
		t.Fatal("the member that could not be checked must be reported")
	}
	if !strings.Contains(err.Error(), "go") {
		t.Errorf("error should name go, got %q", err)
	}
}

func TestOutdatedOmitsAbsentMembers(t *testing.T) {
	// A server with no Rust and no Go yields no rows for them — absence is not
	// an error, and not a phantom row.
	f := &fakeRunner{out: map[string][]byte{"rustup": []byte(`stable-x86_64-unknown-linux-gnu - update available : 1.98.0 -> 1.99.0`)}}
	pkgs, err := New(f).Outdated(cancelledCtx())
	if err != nil {
		t.Fatalf("absent members are not errors: %v", err)
	}
	if len(pkgs) != 1 || pkgs[0].Name != "rust" {
		t.Fatalf("want only the rust row, got %+v", pkgs)
	}
}

func TestSortByMemberOrder(t *testing.T) {
	pkgs := []Package{{Name: "go"}, {Name: "rust (nightly)"}, {Name: "chezmoi"}, {Name: "mise"}}
	sortByMemberOrder(pkgs)
	want := []string{"mise", "chezmoi", "rust (nightly)", "go"}
	for i, w := range want {
		if pkgs[i].Name != w {
			t.Fatalf("position %d: got %q, want %q (full: %+v)", i, pkgs[i].Name, w, pkgs)
		}
	}
}
