package ui

import (
	"fmt"

	"github.com/WilsonSousajr/omatty/internal/coverage"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// uncoveredMark is what an added line no test covers draws instead of its +.
const uncoveredMark = "!"

// uncovered reports whether e is an added line the overlay has a verdict for
// and that verdict is "never ran".
//
// Three states, and the third is the one that earns the check its shape: a
// line the profile does not mention has no verdict at all - it is a
// declaration, a brace, a comment - and silence is never a claim. Only added
// lines are asked about: a context line's coverage is not this change's
// business, and a removed line is not in the tree the profile describes.
func (m *Model) uncovered(e review.Entry) bool {
	if e.Kind != review.EntryLine {
		return false
	}
	line := m.shownDiff().LineAt(e.Pos)
	if line.Kind != review.LineAdded {
		return false
	}
	path := m.shownDiff().Files[e.Pos.File].Path
	// A generated file has no test and never will, so an uncovered marker on
	// it says something true about the file and nothing at all about the
	// change - it only makes the file look worse than it is (#338).
	if m.isGenerated(path) {
		return false
	}
	covered, known := m.overlay().Files[path].Lines[line.NewNo]
	return known && !covered
}

// overlay is the coverage the session under review last loaded, or the zero
// profile - which answers "no verdict" for every line, so no caller needs a
// nil check.
func (m *Model) overlay() coverage.Profile {
	return m.covers[m.review.SessionID]
}

// uncoveredNote counts what the markers under this header will say, and says
// nothing at all when there is nothing to report - a note on every file would
// be decoration rather than a finding.
func (m *Model) uncoveredNote(fi int) string {
	if m.isGenerated(m.shownDiff().Files[fi].Path) {
		return "" // #338, the same argument uncovered makes
	}
	lines := m.overlay().Files[m.shownDiff().Files[fi].Path].Lines
	if len(lines) == 0 {
		return ""
	}
	n := 0
	for _, h := range m.shownDiff().Files[fi].Hunks {
		n += uncoveredInHunk(h, lines)
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("  %d uncovered", n)
}

// uncoveredInHunk is how many of a hunk's added lines the overlay says never
// ran, counted the same way linePrefix marks them.
func uncoveredInHunk(h review.Hunk, lines map[int]bool) int {
	n := 0
	for _, l := range h.Lines {
		if covered, known := lines[l.NewNo]; l.Kind == review.LineAdded && known && !covered {
			n++
		}
	}
	return n
}
