// Key routing: which pane owns a keystroke, and what each omatty command key
// does. Split out of model.go when the review column made a third focus
// target, so model.go stays the state and the message router (#21).

package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/keys"
)

// onKey applies invariant 1: with a pane focused every key reaches it except
// the leader. The terminal, the review column and the note editor are all
// "focused" as far as the router is concerned, so ctrl+o stays the leader
// everywhere and no key is ever inspected to guess where it belongs.
//
// A key reaches the terminal as the message itself, not as text: bubbleterm
// does its own key-to-escape translation, so forwarding msg.String() would
// type the literal word "esc" into Claude.
func (m *Model) onKey(msg tea.KeyPressMsg) tea.Cmd {
	m.lastErr = "" // any keypress acknowledges the last error
	target, focused := m.focus()
	switch m.router.Next(msg.Keystroke(), focused) {
	case keys.ToTerminal:
		return m.dispatch(target, msg)
	case keys.ToOmatty:
		return m.command(msg)
	default: // keys.Swallow - the leader itself
		return nil
	}
}

// focus reports which pane owns plain keystrokes and whether any does. An open
// modal surface or an empty sidebar leaves nothing focused, so every key is an
// omatty command and ctrl+c quits (issue #28).
func (m *Model) focus() (focusTarget, bool) {
	if m.modalOpen() {
		return focusTerminal, false
	}
	if m.review.Note.Active {
		return focusNote, true
	}
	if m.review.Focused {
		return focusReview, true
	}
	return focusTerminal, m.focusedTerminal() != nil
}

// dispatch hands a plain keystroke to the focused pane.
func (m *Model) dispatch(target focusTarget, msg tea.KeyPressMsg) tea.Cmd {
	switch target {
	case focusNote:
		return m.onNoteKey(msg)
	case focusReview:
		return m.onPaneKey(msg.Keystroke())
	default:
		return m.focusedTerminal().Update(msg)
	}
}

// onPaneKey picks the handler for the review column's current view: the three
// views share a focus target but not a keymap (#24).
func (m *Model) onPaneKey(key string) tea.Cmd {
	switch m.review.View {
	case ViewTree:
		return m.onTreeKey(key)
	case ViewPreview:
		return m.onPreviewKey(key)
	default:
		return m.onReviewKey(key)
	}
}

// command runs an omatty command key, pressed after the leader or while a
// modal surface is open. It takes the message rather than the keystroke
// because the text editors need msg.Text: the keystroke name spells a capital
// "shift+f", which is not what belongs in a session title (#41).
func (m *Model) command(msg tea.KeyPressMsg) tea.Cmd {
	// ctrl+c is the unconditional escape hatch, checked before the modal so an
	// open surface cannot trap the operator (issue #28). With a session focused
	// this is never reached: ctrl+c belongs to Claude, which uses it to
	// interrupt a turn (invariant 1).
	if msg.Keystroke() == "ctrl+c" {
		return tea.Quit
	}
	if m.modalOpen() {
		return m.onModalKey(msg)
	}
	return m.navigate(msg.Keystroke())
}

// navigate runs a command key while no prompt is open.
func (m *Model) navigate(key string) tea.Cmd {
	switch key {
	case "j":
		return m.moveCursor(m.sidebar.MoveDown)
	case "k":
		return m.moveCursor(m.sidebar.MoveUp)
	case "n":
		m.modal = modal{Kind: modalPrompt}
	// Keystroke() spells a shifted letter with the base key in lower case, so
	// a terminal reporting the modifier gives "shift+n"; the bare "N" is
	// accepted too, because a legacy terminal cannot report shift at all. The
	// upper-case "shift+N" spelling never occurs and was dead (issue #87).
	case "shift+n", "shift+N", "N":
		m.modal = modal{Kind: modalPrompt, Editor: lineEditor{Worktree: true}}
	default:
		return m.paneCommand(key)
	}
	return nil
}

// paneCommand runs the leader commands that act on the focused session.
func (m *Model) paneCommand(key string) tea.Cmd {
	switch key {
	case "r":
		return m.restartFocused()
	case "d":
		return m.toggleView(ViewDiff)
	case "f":
		return m.toggleView(ViewTree)
	case "q":
		return tea.Quit
	}
	return m.modalCommand(key)
}

// modalCommand opens a surface that takes the keyboard. It is a third table
// beside navigate and paneCommand for the reason paneCommand was split off in
// the first place: M4's keys would push one switch past gocyclo's limit.
func (m *Model) modalCommand(key string) tea.Cmd {
	switch key {
	// Two spellings: a terminal reporting the modifier gives "shift+r", a
	// legacy one the bare "R"; the upper-case "shift+R" never occurs (issue
	// #87). Lower-case r is restart, so getting this wrong is silent.
	case "shift+r", "R":
		m.openRename()
	case "x":
		m.openConfirm()
	}
	return nil
}
