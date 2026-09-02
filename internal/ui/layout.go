package ui

// SidebarWidth is the sidebar box's outer width, borders included. Fixed for
// M1; the terminal takes whatever remains.
const SidebarWidth = 28

// footerRows is the keymap line below both panes.
const footerRows = 1

// DefaultWidth and DefaultHeight are the size assumed before the terminal
// reports its own, and the fallback cmd uses when it cannot query one.
const (
	DefaultWidth  = 80
	DefaultHeight = 24
)

// Floors so a tiny window still renders something rather than a negative size.
const (
	minTermWidth  = 20
	minTermHeight = 4
)

// PaneSize returns the terminal's content size for a window. Each box spends
// one column and one row on each side for its border, so the terminal's
// content is the window minus the sidebar, minus its own two border columns;
// its rows are the window minus the footer and two border rows (issue #35).
//
//	w, h := ui.PaneSize(120, 40) // 90, 37
func PaneSize(width, height int) (termW, termH int) {
	termW = width - SidebarWidth - 2
	termH = height - footerRows - 2
	if termW < minTermWidth {
		termW = minTermWidth
	}
	if termH < minTermHeight {
		termH = minTermHeight
	}
	return termW, termH
}

// PTYSize is the embedded terminal's size for a window: the pane's content
// minus the title row the pane draws above it. It is the one place the PTY
// dimensions are derived, for birth and for every resize, so the two can
// never drift (issues #51, #75).
//
//	w, h := ui.PTYSize(120, 40) // 90, 36
func PTYSize(width, height int) (w, h int) {
	w, h = PaneSize(width, height)
	return w, h - 1
}
