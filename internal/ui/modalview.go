// Drawing the modal surfaces.
//
// A modal replaces the terminal pane's *content* and never its *geometry*, so
// the PTY behind one stays sized to the same box. That is what keeps issue #95
// fixed for every surface rather than only for the prompt, and it is why the
// pane is not shrunk to make room the way the note editor shrinks the review
// column: doing that here would clip the terminal's last row (#75).

package ui

import "strconv"

// modalLines is the open surface's body. It returns nil when nothing is open,
// which renderTerminal never asks for.
func (m *Model) modalLines() []string {
	switch m.modal.Kind {
	case modalPrompt, modalRename:
		return m.editorLines()
	case modalConfirm:
		return m.confirmLines()
	case modalList, modalPicker:
		return m.pickLines()
	case modalHelp:
		return helpLines()
	}
	return nil
}

// leaderKeys is every leader binding, in the order an operator meets them.
// This is the one place the full keymap is written down: the footer shows a
// working subset, because it is truncated to the window (#30, #103).
var leaderKeys = [][2]string{
	{"j / k", "move between sessions"},
	{"/", "jump to a session by name"},
	{"n", "new session on the main checkout"},
	{"N", "new session on a fresh worktree"},
	{"a", "register a project claude already knows"},
	{"R", "rename the selected session"},
	{"x", "archive the selected session"},
	{"r", "restart a crashed session"},
	{"d", "open or close the diff pane"},
	{"f", "open or close the file tree"},
	{"?", "this list"},
	{"q", "quit"},
}

// claudeKeys are keys omatty does not own. They reach the session untouched,
// and they are listed because nothing else says so: pgup/pgdn always scrolled
// Claude's transcript and looked broken only because it was undocumented, and
// shift+drag is what selecting text costs now that omatty asks the terminal
// for the wheel (#107).
var claudeKeys = [][2]string{
	{"pgup / pgdn", "scroll the transcript"},
	{"shift+drag", "select text"},
}

// helpLines draws the full keymap. It exists because the footer constant
// outgrew the window: at 114 columns it was already truncating `ctrl+o f`
// before M4 added four more keys (#103).
func helpLines() []string {
	lines := make([]string, 0, len(leaderKeys)+2)
	lines = append(lines, Leader+" keys", "")
	for _, k := range leaderKeys {
		lines = append(lines, "  "+padRight(Leader+" "+k[0], 12)+"  "+k[1])
	}
	lines = append(lines, "", "in the session")
	for _, k := range claudeKeys {
		lines = append(lines, "  "+padRight(k[0], 12)+"  "+k[1])
	}
	return append(lines, "", "esc to close")
}

// confirmLines draws the question and one line per answer. The answers are
// spelled out rather than abbreviated to y/n, because one of them discards
// uncommitted work and the operator should read what they are agreeing to.
func (m *Model) confirmLines() []string {
	c := m.modal.Confirm
	lines := []string{"", "archive session " + strconv.Quote(c.Title) + "?", ""}
	for _, choice := range c.Choices {
		lines = append(lines, "  ["+choice.Key+"] "+choice.Label)
		if choice.Warn != "" {
			lines = append(lines, "      "+choice.Warn)
		}
	}
	return append(lines, "  [esc] cancel")
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
func modalFooter(md modal) string {
	switch md.Kind {
	case modalPrompt, modalRename:
		return "enter confirm  esc cancel  ctrl+c quit"
	case modalConfirm:
		// The answers are listed in full in the pane directly above, and they
		// differ between a worktree session and a main-checkout one, so
		// repeating them here would only risk disagreeing with them.
		return "answer above  esc cancel  ctrl+c quit"
	case modalList:
		// ctrl+j/ctrl+k rather than j/k, which are filter text here. This is
		// the one place M4 departs from the sidebar's keymap, so it is said
		// out loud (#42).
		return "type to filter  ctrl+j/ctrl+k move  enter jump  esc cancel"
	case modalPicker:
		return pickerFooter(md.List.markedCount())
	case modalHelp:
		return "esc to close"
	}
	return ""
}
