// The turn baseline (#311): taken when a prompt is submitted, so the review
// column can show only what the current turn changed.

package ui

import (
	"errors"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// TurnSnappedMsg carries a baseline snapshot's outcome into Update. Exported
// so tests can send one.
type TurnSnappedMsg struct {
	SessionID string
	Err       error
}

// maybeSnapTurn records the session's working tree as a turn begins. Only
// the hook counts: the tailer reports PromptSubmitted for every tool result,
// and a snapshot taken mid-turn would drop the turn's earlier edits from its
// own diff. A snapshot already in flight absorbs the prompt.
func (m *Model) maybeSnapTurn(e watcher.Event) tea.Cmd {
	if e.Kind != watcher.PromptSubmitted || !e.Hook || m.turnPending[e.SessionID] {
		return nil
	}
	sess, ok := m.session(e.SessionID)
	if !ok {
		return nil
	}
	m.turnPending[sess.ID] = true
	snap := m.turn.Snap
	return func() tea.Msg { return TurnSnappedMsg{SessionID: sess.ID, Err: snap(sess)} }
}

// onTurnSnapped settles a snapshot. A failure is kept until the next success:
// the previous turn's ref is still standing, and diffing against it would
// show two turns as one.
func (m *Model) onTurnSnapped(msg TurnSnappedMsg) tea.Cmd {
	delete(m.turnPending, msg.SessionID)
	if !m.knownSession(msg.SessionID) {
		return nil
	}
	if msg.Err != nil {
		slog.Warn("taking a turn baseline", "session", msg.SessionID, "err", msg.Err)
		m.turnErr[msg.SessionID] = msg.Err
		return m.reloadTurn(msg.SessionID)
	}
	delete(m.turnErr, msg.SessionID)
	return m.reloadTurn(msg.SessionID)
}

// reloadTurn answers a new baseline. An open turn view loads at once; a
// closed column holding this session's turn view is marked stale, since the
// turn diff it kept for the reopen (#124) is now the previous turn's and would
// be shown as this one.
func (m *Model) reloadTurn(id string) tea.Cmd {
	if id == m.review.SessionID && !m.review.Open && m.review.Scope == scopeTurn {
		m.review.Stale = true
	}
	return m.loadTurn(id)
}

// dropTurnCmd deletes an archived session's baseline off the Update
// goroutine. A failure is logged and nothing more: the archive has happened,
// and a stray ref costs a few objects, not correctness.
func (m *Model) dropTurnCmd(sess registry.Session) tea.Cmd {
	root, drop := m.projectRoot(sess.Project), m.turn.Drop
	return func() tea.Msg {
		if err := drop(sess, root); err != nil {
			slog.Warn("deleting a turn baseline", "session", sess.ID, "err", err)
		}
		return nil
	}
}

// toggleScope switches the diff between the whole session and this turn,
// starting from the top: the two are different lists of rows.
func (m *Model) toggleScope() tea.Cmd {
	if m.review.Scope == scopeTurn {
		m.review.Scope = scopeSession
	} else {
		m.review.Scope = scopeTurn
	}
	m.review.Cursor, m.review.Offset, m.review.ColOffset = 0, 0, 0
	m.review.TurnDiff, m.review.TurnErr, m.review.TurnReady = review.Diff{}, nil, false
	m.rebuildEntries()
	// Both diffs: the turn view places and anchors comments through the
	// session diff, so that one must be as fresh as the turn.
	return m.loadDiff(m.review.SessionID)
}

// turnNotice is what the turn scope says instead of rows, split into lines
// short enough for the narrowest column; nil when there are rows to show.
// isErr says a load failed, as opposed to there being nothing to show yet.
func (m *Model) turnNotice() (lines []string, isErr bool) {
	id := m.review.SessionID
	switch {
	case m.hooksDown:
		// Any ref standing now was left by another run; diffing against it
		// would call someone else's turn this one (#49).
		return []string{"hooks are not arriving,", "so no turn baseline", "can be taken: see the log"}, true
	case m.turnErr[id] != nil:
		return []string{"this turn's baseline", "could not be taken:", causeOf(m.turnErr[id]), logHint}, true
	case errors.Is(m.review.TurnErr, review.ErrNoTurn):
		return []string{"no turn recorded yet:", "a baseline is taken", "when you send a prompt"}, false
	case m.review.TurnErr != nil:
		return []string{"reading this turn failed:", causeOf(m.review.TurnErr), logHint}, true
	case !m.review.TurnReady:
		return []string{"reading this turn..."}, false
	}
	return nil, false
}
