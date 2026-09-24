// A session's pull request on its card (#310): line two's branch becomes the
// pull request and one CI mark once there is one. Derived at render time from
// the project's last poll and the session's branch - never stored (invariant 9).

package ui

import (
	"strconv"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// prFor is the pull request a session's branch has: the newest one, since a
// branch name can be reused. A main-checkout session takes open ones only -
// the checkout sitting on develop is not the promotion PR that merged develop
// into main, and saying "merged" there would be noise.
func (m *Model) prFor(sess registry.Session) (forge.PR, bool) {
	branch := m.repoStat[sess.ID].Branch
	if branch == "" {
		return forge.PR{}, false
	}
	var best forge.PR
	found := false
	for _, pr := range m.prs[sess.Project] {
		if pr.Branch != branch || (!sess.Worktree && pr.State != forge.Open) {
			continue
		}
		if !found || pr.Number > best.Number {
			best, found = pr, true
		}
	}
	return best, found
}

// prLabel is "#349" and what to know about it: "?" when the project's last
// poll failed - unknown is never shown as the old verdict (Orca #18484) -
// "merged" or "closed", or the CI mark. Uncoloured, as the gate strip is.
func (m *Model) prLabel(pr forge.PR, project string) string {
	label := "#" + strconv.Itoa(pr.Number)
	switch {
	case m.prFailed[project]:
		return label + " ?"
	case pr.State == forge.Merged:
		return label + " merged"
	case pr.State == forge.Closed:
		return label + " closed"
	}
	if mark := ciMark(pr); mark != "" {
		return label + " " + mark
	}
	return label
}

// ciMark is one cell by precedence: failing, then conflict or behind, then
// running, then passing; nothing when the pull request has no checks.
func ciMark(pr forge.PR) string {
	switch {
	case pr.CI == forge.CIFailing:
		return "✗"
	case pr.Conflict:
		return "⚠"
	case pr.CI == forge.CIRunning:
		return "◍"
	case pr.CI == forge.CIPassing:
		return "✓"
	}
	return ""
}
