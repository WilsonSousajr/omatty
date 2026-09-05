// The one-line text input shared by every editor omatty has: the new-session
// prompt, the rename box (#41) and the review column's note (#22).
//
// One implementation, because the copy is what lost the guard: onEditorKey was
// written from onNoteKey and dropped its TrimSpace, so a title of nothing but
// spaces reached state.json while the identical note editor rejected one (#41).

package ui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// editKey applies one keystroke to a text buffer, reporting what the caller
// should do next.
//
// Typed text comes from msg.Text, which carries the character with its
// modifiers already applied, so a capital arrives as "F" and a space as " ".
// The keystroke *name* would be "shift+f" and "space", which is what the old
// guard on len([]rune(key)) == 1 silently dropped: neither a capital nor a
// space could be typed into a session title (#41).
type editAction int

const (
	// editContinue means the buffer changed and the editor stays open.
	editContinue editAction = iota
	// editCommit means enter was pressed.
	editCommit
	// editCancel means esc was pressed.
	editCancel
)

// editKey folds msg into buffer and says what happened.
func editKey(buffer string, msg tea.KeyPressMsg) (string, editAction) {
	switch msg.Keystroke() {
	case "esc":
		return buffer, editCancel
	case "enter":
		return buffer, editCommit
	case "backspace":
		return trimLastRune(buffer), editContinue
	default:
		return buffer + msg.Text, editContinue
	}
}

// editLine renders a labelled one-line input, scrolled so the cursor stays
// visible.
//
// The pane is 30 columns on a 60-column window and a rename box opens
// pre-filled with the existing title, so a plain fitLine cut the tail of the
// buffer *and* the cursor block off the right edge: typing past the width gave
// no feedback at all while the buffer went on growing invisibly. panLine is
// what the review column already uses to reach text past the edge (#94, #41).
func editLine(label, buffer string, width int) string {
	line := label + ": " + buffer + "_"
	if over := lipgloss.Width(line) - width; over > 0 {
		return fitLine(panLine(line, over), width)
	}
	return fitLine(line, width)
}
