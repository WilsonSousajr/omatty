// One list window for the review column's cursor faces (#424).
//
// The diff, the tree and the tracker each kept their own cursor and offset and
// their own copy of the same two lines of arithmetic - clamp, then scroll as
// little as keeps the cursor on screen. They agreed by accident; the gate did
// not, and could not scroll at all (#421). listWindow is those two lines once,
// so the faces move alike because they share the code that moves them.

package ui

import "fmt"

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

// pageAll is a delta past either end of any list: g and G are "as far as it
// goes", which every mover already clamps.
const pageAll = 1 << 20

// pageDelta turns the column's paging keys into a move, the same on every face:
// g and G to the ends, ctrl+d and ctrl+u by half the window - the keys a pager
// or an editor has taught every terminal reader (#424).
func (m *Model) pageDelta(key string) (int, bool) {
	switch key {
	case "g":
		return -pageAll, true
	case "G", "shift+g", "shift+G":
		return pageAll, true
	case "ctrl+d":
		return max(m.reviewRows()/2, 1), true
	case "ctrl+u":
		return -max(m.reviewRows()/2, 1), true
	}
	return 0, false
}

// pageKey runs a paging key on whichever face is on show, and reports whether
// key was one - checked before the face's own keymap, so no face can shadow it.
func (m *Model) pageKey(key string) bool {
	delta, ok := m.pageDelta(key)
	if ok {
		m.moveFace(delta)
	}
	return ok
}

// moveFace moves the face on show by delta rows, through the same clamps j and
// k use there: j/k, the paging keys and a click all come through here.
func (m *Model) moveFace(delta int) {
	switch m.review.View {
	case ViewTree:
		m.moveTreeCursor(delta)
	case ViewPreview:
		m.scrollPreview(delta)
	case ViewGate:
		m.pageGate(delta)
	case ViewTracker:
		m.moveTrackerCursor(delta)
	case ViewTrackerItem:
		m.scrollItem(delta)
	default:
		m.moveReviewCursor(delta)
	}
}

// scrollPreview moves the preview's window, which has no cursor of its own.
func (m *Model) scrollPreview(delta int) {
	last := previewLast(m.review.Preview, m.reviewRows())
	m.review.PreviewOffset = min(max(m.review.PreviewOffset+delta, 0), last)
}

// facePosition is where the face stands: the cursor's row of how many on a
// cursor face, and the last line in view of how many on a face that only
// scrolls, so reaching the end reads N/N on both.
func (m *Model) facePosition() (n, total int) {
	switch m.review.View {
	case ViewTree:
		return m.review.Files.Cursor + 1, len(m.treeRows())
	case ViewPreview:
		total = len(m.review.Preview.Lines)
		return min(m.review.PreviewOffset+m.reviewRows(), total), total
	case ViewGate:
		return m.gatePosition()
	case ViewTracker:
		return m.review.Tracker.Cursor + 1, len(m.trackerRows())
	case ViewTrackerItem:
		total = len(m.itemLines())
		return min(m.review.Tracker.ItemOffset+m.reviewRows(), total), total
	}
	return m.review.DiffList.Cursor + 1, len(m.review.Entries)
}

// positionMark is " · N/M" for the title, or "" when the face shows no rows.
func (m *Model) positionMark() string {
	n, total := m.facePosition()
	if total == 0 {
		return ""
	}
	return fmt.Sprintf(" · %d/%d", n, total)
}
