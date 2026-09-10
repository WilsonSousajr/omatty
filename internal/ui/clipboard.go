// A copy made inside a session pane (#212).
//
// The other half of #190, which fixed paste and could only document copy. A
// program in the pane that copies with OSC 52 - claude's own copy
// affordances, tmux's set-clipboard, neovim's clipboard provider - reached
// nothing, because the emulator under bubbleterm has no clipboard hook and
// dropped the sequence. termwrap now lifts it out of the byte stream instead;
// this is where it turns back into a write on the operator's own clipboard.

package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// ClipboardMsg carries one OSC 52 copy made inside a session's pane into the
// Update loop, on its way to the host terminal.
type ClipboardMsg struct {
	SessionID string
	Write     termwrap.ClipboardWrite
}

// waitForClipboard blocks on the next copy made in session id's pane, the way
// waitForEvent blocks on the next status event.
//
// A session with no terminal, and a terminal reporting no stream - the Guard
// does that once it has panicked - are not waited on at all. Arming them
// would park a goroutine on a channel nothing can ever fill.
func (m *Model) waitForClipboard(id string) tea.Cmd {
	term := m.terms[id]
	if term == nil {
		return nil
	}
	writes := term.ClipboardWrites()
	if writes == nil {
		return nil
	}
	return func() tea.Msg { return ClipboardMsg{SessionID: id, Write: <-writes} }
}

// onClipboard puts the copy on the host's clipboard and waits for the next
// one. A pane copies more than once; without the re-arm the first copy of a
// session would be the only one that ever arrived.
func (m *Model) onClipboard(msg ClipboardMsg) tea.Cmd {
	return tea.Batch(setHostClipboard(msg.Write), m.waitForClipboard(msg.SessionID))
}

// setHostClipboard hands the text to bubbletea, which emits the OSC 52 to the
// host itself - so the write goes through the runtime that owns the screen
// rather than past it (invariant 5). It takes plain text and encodes it,
// which is why termwrap decoded in the first place.
//
// 'p' is OSC 52's primary selection, an X11 and Wayland notion. Everything
// else is the system clipboard, which is what claude, tmux and neovim ask
// for, and what a selection field naming nothing means.
func setHostClipboard(w termwrap.ClipboardWrite) tea.Cmd {
	if w.Selection == 'p' {
		return tea.SetPrimaryClipboard(w.Text)
	}
	return tea.SetClipboard(w.Text)
}
