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
		return m.command(msg.Keystroke())
	default: // keys.Swallow - the leader itself
		return nil
	}
}

// focus reports which pane owns plain keystrokes and whether any does. A
// prompt or an empty sidebar leaves nothing focused, so every key is an omatty
// command and ctrl+c quits (issue #28).
func (m *Model) focus() (focusTarget, bool) {
	if m.prompt.Active {
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
		return m.onReviewKey(msg.Keystroke())
	default:
		return m.focusedTerminal().Update(msg)
	}
}

// command runs an omatty command key, pressed after the leader or while a
// prompt is open.
func (m *Model) command(key string) tea.Cmd {
	// ctrl+c is the unconditional escape hatch, checked before the prompt so
	// an open prompt cannot trap the operator (issue #28). With a session
	// focused this is never reached: ctrl+c belongs to Claude, which uses it
	// to interrupt a turn (invariant 1).
	if key == "ctrl+c" {
		return tea.Quit
	}
	if m.prompt.Active {
		return m.onPromptKey(key)
	}
	return m.navigate(key)
}

// navigate runs a command key while no prompt is open.
func (m *Model) navigate(key string) tea.Cmd {
	switch key {
	case "j":
		return m.moveCursor(m.sidebar.MoveDown)
	case "k":
		return m.moveCursor(m.sidebar.MoveUp)
	case "n":
		m.prompt = Prompt{Active: true}
	// Keystroke() spells a shifted letter with the base key in lower case, so
	// a terminal reporting the modifier gives "shift+n"; the bare "N" is
	// accepted too, because a legacy terminal cannot report shift at all. The
	// upper-case "shift+N" spelling never occurs and was dead (issue #87).
	case "shift+n", "shift+N", "N":
		m.prompt = Prompt{Active: true, Worktree: true}
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
		return m.toggleReview()
	case "q":
		return tea.Quit
	}
	return nil
}
