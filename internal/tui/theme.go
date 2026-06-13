package tui

import (
	catppuccin "github.com/catppuccin/go"
	"github.com/charmbracelet/lipgloss"
)

// Theme is the dashboard's palette, keyed by semantic ROLE rather than by
// Catppuccin color name. Render code reads roles (m.theme.PaneTitle,
// m.theme.Selected, …) and never references a raw color, so re-theming is a
// palette swap in NewTheme — not a find-and-replace across views. See
// docs/adrs/0011-catppuccin-theming.md.
type Theme struct {
	Flavor catppuccin.Flavor

	PaneStyle   lipgloss.Style // bordered pane (unfocused)
	PaneFocused lipgloss.Style // bordered pane with the focus ring
	PaneTitle   lipgloss.Style // pane heading
	Header      lipgloss.Style // top bar title
	Rule        lipgloss.Style // horizontal rule under the header
	Help        lipgloss.Style // footer / key hints
	Subtle      lipgloss.Style // log tail, secondary text
	Err         lipgloss.Style // error lines
	Notice      lipgloss.Style // update-available notices
	Add         lipgloss.Style // local edits / missing tools / diff "+"
	Del         lipgloss.Style // to-apply / outdated / diff "-"
	Hunk        lipgloss.Style // diff "@@" hunk headers
	Selected    lipgloss.Style // highlighted (selected) row
	Progress    lipgloss.Style // progress / in-flight step marker

	// Card chrome: each dashboard card has its own accent (Dotfiles / Tools /
	// Updates), shown in the title's leading bar and its count chip.
	Accent        [3]lipgloss.Style // accent-colored card title
	ChipAttn      [3]lipgloss.Style // count chip when the card has items
	ChipClear     lipgloss.Style    // "all clear" chip (✓)
	AccentNeutral lipgloss.Style    // Activity card title
	AccentDetail  lipgloss.Style    // Detail panel title
	Modal         lipgloss.Style    // floating window (e.g. the update modal)

	OK   lipgloss.Style // verdict badge: in sync
	Warn lipgloss.Style // verdict badge: drifted / unknown
	Bad  lipgloss.Style // verdict badge: error
}

var paneBorder = lipgloss.RoundedBorder()

// NewTheme maps a Catppuccin flavor onto the semantic roles. This is the one
// place the palette is consulted.
func NewTheme(f catppuccin.Flavor) Theme {
	c := func(col catppuccin.Color) lipgloss.Color { return lipgloss.Color(col.Hex) }

	pane := lipgloss.NewStyle().Border(paneBorder).Padding(0, 1)
	badge := func(bg catppuccin.Color) lipgloss.Style {
		return lipgloss.NewStyle().Background(c(bg)).Foreground(c(f.Base())).Bold(true).Padding(0, 1)
	}

	accents := [3]catppuccin.Color{f.Blue(), f.Mauve(), f.Peach()}
	var accent [3]lipgloss.Style
	var chipAttn [3]lipgloss.Style
	for i, ac := range accents {
		accent[i] = lipgloss.NewStyle().Bold(true).Foreground(c(ac))
		chipAttn[i] = lipgloss.NewStyle().Bold(true).Background(c(ac)).Foreground(c(f.Base())).Padding(0, 1)
	}

	return Theme{
		Flavor:        f,
		Accent:        accent,
		ChipAttn:      chipAttn,
		ChipClear:     lipgloss.NewStyle().Bold(true).Background(c(f.Green())).Foreground(c(f.Base())).Padding(0, 1),
		AccentNeutral: lipgloss.NewStyle().Bold(true).Foreground(c(f.Teal())),
		AccentDetail:  lipgloss.NewStyle().Bold(true).Foreground(c(f.Lavender())),
		Modal: lipgloss.NewStyle().Border(paneBorder).BorderForeground(c(f.Lavender())).
			Background(c(f.Surface0())).Foreground(c(f.Text())).Padding(1, 3),
		PaneStyle:   pane.BorderForeground(c(f.Surface1())),
		PaneFocused: pane.BorderForeground(c(f.Lavender())),
		PaneTitle:   lipgloss.NewStyle().Bold(true).Foreground(c(f.Mauve())),
		Header:      lipgloss.NewStyle().Bold(true).Foreground(c(f.Text())),
		Rule:        lipgloss.NewStyle().Foreground(c(f.Surface1())),
		Help:        lipgloss.NewStyle().Foreground(c(f.Subtext0())),
		Subtle:      lipgloss.NewStyle().Foreground(c(f.Overlay1())),
		Err:         lipgloss.NewStyle().Foreground(c(f.Red())),
		Notice:      lipgloss.NewStyle().Foreground(c(f.Yellow())),
		Add:         lipgloss.NewStyle().Foreground(c(f.Green())),
		Del:         lipgloss.NewStyle().Foreground(c(f.Peach())),
		Hunk:        lipgloss.NewStyle().Foreground(c(f.Sky())),
		Selected:    lipgloss.NewStyle().Background(c(f.Surface0())).Foreground(c(f.Text())),
		Progress:    lipgloss.NewStyle().Foreground(c(f.Sapphire())),
		OK:          badge(f.Green()),
		Warn:        badge(f.Yellow()),
		Bad:         badge(f.Red()),
	}
}

// DefaultTheme picks Mocha (dark) or Latte (light) from the terminal
// background, defaulting to Mocha when detection is ambiguous. Call it once at
// model construction — HasDarkBackground() probes the terminal and shouldn't
// run per frame (see docs/guides/charm-tui-stack.md).
func DefaultTheme() Theme {
	if lipgloss.HasDarkBackground() {
		return NewTheme(catppuccin.Mocha)
	}
	return NewTheme(catppuccin.Latte)
}
