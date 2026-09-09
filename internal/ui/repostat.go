// Each session card's branch and diffstat (#180). New data, so a new seam
// following the diff's route: vcs.Shortstat under review.Source.Stat under
// a typed func here, polled off the render path and kept in memory only.

package ui

import (
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// RepoStatMsg carries one poll's answer into Update. Exported so tests can
// send one.
type RepoStatMsg struct {
	SessionID string
	Stat      review.Stat
	Err       error
}

// StatTickMsg is the heartbeat that polls every session's checkout.
// Exported so tests can send one.
type StatTickMsg time.Time

// statEvery is the poll's period. Ten seconds: a diffstat changes when a
// turn ends, and those moments poll at once (refreshStat); the tick catches
// edits made outside claude.
const statEvery = 10 * time.Second

func scheduleStatTick() tea.Cmd {
	return tea.Tick(statEvery, func(t time.Time) tea.Msg { return StatTickMsg(t) })
}

// onStatTick polls every session and re-arms the tick.
func (m *Model) onStatTick() tea.Cmd { return tea.Batch(m.pollAll(), scheduleStatTick()) }

// pollAll is one poll per session, nil for a model with no reader.
func (m *Model) pollAll() tea.Cmd {
	if m.stat == nil {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(m.state.Sessions))
	for _, sess := range m.state.Sessions {
		cmds = append(cmds, m.pollStat(sess.ID))
	}
	return tea.Batch(cmds...)
}

// pollStat reads one session's stat off the Update goroutine, the shape
// loadDiff uses. A poll already in flight is not repeated.
func (m *Model) pollStat(id string) tea.Cmd {
	sess, ok := m.session(id)
	if !ok || m.stat == nil || m.statPending[id] {
		return nil
	}
	m.statPending[id] = true
	root, read := m.projectRoot(sess.Project), m.stat
	return func() tea.Msg {
		st, err := read(sess, root)
		return RepoStatMsg{SessionID: id, Stat: st, Err: err}
	}
}

// refreshStat polls a session the moment its turn ends or it stops for a
// question: the two moments its numbers change (the rule refreshReview uses).
func (m *Model) refreshStat(id string, before, after watcher.Status) tea.Cmd {
	if before == after || (after != watcher.StatusDone && after != watcher.StatusWaiting) {
		return nil
	}
	return m.pollStat(id)
}

// onRepoStat stores an answer. A failure keeps the last stat - the card says
// what it last knew rather than nothing - and is logged once per outage; the
// once-flag clears on the next success so a checkout that breaks again logs
// again. The footer says nothing: a broken checkout is loud in the review
// column, which is where it is actionable.
func (m *Model) onRepoStat(msg RepoStatMsg) tea.Cmd {
	delete(m.statPending, msg.SessionID)
	if msg.Err == nil {
		delete(m.statFailed, msg.SessionID)
		m.repoStat[msg.SessionID] = msg.Stat
		return nil
	}
	if !m.statFailed[msg.SessionID] {
		slog.Warn("reading repo stat", "session", msg.SessionID, "err", msg.Err)
	}
	m.statFailed[msg.SessionID] = true
	return nil
}
