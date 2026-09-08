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
// The exit comes first because the keymap is truncated to the window: whatever
// falls off the end, the only way out stays on screen (issues #30, #21).
//
// It is a working subset, not the whole keymap. At 114 columns it was already
// truncating on a 100-column window, and M4's four new keys would have taken
// it to 183, pushing a working key off the end for each one added. `ctrl+o ?`
// is the complete list now, and helpLines is where it lives (#103).
//
// 76 columns, so at DefaultWidth 80 it fits whole for the first time - and the
// key that reaches the rest of the keymap is second, where truncation cannot
// reach it. Its contents are the four keys earlier issues won a guarantee for
// - q, j/k and n (#30, #28) and d (#21) - plus ? for everything else. A
// binding added here rather than to helpLines must keep all of that true.
// exitKeyFor is the guarantee the footer describes, named once so the startup
// notice can keep it in front of itself rather than replacing the line that
// carries it (#28, #30, #43). Built per leader, because the leader is
// configurable (#44).
func exitKeyFor(leader string) string { return leader + " q quit" }

// footerLine is the resting keymap. At DefaultLeader it is the 76 columns
// #103 measured; a longer leader is cut from the right, where the help key
// is, and never from the exit key (#44).
func footerLine(leader string) string {
	return exitKeyFor(leader) + "  " + leader + " ? keys  " + leader + " j/k switch  " +
		leader + " n new  " + leader + " d diff"
}

// reviewFooterLine replaces the footer while the review column has focus:
// those keys are the ones that do anything there.
//
// It ends in the help key for the same reason footer does. At 84 columns it
// overflowed an 80-column window and cut `h/l pan` off the end entirely - the
// defect #103 fixed in footer and left standing one constant below it. The
// keys that came off are in the help modal, which is what the help key reaches
// (#103).
func reviewFooterLine(leader string) string {
	return "j/k move  c comment  d delete  r reload  S submit  esc back  " + leader + " ? keys"
}

// treeFooterLine replaces reviewFooterLine in the tree and preview views,
// where c and S do nothing and enter does the work (#24).
//
// It names h/l because it has the room to: at 64 columns it still fits the
// 80-column DefaultWidth, while the same eight cells would take reviewFooter
// to 83 and push a working key off the end. Half of #125 was that the axis
// existed and nothing on screen said so.
func treeFooterLine(leader string) string {
	return "j/k move  h/l/0 pan  enter open  r reload  esc back  " + leader + " ? keys"
}

// emptyTreeHint is the tree's empty state: a repository that listed
// successfully and holds nothing, which is not a listing still in flight (#131).
const emptyTreeHint = "no files - the repository is empty; press r to list again"

// emptyStateHint names the next useful action. With no projects registered,
// creating a session can only fail, so it points at `omatty add` instead.
// emptyStateHint says what to do next. It names the project when the cursor
// rests on an empty one, because with other projects' sessions on screen
// "no sessions" alone reads as a lie (#158).
func (m *Model) emptyStateHint() string {
	if len(m.sidebar.Rows()) == 0 {
		return "no projects - run `omatty add <dir>` to register one"
	}
	p, _ := m.sidebar.SelectedHeader()
	return "no sessions in " + p + " - press " + m.leader + " n to create one"
}

// View lays the sidebar beside the focused session's terminal, with the
// keymap underneath (issue #35). Both boxes are sized exactly before the
// border is applied, so lipgloss adds precisely one column and row per side
// and the frame never exceeds the window.
func (m *Model) View() tea.View {
	termW, termH := PaneSize(m.width, m.height, m.review.Open)
	now := m.clock() // once per frame, so every row ages against the same instant
	columns := []string{m.renderSidebar(termH), m.renderTerminal(termW, termH, now)}
	if m.review.Open {
		columns = append(columns, m.renderReview(ReviewWidth(m.width, true)-2, termH))
	}
	panes := lipgloss.JoinHorizontal(lipgloss.Top, columns...)
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, panes, m.renderFooter()))
	v.AltScreen = true
	v.ReportFocus = true // so FocusMsg/BlurMsg drive notifications
	// Without this the host sends no mouse events at all, and its alternate
	// scroll turns every wheel notch into arrow keys that land in Claude's
	// prompt. The cost is the crosshair pointer and shift-drag to select
	// text, which is what asking for the wheel costs anywhere (#107).
	v.MouseMode = tea.MouseModeCellMotion
	v.Cursor = m.paneCursor()
	return v
}

// renderSidebar boxes the project/session rows at exactly SidebarWidth. The
// "projects" line is chrome and stays pinned; the rows beneath it scroll so
// the cursor is always drawn (#129). A project header scrolled off leaves its
// sessions unlabelled, deliberately: pinning the current project's header
// would spend a second row of chrome in a pane that is already short, and the
// marker and the pane title name the session either way.
func (m *Model) renderSidebar(rows int) string {
	inner := SidebarWidth - 2
	lines := make([]string, 0, rows)
	for _, row := range m.sidebar.Window(rows - sidebarHeaderRows) {
		lines = append(lines, m.renderRow(row, inner))
	}
	return titledBox(false, "projects", inner, fitBlock(lines, inner, rows))
}

// renderTerminal boxes the open modal surface, the focused terminal, or the
// empty-state guidance. The modal takes the pane's content at the pane's own
// size, never a size of its own (#95).
func (m *Model) renderTerminal(w, h int, now time.Time) string {
	if m.modalOpen() {
		return titledBox(true, "", w, fitBlock(m.modalLines(), w, h))
	}
	if term := m.focusedTerminal(); term != nil {
		// h rows, not h-1: the title is in the rule now (#128).
		body := fitBlock(strings.Split(term.View(), "\n"), w, h)
		// Dimmed while the review column has the keys, so the border says
		// where a keystroke will land (#21).
		return titledBox(!m.review.Focused, m.terminalTitle(now), w, body)
	}
	return titledBox(false, "", w, fitBlock(m.emptyLines(), w, h))
}

// emptyLines is the pane with no session to show: what to do next, and the way
// out. With no session focused ctrl+c also quits, which is worth saying because
// it is the reflex an operator reaches for first (issue #28).
func (m *Model) emptyLines() []string {
	return []string{"", m.emptyStateHint(), "", "ctrl+c or " + m.leader + " q to quit"}
}

// terminalTitle is the header line inside the focused session's box: its
// title, coloured status, age and cumulative tokens.
func (m *Model) terminalTitle(now time.Time) string {
	row, ok := m.sidebar.Selected()
	if !ok {
		return ""
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
	return parts
}

// renderFooter shows the keymap, or the last error until the next keypress.
// Errors live here rather than in a pane so they are visible whether or not
// a session has focus. fitLine, not padRight: on a narrow window the keymap is
// truncated rather than pushing the frame wider than the screen.
// A startup notice sits between the two: it outranks the keymap, because it
// says something the screen cannot, and is outranked by an error, because an
// error is about what the operator just did. Both are cleared by the next
// keypress (#43).
func (m *Model) renderFooter() string {
	if m.lastErr != "" {
		return errorStyle.Render(fitLine(" error: "+m.lastErr, m.width))
	}
	if m.notice != "" {
		// The exit comes first, for the reason the footer const gives: the line
		// is truncated to the window, so whatever falls off the end, the only
		// way out stays on screen. Replacing the whole line hid `ctrl+o q` on
		// exactly the machines the notice appears on - a fresh install without
		// dtach - where the terminal pane owns ctrl+c and the footer is the
		// only place the way out is written down (#28, #30, #43).
		return footerStyle.Render(fitLine(" "+exitKeyFor(m.leader)+"  "+m.notice, m.width))
	}
	return footerStyle.Render(fitLine(" "+m.footerKeys(), m.width))
}

// footerKeys is the keymap for whatever has focus. A modal surface comes
// first: while one is open its keys are the only ones that do anything.
func (m *Model) footerKeys() string {
	if s := modalFooter(m.modal); s != "" {
		return s
	}
	if !m.reviewOwnsKeys() {
		return footerLine(m.leader)
	}
	if m.review.View == ViewDiff {
		return reviewFooterLine(m.leader)
	}
	return treeFooterLine(m.leader)
}

// renderRow draws one sidebar line: a project header, or a session with its
// focus marker, coloured status glyph, name and activity lane:
//
//	» ● parser-fix    ▁▃▇█▅▂▁▁
//
// The age left this row for the focused pane's rule, the one place it is
// worth a look, and its columns became the lane (#128). Every width here is
// lipgloss.Width: the old len(age) was a byte count, correct only while
// AgeString returned pure ASCII, and the lane is the first non-ASCII thing
// in this expression.
func (m *Model) renderRow(row Row, width int) string {
	if row.Session == nil {
		return mutedStyle.Render(padRight(m.headerMarker(row.Project)+row.Project, width))
	}
	marker := "  "
	if sel, ok := m.sidebar.Selected(); ok && sel.Session.ID == row.Session.ID {
		marker = "» "
	}
	glyph := glyphStyle(row.Status).Render(statusGlyph(row.Status))
	title := fitLine(row.Session.Title, width-rowChrome)
	return marker + glyph + " " + title + " " + m.renderLane(row.Session.ID)
}

// headerMarker is "» " on the empty project the cursor rests on, and the
// plain "> " on every other header, so the two spend the same two cells and
// the names do not jog when the cursor arrives (#158).
func (m *Model) headerMarker(project string) string {
	if p, ok := m.sidebar.SelectedHeader(); ok && p == project {
		return "» "
	}
	return "> "
}

// rowChrome is what a session row spends on anything but its name: the two
// marker cells, the glyph and the space after it, the lane and the space
// before it. Thirteen columns of name at SidebarWidth 28.
const rowChrome = 2 + 1 + 1 + 1 + laneCells

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
