// Naming a session from its first prompt (#127). A session created with
// ctrl+o n and no name is registered under a placeholder, and the first thing
// the operator types into it becomes its title - the rule adoption has used
// since #122, applied to the sessions omatty creates itself. Nothing here
// blocks: reading a transcript head is a tea.Cmd, and a failure is a log line
// and the name the session already had.

package ui

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// NameFunc reads a session's first typed prompt and returns the title to give
// it. Injected because ui may not read the transcript store itself; cmd
// closes it over discover.FirstPromptTitle. An empty title with a nil error
// means the session has not been spoken to yet, which is not a failure.
//
//	deps.Name = func(sess registry.Session) (string, error) {
//	        return discover.FirstPromptTitle(paths.Transcript(home, sess.Dir, sess.ID))
//	}
type NameFunc func(sess registry.Session) (string, error)

// noName is the Deps.Name default: no reader, so nothing is auto-named. Like
// noTailStart and unlike noRename, it does nothing rather than naming missing
// wiring - a model built without a transcript store has no first prompt to
// read, and an error on every status event would be noise.
func noName(registry.Session) (string, error) { return "", nil }

// NamedMsg carries a derived title back into Update. From is the title the
// derivation started from, so a name the operator typed with ctrl+o R while
// the read was in flight is not overwritten (#41, #127).
type NamedMsg struct {
	SessionID string
	From      string
	Title     string
	Err       error
}

// isPlaceholderTitle reports whether a session still carries the name it was
// given before it had one. Derived from the session rather than tracked in a
// map, and that is the whole design: a map is empty after a relaunch, so an
// untitled session that already has a prompt in its transcript would keep its
// uuid forever - the case invariant 9 exists for. Both spellings are accepted
// because there are two: a created session gets the short uuid, an adopted
// one with no typed prompt gets the whole uuid (discover.titleOf, #122).
func isPlaceholderTitle(sess registry.Session) bool {
	return sess.Title == registry.PlaceholderTitle(sess.ID) || sess.Title == sess.ID
}

// maybeName asks for a title for a session that still carries its placeholder.
//
// Any status event is the trigger, not PromptSubmitted alone: the tailer
// emits one event per poll derived from the tail of the transcript, so a
// relaunched session's first event is usually TurnEnded - and a session
// created untitled, given a prompt, then restarted would never be named
// (invariant 9). Reading the transcript decides, so a trigger that is not a
// prompt (DeriveKind also reports PromptSubmitted for a tool result) costs
// one bounded read and answers "". It sits beside maybeNotify, never inside
// its startedAt guard: a replayed event is exactly the relaunch case.
func (m *Model) maybeName(sessionID string) tea.Cmd {
	sess, ok := m.session(sessionID)
	if !ok || !isPlaceholderTitle(sess) || m.namePending[sessionID] {
		return nil
	}
	m.namePending[sessionID] = true
	read := m.name
	return func() tea.Msg {
		title, err := read(sess)
		return NamedMsg{SessionID: sessionID, From: sess.Title, Title: title, Err: err}
	}
}

// onNamed applies a derived title, unless the title moved on while the read
// was in flight or the read produced nothing.
func (m *Model) onNamed(msg NamedMsg) tea.Cmd {
	delete(m.namePending, msg.SessionID)
	if msg.Err != nil {
		slog.Warn("naming session", "session", msg.SessionID, "err", msg.Err)
		return nil
	}
	sess, ok := m.session(msg.SessionID)
	if !ok || msg.Title == "" || sess.Title != msg.From {
		return nil
	}
	if err := m.applyTitle(msg.SessionID, msg.Title); err != nil {
		slog.Warn("naming session", "session", msg.SessionID, "title", msg.Title, "err", err)
		return nil
	}
	return m.modelName(msg.SessionID, msg.Title)
}

// ModelNameFunc asks the agent for a better name than the prompt-derived
// one. Injected because ui may not run a binary (invariant 4), and left nil
// unless [naming] model is on in the config file - the nil is the switch, so
// there is no config value consulted inside Update and no call the operator
// did not ask for (#44, #127).
type ModelNameFunc func(prompt string) (string, error)

// ModelNamedMsg carries the model's suggestion back into Update. From is the
// step-1 title it was asked to improve.
type ModelNamedMsg struct {
	SessionID string
	From      string
	Title     string
	Err       error
}

// modelName asks for an improved name once step 1 has set one. It returns
// nil when the config left the call off, which is the default. It is given
// the step-1 title rather than a second read of the transcript: that title is
// the first prompt flattened to sixty cells, which is as much as a person
// would name it from.
func (m *Model) modelName(sessionID, title string) tea.Cmd {
	if m.modelNamer == nil {
		return nil
	}
	ask := m.modelNamer
	return func() tea.Msg {
		got, err := ask(title)
		return ModelNamedMsg{SessionID: sessionID, From: title, Title: got, Err: err}
	}
}

// onModelNamed applies the improved name unless the title moved on while the
// call was in flight. It renames through applyTitle alone and never through
// onNamed: onNamed is what dispatches modelName, so routing back through it
// would ask the model to improve its own answer, forever.
func (m *Model) onModelNamed(msg ModelNamedMsg) tea.Cmd {
	if msg.Err != nil {
		slog.Warn("model naming", "session", msg.SessionID, "err", msg.Err)
		return nil
	}
	sess, ok := m.session(msg.SessionID)
	if !ok || msg.Title == "" || sess.Title != msg.From {
		return nil
	}
	if err := m.applyTitle(msg.SessionID, msg.Title); err != nil {
		slog.Warn("model naming", "session", msg.SessionID, "title", msg.Title, "err", err)
	}
	return nil
}
