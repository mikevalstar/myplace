package skills

import (
	"context"
	"errors"
	"testing"
)

func TestParseCheckUpToDate(t *testing.T) {
	// Real, captured `skills check -g` output (v1.5.14), ANSI included, when
	// everything is current: progress lines + the all-clear. Zero outdated.
	out := []byte("\x1b[38;5;145mChecking for skill updates...\x1b[0m\n\n" +
		"\x1b[38;5;102mChecking skills from source: ogulcancelik/herdr\x1b[0m\x1b[K\n" +
		"\x1b[K\x1b[38;5;145m✓ All global skills are up to date\x1b[0m\n")
	pkgs, err := ParseCheck(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 0 {
		t.Fatalf("up-to-date output should yield 0 outdated skills, got %d: %v", len(pkgs), pkgs)
	}
}

func TestParseCheckEmpty(t *testing.T) {
	for _, in := range []string{"", "\n\n"} {
		pkgs, err := ParseCheck([]byte(in))
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if len(pkgs) != 0 {
			t.Fatalf("%q: want 0, got %d", in, len(pkgs))
		}
	}
}

func TestParseCheckArrowDelta(t *testing.T) {
	// Best-effort has-updates parse: a version-arrow line yields name/current/
	// latest; a keyword line yields a name with a generic "update available".
	// (The real outdated-line shape is unconfirmed — see ParseCheck's NOTE.)
	out := []byte("Checking for skill updates...\n" +
		"✗ astro-seo 1.2.0 → 1.3.0\n" +
		"pr-review: update available\n" +
		"✓ some-other skill up to date\n")
	pkgs, err := ParseCheck(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("want 2 outdated (arrow + keyword; up-to-date line skipped), got %d: %v", len(pkgs), pkgs)
	}
	byName := map[string]Package{}
	for _, p := range pkgs {
		byName[p.Name] = p
	}
	if s := byName["astro-seo"]; s.Current != "1.2.0" || s.Latest != "1.3.0" {
		t.Errorf("astro-seo: got %+v, want current 1.2.0 / latest 1.3.0", s)
	}
	if s, ok := byName["pr-review"]; !ok || s.Latest != "update available" {
		t.Errorf("pr-review keyword line: got %+v (ok=%v)", s, ok)
	}
}

// fakeRunner records calls and returns scripted results keyed by the first arg
// after the binary name, so we can exercise CLI resolution without a real skills.
type fakeRunner struct {
	responses map[string]struct {
		out []byte
		err error
	}
	calls [][]string
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	key := name
	if len(args) > 0 {
		key = name + " " + args[0]
	}
	if r, ok := f.responses[key]; ok {
		return r.out, r.err
	}
	return nil, errors.New("command not found")
}

func newFake(resp map[string]struct {
	out []byte
	err error
}) *fakeRunner {
	return &fakeRunner{responses: resp}
}

func TestResolvePrefersGlobalBinary(t *testing.T) {
	f := newFake(map[string]struct {
		out []byte
		err error
	}{"skills --version": {out: []byte("1.5.14")}})
	c := New(f)
	if !c.Installed(context.Background()) {
		t.Fatal("expected skills to be available via the global binary")
	}
	if got := c.argv; len(got) != 1 || got[0] != "skills" {
		t.Fatalf("resolved argv = %v, want [skills]", got)
	}
	// resolve is cached: a second Installed must not re-probe.
	before := len(f.calls)
	c.Installed(context.Background())
	if len(f.calls) != before {
		t.Errorf("resolve should be cached; got %d extra calls", len(f.calls)-before)
	}
}

func TestResolveFallsBackToNpx(t *testing.T) {
	f := newFake(map[string]struct {
		out []byte
		err error
	}{"npx --no-install": {out: []byte("1.5.14")}})
	c := New(f)
	if !c.Installed(context.Background()) {
		t.Fatal("expected fallback to npx --no-install skills")
	}
	if got := c.argv; len(got) != 3 || got[0] != "npx" || got[2] != "skills" {
		t.Fatalf("resolved argv = %v, want [npx --no-install skills]", got)
	}
}

func TestNotAvailable(t *testing.T) {
	c := New(newFake(nil)) // nothing resolves
	if c.Installed(context.Background()) {
		t.Fatal("expected skills to be unavailable when neither binary resolves")
	}
	if _, err := c.Outdated(context.Background()); err == nil {
		t.Error("Outdated should error when the CLI is unavailable")
	}
}

func TestOutdatedRunsCheckGlobal(t *testing.T) {
	f := newFake(map[string]struct {
		out []byte
		err error
	}{
		"skills --version": {out: []byte("1.5.14")},
		"skills check":     {out: []byte("✓ All global skills are up to date\n")},
	})
	c := New(f)
	pkgs, err := c.Outdated(context.Background())
	if err != nil {
		t.Fatalf("Outdated: %v", err)
	}
	if len(pkgs) != 0 {
		t.Fatalf("want 0 outdated, got %v", pkgs)
	}
	// It must invoke `check -g` (global scope, avoids the interactive prompt).
	var sawCheckG bool
	for _, call := range f.calls {
		if len(call) >= 3 && call[1] == "check" && call[2] == "-g" {
			sawCheckG = true
		}
	}
	if !sawCheckG {
		t.Errorf("expected a `skills check -g` invocation, calls were %v", f.calls)
	}
}
