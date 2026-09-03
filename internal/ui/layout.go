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

// The review column takes reviewNum/reviewDen of the width left after the
// sidebar: two fifths keeps about 40 columns of claude at 100 wide (#21).
const (
	reviewNum      = 2
	reviewDen      = 5
	minReviewWidth = 24
)

// ReviewWidth is the review column's outer width, borders included, for a
// window; 0 while the column is closed.
//
//	ui.ReviewWidth(100, true) // 28
func ReviewWidth(width int, open bool) int {
	if !open {
		return 0
	}
	w := (width - SidebarWidth) * reviewNum / reviewDen
	if w < minReviewWidth {
		w = minReviewWidth
	}
	return w
}

// PaneSize returns the terminal's content size for a window. Each box spends
// one column and one row on each side for its border, so the terminal's
// content is the window minus the sidebar, minus the review column when it is
// open, minus its own two border columns; its rows are the window minus the
// footer and two border rows (issues #35, #21).
//
//	w, h := ui.PaneSize(120, 40, false) // 90, 37
func PaneSize(width, height int, reviewOpen bool) (termW, termH int) {
	termW = width - SidebarWidth - ReviewWidth(width, reviewOpen) - 2
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
//	w, h := ui.PTYSize(120, 40, false) // 90, 36
func PTYSize(width, height int, reviewOpen bool) (w, h int) {
	w, h = PaneSize(width, height, reviewOpen)
	return w, h - 1
}
