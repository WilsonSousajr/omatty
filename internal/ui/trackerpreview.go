// The tracker's preview (#434): zoomed and wide, the list and the item under
// its cursor side by side, the way gh-dash's preview sits beside its list.
// Below the width, or unzoomed, the tracker is the list alone and enter opens
// an item as it always has.

package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// previewMinWidth is the column the preview needs: under it, a list and an
// item side by side are two columns too narrow to read.
const previewMinWidth = 100

// previewRest is how long the cursor must rest on a row before its item is
// read: a burst of j passes rows, and reading each one it passed is the
// per-item call inside a list that tripped GitHub's rate limit (#358).
const previewRest = 250 * time.Millisecond

// previewSeam is what divides the list from the preview.
const previewSeam = " │ "

// previewRestMsg says the cursor was put on key previewRest ago.
type previewRestMsg struct{ key itemKey }

// previewing reports whether the tracker is drawn with its preview.
func (m *Model) previewing() bool {
	return m.review.View == ViewTracker && m.zoomed() && m.columnWidth() >= previewMinWidth
}

// trackerListWidth is the list's share of the column: all of it, or a little
// over half beside the preview (gh-dash gives its preview 45%).
func (m *Model) trackerListWidth() int {
	if m.previewing() {
		return m.columnWidth() * 55 / 100
	}
	return m.columnWidth()
}

// panWidth is the width h and l pan against: the list's while it shares the
// column, so the clamp reaches the end of a title in the narrower list.
func (m *Model) panWidth() int {
	if m.review.View == ViewTracker {
		return m.trackerListWidth()
	}
	return m.columnWidth()
}

// cursorItem is the item under the list's cursor, false on a heading.
func (m *Model) cursorItem() (itemKey, bool) {
	rows := m.trackerRows()
	c := m.review.Tracker.Cursor
	if c >= len(rows) || rows[c].Kind == rowRule {
		return itemKey{}, false
	}
	return itemKey{Project: m.review.Tracker.Project, PR: rows[c].Kind == rowPR, Number: rows[c].Number}, true
}

// withPreview lays the item under the cursor beside the list, row by row, or
// gives the list back unchanged when there is no preview.
func (m *Model) withPreview(list []string, h int) []string {
	if !m.previewing() {
		return list
	}
	lw := m.trackerListWidth()
	pw := max(m.columnWidth()-lw-len([]rune(previewSeam)), 1)
	var item []string
	if key, ok := m.cursorItem(); ok {
		item = m.itemLinesFor(key, pw)
	}
	out := make([]string, h)
	for i := range out {
		out[i] = fitLine(lineAt(list, i), lw) + mutedStyle.Render(previewSeam) + fitLine(lineAt(item, i), pw)
	}
	return out
}

// lineAt is lines[i], or "" past the end.
func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return ""
}

// armPreview asks to be told once the cursor has rested on an item not yet
// read. Called after every message, as armSpin is, so it catches the cursor
// however it moved - a key, a click, the zoom, a list arriving - and only once
// per row it lands on.
func (m *Model) armPreview() tea.Cmd {
	key, ok := m.cursorItem()
	if !ok || !m.previewing() || m.previewArmed == key {
		return nil
	}
	if _, held := m.items[key]; held || m.itemPending[key] {
		return nil
	}
	m.previewArmed = key
	return m.spinTick(previewRest, func(time.Time) tea.Msg { return previewRestMsg{key} })
}

// onPreviewRest reads the item if the cursor is still on it; if it moved on,
// armPreview has already asked about the row it moved to.
func (m *Model) onPreviewRest(msg previewRestMsg) tea.Cmd {
	if m.previewArmed == msg.key {
		m.previewArmed = itemKey{}
	}
	if key, ok := m.cursorItem(); !ok || key != msg.key || !m.previewing() {
		return nil
	}
	return m.readItem(msg.key, false)
}
