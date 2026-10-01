// Shipping a green session: push it, open its pull request, or merge one that is
// already green (#331).
//
// The boundary this sits inside is argued in `docs/ROADMAP.md`'s shipping
// section, and it is not "does it touch the forge" - it is **does omatty act
// while nobody is reading**. One keypress, one session, every time, after a
// person has read the verdict. The same shape as S sending the gate's failures
// back.
//
// Never merging when checks *go* green - that is a row in the orchestrator table
// and it belongs on the forge, which has auto-merge. Never a force-push, never a
// branch deletion, and never into a protected branch.

package app

import (
	dsession "github.com/WilsonSousajr/omatty/internal/domain/session"
	"log/slog"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/service/review"
)

// ShippedMsg carries the outcome of a ship into Update. Exported so tests can
// send one.
type ShippedMsg struct {
	SessionID string
	// Number is the pull request opened or merged, and Merged says which of the
	// two happened, so the notice can name what the operator got.
	Number int
	Merged bool
	// Queued is a merge the forge accepted and has not finished - Azure
	// completes asynchronously - so the notice does not say merged (#464).
	Queued bool
	Err    error
}

// shipSelected is ctrl+o p: push and open the pull request, or merge one whose
// local and remote verdicts are both already green.
//
// Every refusal names which of the two verdicts is missing, because that is the
// question the operator is actually asking when the key appears to do nothing.
func (m *Model) shipSelected() tea.Cmd {
	row, ok := m.sidebar.Selected()
	if !ok {
		return nil
	}
	sess := *row.Session
	if pr, found := m.prFor(sess); found && pr.State == forge.Open {
		return m.mergeIfGreen(sess, pr)
	}
	return m.openPullRequest(sess)
}

// openPullRequest is step 1: push the branch and open the pull request.
//
// The gate verdict and the base branch are checked here, where the answer is
// already in memory. Whether the worktree is clean is read in the command,
// because it costs two git calls and is only ever wanted at this moment.
func (m *Model) openPullRequest(sess dsession.Session) tea.Cmd {
	if !m.gateGreen(sess.ID) || !m.readyToShip(sess.ID) {
		m.lastErr = "the gate is not green for " + sess.Title + "; run it with " + m.leader + " g first"
		return nil
	}
	// The session's own Base rather than a project setting: it is what this work
	// actually diverged from, and it is already persisted (#21). A main-checkout
	// session has none, and there is nothing to guess - nobody opened a branch
	// for this work.
	if sess.Base == "" || sess.Branch == "" {
		m.lastErr = sess.Title + " is not on a worktree branch, so there is nothing to open a " + m.label(sess.Project).Change + " from"
		return nil
	}
	return m.pushAndOpen(sess)
}

// pushAndOpen reads the worktree, then pushes and opens, all off the Update
// goroutine - the way browseItem does it, because git and gh both spawn
// processes and a slow one must not hold the frame.
//
// The dirty-worktree refusal is the interesting one. The gate verified a working
// tree and a push moves commits, so shipping uncommitted work would open a pull
// request that differs from what was verified. omatty refuses and says to commit
// it in the session, rather than authoring a commit with a message it guessed:
// the same line as "omatty never submits a turn on your behalf".
func (m *Model) pushAndOpen(sess dsession.Session) tea.Cmd {
	ship, push, create := m.ship.Shippable, m.ship.Push, m.ship.CreatePR
	root, id := m.projectRoot(sess.Project), sess.ID
	return func() tea.Msg {
		state, err := ship(sess, root)
		if err != nil {
			return ShippedMsg{SessionID: id, Err: err}
		}
		if reason := notPushable(sess, state); reason != "" {
			return ShippedMsg{SessionID: id, Err: errShip(reason)}
		}
		if err := push(sess.Dir, sess.Branch); err != nil {
			return ShippedMsg{SessionID: id, Err: err}
		}
		number, err := create(root, sess.Branch, sess.Base, sess.Title)
		return ShippedMsg{SessionID: id, Number: number, Err: err}
	}
}

// notPushable names why a worktree cannot be shipped, or "" when it can.
func notPushable(sess dsession.Session, state review.Shippable) string {
	switch {
	case state.Uncommitted > 0:
		return sess.Title + " has " + strconv.Itoa(state.Uncommitted) +
			" uncommitted file(s); commit them in the session first"
	case state.Commits == 0:
		return sess.Title + " has no commit that " + sess.Base + " does not"
	}
	return ""
}

// mergeIfGreen is step 2: merge a pull request whose local *and* remote verdicts
// are already green, and otherwise say which is missing.
//
// Five reasons not to, each its own sentence. The protected-branch check is
// last because it costs a gh call, and it is the one that fails closed.
func (m *Model) mergeIfGreen(sess dsession.Session, pr forge.PR) tea.Cmd {
	reason := notMergeable(m.gateGreen(sess.ID) && m.readyToShip(sess.ID), pr, onForge(m.label(sess.Project)))
	if reason == "" {
		reason = aimedElsewhere(pr, sess.Base)
	}
	if reason != "" {
		m.lastErr = "cannot merge " + m.changeRef(sess.Project, pr.Number) + ": " + reason
		return nil
	}
	return m.mergeUnlessProtected(sess, pr)
}

// aimedElsewhere names a pull request that merges into another branch than the
// one the session was forked from, or "" when it does not. One Claude opened
// against main from a session on develop had develop's protection read and was
// merged into main (#598).
func aimedElsewhere(pr forge.PR, base string) string {
	if pr.Base == "" || base == "" || pr.Base == base {
		return ""
	}
	return "it merges into " + pr.Base + ", not " + base + ", the branch this session was forked from"
}

// notMergeable names why a pull request must not be merged now, or "" when it
// may be; where is the forge it is on, "on GitLab" (#464).
func notMergeable(gateGreen bool, pr forge.PR, where string) string {
	switch {
	case !gateGreen:
		return "the local gate is not green"
	case pr.CI != forge.CIPassing:
		return "its checks are not passing " + where
	case pr.Conflict:
		return "it cannot merge as it stands"
	case pr.Draft:
		return "it is a draft"
	}
	return ""
}

// mergeUnlessProtected asks the forge whether the base is protected, then
// merges. Both calls are off the Update goroutine.
func (m *Model) mergeUnlessProtected(sess dsession.Session, pr forge.PR) tea.Cmd {
	protected, merge := m.ship.BranchProtected, m.ship.MergePR
	root, id, number, base, head := m.projectRoot(sess.Project), sess.ID, pr.Number, sess.Base, pr.Head
	if pr.Base != "" {
		base = pr.Base // where it would merge, as the forge says (#598)
	}
	label := m.label(sess.Project)
	return func() tea.Msg {
		if isProtected, err := protected(root, base); err != nil || isProtected {
			return ShippedMsg{SessionID: id, Number: number, Err: protectedRefusal(base, label, err)}
		}
		// The head the card showed green: a push since is not merged (#599).
		merged, err := merge(root, number, head)
		return ShippedMsg{SessionID: id, Number: number, Merged: merged, Queued: !merged && err == nil, Err: err}
	}
}

// protectedRefusal explains a merge omatty will not make.
//
// A protection flag it could not read refuses too, and says so differently: the
// operator can merge on the forge, and being told why is more use than being
// told no.
func protectedRefusal(base string, l forge.Label, err error) error {
	if err != nil {
		return errShip("cannot tell whether " + base + " is protected " + onForge(l) + ", so refusing to merge: " + err.Error())
	}
	return errShip(base + " is protected " + onForge(l) + "; omatty moves a protected branch only by a promotion " + l.Change)
}

// onForge is where a change's checks and protection live, in words: "on
// GitLab", or "on the forge" for one omatty cannot name (#464).
func onForge(l forge.Label) string { return "on " + forgeName(l) }

// forgeName is the forge by name, or "the forge" for one omatty cannot name.
func forgeName(l forge.Label) string {
	if l.Forge == "" {
		return "the forge"
	}
	return l.Forge
}

// errShip is a refusal the footer shows. A string is enough: nothing branches on
// these, and they exist to be read.
type errShip string

func (e errShip) Error() string { return string(e) }

// shippedNotice names what the operator got: a merge, a merge the forge has
// only accepted (#464's review), or a pull request opened.
func (m *Model) shippedNotice(project string, msg ShippedMsg) string {
	ref := m.changeRef(project, msg.Number)
	switch {
	case msg.Merged:
		return "merged " + ref
	case msg.Queued:
		return "asked " + forgeName(m.label(project)) + " to merge " + ref + "; it has not finished yet"
	}
	return "pushed, and opened " + ref
}

// onShipped reports what happened. A push that went and a pull request that did
// not open must not read as success, which is why the message carries both.
func (m *Model) onShipped(msg ShippedMsg) tea.Cmd {
	if msg.Err != nil {
		slog.Warn("shipping a session", "session", msg.SessionID, "err", msg.Err)
		m.lastErr = msg.Err.Error()
		return nil
	}
	sess, ok := m.session(msg.SessionID)
	m.notice = m.shippedNotice(sess.Project, msg)
	// The card's pull request state is now a poll behind what just happened.
	if !ok {
		return nil
	}
	return m.pollProjectPRs(sess.Project)
}
