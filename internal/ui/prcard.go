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
// branch name can be reused. A fork's pull request is never it - its branch
// only shares a name. A finished one counts only on a worktree session whose
// HEAD is its head commit: otherwise it was an earlier use of the same slug,
// and a main checkout sitting on develop is not the promotion PR that merged
// develop into main.
func (m *Model) prFor(sess registry.Session) (forge.PR, bool) {
	st := m.repoStat[sess.ID]
	if st.Branch == "" {
		return forge.PR{}, false
	}
	var best forge.PR
	found := false
	for _, pr := range m.prs[sess.Project] {
		if pr.Branch != st.Branch || !isThisWork(pr, sess, st.Head) {
			continue
		}
		if !found || pr.Number > best.Number {
			best, found = pr, true
		}
	}
	return best, found
}

// isThisWork rules out what only looks like the session's pull request: a
// fork's branch of the same name, and a finished one on another commit.
func isThisWork(pr forge.PR, sess registry.Session, head string) bool {
	if pr.Fork {
		return false
	}
	if pr.State == forge.Open {
		return true
	}
	return sess.Worktree && head != "" && pr.Head == head
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
