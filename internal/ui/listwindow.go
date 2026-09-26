// One list window for the review column's cursor faces (#424).
//
// The diff, the tree and the tracker each kept their own cursor and offset and
// their own copy of the same two lines of arithmetic - clamp, then scroll as
// little as keeps the cursor on screen. They agreed by accident; the gate did
// not, and could not scroll at all (#421). listWindow is those two lines once,
// so the faces move alike because they share the code that moves them.

package ui

// listWindow is a cursor over a list and the first row drawn: the shape every
// cursor face of the review column scrolls with.
//
//	m.review.DiffList.move(1, len(m.review.Entries), m.reviewRows())
type listWindow struct {
	Cursor int
	Offset int // the first row drawn
}

// move puts the cursor delta rows on, clamped to the n rows there are, and
// scrolls the window as little as keeps it on screen. An empty list parks
// both at the top, so rows arriving later start from there.
func (l *listWindow) move(delta, n, rows int) {
	if n == 0 {
		*l = listWindow{}
		return
	}
	l.Cursor = min(max(l.Cursor+delta, 0), n-1)
	l.Offset = ScrollOffset(l.Cursor, l.Offset, rows)
}
