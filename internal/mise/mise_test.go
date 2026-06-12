package mise

import "testing"

func TestParseLs(t *testing.T) {
	out := []byte(`{
		"jq": [{"version": "1.7.1", "requested_version": "latest", "installed": true}],
		"node": [{"version": "22.3.0", "requested_version": "lts", "installed": false}],
		"ripgrep": [{"version": "14.1.0", "future_field": {"nested": true}}]
	}`)
	tools, err := ParseLs(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 3 {
		t.Fatalf("want 3 tools, got %d", len(tools))
	}
	byName := map[string]Tool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	if !byName["jq"].Installed {
		t.Error("jq should be installed")
	}
	if byName["node"].Installed {
		t.Error("node should be missing")
	}
	// no "installed" key → assume installed; unknown fields tolerated
	if !byName["ripgrep"].Installed {
		t.Error("ripgrep should default to installed")
	}
}

func TestParseOutdated(t *testing.T) {
	out := []byte(`{
		"node": {"current": "22.1.0", "requested": "lts", "latest": "22.3.0"},
		"jq": {"current": "", "latest": "1.8.0"}
	}`)
	outdated, err := ParseOutdated(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(outdated) != 1 {
		t.Fatalf("want 1 outdated (empty current skipped), got %d: %v", len(outdated), outdated)
	}
	if outdated[0].Name != "node" || outdated[0].Wanted != "22.3.0" {
		t.Errorf("unexpected entry: %+v", outdated[0])
	}
}
