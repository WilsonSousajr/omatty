// A project's open counts on its header row (#395): "13i 2p" right-aligned in
// the one sidebar line that did nothing but name the project.
//
// The first question the tracker answers - how many issues and pull requests
// are open - should not need a keypress, and the sidebar already has a row per
// project. Derived at render time from the two polls, never stored (invariant
// 9), the way a card's pull request is (#310).

package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// nameFloor is the least a header keeps for its project's name: fifteen cells,
// which is what #155 settled still identifies a session, and a project name is
// no different. The counts come off before the name does - a count is worth
// reading, and a header that has given its row to counts is not a header any
// more.
const nameFloor = 15

// forgeCounts is a project's open counts as its header draws them - "13i 2p" -
// or empty when there is nothing known to draw. Without gh, for a checkout gh
// cannot map to GitHub, or before the first poll has answered, the header is
// exactly what it was before this feature: an unknown count is never drawn as
// zero, the rule a card's "?" follows (Orca #18484).
func (m *Model) forgeCounts(project string) string {
	if m.ghMissing || m.notGitHub[project] {
		return ""
	}
	parts := make([]string, 0, 2)
	if issues, polled := m.issues[project]; polled {
		parts = append(parts, strconv.Itoa(len(issues))+"i")
	}
	if n, polled := m.openPRCount(project); polled {
		parts = append(parts, strconv.Itoa(n)+"p")
	}
	return fitCounts(parts)
}

// openPRCount is how many of a project's pull requests are open, and whether it
// has been asked at all. Only the open ones: the map holds the finished ones
// too, for the card that says "merged" (#310), and a header counting those
// would answer a question nobody asked.
func (m *Model) openPRCount(project string) (int, bool) {
	prs, polled := m.prs[project]
	if !polled {
		return 0, false
	}
	open := 0
	for _, pr := range prs {
		if pr.State == forge.Open {
			open++
		}
	}
	return open, true
}

// fitCounts drops the pull request half, and then the counts entirely, rather
// than let them take the row the project's name needs.
func fitCounts(parts []string) string {
	for len(parts) > 0 {
		s := strings.Join(parts, " ")
		if cardCols-1-1-lipgloss.Width(s) >= nameFloor {
			return s
		}
		parts = parts[:len(parts)-1]
	}
	return ""
}
