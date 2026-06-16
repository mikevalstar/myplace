package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/outdated"
)

func ptr[T any](v T) *T { return &v }

// sampleModel builds a populated, non-loading dashboard for render checks. The
// detail viewport is sized and synced so the master-detail panel has content.
func sampleModel(w, h int) Model {
	m := New(nil, nil, nil, nil, "0.1.0")
	m.loading = false
	m.invLoading = false
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
	m.inventory = &outdated.Inventory{
		Schema:    outdated.Schema,
		Machine:   "mikes-macbook-air",
		CheckedAt: time.Date(2026, 6, 12, 20, 43, 0, 0, time.UTC),
		Sources: []outdated.SourceResult{
			{Name: "mise", Available: true, Packages: []outdated.Package{
				{Name: "node", Current: "22.1.0", Latest: "22.3.0"},
			}},
			{Name: "brew", Available: true, Packages: []outdated.Package{
				{Name: "git", Current: "2.43.0", Latest: "2.45.0"},
				{Name: "htop", Current: "3.5.0", Latest: "3.5.1"},
			}},
		},
	}
	m.activity = []string{
		`2026-06-12T20:43:45-04:00 DEBU exec cmd=tui pid=74117 tool=chezmoi args="--no-tty data --format json" dir="" dur=11ms`,
		`2026-06-12T20:43:46-04:00 DEBU exec cmd=tui pid=74117 tool=chezmoi args="--no-tty git -- fetch --quiet" dir="" dur=534ms`,
		`2026-06-12T20:43:46-04:00 INFO status cmd=tui pid=74117 verdict=drifted to_apply=1 local_modified=1`,
	}
	m.sizeDetail()
	m.syncDetail()
	return m
}

func assertNoOverflow(t *testing.T, label, out string, w int) {
	t.Helper()
	for i, line := range strings.Split(out, "\n") {
		if lw := lipgloss.Width(line); lw > w {
			t.Errorf("%s w=%d line %d width %d exceeds terminal (%q)", label, w, i, lw, line)
		}
	}
}

// TestRenderNoWrap asserts every rendered line fits the terminal width — i.e.
// nothing wraps past the frame — across narrow, wide, and tall terminals (the
// master-detail split and focus highlight must hold too).
func TestRenderNoWrap(t *testing.T) {
	// Tiny terminals fall back to the stacked smallView — it must still fit.
	for _, wh := range [][2]int{{40, 12}, {50, 10}} {
		m := sampleModel(wh[0], wh[1])
		assertNoOverflow(t, "small", m.View(), wh[0])
	}
	for _, w := range []int{60, 80, 100, 120, 160, 200, 240} {
		for _, h := range []int{16, 30, 50} {
			m := sampleModel(w, h)
			assertNoOverflow(t, "dashboard", m.View(), w)
			// Focus each pane and select an item, to exercise the highlight.
			for f := focus(0); f < focusCount; f++ {
				fm := sampleModel(w, h)
				fm.focus = f
				fm.sel[f] = 1
				fm.syncDetail()
				assertNoOverflow(t, fmt.Sprintf("focus=%d", f), fm.View(), w)
			}
		}
	}
}

// TestRenderDetailPanel checks the wide master-detail panel renders a selected
// dotfile's diff with +/- coloring and without overflow.
func TestRenderDetailPanel(t *testing.T) {
	for _, w := range []int{120, 160, 200} {
		m := sampleModel(w, 40)
		m.focus = focusDotfiles
		m.sel[focusDotfiles] = 0
		m.diffCache[".config/mise/config.toml"] = "--- a/.config/mise/config.toml\n+++ b/.config/mise/config.toml\n@@ -1,2 +1,2 @@\n-old = \"value\"\n+new = \"value\"\n"
		m.syncDetail()
		out := m.View()
		assertNoOverflow(t, "detail", out, w)
		for _, want := range []string{"new = ", "old = ", "config.toml"} {
			if !strings.Contains(out, want) {
				t.Errorf("w=%d detail panel missing %q", w, want)
			}
		}
	}
}

// TestRenderHelpOverlay checks the `?` overlay lists keys and fits.
func TestRenderHelpOverlay(t *testing.T) {
	for _, w := range []int{80, 160} {
		m := sampleModel(w, 40)
		m.showHelp = true
		out := m.View()
		assertNoOverflow(t, "help", out, w)
		for _, want := range []string{"keys", "refresh", "update", "quit"} {
			if !strings.Contains(out, want) {
				t.Errorf("w=%d help overlay missing %q", w, want)
			}
		}
	}
}

// TestRenderUpdateProgress checks the update modal floats over the dashboard
// (the backdrop survives) and the per-step checklist renders at each step,
// without overflow — in both narrow and wide (composited) layouts.
func TestRenderUpdateProgress(t *testing.T) {
	for _, w := range []int{100, 160} {
		for _, step := range []updateStep{stepChezmoi, stepMiseInstall, stepMiseUpgrade} {
			m := sampleModel(w, 30)
			m.updating = true
			m.updateStep = step
			out := m.View()
			assertNoOverflow(t, "progress", out, w)
			for _, want := range []string{"chezmoi apply", "mise install", "mise upgrade", "updating"} {
				if !strings.Contains(out, want) {
					t.Errorf("w=%d step=%d modal missing %q", w, step, want)
				}
			}
			// The dimmed dashboard backdrop should still be present behind it.
			if !strings.Contains(out, "Activity") {
				t.Errorf("w=%d step=%d expected dimmed dashboard backdrop behind modal", w, step)
			}
		}
		// The post-step reload (stepDone) must keep the modal up — not flash
		// the full-screen "checking status…" loading screen.
		m := sampleModel(w, 30)
		m.updating = true
		m.updateStep = stepDone
		out := m.View()
		assertNoOverflow(t, "progress-done", out, w)
		if strings.Contains(out, "checking status") {
			t.Errorf("w=%d reload phase flashed the loading screen instead of keeping the modal", w)
		}
		if !strings.Contains(out, "refreshing status") {
			t.Errorf("w=%d reload phase should show 'refreshing status' in the modal", w)
		}
	}
}

// TestRenderOutdatedView checks the `o` detail screen fits and lists sources,
// in both sort modes and when filtered.
func TestRenderOutdatedView(t *testing.T) {
	for _, w := range []int{80, 100, 120} {
		h := 30
		// default: grouped by source
		m := sampleModel(w, h)
		m.mode = modeOutdated
		m.vp = viewport.New(w, h-3)
		m.vp.SetContent(m.outdatedContent())
		out := m.outdatedView()
		assertNoOverflow(t, "outdated", out, w)
		for _, want := range []string{"mise", "brew", "node", "git", "outdated across"} {
			if !strings.Contains(out, want) {
				t.Errorf("w=%d outdated view missing %q", w, want)
			}
		}

		// sort by name
		mn := sampleModel(w, h)
		mn.mode = modeOutdated
		mn.sort = sortByName
		mn.vp = viewport.New(w, h-3)
		mn.vp.SetContent(mn.outdatedContent())
		assertNoOverflow(t, "outdated-name", mn.outdatedView(), w)
		if !strings.Contains(mn.outdatedContent(), "by name") {
			t.Errorf("w=%d sort-by-name summary missing", w)
		}

		// filtered to "git"
		mf := sampleModel(w, h)
		mf.mode = modeOutdated
		mf.filter.SetValue("git")
		mf.vp = viewport.New(w, h-3)
		body := mf.outdatedContent()
		assertNoOverflow(t, "outdated-filter", mf.outdatedView(), w)
		if !strings.Contains(body, "git") {
			t.Errorf("w=%d filtered view should keep git", w)
		}
		if strings.Contains(body, "node ") || strings.Contains(body, "htop ") {
			t.Errorf("w=%d filtered view should hide non-matching packages:\n%s", w, body)
		}
		if !strings.Contains(body, "shown") {
			t.Errorf("w=%d filtered summary should show an X of N count", w)
		}
	}
}

// TestNavigation drives focus/selection/detail transitions through Update.
func TestNavigation(t *testing.T) {
	m := sampleModel(100, 30) // narrow → enter opens full-screen detail

	// select down within the focused (Dotfiles) pane
	m = step(m, keyMsg("j"))
	if m.sel[focusDotfiles] != 1 {
		t.Fatalf("expected dotfiles selection 1, got %d", m.sel[focusDotfiles])
	}

	// tab moves focus to Tools
	m = step(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusTools {
		t.Fatalf("expected focus Tools, got %d", m.focus)
	}

	// enter opens the full-screen detail (narrow terminal)
	m = step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeDetail {
		t.Fatalf("expected modeDetail after enter, got %d", m.mode)
	}

	// esc returns to the dashboard
	m = step(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeDashboard {
		t.Fatalf("expected modeDashboard after esc, got %d", m.mode)
	}

	// ? toggles the help overlay
	m = step(m, keyMsg("?"))
	if !m.showHelp {
		t.Fatalf("expected help overlay open")
	}
}

func step(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestRenderSnapshot(t *testing.T) {
	if testing.Verbose() {
		fmt.Println(sampleModel(160, 44).View())
		m := sampleModel(100, 28)
		m.mode = modeOutdated
		m.vp = viewport.New(100, 25)
		m.vp.SetContent(m.outdatedContent())
		fmt.Println(m.outdatedView())
	}
}
