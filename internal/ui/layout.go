package ui

// SidebarWidth is the sidebar box's outer width, borders included. Fixed for
// M1; the terminal takes whatever remains.
const SidebarWidth = 28

// footerRows is the keymap line below both panes.
const footerRows = 1

// The size assumed before the terminal reports its own.
const (
	defaultWidth  = 80
	defaultHeight = 24
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
