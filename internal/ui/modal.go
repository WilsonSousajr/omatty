// The modal layer: the surfaces that take the keyboard away from the panes.
// Split out of model.go when the rename box (#41) made the new-session prompt
// the second of several, so model.go stays the state and the message router.
//
// A modal never reaches dispatch. focus reports "nothing focused" while one is
// open, so keys.Router routes every key down the omatty path - the trick the
// prompt already used, generalised. Invariant 1's structure therefore survives
// the new surfaces without the router learning anything about them.

package ui

import (
	tea "charm.land/bubbletea/v2"
)

// modalKind is which surface owns the keyboard. Exactly one can, so this is a
// single field rather than a bool per surface: two open at once is not a state
// omatty has, and independent flags would make it representable.
type modalKind int

const (
	modalNone modalKind = iota
	// modalPrompt is the new-session input, opened with n or N.
	modalPrompt
	// modalRename retitles the selected session, opened with R (#41).
	modalRename
	// modalConfirm asks before archiving it, opened with x (#40).
	modalConfirm
	// modalList is the session switcher, opened with / (#42).
	modalList
	// modalPicker is the same list over discovered repositories, opened with a
	// (#91). A separate kind rather than a flag on the list, because only the
	// commit differs and Kind is what already selects a commit.
	modalPicker
	// modalHelp lists every leader key, opened with ? (#103). It takes no
	// input at all: any key closes it.
	modalHelp
)

// modal is the open surface's state. Only the member matching Kind is live;
// the others hold their zero value, which is what closing one means.
type modal struct {
	Kind    modalKind
	Editor  lineEditor
	Confirm confirmBox
	List    pickList
}

// modalOpen reports whether a surface owns the keyboard. It is the single
// predicate every other site asks, so nothing else compares Kind to modalNone.
func (m *Model) modalOpen() bool { return m.modal.Kind != modalNone }

// lineEditor is the one-line text input behind both the new-session prompt and
// the rename box. Target names the session being renamed and is empty for a
// prompt, which creates one rather than editing one.
type lineEditor struct {
	// Worktree is true when the prompt was opened with N, meaning the buffer
	// names a branch to create a worktree on.
	Worktree bool
	Target   string
	Buffer   string
}

// Prompt is the pending new-session input. The zero value means no prompt.
type Prompt struct {
	Active bool
	// Worktree is true when the prompt was opened with N, meaning the buffer
	// names a branch to create a worktree on.
	Worktree bool
	Buffer   string
}

// Prompt returns the pending new-session input, if any.
//
// It projects the modal rather than holding state of its own: #41 folded the
// prompt and the rename box into one editor, and this stays the model's public
// answer to "is a new-session prompt open".
//
//	if p := m.Prompt(); p.Active { ... }
func (m *Model) Prompt() Prompt {
	if m.modal.Kind != modalPrompt {
		return Prompt{}
	}
	e := m.modal.Editor
	return Prompt{Active: true, Worktree: e.Worktree, Buffer: e.Buffer}
}

// onModalKey hands a key to the open surface. The prompt and the rename box
// share a handler; only the commit differs, and Kind selects that.
func (m *Model) onModalKey(msg tea.KeyPressMsg) tea.Cmd {
	switch m.modal.Kind {
	case modalPrompt, modalRename:
		return m.onEditorKey(msg)
	case modalConfirm:
		return m.onConfirmKey(msg.Keystroke())
	case modalList, modalPicker:
		return m.onListKey(msg)
	case modalHelp:
		// It shows a keymap and takes no input, so any key dismisses it -
		// including the one you reached for next.
		m.modal = modal{}
	}
	return nil
}

// onEditorKey edits the buffer.
//
// Typed text comes from msg.Text, which carries the character with its
// modifiers already applied, so a capital arrives as "F". The keystroke name
// would be "shift+f", which the old guard on len([]rune(key)) == 1 silently
// dropped: you could not type a capital into a session title. The note editor
// has always done it this way (#22).
func (m *Model) onEditorKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Keystroke() {
	case "esc":
		m.modal = modal{}
	case "enter":
		return m.commitEditor()
	case "backspace":
		m.modal.Editor.Buffer = trimLastRune(m.modal.Editor.Buffer)
	default:
		m.modal.Editor.Buffer += msg.Text
	}
	return nil
}

// commitEditor applies the buffer: a prompt creates a session, a rename
// retitles one. An empty buffer leaves the editor open rather than registering
// a nameless session or blanking a title.
func (m *Model) commitEditor() tea.Cmd {
	if m.modal.Editor.Buffer == "" {
		return nil
	}
	if m.modal.Kind == modalRename {
		return m.commitRename()
	}
	return m.submitPrompt()
}
