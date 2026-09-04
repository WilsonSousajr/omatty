// The filtered list behind the session switcher (#42) and, after it, the
// project picker (#91). One widget rather than two: they differ only in what
// an item resolves back to and whether several can be marked, and `dupl` would
// have flagged a second copy of the scroll and filter logic anyway.

package ui

import (
	"strconv"

	"github.com/WilsonSousajr/omatty/internal/fuzzy"
)

// pickItem is one row of a pick list. ID is what the caller resolves back to a
// domain value - a session id for the switcher, a repository root for the
// picker - so the widget itself holds no domain type and nothing untyped
// crosses into it.
type pickItem struct {
	ID     string
	Label  string
	Detail string
	Marked bool
}

// pickList is a filtered, scrollable list with its own cursor and query.
//
// matches holds the indices the query keeps, best first, recomputed on every
// keystroke so the cursor can never point at a row that is no longer shown.
type pickList struct {
	Items  []pickItem
	Query  string
	Cursor int
	Offset int
	// Multi lets several rows be marked before committing (#91). The switcher
	// jumps to one row, so it leaves this false.
	Multi   bool
	matches []int
	// hay is Label plus Detail per item, built once when the list opens rather
	// than per keystroke, so typing allocates nothing.
	hay []string
}

// newPickList builds a list over items, unfiltered and on its first row.
func newPickList(items []pickItem, multi bool) pickList {
	hay := make([]string, len(items))
	for i, it := range items {
		hay[i] = it.Label + " " + it.Detail
	}
	l := pickList{Items: items, Multi: multi, hay: hay}
	l.SetQuery("")
	return l
}

// SetQuery refilters and clamps the cursor. It does no scroll maths: the
// window follows on the next Move or Window call.
func (l *pickList) SetQuery(q string) {
	l.Query = q
	l.matches = fuzzy.Rank(q, l.hay)
	if l.Cursor >= len(l.matches) {
		l.Cursor = max(len(l.matches)-1, 0)
	}
}

// Move steps the cursor by delta within the matches, stopping at the ends
// rather than wrapping, and scrolls the window to follow.
func (l *pickList) Move(delta, rows int) {
	if len(l.matches) == 0 {
		return
	}
	l.Cursor = min(max(l.Cursor+delta, 0), len(l.matches)-1)
	l.Offset = ScrollOffset(l.Cursor, l.Offset, rows)
}

// Current is the item under the cursor, if the query matched anything.
func (l *pickList) Current() (pickItem, bool) {
	if l.Cursor >= len(l.matches) {
		return pickItem{}, false
	}
	return l.Items[l.matches[l.Cursor]], true
}

// ToggleMark marks or unmarks the row under the cursor. It does nothing on a
// single-select list, where the cursor already is the selection.
func (l *pickList) ToggleMark() {
	if !l.Multi || l.Cursor >= len(l.matches) {
		return
	}
	i := l.matches[l.Cursor]
	l.Items[i].Marked = !l.Items[i].Marked
}

// Chosen is what committing the list means: every marked item, or the one
// under the cursor when nothing is marked.
func (l *pickList) Chosen() []pickItem {
	marked := make([]pickItem, 0, len(l.Items))
	for _, it := range l.Items {
		if it.Marked {
			marked = append(marked, it)
		}
	}
	if len(marked) > 0 {
		return marked
	}
	if cur, ok := l.Current(); ok {
		return []pickItem{cur}
	}
	return nil
}

// Window is the visible slice of matches, rows high.
func (l *pickList) Window(rows int) []pickItem {
	l.Offset = ScrollOffset(l.Cursor, l.Offset, rows)
	end := min(l.Offset+rows, len(l.matches))
	if rows <= 0 || l.Offset >= end {
		return nil
	}
	shown := make([]pickItem, 0, end-l.Offset)
	for _, i := range l.matches[l.Offset:end] {
		shown = append(shown, l.Items[i])
	}
	return shown
}

// pickRows is how many rows the list shows: the pane, minus its query line and
// the count beneath it.
func (m *Model) pickRows() int {
	_, h := PaneSize(m.width, m.height, m.review.Open)
	return h - 3
}

// pickLines draws the query, the matches, and how many of them there are.
// The marker column is two cells wide whether or not the list is
// multi-select, so the labels do not shift as marks come and go.
func (m *Model) pickLines(title string) []string {
	l := &m.modal.List
	lines := []string{title + ": " + l.Query + "_", ""}
	for _, it := range l.Window(m.pickRows()) {
		lines = append(lines, pickRow(it, l))
	}
	return append(lines, "", strconv.Itoa(len(l.matches))+" of "+strconv.Itoa(len(l.Items)))
}

// pickRow draws one row: its marker, its label, and its detail.
func pickRow(it pickItem, l *pickList) string {
	marker := "  "
	if cur, ok := l.Current(); ok && cur.ID == it.ID {
		marker = "» "
	}
	mark := ""
	if it.Marked {
		mark = "* "
	}
	return marker + mark + it.Label + "  " + mutedStyle.Render(it.Detail)
}
