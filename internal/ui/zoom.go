// Zooming the review column (#427).
//
// The column is (width-28)*2/5 - 23 cells of content at 80 columns. That is
// the right default beside a live session and too narrow to read a long diff,
// a tracker item or a gate's failing output. ctrl+o z gives it the session
// pane's width until ctrl+o z again. The session keeps running at its own size
// underneath: a PTY resized for a view it is not drawn in would make claude
// reflow for nothing.

package ui

// zoomed reports whether the column is drawn over the pane now. Only while it
// has the keys: esc hands them to claude, and a session taking keystrokes
// while the zoom hid it would be typed into blind, so the split comes back.
func (m *Model) zoomed() bool {
	return m.review.Open && m.review.Focused && m.review.Zoomed
}

// toggleZoom is ctrl+o z. It gives the column the keys as it zooms, since the
// zoom shows only while the column has them; with no column open it says what
// would open one rather than doing nothing where nobody can see why.
func (m *Model) toggleZoom() {
	if !m.review.Open {
		m.lastErr = "nothing to zoom - open a face first: ctrl+o d, f, g or i"
		return
	}
	m.review.Zoomed = !m.zoomed()
	m.review.Focused = true
}
