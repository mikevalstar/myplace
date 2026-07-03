package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap is the dashboard's full binding set. Defining bindings once here (and
// implementing help.KeyMap) keeps the footer hints and the `?` overlay in sync
// with reality — bubbles/help renders both from this.
type keyMap struct {
	Focus    key.Binding // tab / shift+tab between panes
	Left     key.Binding // h / ← focus prev pane
	Right    key.Binding // l / → focus next pane
	Up       key.Binding // k / ↑ select up / scroll
	Down     key.Binding // j / ↓ select down / scroll
	Enter    key.Binding // open detail (narrow)
	Refresh  key.Binding // r (also re-runs checks in the doctor view)
	Update   key.Binding // u
	Outdated key.Binding // o
	Doctor   key.Binding // d
	Activity key.Binding // a
	Sort     key.Binding // s (outdated view)
	Filter   key.Binding // / (outdated + activity views)
	Follow   key.Binding // f (activity view: pin to newest lines)
	Help     key.Binding // ? toggle overlay
	Esc      key.Binding // back / close / clear
	Quit     key.Binding // q / ctrl+c
}

func newKeyMap() keyMap {
	return keyMap{
		Focus:    key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab", "focus pane")),
		Left:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "prev pane")),
		Right:    key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next pane")),
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "detail")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Update:   key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "update")),
		Outdated: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "outdated")),
		Doctor:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "doctor")),
		Activity: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "activity")),
		Sort:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Follow:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "follow")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Esc:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// ShortHelp is the footer line.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Focus, k.Up, k.Down, k.Refresh, k.Update, k.Outdated, k.Doctor, k.Activity, k.Help, k.Quit}
}

// FullHelp is the `?` overlay grid.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Focus, k.Left, k.Right, k.Up, k.Down},
		{k.Enter, k.Refresh, k.Update, k.Outdated, k.Doctor, k.Activity},
		{k.Sort, k.Filter, k.Follow, k.Help, k.Esc, k.Quit},
	}
}
