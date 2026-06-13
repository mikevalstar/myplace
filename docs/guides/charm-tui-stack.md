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
| [bubbles](https://github.com/charmbracelet/bubbles) | Stock components: spinner, table, list, viewport, progress, textinput, help |
| [lipgloss](https://github.com/charmbracelet/lipgloss) | Styling and layout (colors, borders, joining boxes) |
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

### Layout with Lip Gloss

Build strings with styles, then compose with `lipgloss.JoinHorizontal` / `JoinVertical`. Define styles once at package level, not per-frame.

```go
var paneStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
```

### Forms with huh

`huh` forms can run standalone (`form.Run()`, fine inside plain cobra commands) or be embedded in a Bubble Tea model as a `tea.Model` — use embedded mode inside the TUI so the wizard doesn't fight the running program for the terminal.

## Gotchas

- **Never block in `Update` or `View`.** All subprocess calls and file I/O go through `tea.Cmd`. (Repeated because it's the one that always happens.)
- **A subprocess can still hang you even from a `tea.Cmd`.** A `tea.Cmd` runs off the render loop, so it won't freeze rendering — but if the child opens `/dev/tty` directly for a prompt (chezmoi's "file changed since I last wrote it?", git credential prompts, ssh passphrase), it grabs the same terminal the TUI owns and waits forever for a keypress the TUI is consuming. The spinner keeps spinning; the command never returns. Defend at the exec layer: set `cmd.Stdin = nil` (closed → EOF, not the inherited TTY) **and** pass the tool's no-prompt flag (`chezmoi --no-tty`, `GIT_TERMINAL_PROMPT=0`). Then a prompt fails fast instead of hanging, and you surface the error. For genuinely interactive subprocesses (an editor, a guided merge), do the opposite — hand over the real terminal with `tea.ExecProcess`, never a background `tea.Cmd`. (This one cost us a stuck dashboard: `chezmoi update` over a locally-modified file.)
- **v1 vs v2:** Bubble Tea has a v2 (`charmbracelet/bubbletea/v2`) with breaking API changes, but most examples, blog posts, and AI training data are v1. Check `go.mod` before pasting any example, and keep bubbles/huh on matching majors — mixing them produces confusing interface-mismatch errors.
- **Measure with `lipgloss.Width()`, never `len()`.** Styled strings contain ANSI escapes, and Unicode is multi-byte; `len()` lies about both.
- **`lipgloss.Width(n)` counts padding but not the border, and it *wraps* overflow.** For a pane meant to occupy `W` total columns with `RoundedBorder()` + `Padding(0,1)`: set `.Width(W-2)` (the border adds 2 outside the width), and the usable content width is `W-4`. Truncate every content line to `W-4` (with an ANSI-aware truncator — `charmbracelet/x/ansi`'s `Truncate`, not a rune slice that cuts mid-escape) before rendering, or lipgloss silently wraps the long line into a second row and your fixed-height layout overflows. Same for `Height`: clip the line count yourself; `.Height(h)` pads short content but won't truncate tall content. (Getting this off by 2 makes panes render short and long lines wrap — it cost an afternoon here.)
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
