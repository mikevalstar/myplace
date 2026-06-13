---
title: Working with the Charm TUI stack
status: active
created: 2026-06-12
updated: 2026-06-12
tags: [go, charm, bubbletea, lipgloss, tui]
audience: both
---

# Working with the Charm TUI stack

## Purpose

How to build and extend the `myplace` TUI using the Charm libraries — the core architecture, where each library fits, and the sharp edges that cost afternoons. Chosen in [ADR-0002](../adrs/0002-go-and-charm-for-the-tui.md).

## Background

The Charm ecosystem (charm.sh) is a family of Go libraries for terminal apps:

| Library | Role in myplace |
|---------|-----------------|
| [bubbletea](https://github.com/charmbracelet/bubbletea) | The framework. Elm architecture: Model → Update → View loop |
| [bubbles](https://github.com/charmbracelet/bubbles) | Stock components. In use: `spinner`, `viewport` (diff + outdated), `progress` (update steps), `textinput` (outdated filter), `key` + `help` (keymap + `?` overlay). Deliberately **not** used: `list`/`table` — their built-in chrome (borders, padding, filtering UI) fights the dashboard's exact-tiling discipline; selection is a single int index per pane instead |
| [lipgloss](https://github.com/charmbracelet/lipgloss) | Styling and layout (colors, borders, joining boxes). All colors go through the semantic theme (see below), never hardcoded. Its `lipgloss/table` subpackage renders the outdated view |
| [catppuccin/go](https://github.com/catppuccin/go) | The palette behind the theme — Mocha (dark) / Latte (light). A direct dep as of [ADR-0011](../adrs/0011-catppuccin-theming.md) |
| [huh](https://github.com/charmbracelet/huh) | Forms and prompts — the bootstrap wizard, confirmations |
| [log](https://github.com/charmbracelet/log) | Logging (to a file when the TUI owns the terminal) |
| [fang](https://github.com/charmbracelet/fang) | Polished help/errors/version handling for cobra CLIs |

Plus **cobra** (not Charm, but the standard pairing) for the CLI layer: `myplace status --json` etc.

## The guide

### The Elm architecture in 30 seconds

A Bubble Tea program is one type implementing three methods:

```go
type model struct {
    spinner spinner.Model
    status  *drift.Report // nil until loaded
    err     error
}

func (m model) Init() tea.Cmd { return tea.Batch(m.spinner.Tick, loadStatus) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case statusMsg:
        m.status = msg.report
    case errMsg:
        m.err = msg.err
    case tea.KeyMsg:
        if msg.String() == "q" {
            return m, tea.Quit
        }
    }
    var cmd tea.Cmd
    m.spinner, cmd = m.spinner.Update(msg)
    return m, cmd
}

func (m model) View() string { /* render from state, nothing else */ }
```

State only changes in `Update`, and only in response to a `tea.Msg`. Rendering reads state and does nothing else.

### Side effects are commands — never block in Update

All I/O (and for us that means *every* chezmoi/mise invocation) goes in a `tea.Cmd`: a `func() tea.Msg` that Bubble Tea runs in a goroutine and feeds back into `Update`:

```go
func loadStatus() tea.Msg {
    report, err := drift.Compute() // calls chezmoi/mise via os/exec
    if err != nil {
        return errMsg{err}
    }
    return statusMsg{report}
}
```

If you call `exec.Command(...).Run()` directly inside `Update`, the whole UI freezes. This is the #1 mistake to watch for in review.

### Handing the terminal to a subprocess

Some operations need the real terminal — `chezmoi edit` (opens `$EDITOR`), anything that prompts. Use `tea.ExecProcess`, which suspends the TUI, runs the command, and resumes with a message:

```go
return m, tea.ExecProcess(exec.Command("chezmoi", "edit", path), func(err error) tea.Msg {
    return editFinishedMsg{err}
})
```

For non-interactive calls, prefer `--no-tty`/`--force`-style flags and JSON output (`chezmoi status`, `mise ls --json`) captured via a normal `tea.Cmd`.

### Package layout: the TUI is a skin

```
cmd/myplace/      cobra commands; each supports --json (headless) and falls through to the TUI
internal/chezmoi/ wraps the chezmoi CLI, parses output  ── no TUI imports
internal/mise/    wraps the mise CLI                    ── no TUI imports
internal/drift/   computes sync status from both        ── no TUI imports
internal/tui/     bubbletea models, lipgloss styles — the ONLY place Charm UI libs appear
```

`internal/{chezmoi,mise,drift}` must never import `internal/tui` or bubbletea. This is what makes `--json` (and phase 2's reporting agent) free.

`internal/tui` is one package split across files by concern: `theme.go` (the semantic theme), `keys.go` (the keymap), `model.go` (the `Model`, messages, `Init`, `Run`), `update.go` (`Update` + every `tea.Cmd`), `views.go` (`View` + all render helpers). One package keeps the lipgloss-only rule intact while staying readable.

### Layout with Lip Gloss

Build strings with styles, then compose with `lipgloss.JoinHorizontal` / `JoinVertical`. Define styles once at package level, not per-frame.

```go
var paneStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
```

### Theming: semantic roles, not color names

All colors live in one `Theme` struct in `internal/tui` (it returns `lipgloss.Style`, so the layering rule pins it there — [ADR-0011](../adrs/0011-catppuccin-theming.md)). Render code asks for a **role** (`m.theme.PaneTitle`, `m.theme.Selected`, `m.theme.Del`), never a Catppuccin color. The Catppuccin→role mapping happens once in `NewTheme(flavor)`:

```go
func NewTheme(f catppuccingo.Flavor) Theme {
    color := func(c catppuccingo.Color) lipgloss.Color { return lipgloss.Color(c.Hex) }
    return Theme{
        PaneTitle: lipgloss.NewStyle().Bold(true).Foreground(color(f.Mauve())),
        Selected:  lipgloss.NewStyle().Background(color(f.Surface0())).Foreground(color(f.Text())),
        // …one field per role
    }
}
```

`DefaultTheme()` picks Mocha or Latte from `lipgloss.HasDarkBackground()`, defaulting to Mocha when detection is ambiguous. Build the theme once in `New()` and store it on the model — **not per frame** (see the gotcha below).

### Keymap with bubbles/key + help

Define every binding as a `key.Binding` in a `keyMap` struct, then implement `ShortHelp()`/`FullHelp()`. `bubbles/help` renders both for free — `help.View(keys)` for the footer line, the full grid for the `?` overlay — so the key list lives in exactly one place instead of drifting between the footer string and reality.

### Streaming multi-step progress without channels

To show per-step progress (chezmoi apply → mise install → mise upgrade) in the Elm architecture, **don't** run all steps in one `tea.Cmd`. Make each step its own command that returns a `stepDoneMsg`, and have `Update` advance the step state and fire the *next* command on each message:

```go
case stepDoneMsg:
    m.recordStep(msg)            // append any error/skip notice
    switch msg.step {
    case stepChezmoi:    m.updateStep = stepMiseInstall; return m, m.miseInstallCmd()
    case stepMiseInstall: m.updateStep = stepMiseUpgrade; return m, m.miseUpgradeCmd()
    case stepMiseUpgrade: m.updating = false; m.loading = true; return m, tea.Batch(m.spinner.Tick, m.loadCmd())
    }
```

The view renders a `progress.Model` (`step/total`) plus a `✓/▸/·` checklist from `m.updateStep`. Keeps each step observable while staying converge-only — no channels, no goroutine plumbing.

### Master-detail layout math

The detail panel is a `viewport.Model` sized from the window: on a wide terminal, `leftW := w*60/100` for the panes and `w-leftW` for the panel, joined with `lipgloss.JoinHorizontal`; on narrow, the panel replaces the body full-width on `enter`. Same exact-tiling rules as everything else — the panel is a `box` whose content (a colored diff, or version detail) is `ansi.Truncate`d to its content width. Fetch the diff in a `tea.Cmd` and **cache it per file** so re-selecting doesn't re-shell `chezmoi diff` on every keystroke.

### Modal overlays (there's no built-in compositor in v1)

Lip Gloss v1 can't layer one render over another — `lipgloss.Place` centers on a *blank* screen. To float a modal (the update window) over the dashboard, composite by hand: render the dashboard, dim it, then splice the modal box into the center rows. Dimming is "strip the colors, recolor in one muted tone, per line" (`ansi.Strip` then a subtle style) so the backdrop reads as disabled. The splice is ANSI-aware column surgery with `charmbracelet/x/ansi`:

```go
left  := ansi.Truncate(bgLine, x, "")        // bg columns [0, x)
right := ansi.TruncateLeft(bgLine, x+boxW, "") // bg columns [x+boxW, …)
bgLines[row] = left + modalLine + right         // modal is opaque over the gap
```

Measure with `ansi.StringWidth`, never `len`. Keep the modal opaque (give it a background) so it cleanly covers the dimmed text underneath. Guard tiny terminals — fall back to `lipgloss.Place` on a blank screen when there isn't room to composite.

### Forms with huh

`huh` forms can run standalone (`form.Run()`, fine inside plain cobra commands) or be embedded in a Bubble Tea model as a `tea.Model` — use embedded mode inside the TUI so the wizard doesn't fight the running program for the terminal.

## Gotchas

- **Never block in `Update` or `View`.** All subprocess calls and file I/O go through `tea.Cmd`. (Repeated because it's the one that always happens.)
- **A subprocess can still hang you even from a `tea.Cmd`.** A `tea.Cmd` runs off the render loop, so it won't freeze rendering — but if the child opens `/dev/tty` directly for a prompt (chezmoi's "file changed since I last wrote it?", git credential prompts, ssh passphrase), it grabs the same terminal the TUI owns and waits forever for a keypress the TUI is consuming. The spinner keeps spinning; the command never returns. Defend at the exec layer: set `cmd.Stdin = nil` (closed → EOF, not the inherited TTY) **and** pass the tool's no-prompt flag (`chezmoi --no-tty`, `GIT_TERMINAL_PROMPT=0`). Then a prompt fails fast instead of hanging, and you surface the error. For genuinely interactive subprocesses (an editor, a guided merge), do the opposite — hand over the real terminal with `tea.ExecProcess`, never a background `tea.Cmd`. (This one cost us a stuck dashboard: `chezmoi update` over a locally-modified file.)
- **v1 vs v2:** Bubble Tea has a v2 (`charmbracelet/bubbletea/v2`) with breaking API changes, but most examples, blog posts, and AI training data are v1. Check `go.mod` before pasting any example, and keep bubbles/huh on matching majors — mixing them produces confusing interface-mismatch errors.
- **Measure with `lipgloss.Width()`, never `len()`.** Styled strings contain ANSI escapes, and Unicode is multi-byte; `len()` lies about both.
- **`lipgloss.Width(n)` counts padding but not the border, and it *wraps* overflow.** For a pane meant to occupy `W` total columns with `RoundedBorder()` + `Padding(0,1)`: set `.Width(W-2)` (the border adds 2 outside the width), and the usable content width is `W-4`. Truncate every content line to `W-4` (with an ANSI-aware truncator — `charmbracelet/x/ansi`'s `Truncate`, not a rune slice that cuts mid-escape) before rendering, or lipgloss silently wraps the long line into a second row and your fixed-height layout overflows. Same for `Height`: clip the line count yourself; `.Height(h)` pads short content but won't truncate tall content. (Getting this off by 2 makes panes render short and long lines wrap — it cost an afternoon here.)
- **`lipgloss.HasDarkBackground()` queries the terminal — read it once, early, and have a fallback.** It probes the terminal's background and can be wrong or slow over some multiplexers/SSH, and probing every frame is wasteful. Call it once when building the theme in `New()`, store the resulting `Theme` on the model, and default to Mocha (dark) when the answer is ambiguous — a dark theme on a dark terminal is the safe miss.
- **Handle `tea.WindowSizeMsg`.** It arrives once at startup and on every resize; layout that ignores it renders wrong in tmux panes and resized windows. There is no reliable way to get the size *before* the first message.
- **Don't print to stdout while the TUI runs.** A stray `fmt.Println` (or library log line) corrupts the alt-screen render. Route logs to a file via `charmbracelet/log` + `tea.LogToFile`.
- **Commands run concurrently.** Two in-flight `tea.Cmd`s complete in any order — tag messages with what they belong to (e.g. a request ID or enum) instead of assuming the last-issued command answers first.
- **Quitting:** `tea.Quit` is a command, not a function call — `return m, tea.Quit`. Cleanup belongs after `program.Run()` returns, not in `Update`.
- **Testing:** use `charmbracelet/x/exp/teatest` for golden-file tests of full models; keep `internal/{chezmoi,mise,drift}` testable with plain table tests by injecting a command-runner interface instead of calling `exec` directly.

## References

- Bubble Tea docs & tutorials: https://github.com/charmbracelet/bubbletea (see `examples/` — the fastest way to learn a component)
- Lip Gloss: https://github.com/charmbracelet/lipgloss
- Huh: https://github.com/charmbracelet/huh
- Charm blog (patterns, v2 migration notes): https://charm.sh/blog/
- [ADR-0002 — Go with the Charm stack](../adrs/0002-go-and-charm-for-the-tui.md)
