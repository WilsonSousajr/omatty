// Drawing the modal surfaces.
//
// A modal replaces the terminal pane's *content* and never its *geometry*, so
// the PTY behind one stays sized to the same box. That is what keeps issue #95
// fixed for every surface rather than only for the prompt, and it is why the
// pane is not shrunk to make room the way the note editor shrinks the review
// column: doing that here would clip the terminal's last row (#75).

package ui

import (
	"charm.land/lipgloss/v2"
)

// modalLines is the open surface's body. It returns nil when nothing is open,
// which renderTerminal never asks for.
func (m *Model) modalLines() []string {
	switch m.modal.Kind {
	case modalPrompt, modalRename:
		return m.editorLines()
	case modalConfirm:
		return m.confirmLines()
	case modalList, modalPicker, modalAdopt:
		return m.pickLines()
	case modalHelp:
		return m.helpLines()
	}
	return nil
}

// keyHelp is one row of the keymap: a keystroke and what it does. A named pair
// rather than a [2]string, so the render loop reads Key and Does instead of
// indexing an anonymous array (#103).
type keyHelp struct {
	Key  string
	Does string
}

// leaderKeys is every leader binding, in the order an operator meets them.
// This is the one place the full keymap is written down: the footer shows a
// working subset, because it is truncated to the window (#30, #103).
var leaderKeys = []keyHelp{
	{"j / k", "next / previous session, wrapping"},
	{"] / [", "next / previous project"},
	{"/", "jump to a session by name or project"},
	{"n", "new session on the main checkout"},
	{"N", "new session on a fresh worktree"},
	{"a", "register a project claude already knows"},
	{"A", "adopt a session claude already knows"},
	{"R", "rename the selected session"},
	{"x", "archive the session, or forget an empty project"},
	{"r", "restart a crashed session"},
	{"d", "open or close the diff pane"},
	{"f", "open or close the file tree"},
	{"?", "this list"},
	{"q", "quit"},
}

// claudeKeys are keys omatty does not own. They reach the session untouched,
// and they are listed because nothing else says so: pgup/pgdn always scrolled
// Claude's transcript and looked broken only because it was undocumented, and
// a modifier-drag is what selecting text costs now that omatty asks the
// terminal for the wheel (#107).
//
// Which modifier is the terminal's business, not omatty's: Ghostty, kitty,
// xterm and Alacritty bypass mouse reporting on shift, while Apple Terminal
// and iTerm2 use option. Naming only shift sent half the operators dragging
// out a stream of escape sequences instead of a selection.
//
// The drag is still how you take text off the screen yourself. It is listed
// beside the copy the program makes, which since #212 reaches the host
// clipboard on its own - the two answer different questions, and listing
// only one of them was what made copy look broken.
var claudeKeys = []keyHelp{
	{"pgup / pgdn", "scroll the transcript"},
	{"shift/opt+drag", "select text yourself (your terminal picks the modifier)"},
	{"claude's copy", "reaches your clipboard on its own"},
	{"paste", "goes to this pane as pasted text; it does not submit"},
}

// reviewKeys are the review column's own bindings. They live here because the
// two review footers are truncated to the window just as the main one is, and
// these are the keys that came off the end when reviewFooter was cut to fit
// (#103).
var reviewKeys = []keyHelp{
	{"j / k", "move the cursor, or scroll a preview"},
	{"r", "reload the diff, or re-list the tree"},
	{"h / l", "pan the diff, tree or preview sideways"},
	{"wheel sideways", "pan too - shift+wheel where the terminal sends it"},
	{"0", "jump back to the left edge"},
	{"M A D R", "a tree row the session modified, added, deleted or renamed"},
	{"/", "filter the tree as you type; enter keeps it, esc clears it"},
	{"a", "attach the tree row or previewed file to the prompt as @path"},
	{"o", "jump between a diff line and the file at that line, both ways"},
	{"c", "comment on the line under the cursor"},
	{"S", "submit the queued comments"},
	{"esc", "leave the column"},
}

// helpChrome is what helpLines spends on anything but a keymap row: the
// closing hint. The title and the blank under it went when the header row
// began naming the open modal (#177, #188); those two rows are keymap now.
const helpChrome = 1

// helpRows is how many keymap rows the help modal shows at once.
func (m *Model) helpRows() int {
	_, h := PaneSize(m.width, m.height, m.review.Open)
	return max(h-helpChrome, 1)
}

// helpLines draws the full keymap, windowed to the pane. It exists because the
// footer constant outgrew the window: at 114 columns it was already truncating
// `ctrl+o f` before M4 added four more keys (#103).
//
// It scrolls rather than trusting the list to fit. The body is 16 rows and the
// pane is the window minus three, so on the 20-row window the M4 smoke test
// uses, the entries at the bottom - including the ones #107 had just added -
// were cut off with no way to reach them (#103, #107). The body carries no
// title of its own: the header row names the modal (#177, #188).
func (m *Model) helpLines() []string {
	w, _ := PaneSize(m.width, m.height, m.review.Open)
	body, rows := helpBody(m.leader, w), m.helpRows()
	start := min(max(m.modal.HelpOffset, 0), max(len(body)-rows, 0))
	end := min(start+rows, len(body))
	lines := append([]string{}, body[start:end]...)
	if len(body) > rows {
		return append(lines, "j/k scroll  esc close  ctrl+c quit")
	}
	return append(lines, "esc to close  ctrl+c quit")
}

// helpBody is one line per binding, keys padded into a column and descriptions
// trimmed to the pane. A narrow pane loses the description rather than wrapping
// the key away from what it does.
func helpBody(leader string, width int) []string {
	gutter := helpGutter(leader)
	lines := make([]string, 0, len(leaderKeys)+len(reviewKeys)+len(claudeKeys)+4)
	for _, k := range leaderKeys {
		lines = append(lines, helpRow(leader+" "+k.Key, k.Does, gutter, width))
	}
	for _, section := range []struct {
		title string
		keys  []keyHelp
	}{
		{"in the review column", reviewKeys},
		{"in the session", claudeKeys},
	} {
		lines = append(lines, "", section.title)
		for _, k := range section.keys {
			lines = append(lines, helpRow(k.Key, k.Does, gutter, width))
		}
	}
	return lines
}

// helpRow draws one binding.
func helpRow(key, does string, gutter, width int) string {
	return fitLine("  "+padRight(key, gutter)+"  "+does, width)
}

// helpGutter is the key column's width: the longest key in either table, so a
// new binding widens the column instead of pushing its description out of line
// with every other one (#103).
func helpGutter(leader string) int {
	w := 0
	for _, k := range leaderKeys {
		w = max(w, lipgloss.Width(leader+" "+k.Key))
	}
	for _, table := range [][]keyHelp{reviewKeys, claudeKeys} {
		for _, k := range table {
			w = max(w, lipgloss.Width(k.Key))
		}
	}
	return w
}

// confirmLines draws the question and one line per answer. The answers are
// spelled out rather than abbreviated to y/n, because one of them discards
// uncommitted work and the operator should read what they are agreeing to.
//
// The question is elided in its middle rather than cut off the right: a long
// session title used to take the closing quote and the question mark with it,
// leaving two similarly-prefixed sessions indistinguishable in the one line
// that says which one is about to be destroyed (#40).
func (m *Model) confirmLines() []string {
	c := m.modal.Confirm
	w, _ := PaneSize(m.width, m.height, m.review.Open)
	lines := []string{"", elideMiddle(c.Question, w), ""}
	if c.Note != "" {
		lines = append(lines, fitLine(c.Note, w), "")
	}
	for _, choice := range c.Choices {
		lines = append(lines, "  ["+choice.Key+"] "+choice.Label)
		if choice.Warn != "" {
			lines = append(lines, "      "+choice.Warn)
		}
	}
	return append(lines, "  [esc] cancel")
}

// elideMiddle shortens s to width by replacing its middle with an ellipsis, so
// both ends survive. Used where the ends carry meaning the middle does not: a
// quoted title's closing quote and the question mark after it.
func elideMiddle(s string, width int) string {
	if width <= 1 || lipgloss.Width(s) <= width {
		return s
	}
	r := []rune(s)
	head := (width - 1) / 2
	tail := width - 1 - head
	return string(r[:head]) + "…" + string(r[len(r)-tail:])
}

// editorLines draws the one-line input: a blank lead-in, the label and buffer
// with a cursor block, then how to get out. The keys are repeated in the
// footer, but an operator looking at a box wants them next to the box.
func (m *Model) editorLines() []string {
	w, _ := PaneSize(m.width, m.height, m.review.Open)
	return []string{
		"",
		editLine(m.editorLabel(), m.modal.Editor.Buffer, w),
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
	return "new session title (blank: named by its first prompt)"
}

// modalNames is what the header row calls each surface that opens one way
// (#177), in sentence case; a kind not listed - none - names nothing.
var modalNames = map[modalKind]string{
	modalRename: "rename", modalConfirm: "confirm", modalList: "switch",
	modalPicker: "register project", modalAdopt: "adopt session", modalHelp: "keys",
}

// modalName is the open surface's name: one per surface as opened. The
// prompt is one kind opened two ways, so it is named by how it was opened.
func modalName(md modal) string {
	if md.Kind != modalPrompt {
		return modalNames[md.Kind]
	}
	if md.Editor.Worktree {
		return "new worktree session"
	}
	return "new session"
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
	case modalAdopt:
		return adoptFooter(md.List.markedCount())
	case modalHelp:
		// The body already carries its own closing hint, so this names the way
		// out that every other modal footer names and the body does not.
		return "esc close  ctrl+c quit"
	}
	return ""
}
