// Drawing the modal surfaces.
//
// A modal replaces the terminal pane's *content* and never its *geometry*, so
// the PTY behind one stays sized to the same box. That is what keeps issue #95
// fixed for every surface rather than only for the prompt, and it is why the
// pane is not shrunk to make room the way the note editor shrinks the review
// column: doing that here would clip the terminal's last row (#75).

package app

import (
	"charm.land/lipgloss/v2"
)

// modalLines is the open surface's body. It returns nil when nothing is open,
// which renderTerminal never asks for.
func (m *Model) modalLines() []string {
	switch m.modal.Kind {
	case modalPrompt, modalRename:
		return m.editorLines()
	case modalConfirm, modalRevert:
		return m.confirmLines()
	case modalList, modalPicker, modalAdopt, modalAgent, modalProjectAgent:
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
	{"tab", "fold or unfold the project, keeping its sessions"},
	{"/", "jump to a session by name or project"},
	{"n", "new session on the main checkout"},
	{"N", "new session on a fresh worktree"},
	{"a", "register a project claude already knows"},
	{"A", "adopt a session claude already knows"},
	{"R", "rename the selected session"},
	{"B", "rename a worktree session's branch"},
	{"c", "choose the agent the selected project's new sessions run"},
	{"x", "archive the session, or forget an empty project"},
	{"r", "restart a crashed session"},
	{"s", "stop the session's process, keeping it; enter resumes it"},
	{"u", "put the session's worktree back to the start of its last turn"},
	{"p", "ship a green session: push and open its pull request, or merge a green one"},
	{"d", "open or close the diff pane"},
	{"f", "open or close the file tree"},
	{"g", "open or close the gate pane"},
	{"i", "open or close this project's issues and pull requests"},
	{"z", "zoom the review column over the session, or back"},
	{"m", "hand the mouse back to your terminal, or take it back"},
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
// The drag is still how you take text off the screen yourself, and it is
// still the quick answer for a one-off selection. It is listed beside the copy
// the program makes, which since #212 reaches the host clipboard on its own -
// the two answer different questions, and listing only one of them was what
// made copy look broken. For anything longer than one drag, `m` above hands
// the mouse back entirely and the terminal behaves as it does everywhere
// else (#217).
var claudeKeys = []keyHelp{
	{"pgup / pgdn", "scroll the transcript"},
	{"drag in a pane", "copies that pane's text, clipped to it"},
	{"shift/opt+drag", "select across omatty's own panes (your terminal picks the modifier)"},
	{"claude's copy", "reaches your clipboard on its own"},
	{"paste", "goes to this pane as pasted text; it does not submit"},
}

// The review column's own bindings, one table per face. They live here because
// the column's footers are truncated to the window just as the main one is, and
// these are the keys that came off the end when reviewFooter was cut to fit
// (#103).
//
// One table per face, not one for the column: a single table written for the
// diff and the tree went stale as the gate (M9) and the tracker (M14) added
// their own keys, and a key another face happened to document - S, a - hid
// that its own face had none (#422). TestHelp_everyColumnKeyIsDocumented_issue422
// checks each face's handlers against its table.

// columnKeys work the same on every face.
var columnKeys = rowsOf(columnBind.Down, columnBind.Up, columnBind.Top, columnBind.Bottom,
	columnBind.HalfDown, columnBind.HalfUp, columnBind.Left, columnBind.Right, columnBind.Wheel,
	columnBind.Home, columnBind.Back, columnBind.Interrupt)

var diffKeys = rowsOf(diffBind.Comment, diffBind.CommentPart, diffBind.Delete, diffBind.Submit,
	diffBind.Scope, diffBind.Open, diffBind.NextFile, diffBind.PrevFile, diffBind.NextHunk,
	diffBind.PrevHunk, diffBind.Fold, diffBind.Reload)

var treeKeys = rowsOf(treeBind.Enter, treeBind.Read, treeBind.Generated, treeBind.Filter,
	treeBind.Attach, treeBind.Open, treeBind.Reload, treeBind.Changed, treeBind.Legend)

var gateKeys = rowsOf(gateBind.Fold, gateBind.Send, gateBind.Rerun, gateBind.Search,
	gateBind.NextMatch, gateBind.PrevMatch)

var trackerKeys = rowsOf(trackerBind.Read, trackerBind.Filter, trackerBind.Start,
	trackerBind.Attach, trackerBind.Browse, trackerBind.Reload, trackerBind.NextSection)

// helpSection is one titled block of the help modal below the leader keys.
type helpSection struct {
	Title string
	Keys  []keyHelp
	// Prefix is written before every key: the leader, for the leader's own
	// section, which #438 made a section like the others.
	Prefix string
}

// helpSections is the modal's order below the leader keys: the column's shared
// keys, each face's own, then the keys that reach the session untouched. It is
// the one list both helpBody and helpGutter walk, so a new section cannot be
// drawn without also being measured.
var helpSections = []helpSection{
	{Title: "anywhere in the review column", Keys: columnKeys},
	{Title: "in the diff", Keys: diffKeys},
	{Title: "in the file tree", Keys: treeKeys},
	{Title: "in the gate", Keys: gateKeys},
	{Title: "in the tracker", Keys: trackerKeys},
	{Title: "in the session", Keys: claudeKeys},
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
	body, rows := m.helpBody(w), m.helpRows()
	start := min(max(m.modal.HelpOffset, 0), max(len(body)-rows, 0))
	end := min(start+rows, len(body))
	lines := append([]string{}, body[start:end]...)
	return append(lines, m.helpFoot(w, len(body) > rows))
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
	if m.modal.Kind == modalBranch {
		return "rename branch"
	}
	if m.modal.Editor.Worktree {
		return "new branch (worktree)"
	}
	return "new session title (blank: named by its first prompt)"
}

// modalNames is what the header row calls each surface that opens one way
// (#177), in sentence case; a kind not listed - none - names nothing.
var modalNames = map[modalKind]string{
	modalRename: "rename", modalBranch: "rename branch", modalConfirm: "confirm", modalList: "switch",
	modalPicker: "register project", modalAdopt: "adopt session", modalHelp: "keys",
	modalAgent: "choose agent", modalProjectAgent: "project agent",
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
	if md.Kind == modalAgent || md.Kind == modalProjectAgent {
		return agentFooter
	}
	return surfaceFooter(md)
}

// surfaceFooter is every other surface's footer, split from modalFooter when
// the agent lists (#524) pushed it past the length limit.
func surfaceFooter(md modal) string {
	switch md.Kind {
	case modalPrompt, modalRename, modalBranch:
		return "enter confirm  esc cancel  ctrl+c quit"
	case modalConfirm, modalRevert:
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

// agentFooter is the agent lists' footer (#524): the switcher's movement,
// because j and k are filter text here too.
const agentFooter = "type to filter  ctrl+j/ctrl+k move  enter choose  esc cancel"
