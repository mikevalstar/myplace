package cargo

import "testing"

// Shape of `cargo install-update --list`, per cargo-update's own man page
// (v22.1.1): a "Polling registry" preamble with progress dots, then a tabwriter
// table with a Yes/No "Needs update" column.
const realOutput = `    Polling registry 'https://index.crates.io/'........

  Package         Installed  Latest   Needs update
  checksums       v0.5.0     v0.5.2   Yes
  treesize        v0.2.0     v0.2.1   Yes
  cargo-count     v0.2.2     v0.2.2   No
  cargo-outdated  v0.2.0     v0.2.0   No
  racer           v1.2.10    v1.2.10  No
`

func TestParseList(t *testing.T) {
	pkgs, err := ParseList([]byte(realOutput))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("want only the 2 'Yes' rows, got %d: %v", len(pkgs), pkgs)
	}
	// The leading "v" is trimmed so versions read like every other source.
	if p := pkgs[0]; p.Name != "checksums" || p.Current != "0.5.0" || p.Latest != "0.5.2" {
		t.Errorf("first package: got %+v, want checksums 0.5.0 → 0.5.2", p)
	}
	if p := pkgs[1]; p.Name != "treesize" || p.Latest != "0.2.1" {
		t.Errorf("second package: got %+v, want treesize latest 0.2.1", p)
	}
}

func TestParseListGitTable(t *testing.T) {
	// `--list` prints a second table for git-origin packages, same shape but
	// with commit hashes for versions. A new header starts a new table, and the
	// hashes must survive the "v"-trimming untouched.
	out := `  Package    Installed  Latest   Needs update
  treesize   v0.2.0     v0.2.1   Yes

  Package    Installed  Latest    Needs update
  alacritty  5f788574   8bd08de9  Yes
`
	pkgs, err := ParseList([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("want 1 registry + 1 git package, got %d: %v", len(pkgs), pkgs)
	}
	if p := pkgs[1]; p.Name != "alacritty" || p.Current != "5f788574" || p.Latest != "8bd08de9" {
		t.Errorf("git package: got %+v, want the commit refs verbatim", p)
	}
}

func TestParseListNothingOutdated(t *testing.T) {
	// Everything current, and the degenerate outputs: no packages, no table,
	// empty. None of these is an error — they all mean "nothing outdated".
	for _, in := range []string{
		"    Polling registry 'https://index.crates.io/'...\n\n  Package  Installed  Latest  Needs update\n  racer    v1.2.10    v1.2.10  No\n",
		"  Package  Installed  Latest  Needs update\n",
		"    Polling registry 'https://index.crates.io/'...\n",
		"",
		"   \n",
	} {
		pkgs, err := ParseList([]byte(in))
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if len(pkgs) != 0 {
			t.Fatalf("%q: want 0 packages, got %v", in, pkgs)
		}
	}
}

func TestParseListIgnoresPreambleNoise(t *testing.T) {
	// A stray line that happens to split into 4+ cells must not be read as a row
	// while no header has been seen.
	out := "  some  stray  banner  Yes\n\n  Package  Installed  Latest  Needs update\n  tokei    v12.1.2    v13.0.0  Yes\n"
	pkgs, err := ParseList([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || pkgs[0].Name != "tokei" {
		t.Fatalf("want only the post-header row, got %v", pkgs)
	}
}
