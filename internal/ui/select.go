// Dragging a selection inside the session pane (#360).
//
// The host terminal's own selection is linear over the whole screen, and
// omatty draws the sidebar, the session pane and the review column on the
// same screen rows. So as soon as a drag wrapped a line, the copy carried
// every column on those rows. tmux has the same layout and answers it the
// same way: own the drag, clip it to the pane, and copy through OSC 52.
//
// The two halves were already here - ?1002h delivers press, motion and
// release (#107), and tea.SetClipboard forwards a copy to the host (#212).
// Only the selection between them was missing.
//
// Invariant 1 is untouched: a drag is a mouse event, so the modal key router
// never sees it. Invariant 2 is argued in termwrap/text.go, where the cells
// are read.

package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// paneSelection is a drag in progress over the focused session's pane, in
// that pane's own cell coordinates. The zero value is no selection.
//
// Moved is what separates a drag from a click: a release with no motion
// between press and release must leave sidebar (#45) and review (#168)
// clicks exactly as they were.
type paneSelection struct {
	Active bool
	Moved  bool
	X0, Y0 int
	X1, Y1 int
}

// startSelection anchors a selection when a left press lands in the pane's
// grid. A press anywhere else clears any selection, so the highlight never
// outlives the drag that made it.
func (m *Model) startSelection(msg tea.MouseClickMsg) {
	x, y, ok := m.paneCellAt(msg.X, msg.Y)
	if msg.Button != tea.MouseLeft || m.modalOpen() || !ok {
		m.sel = paneSelection{}
		return
	}
	m.sel = paneSelection{Active: true, X0: x, Y0: y, X1: x, Y1: y}
}

// extendSelection follows the pointer while the button is down, clamped to
// the pane so a drag into the sidebar or the review column selects the
// pane's edge rather than their columns.
func (m *Model) extendSelection(msg tea.MouseMotionMsg) {
	if !m.sel.Active || msg.Button != tea.MouseLeft {
		return
	}
	m.sel.X1, m.sel.Y1 = m.clampToPane(msg.X, msg.Y)
	m.sel.Moved = true
}

// finishSelection copies the selected cells to the host clipboard and ends
// the drag. A release with no motion copies nothing: it was a click.
func (m *Model) finishSelection(msg tea.MouseReleaseMsg) tea.Cmd {
	sel := m.sel
	m.sel = paneSelection{}
	if !sel.Active || !sel.Moved {
		return nil
	}
	x1, y1 := m.clampToPane(msg.X, msg.Y)
	term := m.focusedTerminal()
	if term == nil {
		return nil
	}
	text := term.Text(sel.X0, sel.Y0, x1, y1)
	if text == "" {
		return nil
	}
	return tea.SetClipboard(text)
}

// paneCellAt converts a window position to a cell of the embedded terminal's
// grid, reporting false for a position the pane does not draw. It is
// inPaneGrid's question asked in window coordinates, which is what every
// mouse message carries (#106, #107).
func (m *Model) paneCellAt(winX, winY int) (x, y int, ok bool) {
	ox, oy := PaneOrigin()
	x, y = winX-ox, winY-oy
	return x, y, m.inPaneGrid(x, y)
}

// clampToPane converts a window position to the nearest cell the pane draws,
// so a drag that leaves the pane stops at its edge instead of running into
// the columns beside it.
func (m *Model) clampToPane(winX, winY int) (x, y int) {
	ox, oy := PaneOrigin()
	w, h := PTYSize(m.width, m.height, m.review.Open)
	return clamp(winX-ox, 0, w-1), clamp(winY-oy, 0, h-1)
}

// clamp bounds v to [lo, hi]. A pane narrower than a cell would invert the
// range, so lo wins.
func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return min(max(v, lo), hi)
}

// ordered returns the selection's ends in reading order. A drag up or to the
// left arrives with them reversed.
func (s paneSelection) ordered() (x0, y0, x1, y1 int) {
	if s.Y1 < s.Y0 || (s.Y1 == s.Y0 && s.X1 < s.X0) {
		return s.X1, s.Y1, s.X0, s.Y0
	}
	return s.X0, s.Y0, s.X1, s.Y1
}

// highlightSelection draws the run in reverse video while the button is down,
// so the operator can see what a release will copy. It runs on the rendered
// rows rather than the cells, because that is what the pane draws.
func (m *Model) highlightSelection(rows []string, w int) []string {
	if !m.sel.Active || !m.sel.Moved {
		return rows
	}
	x0, y0, x1, y1 := m.sel.ordered()
	for y := y0; y <= y1 && y < len(rows); y++ {
		if y < 0 {
			continue
		}
		from, to := 0, w-1
		if y == y0 {
			from = x0
		}
		if y == y1 {
			to = x1
		}
		rows[y] = reverseRun(rows[y], from, to)
	}
	return rows
}

// reverseRun splices reverse video over one row's cells from..to.
//
// The run's own colours are dropped rather than nested: reverse video wrapped
// around a segment that carries its own SGR is reset by the first code inside
// it, so the highlight would show through in patches. A selection that
// overrides the text's colour is also what a terminal's own does.
//
// The cut is by cell with x/ansi, never by byte, for the reason fitStyled
// gives: a byte slice would cut inside an escape sequence (#197).
func reverseRun(row string, from, to int) string {
	if to < from {
		return row
	}
	left := ansi.Cut(row, 0, from)
	mid := ansi.Strip(ansi.Cut(row, from, to+1))
	right := ansi.TruncateLeft(row, to+1, "")
	return left + cursorStyle.Render(mid) + right
}
