// The review follows the session's working directory (#659): claude moves
// into worktrees and other checkouts of its repository during a session, and
// the tree, the diff, the card's stat, the gate and ship kept reading the
// directory omatty launched it in.
//
// The directory comes from the transcript (invariant 2), on the tailer's
// status events. It is remembered as it arrives and acted on only at a turn's
// end: inside a run claude's directory jumps back to the launch directory for
// a line or two, and switching there made the tree flicker. Nothing here is
// stored - Session.Dir never changes, and the transcript is re-read on every
// start, so the directory is derived again (invariant 9).

package app

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"

	dsession "github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
)

// FollowFunc resolves a directory the agent reports working in to the session
// as the review should read it: Dir at that checkout's top level and Branch its
// branch, or sess unchanged when the directory does not move the checkout.
// cmd passes internal/service/review's Source.Follow.
//
//	d.Follow = src.Follow
type FollowFunc func(sess dsession.Session, cwd string) (dsession.Session, error)

// stayHome is the Deps.Follow default: the review never moves.
func stayHome(sess dsession.Session, _ string) (dsession.Session, error) { return sess, nil }

// WorkTreeMsg is a Follow's answer. Exported so tests can send one.
type WorkTreeMsg struct {
	SessionID string
	Cwd       string
	Sess      dsession.Session
	Err       error
}

// workDir is one session's working directory as the review knows it.
type workDir struct {
	Seen        string // the last directory a status event reported
	Asked       string // the last one Follow was asked about
	Dir, Branch string // where the review reads, while Moved
	Moved       bool
}

// followWorkDir remembers the directory e reports and, at a turn's end, asks
// once whether it moves the review. A directory already asked about is not
// asked again, so git runs once per move rather than once per turn.
func (m *Model) followWorkDir(e dstatus.Event, after dstatus.Status) tea.Cmd {
	sess, ok := m.session(e.SessionID)
	if !ok {
		return nil
	}
	w := m.workDirs[sess.ID]
	if e.Cwd != "" {
		w.Seen = e.Cwd
	}
	m.workDirs[sess.ID] = w
	if w.Seen == "" || w.Seen == w.Asked || !turnOver(after) {
		return nil
	}
	w.Asked = w.Seen
	m.workDirs[sess.ID] = w
	follow, cwd := m.follow, w.Seen
	return func() tea.Msg {
		followed, err := follow(sess, cwd)
		return WorkTreeMsg{SessionID: sess.ID, Cwd: cwd, Sess: followed, Err: err}
	}
}

// turnOver is whether a status ends a turn: the moments refreshReview reads.
func turnOver(s dstatus.Status) bool { return s == dstatus.StatusDone || s == dstatus.StatusWaiting }

// onWorkTree moves the review to where Follow says the session works, and
// reloads what reads it when that changed.
func (m *Model) onWorkTree(msg WorkTreeMsg) tea.Cmd {
	if msg.Err != nil {
		slog.Warn("following the session's working directory", "session", msg.SessionID, "cwd", msg.Cwd, "err", msg.Err)
		return nil
	}
	home, ok := m.session(msg.SessionID)
	if !ok {
		return nil
	}
	w := m.workDirs[msg.SessionID]
	next := workDir{Seen: w.Seen, Asked: w.Asked, Moved: msg.Sess.Dir != home.Dir}
	if next.Moved {
		next.Dir, next.Branch = msg.Sess.Dir, msg.Sess.Branch
	}
	m.workDirs[msg.SessionID] = next
	if next == w {
		return nil
	}
	return m.reloadWorkDir(msg.SessionID)
}

// reloadWorkDir rereads everything that reads the session's checkout: the
// card's stat always, and the column when it shows this session - or, closed,
// on its reopen (#124's rule).
func (m *Model) reloadWorkDir(id string) tea.Cmd {
	stat := m.pollStat(id)
	if id != m.review.SessionID {
		return stat
	}
	if !m.review.Open {
		m.review.Stale = true
		return stat
	}
	return tea.Batch(stat, m.loadDiff(id), m.relistFiles(id))
}

// reviewSession is the session as everything that reads its checkout sees
// it: at the directory it works in now (#659). Its own launch directory
// stays the session's for the transcript, --resume and the archive.
func (m *Model) reviewSession(id string) (dsession.Session, bool) {
	sess, ok := m.session(id)
	return m.atWorkDir(sess), ok
}

// atWorkDir is sess moved to where it works now, or sess when it has not
// moved.
func (m *Model) atWorkDir(sess dsession.Session) dsession.Session {
	if w := m.workDirs[sess.ID]; w.Moved {
		sess.Dir, sess.Branch = w.Dir, w.Branch
	}
	return sess
}
