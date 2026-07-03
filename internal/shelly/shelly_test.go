package shelly

import "testing"

func TestParseOutdated(t *testing.T) {
	// Inferred shape of `shelly check-updates --json`: an object wrapping an
	// `updates` array, one entry per package across every channel. Secondary
	// channels (aur/flatpak/appimage) get channel-prefixed; native repo packages
	// stay bare. Entry with no current version is skipped; unknown fields ignored.
	// (Confirm the real field names on a CachyOS box — see the parser's NOTE.)
	out := []byte(`{
		"updates": [
			{"name": "linux", "source": "core", "current_version": "6.9.1", "new_version": "6.9.2", "download_size": 12345},
			{"name": "yay", "source": "aur", "current_version": "12.4.0", "new_version": "12.4.1"},
			{"name": "org.gimp.GIMP", "source": "flatpak", "current_version": "2.10.36", "new_version": "2.10.38"},
			{"name": "ghost", "source": "core", "current_version": "", "new_version": "1.0.0"}
		]
	}`)
	pkgs, err := ParseOutdated(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 3 {
		t.Fatalf("want 3 packages (empty-current skipped), got %d: %v", len(pkgs), pkgs)
	}
	byName := map[string]Package{}
	for _, p := range pkgs {
		byName[p.Name] = p
	}
	if l := byName["linux"]; l.Current != "6.9.1" || l.Latest != "6.9.2" {
		t.Errorf("linux: got %+v, want current 6.9.1 / latest 6.9.2", l)
	}
	if _, ok := byName["aur:yay"]; !ok {
		t.Errorf("AUR package should be channel-prefixed as aur:yay, got %v", pkgs)
	}
	if _, ok := byName["flatpak:org.gimp.GIMP"]; !ok {
		t.Errorf("flatpak package should be channel-prefixed, got %v", pkgs)
	}
	if _, ok := byName["ghost"]; ok {
		t.Error("entry with no current version should be skipped")
	}
}

func TestParseOutdatedBareArray(t *testing.T) {
	// Tolerate a top-level array too (in case check-updates emits one directly),
	// and field-name aliases for current/new.
	pkgs, err := ParseOutdated([]byte(`[{"name": "vim", "installed": "9.1.0", "available": "9.1.1"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || pkgs[0].Name != "vim" || pkgs[0].Current != "9.1.0" || pkgs[0].Latest != "9.1.1" {
		t.Fatalf("bare-array/alias parse: got %v", pkgs)
	}
}

func TestParseOutdatedEmpty(t *testing.T) {
	// Nothing outdated: an empty updates array, and truly empty output, both
	// yield zero packages without erroring.
	for _, in := range []string{`{"updates": []}`, `[]`, ``} {
		pkgs, err := ParseOutdated([]byte(in))
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if len(pkgs) != 0 {
			t.Fatalf("%q: want 0 packages, got %d", in, len(pkgs))
		}
	}
}

func TestParseOutdatedMalformed(t *testing.T) {
	if _, err := ParseOutdated([]byte(`not json`)); err == nil {
		t.Error("expected an error for malformed JSON")
	}
}
