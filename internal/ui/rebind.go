// Following a session onto the conversation /clear moved it to (#316). claude
// drops the uuid omatty assigned and starts over under a new one; the row
// keeps its ID, which names its terminal, its dtach socket and every map
// here, and records the conversation it is on now.

package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// RebindFunc persists the conversation a session's claude now runs. Injected
// so ui never reaches the registry store itself; cmd/omatty closes it over
// the store.
//
//	deps.Rebind = func(id, conversation string) error {
//	        return registry.RebindSession(store, id, conversation)
//	}
type RebindFunc func(sessionID, conversation string) error

// noRebind is the Deps.Rebind default. It names the missing wiring rather
// than appearing to succeed, which would leave the pane on a conversation
// state.json does not know - the same reasoning as noRename.
func noRebind(sessionID, conversation string) error {
	return fmt.Errorf("ui: no rebind source configured for session %s (conversation %q)", sessionID, conversation)
}

// followClear re-binds the pane a SessionRebound names to the conversation
// it carries: persist, then memory, then the tailer, so a failed save leaves
// memory and disk agreeing on what to resume. The pane comes from e.Owner,
// never from the directory - two panes can share one.
//
// The persist runs off the Update goroutine since #653, so for the moment it
// takes, an event naming the new conversation maps to no row and is dropped;
// the tailer, started once the save lands, re-derives the status from the
// transcript.
func (m *Model) followClear(e status.Event) tea.Cmd {
	i, ok := m.sessionIndex(e.Owner)
	if !ok || m.state.Sessions[i].ConversationID() == e.SessionID {
		return nil
	}
	rebind := m.rebind
	return m.persistCmd("following a cleared session", []any{"session", e.Owner, "conversation", e.SessionID},
		func() error { return rebind(e.Owner, e.SessionID) },
		func(m *Model) tea.Cmd { return m.rebound(e.Owner, e.SessionID) })
}

// rebound points a row that is on disk at its new conversation, and its
// tailer with it.
func (m *Model) rebound(id, conversation string) tea.Cmd {
	i, ok := m.sessionIndex(id)
	if !ok {
		return nil
	}
	m.state.Sessions[i].Conversation = conversation
	m.tailStart(m.state.Sessions[i])
	return nil
}

// sessionOfConversation is the row whose claude is on conversation. Every
// status event names a conversation, and after a /clear that is no longer
// the row's ID (#316).
func (m *Model) sessionOfConversation(conversation string) (string, bool) {
	for _, sess := range m.state.Sessions {
		if sess.ConversationID() == conversation {
			return sess.ID, true
		}
	}
	return "", false
}
