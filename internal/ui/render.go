package ui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/watcher"
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
// (#103). r left for the same reason on 2026-09-09: the column reloads itself
// when a turn ends (#21, #195), and o (#200) needed the cells to stay under
// 80.
func reviewFooterLine(leader string) string {
	return "j/k move  c comment  d delete  S submit  esc back  " + leader + " ? keys"
}

// treeFooterLine replaces reviewFooterLine in the tree and preview views,
// where c and S do nothing and enter does the work (#24).
//
// It names h/l because it has the room to: the same eight cells would have
// taken reviewFooter past 80 and pushed a working key off the end. Half of
// #125 was that the axis existed and nothing on screen said so. j/k and r
// moved to the help modal on 2026-09-09 so that / (#198), a (#199) and o
// (#200) fit: budgeted together they took this line to 94 columns, and the
// tree re-lists itself at every turn end now (#195), so r is the rare key.
func treeFooterLine(leader string) string {
	return "h/l/0 pan  enter open  / filter  esc back  " + leader + " ? keys"
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

// View lays the header row and the rule over the body, and the keymap under
// it (#35, #174). Every column is sized exactly before it is joined, so the
// frame never exceeds the window.
func (m *Model) View() tea.View {
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, m.frame(), m.renderFooter()))
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

// frame is the header row, the rule and the body: the sidebar, the pane and,
// when open, the review column, a hairline between each pair (#174).
func (m *Model) frame() string {
	termW, termH := PaneSize(m.width, m.height, m.review.Open)
	now := m.clock() // once per frame, so every age is measured against the same instant
	cols, segs := m.bodyColumns(termW, termH, now)
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	return lipgloss.JoinVertical(lipgloss.Left, headerRow(segs), ruleRow(segs), body)
}

// bodyColumns is every column with the hairline before each one but the
// first, and one header segment per column. The accent hairline and the ink
// title go to the same column: the one keyboardEdge names.
func (m *Model) bodyColumns(termW, termH int, now time.Time) ([]string, []segment) {
	edge := m.keyboardEdge()
	cols := []string{m.renderSidebar(termH, now), hairlineColumn(edge == edgePane, termH), m.renderTerminal(termW, termH)}
	// termW-2: the leading space headerRow adds and one blank before the
	// hairline, so the right-aligned usage never touches it (#177).
	segs := []segment{
		{title: m.sidebarSegment(), width: sidebarContentCols},
		{title: m.paneSegment(now, termW-2, edge == edgePane), width: termW, owns: edge == edgePane},
	}
	if !m.review.Open {
		return cols, segs
	}
	rw := reviewContentWidth(m.width)
	cols = append(cols, hairlineColumn(edge == edgeReview, termH), m.renderReview(rw, termH))
	return cols, append(segs, segment{title: m.reviewTitle(), width: rw, owns: edge == edgeReview})
}

// renderSidebar draws the project/session rows in the sidebar's content
// columns; the "projects" title is the header row's (#128, #174). The rows
// scroll so the cursor is always drawn (#129). A project header scrolled off
// leaves its sessions unlabelled, deliberately: pinning the current project's
// header would spend a second row of chrome in a pane that is already short,
// and the marker and the pane title name the session either way. now is the
// cards' ages (#176).
func (m *Model) renderSidebar(rows int, now time.Time) string {
	lines := make([]string, 0, rows)
	for _, row := range m.sidebar.Window(rows - sidebarHeaderRows) {
		lines = append(lines, m.renderRow(row, now)...)
	}
	return fitBlock(lines, sidebarContentCols, rows)
}

// renderTerminal draws the open modal surface, the focused terminal, or the
// empty-state guidance. The modal takes the pane's content at the pane's own
// size, never a size of its own (#95). Which column owns the keys is the
// hairline's to say, decided in keyboardEdge (#174).
func (m *Model) renderTerminal(w, h int) string {
	if m.modalOpen() {
		return fitBlock(m.modalLines(), w, h)
	}
	if term := m.focusedTerminal(); term != nil {
		// h rows, not h-1: the title is in the header row (#128, #174).
		return fitBlock(strings.Split(term.View(), "\n"), w, h)
	}
	return fitBlock(m.emptyLines(), w, h)
}

// emptyLines is the pane with no session to show: what to do next, and the way
// out. With no session focused ctrl+c also quits, which is worth saying because
// it is the reflex an operator reaches for first (issue #28).
func (m *Model) emptyLines() []string {
	return []string{"", m.emptyStateHint(), "", "ctrl+c or " + m.leader + " q to quit"}
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
	return joinEnds(footerStyle.Render(" "+m.footerKeys()), m.footerFacts(), m.width)
}

// footerFacts is the footer's right side (#178): how many sessions there
// are, and how many wait, the waiting count in amber because it is the
// waiting state and the colour rule allows exactly that (#175).
func (m *Model) footerFacts() string {
	facts := mutedStyle.Render(countNoun(len(m.state.Sessions), "session"))
	if k := m.waitingCount(); k > 0 {
		facts += mutedStyle.Render(" · ") + amberStyle.Render(strconv.Itoa(k)+" waiting")
	}
	return facts
}

// waitingCount is how many registered sessions are stopped for the operator.
func (m *Model) waitingCount() int {
	n := 0
	for id, st := range m.status {
		if st.Status == watcher.StatusWaiting && m.knownSession(id) {
			n++
		}
	}
	return n
}

// countNoun is "1 session" or "3 sessions".
func countNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// joinEnds lays left and right on one line of width cells with at least two
// spaces between. When they do not both fit, right is dropped whole and left
// is cut to the window: the footer's keys outrank its facts, and the header's
// title outranks its meter (#178, #177). Both sides may carry SGR.
func joinEnds(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if right == "" || gap < 2 {
		return fitLine(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
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

// fitLine makes s exactly width cells: cut when longer, padded when shorter.
// A cut that lands inside a wide rune drops the whole rune and leaves a cell
// short, so the cut is padded too - the header row's exact width depends on
// it, where the box's rule used to measure the title itself (#174).
func fitLine(s string, width int) string {
	if lipgloss.Width(s) > width {
		s = lipgloss.NewStyle().MaxWidth(width).Render(s)
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
