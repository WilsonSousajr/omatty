// The one notice every face of the review column draws (#761).
//
// A face says one of three things when it has no rows to show: it is loading,
// there is nothing, or something failed. The diff wrapped its failures here
// since #351; the tree, the gate and the tracker each drew their own, and the
// tree's and the gate's were one line cut at the column's edge - the part of
// an error that says what went wrong is its end. Every face's notice now goes
// through noticeLines, so no face can cut one again.

package app

import "github.com/WilsonSousajr/omatty/internal/tui/theme"

// noticeLines draws a notice wrapped to the column rather than cut at its
// edge: a cut line lost the part of an error that says what went wrong
// (#351), and git's stderr can be several lines of its own. An error is in
// the error colour; loading and empty states are muted.
func noticeLines(lines []string, isErr bool, w int) []string {
	style := theme.Muted
	if isErr {
		style = theme.Error
	}
	var out []string
	for _, l := range lines {
		for _, part := range wrapBlock(l, w) {
			out = append(out, style.Render(fitLine(part, w)))
		}
	}
	return out
}
