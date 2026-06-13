// Package tui is the Bubble Tea layer — the ONLY package that imports Charm
// UI libraries. It renders what internal/drift computes; it owns no logic.
// Layout is specified in docs/features/tui-dashboard.md.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/logging"
	"github.com/mikevalstar/myplace/internal/mise"
	"github.com/mikevalstar/myplace/internal/outdated"
)

// Minimum terminal size below which we render a plain stacked fallback rather
// than drawing broken frames.
const (
	minWidth  = 60
	minHeight = 16
)

var (
	paneBorder  = lipgloss.RoundedBorder()
	paneStyle   = lipgloss.NewStyle().Border(paneBorder).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	paneTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	headerStyle = lipgloss.NewStyle().Bold(true)
	ruleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	helpStyle   = lipgloss.NewStyle().Faint(true)
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	noticeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	subtleStyle = lipgloss.NewStyle().Faint(true)
	addStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	delStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	okBadge     = lipgloss.NewStyle().Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)
	warnBadge   = lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)
	errBadge    = lipgloss.NewStyle().Background(lipgloss.Color("1")).Foreground(lipgloss.Color("15")).Bold(true).Padding(0, 1)
)

type reportMsg struct{ report drift.Report }
type inventoryMsg struct{ inv outdated.Inventory }
type updateDoneMsg struct{ errs []string }
type activityTickMsg struct{}

// mode is which screen the dashboard is showing.
type mode int

const (
	modeDashboard mode = iota // the paneled home screen
	modeOutdated              // the scrollable outdated-packages detail view
)

type Model struct {
	ch      *chezmoi.Client
	ms      *mise.Client
	sources []outdated.Source
	version string

	spinner    spinner.Model
	report     *drift.Report
	inventory  *outdated.Inventory
	loading    bool
	invLoading bool
	updating   bool
	updateErrs []string
	activity   []string
	width      int
	height     int

	mode mode
	vp   viewport.Model // scrollable body for modeOutdated
}

func New(ch *chezmoi.Client, ms *mise.Client, sources []outdated.Source, version string) Model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	return Model{ch: ch, ms: ms, sources: sources, version: version, spinner: sp, loading: true, invLoading: true}
}

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

// updateCmd resolves incoming drift and tools — the same converge-only
// behavior as headless `myplace update --yes`. Capturing OUTGOING edits
// (keep/discard per file) needs interactive prompts the dashboard doesn't
// host yet, so when local edits exist we skip the dotfiles apply (rather
// than clobber them) and point the user at the CLI capture flow.
func (m Model) updateCmd() tea.Cmd {
	hasLocalEdits := m.report != nil && len(m.report.Dotfiles.LocalModified) > 0
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		var errs []string
		if hasLocalEdits {
			errs = append(errs, "local edits present — run `myplace update` in a terminal to review them (keep/discard); skipped dotfiles apply to avoid overwriting")
		} else if err := m.ch.Update(ctx); err != nil {
			errs = append(errs, "chezmoi update: "+err.Error())
		}
		m.ms.Trust(ctx)
		if err := m.ms.Install(ctx); err != nil {
			errs = append(errs, "mise install: "+err.Error())
		}
		if err := m.ms.Upgrade(ctx); err != nil {
			errs = append(errs, "mise upgrade: "+err.Error())
		}
		return updateDoneMsg{errs}
	}
}

func activityTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return activityTickMsg{} })
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadCmd(), m.loadInventoryCmd(), activityTick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.mode == modeOutdated {
			m.vp.Width, m.vp.Height = m.width, m.height-3
		}
		return m, nil
	case activityTickMsg:
		m.activity = logging.RecentLines(200)
		return m, activityTick()
	case reportMsg:
		m.report = &msg.report
		m.loading = false
		return m, nil
	case inventoryMsg:
		m.inventory = &msg.inv
		m.invLoading = false
		if m.mode == modeOutdated {
			m.vp.SetContent(m.outdatedContent())
		}
		return m, nil
	case updateDoneMsg:
		m.updating = false
		m.updateErrs = msg.errs
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.loadCmd())
	case tea.KeyMsg:
		// In the outdated detail view, keys drive the scroll/return; the
		// dashboard's r/u/o are inactive there.
		if m.mode == modeOutdated {
			switch msg.String() {
			case "esc", "q":
				m.mode = modeDashboard
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "o":
			m.mode = modeOutdated
			m.vp = viewport.New(m.width, m.height-3)
			m.vp.SetContent(m.outdatedContent())
			return m, nil
		case "r":
			if !m.loading && !m.updating {
				m.loading = true
				m.invLoading = true
				m.updateErrs = nil
				return m, tea.Batch(m.spinner.Tick, m.loadCmd(), m.loadInventoryCmd())
			}
		case "u":
			if !m.loading && !m.updating {
				m.updating = true
				m.updateErrs = nil
				return m, tea.Batch(m.spinner.Tick, m.updateCmd())
			}
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func badge(verdict string) string {
	switch verdict {
	case drift.VerdictInSync:
		return okBadge.Render("IN SYNC")
	case drift.VerdictDrifted:
		return warnBadge.Render("DRIFTED")
	case drift.VerdictUnknown:
		return warnBadge.Render("UNKNOWN")
	default:
		return errBadge.Render("ERROR")
	}
}

func count(p *int) string {
	if p == nil {
		return "?"
	}
	return fmt.Sprintf("%d", *p)
}

// truncate clips s to w display columns, adding an ellipsis when cut. It is
// ANSI-aware (ansi.Truncate), so styled lines keep their escape codes intact
// and never wrap inside a pane.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// box renders title + body lines into a bordered pane exactly w×h. lipgloss
// .Width() counts padding but not the border, so the frame width is w-2 and the
// usable content width is w-4. Lines are truncated to fit width and clipped to
// fit height (with a "+N more" tail) so nothing wraps or overflows the frame.
func box(title string, lines []string, w, h int) string {
	frameW := w - 2   // inside the border (includes padding)
	contentW := w - 4 // inside border and padding
	innerH := h - 2   // inside the (top+bottom) border
	if contentW < 1 || innerH < 1 {
		return ""
	}
	content := []string{paneTitle.Render(truncate(title, contentW))}
	avail := innerH - 1 // minus title row
	if len(lines) > avail && avail > 0 {
		shown := lines[:avail-1]
		for _, ln := range shown {
			content = append(content, truncate(ln, contentW))
		}
		content = append(content, subtleStyle.Render(fmt.Sprintf("…+%d more", len(lines)-(avail-1))))
	} else {
		for _, ln := range lines {
			content = append(content, truncate(ln, contentW))
		}
	}
	return paneStyle.Width(frameW).Height(innerH).Render(strings.Join(content, "\n"))
}

func (m Model) dotfilesLines() []string {
	d := m.report.Dotfiles
	out := []string{
		"behind origin:    " + count(d.BehindOrigin),
		fmt.Sprintf("to apply:         %d", len(d.ToApply)),
	}
	for _, f := range d.ToApply {
		out = append(out, delStyle.Render("  ↓ "+f))
	}
	out = append(out, fmt.Sprintf("modified locally: %d", len(d.LocalModified)))
	for _, f := range d.LocalModified {
		out = append(out, addStyle.Render("  ↑ "+f))
	}
	out = append(out,
		"uncommitted:      "+count(d.UncommittedFiles),
		"unpushed commits: "+count(d.UnpushedCommits),
	)
	return out
}

func (m Model) toolsLines() []string {
	t := m.report.Tools
	out := []string{fmt.Sprintf("missing:  %d", len(t.Missing))}
	for _, n := range t.Missing {
		out = append(out, addStyle.Render("  + "+n))
	}
	out = append(out, fmt.Sprintf("outdated: %d", len(t.Outdated)))
	for _, o := range t.Outdated {
		out = append(out, delStyle.Render(fmt.Sprintf("  %s %s → %s", o.Name, o.Current, o.Wanted)))
	}
	return out
}

// updatesLines is the dashboard's "Updates available" pane: per-source outdated
// counts from the (informational) inventory, with a hint to open the detail
// view. It is independent of the verdict — separate from drift (ADR-0010).
func (m Model) updatesLines() []string {
	if m.invLoading || m.inventory == nil {
		return []string{subtleStyle.Render("checking…")}
	}
	var out []string
	for _, s := range m.inventory.Sources {
		switch {
		case !s.Available:
			out = append(out, subtleStyle.Render(fmt.Sprintf("%s: n/a", s.Name)))
		case s.Error != "":
			out = append(out, errStyle.Render(fmt.Sprintf("%s: error", s.Name)))
		default:
			line := fmt.Sprintf("%s: %d", s.Name, len(s.Packages))
			if len(s.Packages) > 0 {
				line = delStyle.Render(line)
			}
			out = append(out, line)
		}
	}
	out = append(out, "", helpStyle.Render("press o for details"))
	return out
}

// outdatedContent is the scrollable body of the modeOutdated view: every
// outdated package grouped by source. The viewport clips vertically but not
// horizontally, so each line is truncated to the viewport width to keep long
// names/versions inside the frame.
func (m Model) outdatedContent() string {
	if m.invLoading || m.inventory == nil {
		return subtleStyle.Render("checking…")
	}
	cw := m.vp.Width
	if cw <= 0 {
		cw = m.width
	}
	var b strings.Builder
	line := func(s string) { b.WriteString(truncate(s, cw) + "\n") }
	for _, s := range m.inventory.Sources {
		line(paneTitle.Render(s.Name))
		switch {
		case !s.Available:
			line(subtleStyle.Render("  not available"))
		case s.Error != "":
			line(errStyle.Render("  ! " + s.Error))
		case len(s.Packages) == 0:
			line(subtleStyle.Render("  up to date"))
		default:
			for _, p := range s.Packages {
				line(delStyle.Render(fmt.Sprintf("  %s  %s → %s", p.Name, p.Current, p.Latest)))
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// activityLines puts notices (update available, errors) first, then the log tail.
func (m Model) activityLines(max int) []string {
	var out []string
	r := m.report
	if r != nil && r.Myplace.Latest != nil && *r.Myplace.Latest != r.Myplace.Current {
		out = append(out, noticeStyle.Render(fmt.Sprintf("myplace %s → %s available (myplace self-update)", r.Myplace.Current, *r.Myplace.Latest)))
	}
	for _, e := range m.updateErrs {
		out = append(out, errStyle.Render("! "+e))
	}
	if r != nil {
		for _, e := range r.Errors {
			out = append(out, errStyle.Render("! "+e))
		}
	}
	// Fill the rest with the most recent log lines.
	if room := max - len(out); room > 0 && len(m.activity) > 0 {
		tail := m.activity
		if len(tail) > room {
			tail = tail[len(tail)-room:]
		}
		for _, ln := range tail {
			out = append(out, subtleStyle.Render(ln))
		}
	}
	return out
}

func (m Model) header(width int) string {
	r := m.report
	left := headerStyle.Render("myplace "+m.version) + "  " + badge(r.Verdict) +
		subtleStyle.Render(fmt.Sprintf("  %s (%s)", r.Machine, r.Profile))
	right := subtleStyle.Render("checked " + r.CheckedAt.Local().Format("15:04:05"))
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
		right = ""
	}
	bar := left + strings.Repeat(" ", gap) + right
	return truncate(bar, width) + "\n" + ruleStyle.Render(strings.Repeat("─", width))
}

func (m Model) footer(width int) string {
	left := helpStyle.Render("r refresh • u update • o outdated • q quit")
	var right string
	if r := m.report; r != nil && r.Myplace.Latest != nil && *r.Myplace.Latest != r.Myplace.Current {
		right = noticeStyle.Render(fmt.Sprintf("update available: %s ↑", *r.Myplace.Latest))
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return truncate(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) View() string {
	w, h := m.width, m.height
	if w == 0 || h == 0 {
		return "  starting…"
	}

	if m.mode == modeOutdated {
		return m.outdatedView()
	}

	if m.loading || m.updating || m.report == nil {
		msg := "checking status…"
		if m.updating {
			msg = "updating (dotfiles, then tools)…"
		}
		center := fmt.Sprintf("%s  %s\n\n%s", m.spinner.View(), msg,
			subtleStyle.Render("myplace "+m.version))
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, center)
	}

	if w < minWidth || h < minHeight {
		return m.smallView()
	}

	// Layout math: header 2 rows, footer 1 row, body splits ~55/45.
	bodyH := h - 3
	topH := bodyH * 55 / 100
	if topH < 5 {
		topH = 5
	}
	botH := bodyH - topH
	colW := w / 3

	top := lipgloss.JoinHorizontal(lipgloss.Top,
		box("Dotfiles", m.dotfilesLines(), colW, topH),
		box("Tools (mise)", m.toolsLines(), colW, topH),
		box("Updates available", m.updatesLines(), w-2*colW, topH),
	)
	activity := box("Activity", m.activityLines(botH-2), w, botH)

	return strings.Join([]string{
		m.header(w),
		top,
		activity,
		m.footer(w),
	}, "\n")
}

// smallView is the readable fallback for terminals too small to frame.
func (m Model) smallView() string {
	r := m.report
	var b strings.Builder
	head := fmt.Sprintf("myplace %s  %s  %s (%s)", m.version, badge(r.Verdict), r.Machine, r.Profile)
	b.WriteString(truncate(head, m.width) + "\n\n")
	b.WriteString(paneTitle.Render("Dotfiles") + "\n")
	for _, ln := range m.dotfilesLines() {
		b.WriteString(truncate(ln, m.width) + "\n")
	}
	b.WriteString("\n" + paneTitle.Render("Tools") + "\n")
	for _, ln := range m.toolsLines() {
		b.WriteString(truncate(ln, m.width) + "\n")
	}
	b.WriteString("\n" + m.footer(m.width))
	return b.String()
}

// outdatedView is the full-screen scrollable detail of all outdated packages
// (entered with `o`). Its header is self-contained so it renders even before
// the drift report has loaded.
func (m Model) outdatedView() string {
	title := headerStyle.Render("myplace "+m.version) + subtleStyle.Render("  outdated packages")
	header := truncate(title, m.width) + "\n" + ruleStyle.Render(strings.Repeat("─", m.width))
	footer := helpStyle.Render("↑/↓ scroll • esc/q back • ctrl+c quit")
	return strings.Join([]string{header, m.vp.View(), footer}, "\n")
}

// Run starts the dashboard.
func Run(ch *chezmoi.Client, ms *mise.Client, sources []outdated.Source, version string) error {
	_, err := tea.NewProgram(New(ch, ms, sources, version), tea.WithAltScreen()).Run()
	return err
}
