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
	// Title labels the query line, so the same widget can say what it is for.
	Title  string
	Items  []pickItem
	Query  string
	Cursor int
	Offset int
	// Multi lets several rows be marked before committing (#91). The switcher
	// jumps to one row, so it leaves this false.
	Multi   bool
	matches []int
	// hay is Label plus Detail per item, built once when the list opens rather
	// than per keystroke. Ranking still allocates: fuzzy.Match converts both
	// sides to runes per call, so a keystroke costs O(len(Items)) allocations.
	hay []string
}

// newPickList builds a list over items, unfiltered and on its first row.
func newPickList(title string, items []pickItem, multi bool) pickList {
	hay := make([]string, len(items))
	for i, it := range items {
		hay[i] = it.Label + " " + it.Detail
	}
	l := pickList{Title: title, Items: items, Multi: multi, hay: hay}
	l.refilter()
	return l
}

// SetQuery refilters when the query actually changed. It does no scroll maths:
// the window follows on the next Window call.
//
// An unchanged query is a no-op rather than a re-rank, so the navigation keys
// that carry no text cost nothing (#42).
func (l *pickList) SetQuery(q string) {
	if q == l.Query {
		return
	}
	l.Query = q
	l.refilter()
}

// refilter recomputes the matches and returns the cursor to the best of them.
//
// The cursor goes back to the top rather than being clamped in place: a new
// query reorders the matches, so an index kept across the change points at
// whichever row happens to land there. Clamping it left the highlight on the
// worst match after any cursor movement, and enter jumped to a session the
// operator never picked (#42).
func (l *pickList) refilter() {
	l.matches = fuzzy.Rank(l.Query, l.hay)
	l.Cursor, l.Offset = 0, 0
}

// Move steps the cursor by delta within the matches, stopping at the ends
// rather than wrapping. Window does the scroll maths, the way renderEntries
// does it for the review column.
func (l *pickList) Move(delta int) {
	if len(l.matches) == 0 {
		return
	}
	l.Cursor = min(max(l.Cursor+delta, 0), len(l.matches)-1)
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

// markedCount is how many rows are marked, for the footer.
func (l *pickList) markedCount() int {
	n := 0
	for _, it := range l.Items {
		if it.Marked {
			n++
		}
	}
	return n
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

// pickChrome is how many lines pickLines spends on anything but a row: the
// query line, the blank under it, the blank above the count, and the count.
const pickChrome = 4

// pickRows is how many rows the list shows: the pane, minus its chrome.
//
// Counting three left pickLines one line taller than the pane, so fitBlock cut
// the count line off the bottom and "N of M" was invisible on exactly the full
// lists that needed it (#42).
func (m *Model) pickRows() int {
	_, h := PaneSize(m.width, m.height, m.review.Open)
	return h - pickChrome
}

// pickLines draws the query, the matches, and how many of them there are.
func (m *Model) pickLines() []string {
	l := &m.modal.List
	lines := []string{l.Title + ": " + l.Query + "_", ""}
	for _, it := range l.Window(m.pickRows()) {
		lines = append(lines, pickRow(it, l))
	}
	return append(lines, "", strconv.Itoa(len(l.matches))+" of "+strconv.Itoa(len(l.Items)))
}

// pickRow draws one row: its cursor marker, its mark, its label, its detail.
//
// The mark column is two cells on a multi-select list whether or not this row
// is marked, so labels do not jog sideways as marks come and go; a
// single-select list has no marks and spends no cells on them (#42).
func pickRow(it pickItem, l *pickList) string {
	marker := "  "
	if cur, ok := l.Current(); ok && cur.ID == it.ID {
		marker = "» "
	}
	mark := ""
	if l.Multi {
		mark = "  "
		if it.Marked {
			mark = "* "
		}
	}
	return marker + mark + it.Label + "  " + mutedStyle.Render(it.Detail)
}
