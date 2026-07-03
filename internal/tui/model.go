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
	"github.com/mikevalstar/myplace/internal/doctor"
	"github.com/mikevalstar/myplace/internal/drift"
	"github.com/mikevalstar/myplace/internal/mise"
	"github.com/mikevalstar/myplace/internal/outdated"
	"github.com/mikevalstar/myplace/internal/release"
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
	modeDoctor                // the scrollable doctor-preflight view
	modeActivity              // the scrollable full-log activity view
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

// How deep the modeActivity view reads into the log: far deeper than the
// pane's 200-line tail — with ~5 MB rotation, 1 MB is the whole recent file in
// practice — but still a bounded window so the 1-second tick stays cheap.
const (
	activityTailLines  = 5000
	activityTailWindow = 1 << 20
)

type reportMsg struct{ report drift.Report }
type inventoryMsg struct{ inv outdated.Inventory }
type doctorMsg struct{ report doctor.Report }
type releaseMsg struct {
	rel release.Release
	err error
}
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

// selKind is what a selectable's detail shows: pre-rendered lines from
// already-loaded data, a lazily fetched chezmoi diff, or the latest GitHub
// release's notes (the binary-update row). Diff and release detail are both
// fetched once on first selection and cached; everything stays read-only.
type selKind int

const (
	kindStatic selKind = iota
	kindDiff
	kindRelease
)

// selectable is one navigable item in a pane and the detail it shows.
type selectable struct {
	kind  selKind
	path  string   // dotfile relative path (kindDiff)
	title string   // detail-panel title
	body  []string // static detail lines (kindStatic)
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
	docOpts doctor.Options

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

	// latest-release detail for the binary-update row: fetched once on first
	// selection, then cached (same pattern as diffs)
	rel        *release.Release
	relErr     string
	relPending bool

	// doctor preflight: run on demand (first `d`), cached for the session
	doctorRep     *doctor.Report
	doctorLoading bool

	// activity view: the deep log tail and whether the viewport is pinned to
	// the newest lines on each tick
	actLines []string
	follow   bool

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

func New(ch *chezmoi.Client, ms *mise.Client, si *sysinfo.Client, sources []outdated.Source, version string, docOpts doctor.Options) Model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	fi := textinput.New()
	fi.Prompt = "filter: "
	fi.CharLimit = 64
	return Model{
		ch: ch, ms: ms, si: si, sources: sources, version: version, docOpts: docOpts,
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

// Run starts the dashboard. docOpts carries the environment facts the doctor
// view needs (the user's real PATH); the cmd layer resolves them, as it does
// for the doctor CLI command.
func Run(ch *chezmoi.Client, ms *mise.Client, si *sysinfo.Client, sources []outdated.Source, version string, docOpts doctor.Options) error {
	_, err := tea.NewProgram(New(ch, ms, si, sources, version, docOpts), tea.WithAltScreen()).Run()
	return err
}
