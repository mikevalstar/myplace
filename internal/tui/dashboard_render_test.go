package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/mikevalstar/myplace/internal/drift"
)

func ptr[T any](v T) *T { return &v }

// sampleModel builds a populated, non-loading dashboard for render checks.
func sampleModel(w, h int) Model {
	m := New(nil, nil, "0.1.0")
	m.loading = false
	m.width, m.height = w, h
	latest := "0.2.0"
	m.report = &drift.Report{
		Verdict:   drift.VerdictDrifted,
		Machine:   "mikes-macbook-air",
		Profile:   "personal-mac",
		CheckedAt: time.Date(2026, 6, 12, 20, 43, 0, 0, time.UTC),
		Dotfiles: drift.Dotfiles{
			BehindOrigin: ptr(1), ToApply: []string{".config/mise/config.toml"},
			LocalModified: []string{".zshrc"}, UncommittedFiles: ptr(0), UnpushedCommits: ptr(0),
		},
		Tools: drift.Tools{
			Missing:  []string{"fzf"},
			Outdated: []drift.ToolIssue{{Name: "node", Current: "22.1.0", Wanted: "22.3.0"}},
		},
		Myplace: drift.Myplace{Current: "0.1.0", Latest: &latest},
	}
	m.activity = []string{
		`2026-06-12T20:43:45-04:00 DEBU exec cmd=tui pid=74117 tool=chezmoi args="--no-tty data --format json" dir="" dur=11ms`,
		`2026-06-12T20:43:46-04:00 DEBU exec cmd=tui pid=74117 tool=chezmoi args="--no-tty git -- fetch --quiet" dir="" dur=534ms`,
		`2026-06-12T20:43:46-04:00 INFO status cmd=tui pid=74117 verdict=drifted to_apply=1 local_modified=1`,
	}
	return m
}

// TestRenderNoWrap asserts every rendered line fits the terminal width — i.e.
// nothing wraps past the frame. Run `go test -run RenderSnapshot -v` to eyeball.
func TestRenderNoWrap(t *testing.T) {
	for _, w := range []int{80, 100, 120} {
		h := 30
		out := sampleModel(w, h).View()
		for i, line := range strings.Split(out, "\n") {
			if lw := lipgloss.Width(line); lw > w {
				t.Errorf("w=%d line %d width %d exceeds terminal (%q)", w, i, lw, line)
			}
		}
	}
}

func TestRenderSnapshot(t *testing.T) {
	if testing.Verbose() {
		fmt.Println(sampleModel(100, 28).View())
	}
}
