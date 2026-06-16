package tui

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/logging"
	"github.com/mikevalstar/myplace/internal/outdated"
)

// --- commands -------------------------------------------------------------

func (m Model) loadCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		return reportMsg{drift.Compute(ctx, m.ch, m.ms, m.version)}
	}
}

// loadInventoryCmd gathers the cross-manager outdated inventory independently
// of the drift report, so the dashboard isn't blocked waiting on it.
func (m Model) loadInventoryCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		return inventoryMsg{outdated.Collect(ctx, m.sources...)}
	}
}

// loadSysinfoCmd gathers the fastfetch system snapshot for the header band,
// independently of the drift report. fastfetch being absent is not fatal — the
// band shows a notice (sysinfoMsg.err set) and the dashboard renders normally.
func (m Model) loadSysinfoCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		info, err := m.si.Fetch(ctx)
		return sysinfoMsg{info: info, err: err}
	}
}

// diffCmd fetches `chezmoi diff <target>` for a selected dotfile. It is
// read-only (a diff is a read) and the target is built exactly as the CLI
// capture flow does (cmd/myplace/update.go) so the path resolves identically.
func (m Model) diffCmd(relPath string) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return diffMsg{path: relPath, err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, derr := m.ch.Diff(ctx, filepath.Join(home, relPath))
		return diffMsg{path: relPath, diff: out, err: derr}
	}
}

func activityTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return activityTickMsg{} })
}

// --- per-step update chain ------------------------------------------------
//
// The converge-only update (same behavior as headless `myplace update --yes`)
// runs as a sequence of step commands, each returning a stepDoneMsg, so the UI
// can show progress. Capturing OUTGOING edits (keep/discard per file) needs
// interactive prompts the dashboard doesn't host (ADR-0006), so when local
// edits exist we skip the dotfiles apply rather than clobber them.

func (m Model) chezmoiStepCmd() tea.Cmd {
	skip := m.report != nil && len(m.report.Dotfiles.LocalModified) > 0
	return func() tea.Msg {
		if skip {
			return stepDoneMsg{step: stepChezmoi, skipped: true}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		return stepDoneMsg{step: stepChezmoi, err: m.ch.Update(ctx)}
	}
}

func (m Model) miseInstallStepCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		m.ms.Trust(ctx)
		return stepDoneMsg{step: stepMiseInstall, err: m.ms.Install(ctx)}
	}
}

func (m Model) miseUpgradeStepCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		return stepDoneMsg{step: stepMiseUpgrade, err: m.ms.Upgrade(ctx)}
	}
}

// --- Update ---------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = m.width
		if m.mode == modeOutdated {
			m.vp.Width, m.vp.Height = m.width, m.height-3
			m.vp.SetContent(m.outdatedContent())
		}
		m.sizeDetail()
		cmd := m.syncDetail()
		return m, cmd

	case activityTickMsg:
		m.activity = logging.RecentLines(200)
		return m, activityTick()

	case reportMsg:
		m.report = &msg.report
		m.loading = false
		m.updating = false // a post-update reload landed — close the modal
		m.updateStep = stepNone
		m.clampSelections()
		m.sizeDetail()
		return m, m.syncDetail()

	case sysinfoMsg:
		if msg.err != nil {
			m.systemErr = true
		} else {
			m.system = msg.info
		}
		return m, nil

	case inventoryMsg:
		m.inventory = &msg.inv
		m.invLoading = false
		if m.mode == modeOutdated {
			m.vp.SetContent(m.outdatedContent())
		}
		if m.focus == focusUpdates {
			return m, m.syncDetail()
		}
		return m, nil

	case diffMsg:
		delete(m.diffPending, msg.path)
		if msg.err != nil {
			m.diffCache[msg.path] = "! diff failed: " + msg.err.Error()
		} else if msg.diff == "" {
			m.diffCache[msg.path] = "(no diff — nothing for `apply` to change)"
		} else {
			m.diffCache[msg.path] = msg.diff
		}
		// If this diff is for the current selection, show it now.
		return m, m.syncDetail()

	case stepDoneMsg:
		return m.advanceUpdate(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m Model) advanceUpdate(msg stepDoneMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.skipped:
		m.updateErrs = append(m.updateErrs, "local edits present — run `myplace update` in a terminal to review them (keep/discard); skipped dotfiles apply to avoid overwriting")
	case msg.err != nil:
		m.updateErrs = append(m.updateErrs, stepLabel(msg.step)+": "+msg.err.Error())
	}
	switch msg.step {
	case stepChezmoi:
		m.updateStep = stepMiseInstall
		return m, m.miseInstallStepCmd()
	case stepMiseInstall:
		m.updateStep = stepMiseUpgrade
		return m, m.miseUpgradeStepCmd()
	default: // stepMiseUpgrade — steps done; reload status while the modal stays up
		m.updateStep = stepDone
		m.invLoading = true
		// Keep m.updating = true (and m.loading = false) so the modal stays
		// composited over the dashboard during the reload instead of flashing
		// the full-screen loading spinner. reportMsg clears m.updating.
		return m, tea.Batch(m.spinner.Tick, m.loadCmd(), m.loadInventoryCmd())
	}
}

func stepLabel(s updateStep) string {
	switch s {
	case stepChezmoi:
		return "chezmoi update"
	case stepMiseInstall:
		return "mise install"
	case stepMiseUpgrade:
		return "mise upgrade"
	default:
		return "update"
	}
}

// --- key handling ---------------------------------------------------------

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c always quits, even mid-filter.
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	// While typing a filter, keystrokes belong to the text input (except the
	// keys that end filtering, handled in updateOutdated).
	if m.filtering {
		return m.updateOutdated(msg)
	}
	// Help overlay: `?` toggles; while open it swallows everything but esc/?.
	if key.Matches(msg, m.keys.Help) {
		m.showHelp = !m.showHelp
		return m, nil
	}
	if m.showHelp {
		if key.Matches(msg, m.keys.Esc) {
			m.showHelp = false
		}
		return m, nil
	}
	switch m.mode {
	case modeOutdated:
		return m.updateOutdated(msg)
	case modeDetail:
		return m.updateDetailMode(msg)
	default:
		return m.updateDashboard(msg)
	}
}

func (m Model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Outdated):
		m.mode = modeOutdated
		m.vp = viewport.New(m.width, m.height-3)
		m.vp.SetContent(m.outdatedContent())
		return m, nil
	case key.Matches(msg, m.keys.Refresh):
		if !m.loading && !m.updating {
			m.loading = true
			m.invLoading = true
			m.updateErrs = nil
			return m, tea.Batch(m.spinner.Tick, m.loadCmd(), m.loadInventoryCmd())
		}
	case key.Matches(msg, m.keys.Update):
		if !m.loading && !m.updating {
			m.updating = true
			m.updateStep = stepChezmoi
			m.updateErrs = nil
			return m, tea.Batch(m.spinner.Tick, m.chezmoiStepCmd())
		}
	case msg.String() == "shift+tab" || key.Matches(msg, m.keys.Left):
		m.moveFocus(-1)
		return m, m.syncDetail()
	case key.Matches(msg, m.keys.Focus) || key.Matches(msg, m.keys.Right):
		m.moveFocus(1)
		return m, m.syncDetail()
	case key.Matches(msg, m.keys.Up):
		m.moveSel(-1)
		return m, m.syncDetail()
	case key.Matches(msg, m.keys.Down):
		m.moveSel(1)
		return m, m.syncDetail()
	case key.Matches(msg, m.keys.Enter):
		// On narrow terminals the detail isn't shown beside the panes, so
		// `enter` opens it full-screen. On wide terminals it's already visible.
		if !m.wide() {
			m.mode = modeDetail
			m.sizeDetail()
			return m, m.syncDetail()
		}
	}
	return m, nil
}

func (m Model) updateDetailMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Esc) || key.Matches(msg, m.keys.Quit):
		m.mode = modeDashboard
		m.sizeDetail()
		return m, m.syncDetail()
	}
	var cmd tea.Cmd
	m.detailVP, cmd = m.detailVP.Update(msg)
	return m, cmd
}

func (m Model) updateOutdated(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		switch {
		case key.Matches(msg, m.keys.Esc):
			m.filtering = false
			m.filter.SetValue("")
			m.filter.Blur()
			m.vp.SetContent(m.outdatedContent())
			return m, nil
		case msg.String() == "enter":
			m.filtering = false
			m.filter.Blur()
			m.vp.SetContent(m.outdatedContent())
			return m, nil
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.vp.SetContent(m.outdatedContent())
		return m, cmd
	}
	switch {
	case key.Matches(msg, m.keys.Esc), key.Matches(msg, m.keys.Quit):
		m.mode = modeDashboard
		return m, nil
	case key.Matches(msg, m.keys.Sort):
		if m.sort == sortBySource {
			m.sort = sortByName
		} else {
			m.sort = sortBySource
		}
		m.vp.SetContent(m.outdatedContent())
		return m, nil
	case key.Matches(msg, m.keys.Filter):
		m.filtering = true
		m.filter.Focus()
		return m, nil
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// --- navigation helpers ---------------------------------------------------

func (m Model) wide() bool { return m.width >= wideThreshold }

func (m *Model) moveFocus(delta int) {
	m.focus = focus((int(m.focus) + delta + int(focusCount)) % int(focusCount))
}

func (m *Model) moveSel(delta int) {
	n := m.selCount(m.focus)
	if n == 0 {
		m.sel[m.focus] = 0
		return
	}
	s := m.sel[m.focus] + delta
	if s < 0 {
		s = 0
	}
	if s > n-1 {
		s = n - 1
	}
	m.sel[m.focus] = s
}

func (m *Model) clampSelections() {
	for f := focus(0); f < focusCount; f++ {
		n := m.selCount(f)
		if n == 0 {
			m.sel[f] = 0
		} else if m.sel[f] > n-1 {
			m.sel[f] = n - 1
		}
	}
}

func (m Model) selCount(f focus) int {
	if m.report == nil {
		return 0
	}
	_, items := m.paneContent(f)
	return len(items)
}

// sizeDetail sizes the detail viewport for the current context. Called whenever
// the window resizes or the mode/selection changes, so the diff content is
// always truncated to the right width.
func (m *Model) sizeDetail() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	if m.mode == modeDetail || (m.mode == modeDashboard && !m.wide()) {
		// full-screen (header + viewport + footer)
		m.detailVP.Width = m.width
		m.detailVP.Height = max1(m.height - 3)
		return
	}
	// wide inline panel: right ~40%, inside a bordered box with a title row
	leftW := m.width * 60 / 100
	dw := m.width - leftW
	m.detailVP.Width = max1(dw - 4)
	m.detailVP.Height = max1((m.height - 3) - 2 - 1)
}

// syncDetail rebuilds the detail viewport content from the focused selection.
// For a dotfile it shows the cached diff (fetching it lazily, returning the
// diff command) or "loading…"; for tools/packages it renders detail from data
// already loaded.
func (m *Model) syncDetail() tea.Cmd {
	if m.report == nil {
		return nil
	}
	_, items := m.paneContent(m.focus)
	if len(items) == 0 {
		m.detailVP.SetContent(m.theme.Subtle.Render("  nothing selected"))
		return nil
	}
	if m.sel[m.focus] > len(items)-1 {
		m.sel[m.focus] = len(items) - 1
	}
	it := items[m.sel[m.focus]]
	if it.isDiff {
		if d, ok := m.diffCache[it.path]; ok {
			m.detailVP.SetContent(m.renderDiff(d))
			return nil
		}
		m.detailVP.SetContent(m.theme.Subtle.Render("  loading diff…"))
		if m.ch != nil && !m.diffPending[it.path] {
			m.diffPending[it.path] = true
			return m.diffCmd(it.path)
		}
		return nil
	}
	m.detailVP.SetContent(m.renderDetailBody(it))
	return nil
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
