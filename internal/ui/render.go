package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// footer is the keymap, rendered on every frame. It stays visible while a
// session fills the pane because that is exactly the state where ctrl+c
// belongs to Claude and `ctrl+o q` is the only exit (issues #28, #30).
// The exit comes first because the keymap is truncated to the window and the
// review key made it longer than 80 columns: whatever falls off the end, the
// only way out stays on screen (issues #30, #21).
const footer = Leader + " q quit  " + Leader + " j/k switch  " + Leader + " n new  " +
	Leader + " N worktree  " + Leader + " d diff  " + Leader + " r restart  " +
	Leader + " f files"

// reviewFooter replaces footer while the review column has focus: those keys
// are the ones that do anything there.
const reviewFooter = "j/k move  c comment  d delete  r reload  S submit  esc back  " +
	Leader + " d close  h/l pan"

// treeFooter replaces reviewFooter in the tree and preview views, where c and
// S do nothing and enter does the work (#24).
const treeFooter = "j/k move  enter open  r reload  esc back  " + Leader + " f close  h/l pan"

// emptyStateHint names the next useful action. With no projects registered,
// creating a session can only fail, so it points at `omatty add` instead.
func (m *Model) emptyStateHint() string {
	if len(m.sidebar.Rows()) == 0 {
		return "no projects - run `omatty add <dir>` to register one"
	}
	return "no sessions - press " + Leader + " n to create one"
}

// promptLine renders the open new-session prompt.
func (m *Model) promptLine() string {
	label := "new session title"
	if m.prompt.Worktree {
		label = "new branch (worktree)"
	}
	return label + ": " + m.prompt.Buffer + "_"
}

// View lays the sidebar beside the focused session's terminal, with the
// keymap underneath (issue #35). Both boxes are sized exactly before the
// border is applied, so lipgloss adds precisely one column and row per side
// and the frame never exceeds the window.
func (m *Model) View() tea.View {
	termW, termH := PaneSize(m.width, m.height, m.review.Open)
	now := m.clock() // once per frame, so every row ages against the same instant
	columns := []string{m.renderSidebar(termH, now), m.renderTerminal(termW, termH, now)}
	if m.review.Open {
		columns = append(columns, m.renderReview(ReviewWidth(m.width, true)-2, termH))
	}
	panes := lipgloss.JoinHorizontal(lipgloss.Top, columns...)
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, panes, m.renderFooter()))
	v.AltScreen = true
	v.ReportFocus = true // so FocusMsg/BlurMsg drive notifications
	return v
}

// renderSidebar boxes the project/session rows at exactly SidebarWidth.
func (m *Model) renderSidebar(rows int, now time.Time) string {
	inner := SidebarWidth - 2
	lines := make([]string, 0, rows)
	lines = append(lines, headerStyle.Render(padRight("projects", inner)))
	for _, row := range m.sidebar.Rows() {
		lines = append(lines, m.renderRow(row, inner, now))
	}
	return paneBox(false).Render(fitBlock(lines, inner, rows))
}

// renderTerminal boxes the focused terminal, or the empty-state guidance.
func (m *Model) renderTerminal(w, h int, now time.Time) string {
	if term := m.focusedTerminal(); term != nil {
		body := fitBlock(strings.Split(term.View(), "\n"), w, h-1)
		// Dimmed while the review column has the keys, so the border says
		// where a keystroke will land (#21).
		return paneBox(!m.review.Focused).Render(m.terminalTitle(w, now) + "\n" + body)
	}
	lines := []string{""}
	if m.prompt.Active {
		lines = append(lines, m.promptLine())
	} else {
		lines = append(lines, m.emptyStateHint())
	}
	// With no session focused ctrl+c also quits, which is worth saying because
	// it is the reflex an operator reaches for first (issue #28).
	lines = append(lines, "", "ctrl+c or "+Leader+" q to quit")
	return paneBox(m.prompt.Active).Render(fitBlock(lines, w, h))
}

// terminalTitle is the header line inside the focused session's box: its
// title, coloured status, age and cumulative tokens.
func (m *Model) terminalTitle(w int, now time.Time) string {
	row, ok := m.sidebar.Selected()
	if !ok {
		return padRight("", w)
	}
	st := m.status[row.Session.ID]
	parts := row.Session.Title
	if st.Status != "" {
		parts += " · " + glyphStyle(st.Status).Render(statusGlyph(st.Status)+" "+string(st.Status))
	}
	if age := AgeString(now, st.At); age != "" {
		parts += " " + age
	}
	if st.Tokens.In+st.Tokens.Out > 0 {
		parts += " · " + mutedStyle.Render(KString(st.Tokens.In)+" in / "+KString(st.Tokens.Out)+" out")
	}
	return headerStyle.Render(fitLine(parts, w))
}

// renderFooter shows the keymap, or the last error until the next keypress.
// Errors live here rather than in a pane so they are visible whether or not
// a session has focus. fitLine, not padRight: on a narrow window the keymap is
// truncated rather than pushing the frame wider than the screen.
func (m *Model) renderFooter() string {
	if m.lastErr != "" {
		return errorStyle.Render(fitLine(" error: "+m.lastErr, m.width))
	}
	return footerStyle.Render(fitLine(" "+m.footerKeys(), m.width))
}

// footerKeys is the keymap for whatever has focus.
func (m *Model) footerKeys() string {
	if !m.review.Focused {
		return footer
	}
	if m.review.View == ViewDiff {
		return reviewFooter
	}
	return treeFooter
}

// renderRow draws one sidebar line: a project header, or a session with its
// focus marker and coloured status glyph.
func (m *Model) renderRow(row Row, width int, now time.Time) string {
	if row.Session == nil {
		return mutedStyle.Render(padRight("> "+row.Project, width))
	}
	marker := "  "
	if sel, ok := m.sidebar.Selected(); ok && sel.Session.ID == row.Session.ID {
		marker = "» "
	}
	glyph := glyphStyle(row.Status).Render(statusGlyph(row.Status))
	age := ""
	if st, ok := m.status[row.Session.ID]; ok {
		age = AgeString(now, st.At)
	}
	title := padRight(row.Session.Title, width-4-len(age)-1)
	return marker + glyph + " " + title + " " + mutedStyle.Render(age)
}

// fitBlock forces lines to exactly width x height so a border lands
// precisely: short lines are padded, long ones cut, missing rows added.
func fitBlock(lines []string, width, height int) string {
	out := make([]string, height)
	for i := range out {
		if i < len(lines) {
			out[i] = fitLine(lines[i], width)
			continue
		}
		out[i] = strings.Repeat(" ", width)
	}
	return strings.Join(out, "\n")
}

func fitLine(s string, width int) string {
	if lipgloss.Width(s) > width {
		return lipgloss.NewStyle().MaxWidth(width).Render(s)
	}
	return padRight(s, width)
}

// panLine drops the first cols display cells of s, which is how the review
// column reaches text that fitLine would otherwise cut off the right edge
// (issue #94).
//
// It measures cells rather than bytes or runes because a wide rune occupies
// two columns: a byte offset would slice one in half and a rune offset would
// pan half as far as the operator asked. A cut landing inside a wide rune
// drops the whole rune - half a glyph cannot be drawn.
//
//	panLine("hello world", 6) // "world"
func panLine(s string, cols int) string {
	if cols <= 0 {
		return s
	}
	seen := 0
	for i, r := range s {
		if seen >= cols {
			return s[i:]
		}
		seen += lipgloss.Width(string(r))
	}
	return ""
}

// padRight is ANSI-aware: it measures visible width, not bytes.
func padRight(s string, width int) string {
	if n := width - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
