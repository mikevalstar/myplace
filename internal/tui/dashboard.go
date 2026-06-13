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
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/logging"
	"github.com/mikevalstar/myplace/internal/mise"
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
type updateDoneMsg struct{ errs []string }
type activityTickMsg struct{}

type Model struct {
	ch      *chezmoi.Client
	ms      *mise.Client
	version string

	spinner    spinner.Model
	report     *drift.Report
	loading    bool
	updating   bool
	updateErrs []string
	activity   []string
	width      int
	height     int
}

func New(ch *chezmoi.Client, ms *mise.Client, version string) Model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	return Model{ch: ch, ms: ms, version: version, spinner: sp, loading: true}
}

func (m Model) loadCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		return reportMsg{drift.Compute(ctx, m.ch, m.ms, m.version)}
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
	return tea.Batch(m.spinner.Tick, m.loadCmd(), activityTick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case activityTickMsg:
		m.activity = logging.RecentLines(200)
		return m, activityTick()
	case reportMsg:
		m.report = &msg.report
		m.loading = false
		return m, nil
	case updateDoneMsg:
		m.updating = false
		m.updateErrs = msg.errs
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.loadCmd())
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			if !m.loading && !m.updating {
				m.loading = true
				m.updateErrs = nil
				return m, tea.Batch(m.spinner.Tick, m.loadCmd())
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
	left := helpStyle.Render("r refresh • u update • q quit")
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
	leftW := w / 2
	rightW := w - leftW

	top := lipgloss.JoinHorizontal(lipgloss.Top,
		box("Dotfiles", m.dotfilesLines(), leftW, topH),
		box("Tools (mise)", m.toolsLines(), rightW, topH),
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

// Run starts the dashboard.
func Run(ch *chezmoi.Client, ms *mise.Client, version string) error {
	_, err := tea.NewProgram(New(ch, ms, version), tea.WithAltScreen()).Run()
	return err
}
