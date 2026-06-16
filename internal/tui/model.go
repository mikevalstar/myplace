// Package tui is the Bubble Tea layer — the ONLY package that imports Charm UI
// libraries. It renders what internal/drift and internal/outdated compute; it
// owns no logic and (per ADR-0012) mutates nothing beyond the existing `u`
// converge. Layout and interaction are specified in
// docs/features/tui-dashboard.md; the theme in docs/adrs/0011-catppuccin-theming.md.
//
// The package is split by concern across files, all `package tui`:
//
//	theme.go  — the semantic Catppuccin theme
//	keys.go   — the keymap (bubbles/key)
//	model.go  — the Model, messages, Init, Run (this file)
//	update.go — Update and every tea.Cmd
//	views.go  — View and all render helpers
package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mikevalstar/myplace/internal/chezmoi"
	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/mise"
	"github.com/mikevalstar/myplace/internal/outdated"
	"github.com/mikevalstar/myplace/internal/sysinfo"
)

// Minimum terminal size below which we render a plain stacked fallback rather
// than drawing broken frames. wideThreshold is where the master-detail split
// (panes left, detail right) turns on; below it the detail opens full-screen.
const (
	minWidth      = 60
	minHeight     = 16
	wideThreshold = 120
)

// mode is which screen the dashboard is showing.
type mode int

const (
	modeDashboard mode = iota // the paneled home screen
	modeOutdated              // the scrollable outdated-packages detail view
	modeDetail                // full-screen detail (narrow terminals; `enter`)
)

// focus is which pane the keyboard is driving on the dashboard.
type focus int

const (
	focusDotfiles focus = iota
	focusTools
	focusUpdates
	focusCount
)

// sortMode orders the outdated detail view.
type sortMode int

const (
	sortBySource sortMode = iota // grouped by manager (default)
	sortByName                   // one flat list, alphabetical
)

// updateStep tracks the converge-only update as it streams progress.
type updateStep int

const (
	stepNone updateStep = iota
	stepChezmoi
	stepMiseInstall
	stepMiseUpgrade
	stepDone
)

type reportMsg struct{ report drift.Report }
type inventoryMsg struct{ inv outdated.Inventory }
type sysinfoMsg struct {
	info *sysinfo.Info
	err  error
}
type activityTickMsg struct{}
type diffMsg struct {
	path string
	diff string
	err  error
}

// stepDoneMsg reports one update step finishing; Update advances to the next.
type stepDoneMsg struct {
	step    updateStep
	err     error
	skipped bool
}

// selectable is one navigable item in a pane and the detail it shows.
// A dotfile's detail is its (lazily fetched, cached) chezmoi diff; everything
// else carries pre-rendered body lines from already-loaded data.
type selectable struct {
	isDiff bool     // dotfile → show chezmoi diff
	path   string   // dotfile relative path (isDiff)
	title  string   // detail-panel title
	body   []string // static detail lines (!isDiff)
}

// paneRow is one display line in a pane. selIdx ≥ 0 marks it as the Nth
// selectable item (so the focused selection can be highlighted and mapped to a
// detail); -1 is a non-selectable header/count line.
type paneRow struct {
	text   string
	style  lipgloss.Style
	selIdx int
}

type Model struct {
	ch      *chezmoi.Client
	ms      *mise.Client
	si      *sysinfo.Client
	sources []outdated.Source
	version string

	theme Theme
	keys  keyMap
	help  help.Model

	spinner    spinner.Model
	report     *drift.Report
	inventory  *outdated.Inventory
	system     *sysinfo.Info
	systemErr  bool
	loading    bool
	invLoading bool
	width      int
	height     int

	mode mode

	// interactive navigation
	focus    focus
	sel      [focusCount]int // selection index per pane
	detailVP viewport.Model  // diff / detail panel (master-detail + narrow modeDetail)

	// per-file diff cache, so re-selecting doesn't re-shell chezmoi diff
	diffCache   map[string]string
	diffPending map[string]bool

	// help overlay
	showHelp bool

	// converge-only update progress
	updating   bool
	updateStep updateStep
	updateErrs []string
	progress   progress.Model

	// outdated detail view
	vp        viewport.Model // scrollable body for modeOutdated
	sort      sortMode
	filter    textinput.Model
	filtering bool

	activity []string
}

func New(ch *chezmoi.Client, ms *mise.Client, si *sysinfo.Client, sources []outdated.Source, version string) Model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	fi := textinput.New()
	fi.Prompt = "filter: "
	fi.CharLimit = 64
	return Model{
		ch: ch, ms: ms, si: si, sources: sources, version: version,
		theme:       DefaultTheme(),
		keys:        newKeyMap(),
		help:        help.New(),
		spinner:     sp,
		loading:     true,
		invLoading:  true,
		progress:    progress.New(progress.WithoutPercentage()),
		filter:      fi,
		diffCache:   map[string]string{},
		diffPending: map[string]bool{},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadCmd(), m.loadInventoryCmd(), m.loadSysinfoCmd(), activityTick())
}

// Run starts the dashboard.
func Run(ch *chezmoi.Client, ms *mise.Client, si *sysinfo.Client, sources []outdated.Source, version string) error {
	_, err := tea.NewProgram(New(ch, ms, si, sources, version), tea.WithAltScreen()).Run()
	return err
}
