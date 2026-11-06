package theme

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

// CursorGround is the cursor row's background on every face of the review
// column: a dark grey ground under the row's own colours (#763). 237 rather
// than a hue, because every hue already means something here - green added,
// red removed, amber pending - and the cursor means none of those. Exported
// so a test can find the cursor row by the sequence that draws it.
//
//	strings.Contains(line, theme.CursorGround) // the cursor row
const CursorGround = "\x1b[48;5;237m"

// sgrReset is a sequence that clears the background: a full reset, either
// spelling, or the default-background code.
var sgrReset = regexp.MustCompile(`\x1b\[(?:0?|49)m`)

// UnderCursor lays the cursor's ground under an already-styled row and pads
// it to width, keeping every foreground the row carries.
//
// Reverse video swapped them all away, so the row being read was the one row
// with no syntax and no changed words (#763). A lipgloss background wrapped
// around the row would be cleared by the first reset inside it, so the ground
// is put back after each one; a span with a background of its own - a
// changed word's - keeps it until its reset.
//
//	row := theme.UnderCursor(styled, 40)
func UnderCursor(styled string, width int) string {
	grounded := sgrReset.ReplaceAllStringFunc(styled, func(reset string) string { return reset + CursorGround })
	pad := strings.Repeat(" ", max(width-lipgloss.Width(styled), 0))
	return CursorGround + grounded + pad + "\x1b[m"
}
