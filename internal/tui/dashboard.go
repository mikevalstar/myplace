// Package tui is the Bubble Tea layer — the ONLY package that imports Charm
// UI libraries. It renders what internal/drift computes; it owns no logic.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/mise"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	paneStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	paneTitle   = lipgloss.NewStyle().Bold(true).Underline(true)
	helpStyle   = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	okBadge     = lipgloss.NewStyle().Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0")).Padding(0, 1)
	warnBadge   = lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0")).Padding(0, 1)
	errBadge    = lipgloss.NewStyle().Background(lipgloss.Color("1")).Foreground(lipgloss.Color("15")).Padding(0, 1)
	subtleStyle = lipgloss.NewStyle().Faint(true)
)

type reportMsg struct{ report drift.Report }
type updateDoneMsg struct{ errs []string }

type Model struct {
	ch      *chezmoi.Client
	ms      *mise.Client
	version string

	spinner    spinner.Model
	report     *drift.Report
	loading    bool
	updating   bool
	updateErrs []string
	width      int
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
// behavior as headless `myplace update --yes`. Outgoing capture (re-add,
// commit, push) stays a deliberate CLI action for now.
func (m Model) updateCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		var errs []string
		if err := m.ch.Update(ctx); err != nil {
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

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
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

func (m Model) dotfilesPane() string {
	d := m.report.Dotfiles
	var b strings.Builder
	b.WriteString(paneTitle.Render("Dotfiles") + "\n")
	fmt.Fprintf(&b, "behind origin:    %s\n", count(d.BehindOrigin))
	fmt.Fprintf(&b, "to apply:         %d\n", len(d.ToApply))
	for _, f := range d.ToApply {
		b.WriteString(subtleStyle.Render("  ↓ "+f) + "\n")
	}
	fmt.Fprintf(&b, "modified locally: %d\n", len(d.LocalModified))
	for _, f := range d.LocalModified {
		b.WriteString(subtleStyle.Render("  ↑ "+f) + "\n")
	}
	fmt.Fprintf(&b, "uncommitted:      %s\n", count(d.UncommittedFiles))
	fmt.Fprintf(&b, "unpushed commits: %s", count(d.UnpushedCommits))
	return paneStyle.Render(b.String())
}

func (m Model) toolsPane() string {
	t := m.report.Tools
	var b strings.Builder
	b.WriteString(paneTitle.Render("Tools (mise)") + "\n")
	fmt.Fprintf(&b, "missing:  %d\n", len(t.Missing))
	for _, n := range t.Missing {
		b.WriteString(subtleStyle.Render("  + "+n) + "\n")
	}
	fmt.Fprintf(&b, "outdated: %d", len(t.Outdated))
	for _, o := range t.Outdated {
		b.WriteString("\n" + subtleStyle.Render(fmt.Sprintf("  %s %s → %s", o.Name, o.Current, o.Wanted)))
	}
	return paneStyle.Render(b.String())
}

func (m Model) View() string {
	header := titleStyle.Render("myplace " + m.version)
	if m.loading {
		return fmt.Sprintf("\n%s\n\n  %s checking status…\n", header, m.spinner.View())
	}
	if m.updating {
		return fmt.Sprintf("\n%s\n\n  %s updating (dotfiles, then tools)…\n", header, m.spinner.View())
	}
	if m.report == nil {
		return "\n" + header + "\n\n  no report\n"
	}

	r := m.report
	head := lipgloss.JoinHorizontal(lipgloss.Center, header, " ", badge(r.Verdict),
		subtleStyle.Render(fmt.Sprintf("  %s (%s)", r.Machine, r.Profile)))

	panes := lipgloss.JoinHorizontal(lipgloss.Top, m.dotfilesPane(), " ", m.toolsPane())
	if m.width > 0 && lipgloss.Width(panes) > m.width {
		panes = lipgloss.JoinVertical(lipgloss.Left, m.dotfilesPane(), m.toolsPane())
	}

	var errs string
	for _, e := range append(append([]string{}, r.Errors...), m.updateErrs...) {
		errs += errStyle.Render("  ! "+e) + "\n"
	}

	return fmt.Sprintf("\n%s\n\n%s\n%s%s\n", head, panes, errs,
		helpStyle.Render("r refresh • u update • q quit"))
}

// Run starts the dashboard.
func Run(ch *chezmoi.Client, ms *mise.Client, version string) error {
	_, err := tea.NewProgram(New(ch, ms, version), tea.WithAltScreen()).Run()
	return err
}
