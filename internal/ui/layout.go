package ui

// SidebarWidth is the sidebar box's outer width, borders included. Fixed for
// M1; the terminal takes whatever remains.
const SidebarWidth = 28

// footerRows is the keymap line below both panes.
const footerRows = 1

// headerRows is the breadcrumb row across the top of the window and ruleRows
// the rule beneath it (#174). Together they replaced the pane box's two
// border rows, so the pane keeps exactly the rows it had; the two border
// columns went to the pane. Every derivation of the pane's geometry goes
// through them, so the caret, the wheel target, the click hit-test and the
// emulator's size move together (#106, #107, #45).
const (
	headerRows = 1
	ruleRows   = 1
	// titleRows is 0: the pane's title is drawn in the header row rather
	// than on a row of its own (#128, #174). The constant stays because
	// PaneOrigin and PTYSize derive from it and #106 and #107 both measure
	// through it - a title row, if one returns, moves the caret, the wheel
	// target and the emulator's height together.
	titleRows = 0
)

// sidebarContentCols is what the sidebar draws in: its outer width minus the
// hairline on its right, which belongs to no column (#174).
const sidebarContentCols = SidebarWidth - 1

// sidebarHeaderRows is the pinned "projects" line renderSidebar draws above
// the scrolling rows (#129). Named here so the renderer that applies it and
// the click hit-test that undoes it (#45) cannot drift, and so moving the
// header into the box's top rule (#128) is one constant.
const sidebarHeaderRows = 0 // the "projects" line moved into the rule (#128), then the header row (#174)

// sidebarTop is the window row the first sidebar row is drawn at: under the
// header row and the rule, the exact inverse of what View prepends (#45).
func sidebarTop() int { return headerRows + ruleRows + sidebarHeaderRows }

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

// ReviewWidth is the review column's outer width, its left hairline included,
// for a window; 0 while the column is closed.
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

// reviewContentWidth is the review column's content: its outer width minus
// its own left hairline. Named once because the renderer and the pan clamp
// both need it and a literal in each drifted before (#94, #174).
func reviewContentWidth(width int) int { return ReviewWidth(width, true) - 1 }

// PaneSize returns the terminal's content size for a window. No box spends
// columns any more (#174): the window minus the sidebar, minus the review
// column when it is open, is the pane; its rows are the window minus the
// header row, the rule and the footer (issues #35, #21).
//
//	w, h := ui.PaneSize(120, 40, false) // 92, 37
func PaneSize(width, height int, reviewOpen bool) (termW, termH int) {
	termW = width - SidebarWidth - ReviewWidth(width, reviewOpen)
	termH = height - headerRows - ruleRows - footerRows
	if termW < minTermWidth {
		termW = minTermWidth
	}
	if termH < minTermHeight {
		termH = minTermHeight
	}
	return termW, termH
}

// PaneOrigin is the window cell the embedded terminal's top-left cell is
// drawn at: past the sidebar and its hairline, and below the header row, the
// rule and the title row (#174). Cursor placement (#106) and mouse
// translation (#107) both need it, so it is derived here once.
//
//	x, y := ui.PaneOrigin() // 28, 2
func PaneOrigin() (x, y int) {
	return SidebarWidth, headerRows + ruleRows + titleRows
}

// inPaneGrid reports whether a cell of the embedded terminal's own grid is
// one the pane actually draws. fitBlock cuts everything past it, so both the
// cursor omatty places (#106) and the wheel it forwards (#107) must stay
// inside, and a narrowed pane shrinks the target with it.
//
// The bound is PTYSize, not PaneSize: the pane's last row belongs to the title
// (#106). Switching it to PaneSize would put the caret a row outside the grid
// again, which is the bug this replaced.
func (m *Model) inPaneGrid(x, y int) bool {
	w, h := PTYSize(m.width, m.height, m.review.Open)
	return x >= 0 && x < w && y >= 0 && y < h
}

// PTYSize is the embedded terminal's size for a window: the pane's content
// minus the title row the pane draws above it. It is the one place the PTY
// dimensions are derived, for birth and for every resize, so the two can
// never drift (issues #51, #75).
//
//	w, h := ui.PTYSize(120, 40, false) // 92, 37
func PTYSize(width, height int, reviewOpen bool) (w, h int) {
	w, h = PaneSize(width, height, reviewOpen)
	return w, h - titleRows
}
