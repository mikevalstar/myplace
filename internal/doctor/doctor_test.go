package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		name   string
		checks []Check
		want   string
	}{
		{"all pass", []Check{{Status: StatusPass}, {Status: StatusPass}}, VerdictPass},
		{"a warn, no fail", []Check{{Status: StatusPass}, {Status: StatusWarn}}, VerdictIncomplete},
		{"a fail dominates a warn", []Check{{Status: StatusWarn}, {Status: StatusFail}}, VerdictFail},
		{"empty is pass", nil, VerdictPass},
	}
	for _, tc := range cases {
		if got := decide(tc.checks); got != tc.want {
			t.Errorf("%s: decide = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestExitCode(t *testing.T) {
	cases := map[string]int{
		VerdictPass:       0,
		VerdictFail:       1,
		VerdictIncomplete: 2,
		"anything else":   3,
	}
	for verdict, want := range cases {
		if got := ExitCode(verdict); got != want {
			t.Errorf("ExitCode(%q) = %d, want %d", verdict, got, want)
		}
	}
}

func TestCompareDotted(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2.70.5", "2.0.0", 1},
		{"2.0.0", "2.70.5", -1},
		{"2.70.5", "2.70.5", 0},
		{"2026.6.10", "2024.1.0", 1},
		{"1.9.9", "2.0.0", -1},
		{"2.0", "2.0.0", 0}, // missing components compare as 0
		{"2.0.1", "2.0", 1}, // extra component beats the implicit 0
	}
	for _, tc := range cases {
		if got := compareDotted(tc.a, tc.b); got != tc.want {
			t.Errorf("compareDotted(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCheckVersionBelowFloorFails(t *testing.T) {
	c := checkVersion("mise_version", "mise version", true,
		func() (string, error) { return "2023.1.0", nil }, miseFloor, "mise self-update")
	if c.Status != StatusFail {
		t.Fatalf("status = %q, want fail", c.Status)
	}
	if c.Remedy == "" {
		t.Error("a failed version check must name a remedy")
	}
}

func TestCheckVersionUnknownWarnsNotFails(t *testing.T) {
	// Installed but the banner didn't parse: we can't claim a failure.
	c := checkVersion("chezmoi_version", "chezmoi version", true,
		func() (string, error) { return "", nil }, chezmoiFloor, "chezmoi upgrade")
	if c.Status != StatusWarn {
		t.Errorf("status = %q, want warn", c.Status)
	}
}

func TestCheckPATH(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	bin := filepath.Join(home, ".local", "bin")

	present := bin + string(os.PathListSeparator) + "/usr/bin"
	if c := checkPATH(present); c.Status != StatusPass {
		t.Errorf("with ~/.local/bin present: status = %q, want pass", c.Status)
	}

	absent := "/usr/bin" + string(os.PathListSeparator) + "/bin"
	c := checkPATH(absent)
	if c.Status != StatusFail {
		t.Errorf("with ~/.local/bin absent: status = %q, want fail", c.Status)
	}
	if c.Remedy == "" {
		t.Error("a failed PATH check must name a remedy")
	}
}
