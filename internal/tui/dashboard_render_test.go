package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mikevalstar/myplace/internal/doctor"
	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/outdated"
	"github.com/mikevalstar/myplace/internal/release"
)

func ptr[T any](v T) *T { return &v }

// sampleModel builds a populated, non-loading dashboard for render checks. The
// detail viewport is sized and synced so the master-detail panel has content.
func sampleModel(w, h int) Model {
	m := New(nil, nil, nil, nil, "0.1.0", doctor.Options{})
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

// TestDashboardFillsHeight guards against vertical overflow: the paneled
// dashboard (wide and narrow) must render exactly `height` rows. One row too
// many scrolls the header and the top of the system-info band off the
// alt-screen — the regression where the detail viewport wasn't sized for the
// band. Run with the band populated, the realistic case.
func TestDashboardFillsHeight(t *testing.T) {
	for _, w := range []int{80, 100, 120, 160, 200} { // narrow (<120) and wide (≥120)
		for _, h := range []int{20, 30, 50} {
			m := sampleModel(w, h)
			m.system = sampleSysinfo()
			if got := strings.Count(m.View(), "\n") + 1; got != h {
				t.Errorf("w=%d h=%d: rendered %d rows, want %d (overflow clips the top)", w, h, got, h)
			}
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

// sampleDoctorReport is a mixed pass/warn/fail preflight for the `d` view.
func sampleDoctorReport() *doctor.Report {
	return &doctor.Report{
		Schema:    doctor.Schema,
		Machine:   "mikes-macbook-air",
		Profile:   "personal-mac",
		CheckedAt: time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC),
		Verdict:   doctor.VerdictFail,
		Checks: []doctor.Check{
			{ID: "chezmoi_installed", Label: "chezmoi installed", Status: doctor.StatusPass, Detail: "2.52.0"},
			{ID: "dotfiles_remote", Label: "dotfiles remote", Status: doctor.StatusWarn, Detail: "could not reach origin (offline?)"},
			{ID: "path", Label: "PATH", Status: doctor.StatusFail, Detail: "~/.local/bin not on PATH",
				Remedy: `add 'export PATH="$HOME/.local/bin:$PATH"' to your shell profile`},
		},
	}
}

// TestRenderDoctorView checks the `d` screen renders the verdict line, every
// check with its status glyph, and remedies — without overflow.
func TestRenderDoctorView(t *testing.T) {
	for _, w := range []int{80, 120} {
		h := 30
		m := sampleModel(w, h)
		m.mode = modeDoctor
		m.doctorRep = sampleDoctorReport()
		m.vp = viewport.New(w, h-3)
		m.vp.SetContent(m.doctorContent())
		out := m.doctorView()
		assertNoOverflow(t, "doctor", out, w)
		for _, want := range []string{"problems found — 1 failed, 1 warning(s)", "chezmoi installed", "dotfiles remote", "✓", "⚠", "✗", "→ add 'export PATH="} {
			if !strings.Contains(out, want) {
				t.Errorf("w=%d doctor view missing %q", w, want)
			}
		}
	}

	// While the checks run, the body is a centered spinner, not stale content.
	m := sampleModel(100, 30)
	m.mode = modeDoctor
	m.doctorLoading = true
	m.vp = viewport.New(100, 27)
	out := m.doctorView()
	assertNoOverflow(t, "doctor-loading", out, 100)
	if !strings.Contains(out, "running doctor checks") {
		t.Error("loading doctor view should show the in-flight notice")
	}
}

// TestRenderActivityView checks the `a` screen renders the log tail, the
// filter narrows it case-insensitively, and error lines are emphasized.
func TestRenderActivityView(t *testing.T) {
	for _, w := range []int{80, 120} {
		h := 30
		m := sampleModel(w, h)
		m.mode = modeActivity
		m.follow = true
		m.actLines = []string{
			`2026-07-03T10:00:00-04:00 DEBU exec cmd=tui tool=chezmoi args="status"`,
			`2026-07-03T10:00:01-04:00 ERRO exec failed cmd=tui tool=mise err="exit 1"`,
			`2026-07-03T10:00:02-04:00 INFO status cmd=tui verdict=drifted`,
		}
		m.vp = viewport.New(w, h-3)
		m.vp.SetContent(m.activityContent())
		out := m.activityView()
		assertNoOverflow(t, "activity", out, w)
		for _, want := range []string{"3 line(s)", "tool=chezmoi", "f follow: on"} {
			if !strings.Contains(out, want) {
				t.Errorf("w=%d activity view missing %q", w, want)
			}
		}

		m.filter.SetValue("ERRO")
		body := m.activityContent()
		if !strings.Contains(body, "1 of 3 line(s) shown") {
			t.Errorf("w=%d filtered activity should count matches, got:\n%s", w, body)
		}
		if strings.Contains(body, "verdict=drifted") {
			t.Errorf("w=%d filtered activity should hide non-matching lines", w)
		}
	}
}

// TestUpdatesPaneBinaryRow checks the outdated binary appears as a selectable
// Updates item (counted in the chip) and its detail renders the cached release
// notes with the self-update remedy — with no fetch when the cache is warm.
func TestUpdatesPaneBinaryRow(t *testing.T) {
	m := sampleModel(160, 40)
	rows, items := m.updatesContent()
	if len(items) == 0 || items[0].kind != kindRelease {
		t.Fatalf("expected the binary-update row as the first Updates selectable, got %+v", items)
	}
	found := false
	for _, r := range rows {
		if strings.Contains(r.text, "myplace: v0.1.0 → v0.2.0") {
			found = true
		}
	}
	if !found {
		t.Errorf("Updates pane missing the binary row, rows: %+v", rows)
	}
	// binary (1) + node (1) + git/htop (2)
	if got := m.cardCount(focusUpdates); got != 4 {
		t.Errorf("Updates chip should count the binary row: want 4, got %d", got)
	}

	m.focus = focusUpdates
	m.sel[focusUpdates] = 0
	m.rel = &release.Release{
		Tag: "v0.2.0", Name: "v0.2.0 — doctor view",
		Notes:       "## Changes\r\n- doctor view in the TUI\r\n",
		PublishedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		URL:         "https://github.com/mikevalstar/myplace/releases/tag/v0.2.0",
	}
	if cmd := m.syncDetail(); cmd != nil {
		t.Error("warm release cache should not trigger a fetch")
	}
	out := m.View()
	assertNoOverflow(t, "release-detail", out, 160)
	for _, want := range []string{"current: v0.1.0", "latest:  v0.2.0", "myplace self-update", "doctor view in the TUI"} {
		if !strings.Contains(out, want) {
			t.Errorf("release detail missing %q", want)
		}
	}

	// Cold cache: the detail shows a loading line and returns the fetch cmd once.
	mc := sampleModel(160, 40)
	mc.focus = focusUpdates
	mc.sel[focusUpdates] = 0
	if cmd := mc.syncDetail(); cmd == nil {
		t.Error("cold release cache should trigger the lazy fetch")
	}
	if !strings.Contains(mc.View(), "loading release notes") {
		t.Error("cold release detail should show the loading line")
	}
	if cmd := mc.syncDetail(); cmd != nil {
		t.Error("a pending fetch should not be issued twice")
	}
}

// TestDoctorActivityNavigation drives the d/a mode transitions through Update.
func TestDoctorActivityNavigation(t *testing.T) {
	m := sampleModel(100, 30)

	// d opens the doctor view and kicks off a run (the cmd is not executed here)
	m = step(m, keyMsg("d"))
	if m.mode != modeDoctor || !m.doctorLoading {
		t.Fatalf("expected modeDoctor with checks in flight, got mode=%d loading=%v", m.mode, m.doctorLoading)
	}
	m = step(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeDashboard {
		t.Fatalf("expected modeDashboard after esc, got %d", m.mode)
	}

	// re-entering with a cached report must not re-run the checks
	m.doctorLoading = false
	m.doctorRep = sampleDoctorReport()
	next, cmd := m.Update(keyMsg("d"))
	m = next.(Model)
	if cmd != nil {
		t.Error("cached doctor report should not re-run on re-entry")
	}
	if !strings.Contains(m.View(), "problems found") {
		t.Error("doctor view should render the cached report")
	}
	m = step(m, tea.KeyMsg{Type: tea.KeyEsc})

	// the not-ready verdict now surfaces as an Activity-pane notice
	noticed := false
	for _, r := range m.activityRows() {
		if strings.Contains(r.text, "doctor: problems found") {
			noticed = true
		}
	}
	if !noticed {
		t.Error("dashboard Activity pane should carry the doctor notice")
	}

	// a opens the activity view following; f toggles follow off
	m = step(m, keyMsg("a"))
	if m.mode != modeActivity || !m.follow {
		t.Fatalf("expected modeActivity following, got mode=%d follow=%v", m.mode, m.follow)
	}
	m = step(m, keyMsg("f"))
	if m.follow {
		t.Error("f should toggle follow off")
	}
	m = step(m, keyMsg("q"))
	if m.mode != modeDashboard {
		t.Fatalf("expected modeDashboard after q, got %d", m.mode)
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

		d := sampleModel(100, 28)
		d.mode = modeDoctor
		d.doctorRep = sampleDoctorReport()
		d.vp = viewport.New(100, 25)
		d.vp.SetContent(d.doctorContent())
		fmt.Println(d.doctorView())

		a := sampleModel(100, 28)
		a.mode = modeActivity
		a.follow = true
		a.actLines = a.activity
		a.vp = viewport.New(100, 25)
		a.vp.SetContent(a.activityContent())
		fmt.Println(a.activityView())
	}
}
