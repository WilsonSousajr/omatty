// The turn baseline (#311): taken when a prompt is submitted, so the review
// column can show only what the current turn changed.

package ui

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/registry"
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
		m.turnErr[msg.SessionID] = msg.Err.Error()
		return nil
	}
	delete(m.turnErr, msg.SessionID)
	return nil
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
