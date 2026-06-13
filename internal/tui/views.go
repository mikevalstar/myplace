package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"

	"github.com/mikevalstar/myplace/internal/drift"
)

// truncate clips s to w display columns, adding an ellipsis when cut. It is
// ANSI-aware (ansi.Truncate), so styled lines keep their escape codes intact
// and never wrap inside a pane.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// pad right-pads s with spaces to w display columns (s assumed unstyled), so a
// selected-row background spans the full pane width.
func pad(s string, w int) string {
	if g := w - lipgloss.Width(s); g > 0 {
		return s + strings.Repeat(" ", g)
	}
	return s
}

func count(p *int) string {
	if p == nil {
		return "?"
	}
	return fmt.Sprintf("%d", *p)
}

func (m Model) badge(verdict string) string {
	switch verdict {
	case drift.VerdictInSync:
		return m.theme.OK.Render("IN SYNC")
	case drift.VerdictDrifted:
		return m.theme.Warn.Render("DRIFTED")
	case drift.VerdictUnknown:
		return m.theme.Warn.Render("UNKNOWN")
	default:
		return m.theme.Bad.Render("ERROR")
	}
}

// --- pane content (rows + selectables) ------------------------------------

func (m Model) paneContent(f focus) ([]paneRow, []selectable) {
	switch f {
	case focusDotfiles:
		return m.dotfilesContent()
	case focusTools:
		return m.toolsContent()
	case focusUpdates:
		return m.updatesContent()
	}
	return nil, nil
}

func (m Model) dotfilesContent() ([]paneRow, []selectable) {
	th := m.theme
	d := m.report.Dotfiles
	var rows []paneRow
	var items []selectable
	plain := func(s string) { rows = append(rows, paneRow{text: s, selIdx: -1}) }

	plain("behind origin:    " + count(d.BehindOrigin))
	plain(fmt.Sprintf("to apply:         %d", len(d.ToApply)))
	for _, f := range d.ToApply {
		items = append(items, selectable{isDiff: true, path: f, title: "diff · " + f})
		rows = append(rows, paneRow{text: "  ↓ " + f, style: th.Del, selIdx: len(items) - 1})
	}
	plain(fmt.Sprintf("modified locally: %d", len(d.LocalModified)))
	for _, f := range d.LocalModified {
		items = append(items, selectable{isDiff: true, path: f, title: "diff · " + f})
		rows = append(rows, paneRow{text: "  ↑ " + f, style: th.Add, selIdx: len(items) - 1})
	}
	plain("uncommitted:      " + count(d.UncommittedFiles))
	plain("unpushed commits: " + count(d.UnpushedCommits))
	return rows, items
}

func (m Model) toolsContent() ([]paneRow, []selectable) {
	th := m.theme
	t := m.report.Tools
	var rows []paneRow
	var items []selectable

	rows = append(rows, paneRow{text: fmt.Sprintf("missing:  %d", len(t.Missing)), selIdx: -1})
	for _, n := range t.Missing {
		items = append(items, selectable{title: "tool · " + n, body: []string{n, "", "status: not installed", "source: mise"}})
		rows = append(rows, paneRow{text: "  + " + n, style: th.Add, selIdx: len(items) - 1})
	}
	rows = append(rows, paneRow{text: fmt.Sprintf("outdated: %d", len(t.Outdated)), selIdx: -1})
	for _, o := range t.Outdated {
		items = append(items, selectable{title: "tool · " + o.Name, body: []string{
			o.Name, "", "current: " + o.Current, "wanted:  " + o.Wanted, "source:  mise",
		}})
		rows = append(rows, paneRow{text: fmt.Sprintf("  %s %s → %s", o.Name, o.Current, o.Wanted), style: th.Del, selIdx: len(items) - 1})
	}
	return rows, items
}

// updatesContent is the dashboard's "Updates available" pane: per-source
// outdated counts from the (informational) inventory. Selecting a source shows
// its package list in the detail panel. Independent of the verdict (ADR-0010).
func (m Model) updatesContent() ([]paneRow, []selectable) {
	th := m.theme
	var rows []paneRow
	var items []selectable
	if m.invLoading || m.inventory == nil {
		rows = append(rows, paneRow{text: "checking…", style: th.Subtle, selIdx: -1})
		return rows, items
	}
	for _, s := range m.inventory.Sources {
		var line string
		var st lipgloss.Style
		var body []string
		switch {
		case !s.Available:
			line = fmt.Sprintf("%s: n/a", s.Name)
			st = th.Subtle
			body = []string{s.Name, "", "not available on this machine"}
		case s.Error != "":
			line = fmt.Sprintf("%s: error", s.Name)
			st = th.Err
			body = []string{s.Name, "", "! " + s.Error}
		default:
			line = fmt.Sprintf("%s: %d", s.Name, len(s.Packages))
			if len(s.Packages) > 0 {
				st = th.Del
			}
			body = append(body, fmt.Sprintf("%s — %d outdated", s.Name, len(s.Packages)), "")
			if len(s.Packages) == 0 {
				body = append(body, "up to date")
			}
			for _, p := range s.Packages {
				body = append(body, fmt.Sprintf("%s  %s → %s", p.Name, p.Current, p.Latest))
			}
		}
		items = append(items, selectable{title: "updates · " + s.Name, body: body})
		rows = append(rows, paneRow{text: line, style: st, selIdx: len(items) - 1})
	}
	rows = append(rows, paneRow{text: "", selIdx: -1})
	rows = append(rows, paneRow{text: "press o for details", style: th.Help, selIdx: -1})
	return rows, items
}

// --- pane rendering -------------------------------------------------------

// renderPane draws a pre-built header + rows into a bordered pane exactly w×h.
// The focused pane uses the focus-ring border; the selected row (when focused)
// is highlighted. Rows are truncated to width and windowed to fit height (with
// a "+N more" tail) so nothing wraps or overflows.
func (m Model) renderPane(header string, rows []paneRow, w, h int, focused bool, sel int) string {
	th := m.theme
	style := th.PaneStyle
	if focused {
		style = th.PaneFocused
	}
	contentW := w - 4
	innerH := h - 2
	if contentW < 1 || innerH < 1 {
		return ""
	}
	body := m.windowRows(rows, innerH-1, focused, sel, contentW)
	lines := append([]string{header}, body...)
	return style.Width(w - 2).Height(innerH).Render(strings.Join(lines, "\n"))
}

// cardHeader lays out a card title: an accent bar + accent-colored title on the
// left and an optional count chip on the right, sized to contentW.
func cardHeader(accent lipgloss.Style, title, chip string, contentW int) string {
	left := accent.Render("▌ " + title)
	if chip == "" {
		return truncate(left, contentW)
	}
	gap := contentW - lipgloss.Width(left) - lipgloss.Width(chip)
	if gap < 1 {
		return truncate(left, contentW)
	}
	return left + strings.Repeat(" ", gap) + chip
}

// cardChip is the right-aligned count badge for a dashboard card: the accent
// chip when there are items, a green ✓ when the card is clean.
func (m Model) cardChip(f focus, count int) string {
	if count > 0 {
		return m.theme.ChipAttn[int(f)].Render(fmt.Sprintf("%d", count))
	}
	return m.theme.ChipClear.Render("✓")
}

// cardCount is the headline number a card's chip shows: drifted dotfiles,
// missing+outdated tools, or total outdated packages.
func (m Model) cardCount(f focus) int {
	if m.report == nil {
		return 0
	}
	switch f {
	case focusDotfiles:
		return len(m.report.Dotfiles.ToApply) + len(m.report.Dotfiles.LocalModified)
	case focusTools:
		return len(m.report.Tools.Missing) + len(m.report.Tools.Outdated)
	case focusUpdates:
		if m.inventory == nil {
			return 0
		}
		n := 0
		for _, s := range m.inventory.Sources {
			n += len(s.Packages)
		}
		return n
	}
	return 0
}

func (m Model) windowRows(rows []paneRow, avail int, focused bool, sel, contentW int) []string {
	th := m.theme
	if avail < 1 {
		return nil
	}
	render := func(r paneRow) string {
		plain := truncate(r.text, contentW)
		if focused && r.selIdx >= 0 && r.selIdx == sel {
			return th.Selected.Render(pad(plain, contentW))
		}
		return r.style.Render(plain)
	}
	if len(rows) <= avail {
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, render(r))
		}
		return out
	}
	// Overflow: reserve the last line for a "+N more" indicator and window the
	// rest around the selected row so it stays visible.
	usable := avail - 1
	selRow := -1
	if focused {
		for i, r := range rows {
			if r.selIdx == sel {
				selRow = i
				break
			}
		}
	}
	start := 0
	if selRow >= usable {
		start = selRow - usable + 1
	}
	if start > len(rows)-usable {
		start = len(rows) - usable
	}
	if start < 0 {
		start = 0
	}
	out := make([]string, 0, avail)
	for i := start; i < start+usable; i++ {
		out = append(out, render(rows[i]))
	}
	out = append(out, th.Subtle.Render(truncate(fmt.Sprintf("…+%d more", len(rows)-usable), contentW)))
	return out
}

func (m Model) renderActivity(w, h int) string {
	th := m.theme
	contentW := w - 4
	innerH := h - 2
	if contentW < 1 || innerH < 1 {
		return ""
	}
	avail := innerH - 1
	rows := m.activityRows()
	if len(rows) > avail && avail > 0 {
		rows = rows[len(rows)-avail:] // keep the most recent
	}
	lines := []string{cardHeader(th.AccentNeutral, "Activity", "", contentW)}
	for _, r := range rows {
		lines = append(lines, r.style.Render(truncate(r.text, contentW)))
	}
	return th.PaneStyle.Width(w - 2).Height(innerH).Render(strings.Join(lines, "\n"))
}

// activityRows puts notices (update available, errors) first, then the log tail.
func (m Model) activityRows() []paneRow {
	th := m.theme
	var rows []paneRow
	r := m.report
	if r != nil && r.Myplace.Latest != nil && *r.Myplace.Latest != r.Myplace.Current {
		rows = append(rows, paneRow{text: fmt.Sprintf("myplace %s → %s available (myplace self-update)", r.Myplace.Current, *r.Myplace.Latest), style: th.Notice, selIdx: -1})
	}
	for _, e := range m.updateErrs {
		rows = append(rows, paneRow{text: "! " + e, style: th.Err, selIdx: -1})
	}
	if r != nil {
		for _, e := range r.Errors {
			rows = append(rows, paneRow{text: "! " + e, style: th.Err, selIdx: -1})
		}
	}
	for _, ln := range m.activity {
		rows = append(rows, paneRow{text: ln, style: th.Subtle, selIdx: -1})
	}
	return rows
}

// --- detail panel ---------------------------------------------------------

func (m Model) detailTitle() string {
	if m.report == nil {
		return "Detail"
	}
	_, items := m.paneContent(m.focus)
	if len(items) == 0 || m.sel[m.focus] > len(items)-1 {
		return "Detail"
	}
	return items[m.sel[m.focus]].title
}

func (m Model) renderDiff(diff string) string {
	th := m.theme
	cw := m.detailVP.Width
	if cw <= 0 {
		cw = m.width
	}
	var b strings.Builder
	for _, ln := range strings.Split(strings.TrimRight(diff, "\n"), "\n") {
		styled := ln
		switch {
		case strings.HasPrefix(ln, "+++"), strings.HasPrefix(ln, "---"):
			styled = th.Subtle.Render(ln)
		case strings.HasPrefix(ln, "@@"):
			styled = th.Hunk.Render(ln)
		case strings.HasPrefix(ln, "+"):
			styled = th.Add.Render(ln)
		case strings.HasPrefix(ln, "-"):
			styled = th.Del.Render(ln)
		}
		b.WriteString(truncate(styled, cw) + "\n")
	}
	return b.String()
}

func (m Model) renderDetailBody(it selectable) string {
	cw := m.detailVP.Width
	if cw <= 0 {
		cw = m.width
	}
	var b strings.Builder
	for _, ln := range it.body {
		b.WriteString(truncate(ln, cw) + "\n")
	}
	return b.String()
}

// renderDetailPanel is the wide-layout right-hand panel (bordered box wrapping
// the scrollable detail viewport).
func (m Model) renderDetailPanel(w, h int) string {
	th := m.theme
	contentW := w - 4
	innerH := h - 2
	if contentW < 1 || innerH < 1 {
		return ""
	}
	title := cardHeader(th.AccentDetail, m.detailTitle(), "", contentW)
	content := title + "\n" + m.detailVP.View()
	return th.PaneStyle.Width(w - 2).Height(innerH).Render(content)
}

// --- header / footer ------------------------------------------------------

func (m Model) header(width int) string {
	th := m.theme
	r := m.report
	left := th.Header.Render("myplace "+m.version) + "  " + m.badge(r.Verdict) +
		th.Subtle.Render(fmt.Sprintf("  %s (%s)", r.Machine, r.Profile))
	right := th.Subtle.Render("checked " + r.CheckedAt.Local().Format("15:04:05"))
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
		right = ""
	}
	bar := left + strings.Repeat(" ", gap) + right
	return truncate(bar, width) + "\n" + th.Rule.Render(strings.Repeat("─", width))
}

func (m Model) footer(width int) string {
	th := m.theme
	fh := m.help
	fh.ShowAll = false
	fh.Width = width
	left := fh.View(m.keys)
	var right string
	if r := m.report; r != nil && r.Myplace.Latest != nil && *r.Myplace.Latest != r.Myplace.Current {
		right = th.Notice.Render(fmt.Sprintf("update available: %s ↑", *r.Myplace.Latest))
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return truncate(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

// --- top-level views ------------------------------------------------------

func (m Model) View() string {
	w, h := m.width, m.height
	if w == 0 || h == 0 {
		return "  starting…"
	}
	if m.showHelp {
		return m.helpView()
	}
	switch m.mode {
	case modeOutdated:
		return m.outdatedView()
	case modeDetail:
		return m.detailView()
	}
	// Initial load (and the post-update reload) — a centered spinner; there's
	// no dashboard to show behind it yet.
	if m.report == nil || m.loading {
		return m.loadingScreen()
	}
	// Build the dashboard, then float the update modal over a dimmed copy of it
	// so it reads as a window in front of a disabled backdrop.
	var dash string
	switch {
	case w < minWidth || h < minHeight:
		dash = m.smallView()
	case m.wide():
		dash = m.wideView()
	default:
		dash = m.narrowView()
	}
	if m.updating {
		return m.overlayModal(dash, m.updateModal())
	}
	return dash
}

func (m Model) loadingScreen() string {
	th := m.theme
	center := fmt.Sprintf("%s  checking status…\n\n%s", m.spinner.View(), th.Subtle.Render("myplace "+m.version))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, center)
}

// updateModal is the floating "window" shown during a converge: a bordered,
// filled panel with the per-step checklist and a progress bar.
func (m Model) updateModal() string {
	th := m.theme
	innerW := 40
	if m.width-12 < innerW {
		innerW = m.width - 12
	}
	if innerW < 16 {
		innerW = 16
	}
	title := th.AccentDetail.Render("▌ updating") + th.Subtle.Render("  converge-only")
	// The box sizes to its content (the bar, innerW wide); the border + padding
	// wrap around it. Don't set an explicit Width or the padding would squeeze
	// the bar and wrap it.
	return th.Modal.Render(title + "\n\n" + m.progressBlock(innerW))
}

func (m Model) progressBlock(innerW int) string {
	th := m.theme
	steps := []struct {
		s     updateStep
		label string
	}{
		{stepChezmoi, "chezmoi apply"},
		{stepMiseInstall, "mise install"},
		{stepMiseUpgrade, "mise upgrade"},
	}
	var lines []string
	for _, st := range steps {
		var marker string
		var style lipgloss.Style
		switch {
		case m.updateStep > st.s:
			marker, style = "✓", th.Add
		case m.updateStep == st.s:
			marker, style = m.spinner.View(), th.Progress // live spinner on the active step
		default:
			marker, style = "·", th.Subtle
		}
		lines = append(lines, style.Render(marker+" "+st.label))
	}
	done := int(m.updateStep) - int(stepChezmoi)
	if done < 0 {
		done = 0
	}
	if done > 3 {
		done = 3
	}
	p := m.progress
	p.Width = innerW
	lines = append(lines, "", p.ViewAs(float64(done)/3.0))
	if m.updateStep == stepDone {
		lines = append(lines, "", th.Subtle.Render(m.spinner.View()+" refreshing status…"))
	}
	return strings.Join(lines, "\n")
}

// overlayModal composites fg centered over a dimmed bg. Small terminals (where
// compositing would be cramped) just center the modal on a blank screen.
func (m Model) overlayModal(bg, fg string) string {
	if m.width < 44 || m.height < 12 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, fg)
	}
	return overlayCenter(m.dim(bg), fg, m.width, m.height)
}

// dim renders the backdrop monochrome+subtle, so it reads as disabled behind
// the modal (colors stripped, then re-colored in one muted tone, per line so
// every row is uniformly dimmed).
func (m Model) dim(s string) string {
	var b strings.Builder
	for i, ln := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.theme.Subtle.Render(ansi.Strip(ln)))
	}
	return b.String()
}

// overlayCenter places fg centered over bg, ANSI-aware, returning H lines of
// width W. The fg box is opaque — its lines replace the bg columns they cover.
func overlayCenter(bg, fg string, W, H int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(strings.TrimRight(fg, "\n"), "\n")
	boxW := 0
	for _, l := range fgLines {
		if w := ansi.StringWidth(l); w > boxW {
			boxW = w
		}
	}
	x := (W - boxW) / 2
	if x < 0 {
		x = 0
	}
	y := (H - len(fgLines)) / 2
	if y < 0 {
		y = 0
	}
	for i, fl := range fgLines {
		row := y + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		if pad := boxW - ansi.StringWidth(fl); pad > 0 {
			fl += strings.Repeat(" ", pad)
		}
		left := ansi.Truncate(bgLines[row], x, "")
		if w := ansi.StringWidth(left); w < x {
			left += strings.Repeat(" ", x-w)
		}
		right := ansi.TruncateLeft(bgLines[row], x+boxW, "")
		bgLines[row] = left + fl + right
	}
	return strings.Join(bgLines, "\n")
}

func (m Model) helpView() string {
	th := m.theme
	hp := m.help
	hp.ShowAll = true
	hp.Width = m.width
	box := th.PaneStyle.Padding(1, 2).Render(th.PaneTitle.Render("myplace — keys") + "\n\n" + hp.View(m.keys))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// wideView is the master-detail layout: panes on the left ~60%, the detail
// panel on the right ~40%.
func (m Model) wideView() string {
	w, h := m.width, m.height
	bodyH := h - 3
	leftW := w * 60 / 100
	rightW := w - leftW

	topH := bodyH * 55 / 100
	if topH < 5 {
		topH = 5
	}
	botH := bodyH - topH
	colW := leftW / 3

	top := lipgloss.JoinHorizontal(lipgloss.Top,
		m.pane(focusDotfiles, "Dotfiles", colW, topH),
		m.pane(focusTools, "Tools (mise)", colW, topH),
		m.pane(focusUpdates, "Updates available", leftW-2*colW, topH),
	)
	left := lipgloss.JoinVertical(lipgloss.Left, top, m.renderActivity(leftW, botH))
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, m.renderDetailPanel(rightW, bodyH))

	return strings.Join([]string{m.header(w), body, m.footer(w)}, "\n")
}

// narrowView keeps the three panes across the top with Activity below and no
// side panel; `enter` opens the detail full-screen instead.
func (m Model) narrowView() string {
	w, h := m.width, m.height
	bodyH := h - 3
	topH := bodyH * 55 / 100
	if topH < 5 {
		topH = 5
	}
	botH := bodyH - topH
	colW := w / 3

	top := lipgloss.JoinHorizontal(lipgloss.Top,
		m.pane(focusDotfiles, "Dotfiles", colW, topH),
		m.pane(focusTools, "Tools (mise)", colW, topH),
		m.pane(focusUpdates, "Updates available", w-2*colW, topH),
	)
	return strings.Join([]string{m.header(w), top, m.renderActivity(w, botH), m.footer(w)}, "\n")
}

// pane renders one focusable dashboard card with its accent header + count chip.
func (m Model) pane(f focus, title string, w, h int) string {
	rows, _ := m.paneContent(f)
	focused := m.focus == f && m.mode == modeDashboard
	contentW := w - 4
	header := cardHeader(m.theme.Accent[int(f)], title, m.cardChip(f, m.cardCount(f)), contentW)
	return m.renderPane(header, rows, w, h, focused, m.sel[f])
}

// smallView is the readable fallback for terminals too small to frame.
func (m Model) smallView() string {
	th := m.theme
	r := m.report
	var b strings.Builder
	head := th.Header.Render("myplace "+m.version) + " " + m.badge(r.Verdict) +
		th.Subtle.Render(fmt.Sprintf(" %s (%s)", r.Machine, r.Profile))
	b.WriteString(truncate(head, m.width) + "\n\n")
	b.WriteString(th.PaneTitle.Render("Dotfiles") + "\n")
	drows, _ := m.dotfilesContent()
	for _, rw := range drows {
		b.WriteString(rw.style.Render(truncate(rw.text, m.width)) + "\n")
	}
	b.WriteString("\n" + th.PaneTitle.Render("Tools") + "\n")
	trows, _ := m.toolsContent()
	for _, rw := range trows {
		b.WriteString(rw.style.Render(truncate(rw.text, m.width)) + "\n")
	}
	b.WriteString("\n" + m.footer(m.width))
	return b.String()
}

func (m Model) detailView() string {
	th := m.theme
	title := th.Header.Render("myplace "+m.version) + th.Subtle.Render("  "+m.detailTitle())
	header := truncate(title, m.width) + "\n" + th.Rule.Render(strings.Repeat("─", m.width))
	footer := th.Help.Render("↑/↓ scroll • esc back • q quit")
	return strings.Join([]string{header, m.detailVP.View(), footer}, "\n")
}

// --- outdated detail view -------------------------------------------------

func (m Model) outdatedView() string {
	th := m.theme
	title := th.Header.Render("myplace "+m.version) + th.Subtle.Render("  outdated packages")
	header := truncate(title, m.width) + "\n" + th.Rule.Render(strings.Repeat("─", m.width))
	var footer string
	if m.filtering {
		footer = truncate(m.filter.View(), m.width)
	} else {
		footer = th.Help.Render("↑/↓ scroll • s sort • / filter • esc back • q quit")
	}
	return strings.Join([]string{header, m.vp.View(), footer}, "\n")
}

// outdatedContent is the scrollable body of the modeOutdated view: a count
// summary plus a lipgloss/table of outdated packages, ordered by the current
// sort and narrowed by the name filter. The viewport clips vertically; the
// table sizes itself to the viewport width so nothing overflows.
func (m Model) outdatedContent() string {
	th := m.theme
	if m.invLoading || m.inventory == nil {
		return th.Subtle.Render("checking…")
	}
	cw := m.vp.Width
	if cw <= 0 {
		cw = m.width
	}
	filter := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	match := func(name string) bool {
		return filter == "" || strings.Contains(strings.ToLower(name), filter)
	}

	type prow struct{ name, cur, lat, src string }
	var rows []prow
	total, srcCount := 0, 0
	for _, s := range m.inventory.Sources {
		if s.Available && s.Error == "" && len(s.Packages) > 0 {
			srcCount++
		}
		for _, p := range s.Packages {
			total++
			if match(p.Name) {
				rows = append(rows, prow{p.Name, p.Current, p.Latest, s.Name})
			}
		}
	}
	if m.sort == sortByName {
		sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	} else {
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].src != rows[j].src {
				return rows[i].src < rows[j].src
			}
			return rows[i].name < rows[j].name
		})
	}

	sortName := "by source"
	if m.sort == sortByName {
		sortName = "by name"
	}
	summary := fmt.Sprintf("%d outdated across %d source(s)", total, srcCount)
	if filter != "" {
		summary += fmt.Sprintf("  ·  %d of %d shown", len(rows), total)
	}
	head := truncate(th.PaneTitle.Render(summary)+th.Subtle.Render("   sort: "+sortName), cw)

	if len(rows) == 0 {
		return head + "\n\n" + th.Subtle.Render("  (no matches)")
	}

	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(th.Rule).
		BorderColumn(false).
		BorderRow(false).
		Width(cw).
		Headers("PACKAGE", "CURRENT", "LATEST", "SOURCE").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return th.AccentNeutral.Padding(0, 1)
			}
			base := lipgloss.NewStyle().Padding(0, 1)
			switch col {
			case 0:
				return base
			case 2:
				return base.Inherit(th.Del) // latest version pops
			default:
				return base.Inherit(th.Subtle)
			}
		})
	for _, r := range rows {
		tbl.Row(r.name, r.cur, "→ "+r.lat, r.src)
	}
	return head + "\n\n" + tbl.Render()
}
