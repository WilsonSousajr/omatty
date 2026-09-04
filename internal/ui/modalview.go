// Drawing the modal surfaces.
//
// A modal replaces the terminal pane's *content* and never its *geometry*, so
// the PTY behind one stays sized to the same box. That is what keeps issue #95
// fixed for every surface rather than only for the prompt, and it is why the
// pane is not shrunk to make room the way the note editor shrinks the review
// column: doing that here would clip the terminal's last row (#75).

package ui

// modalLines is the open surface's body. It returns nil when nothing is open,
// which renderTerminal never asks for.
func (m *Model) modalLines() []string {
	if m.modal.Kind == modalPrompt || m.modal.Kind == modalRename {
		return m.editorLines()
	}
	return nil
}

// editorLines draws the one-line input: a blank lead-in, the label and buffer
// with a cursor block, then how to get out. The keys are repeated in the
// footer, but an operator looking at a box wants them next to the box.
func (m *Model) editorLines() []string {
	return []string{
		"",
		m.editorLabel() + ": " + m.modal.Editor.Buffer + "_",
		"",
		"enter to confirm, esc to cancel",
	}
}

// editorLabel names what the buffer will become, which is the only thing
// distinguishing the three editors on screen.
func (m *Model) editorLabel() string {
	if m.modal.Kind == modalRename {
		return "rename session"
	}
	if m.modal.Editor.Worktree {
		return "new branch (worktree)"
	}
	return "new session title"
}

// modalFooter is the keymap while a surface is open, or "" when none is. The
// base footer is already truncated at 100 columns (issue #30), so a new key
// earns its place here rather than lengthening that constant.
func modalFooter(k modalKind) string {
	switch k {
	case modalPrompt, modalRename:
		return "enter confirm  esc cancel  ctrl+c quit"
	}
	return ""
}
