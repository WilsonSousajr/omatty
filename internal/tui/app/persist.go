// The registry writes the TUI asks for, off the Update goroutine (ADR 0001
// pain point 5; migration step 5.6b, #653): archive, rename, the auto-namer's
// titles, a project's removal, a fold, a /clear's rebind and the gate tally.
// Each was a state.json load and save inside Update. They stay save-first:
// the screen changes only when the write's answer says it landed.

package app

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
)

// persistedMsg is a registry write's answer, carrying what to do with it: done
// on success; on failure, a log line named by what and attrs, and the footer
// unless warnOnly says the operator did not ask for this write.
type persistedMsg struct {
	err      error
	what     string
	attrs    []any
	warnOnly bool
	done     func(*Model) tea.Cmd
}

// persistCmd queues write on the model's writer, so it runs off the Update
// goroutine and after every write asked for before it, and answers with a
// persistedMsg. done reads the model as it is when the answer arrives, not as
// it was when the write was asked for.
//
//	return m.persistCmd("renaming session", []any{"session", id}, func() error { return rename(id, title) }, retitled)
func (m *Model) persistCmd(what string, attrs []any, write func() error, done func(*Model) tea.Cmd) tea.Cmd {
	result := m.writes.submit(write)
	return func() tea.Msg { return persistedMsg{err: <-result, what: what, attrs: attrs, done: done} }
}

// quietly marks a write the operator did not ask for - the auto-namer's - so a
// failure is logged at warn and kept out of the footer, as it always was.
func quietly(cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		msg := cmd().(persistedMsg)
		msg.warnOnly = true
		return msg
	}
}

// onPersisted applies a write that landed, or reports one that did not.
func (m *Model) onPersisted(msg persistedMsg) tea.Cmd {
	if msg.err != nil {
		attrs := append(append([]any{}, msg.attrs...), "err", msg.err)
		if msg.warnOnly {
			slog.Warn(msg.what, attrs...)
			return nil
		}
		slog.Error(msg.what, attrs...)
		m.lastErr = msg.err.Error()
		return nil
	}
	if msg.done == nil {
		return nil
	}
	return msg.done(m)
}
