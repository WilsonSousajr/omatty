package ui

// SidebarWidth is the sidebar box's outer width, borders included. Fixed for
// M1; the terminal takes whatever remains.
const SidebarWidth = 28

// footerRows is the keymap line below both panes.
const footerRows = 1

// borderRows and titleRows are the pane box's top border and the title line
// renderTerminal draws above the embedded terminal. Together they are how far
// down the window the emulator's first row lands.
const (
	borderRows = 1
	titleRows  = 1
)

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

// PaneOrigin is the window cell the embedded terminal's top-left cell is
// drawn at: past the sidebar box and the pane box's left border, and below
// that box's top border and its title row. Cursor placement (#106) and mouse
// translation (#107) both need it, so it is derived here once.
//
//	x, y := ui.PaneOrigin() // 29, 2
func PaneOrigin() (x, y int) {
	return SidebarWidth + 1, borderRows + titleRows
}

// inPaneGrid reports whether a cell of the embedded terminal's own grid is
// one the pane actually draws. fitBlock cuts everything past it, so both the
// cursor omatty places (#106) and the wheel it forwards (#107) must stay
// inside, and a narrowed pane shrinks the target with it.
func (m *Model) inPaneGrid(x, y int) bool {
	w, h := PTYSize(m.width, m.height, m.review.Open)
	return x >= 0 && x < w && y >= 0 && y < h
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
