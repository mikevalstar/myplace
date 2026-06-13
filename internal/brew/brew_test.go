package brew

import "testing"

func TestParseOutdated(t *testing.T) {
	// Shape of `brew outdated --json=v2`: formulae and casks each carry
	// installed_versions + current_version. The empty-installed entry must be
	// skipped, and unknown fields (pinned, pinned_version) tolerated.
	out := []byte(`{
		"formulae": [
			{"name": "git", "installed_versions": ["2.43.0"], "current_version": "2.45.0", "pinned": false, "pinned_version": null},
			{"name": "ghost", "installed_versions": [], "current_version": "1.0.0"}
		],
		"casks": [
			{"name": "firefox", "installed_versions": ["120.0"], "current_version": "121.0"}
		]
	}`)
	pkgs, err := ParseOutdated(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("want 2 packages (empty-installed skipped), got %d: %v", len(pkgs), pkgs)
	}
	byName := map[string]Package{}
	for _, p := range pkgs {
		byName[p.Name] = p
	}
	if g := byName["git"]; g.Current != "2.43.0" || g.Latest != "2.45.0" {
		t.Errorf("git: got %+v, want current 2.43.0 / latest 2.45.0", g)
	}
	if _, ok := byName["firefox"]; !ok {
		t.Error("cask firefox should be included alongside formulae")
	}
	if _, ok := byName["ghost"]; ok {
		t.Error("entry with no installed version should be skipped")
	}
}

func TestParseOutdatedEmpty(t *testing.T) {
	// `brew outdated --json=v2` with nothing outdated still has both keys.
	pkgs, err := ParseOutdated([]byte(`{"formulae": [], "casks": []}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 0 {
		t.Fatalf("want 0 packages, got %d", len(pkgs))
	}
}

func TestParseOutdatedMalformed(t *testing.T) {
	if _, err := ParseOutdated([]byte(`not json`)); err == nil {
		t.Error("expected an error for malformed JSON")
	}
}
