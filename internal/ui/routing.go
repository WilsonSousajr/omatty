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
	// Any keypress acknowledges the last error and the startup notice alike:
	// both displace the keymap, and neither is worth keeping once the operator
	// has started working (#43).
	m.lastErr, m.notice = "", ""
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
	if m.review.Filter.Active {
		return focusFilter, true
	}
	if m.review.Focused {
		return focusReview, true
	}
	// The row, not the terminal: a stopped session has no terminal and is
	// still the pane that owns the keys. Asking about the terminal sent a
	// stopped pane's keys to omatty's commands, where a bare q quit (#318).
	return focusTerminal, m.paneSelected()
}

// paneSelected reports whether a session row is selected, running or not.
func (m *Model) paneSelected() bool { return m.Selected() != "" }

// dispatch hands a plain keystroke to the focused pane.
func (m *Model) dispatch(target focusTarget, msg tea.KeyPressMsg) tea.Cmd {
	switch target {
	case focusNote:
		return m.onNoteKey(msg)
	case focusFilter:
		return m.onFilterKey(msg)
	case focusReview:
		return m.onPaneKey(msg.Keystroke())
	default:
		if term := m.focusedTerminal(); term != nil {
			// Typing is use, whatever the transcript says (#319).
			m.markActive(m.Selected())
			return term.Update(msg)
		}
		return m.onStoppedKey(msg)
	}
}

// onPaneKey picks the handler for the review column's current view: the three
// views share a focus target but not a keymap (#24).
func (m *Model) onPaneKey(key string) tea.Cmd {
	if m.pageKey(key) {
		return nil
	}
	switch m.review.View {
	case ViewTree:
		return m.onTreeKey(key)
	case ViewPreview:
		return m.onPreviewKey(key)
	case ViewGate:
		return m.onGateKey(key)
	case ViewTracker:
		return m.onTrackerKey(key)
	case ViewTrackerItem:
		return m.onTrackerItemKey(key)
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
		// The leader closes the surface and arms itself rather than being fed
		// to it. An open modal leaves the terminal unfocused, so keys.Router
		// never armed on its own here: `ctrl+o q` appended a literal q to a
		// session title, and in the help box did nothing at all (#41, #103).
		if msg.Keystroke() == m.leader {
			m.modal = modal{}
			m.router.Arm()
			return nil
		}
		return m.onModalKey(msg)
	}
	return m.navigate(msg.Keystroke())
}

// openModal takes the keyboard for a surface, closing the note editor if one
// is open.
//
// The note editor is a fourth keyboard-owning surface that predates modalKind
// and sits outside it, so nothing stopped a rename box opening over an active
// note: two input lines were drawn at once, both panes carried the focused
// border, and only one of them received keys. Exactly one surface owns the
// keyboard, which is what the single Kind field is for (#41).
func (m *Model) openModal(md modal) {
	m.review.Note = noteEditor{}
	m.modal = md
}

// navigate runs a command key while no prompt is open.
func (m *Model) navigate(key string) tea.Cmd {
	if move, ok := m.cursorMove(key); ok {
		return m.moveCursor(move)
	}
	switch key {
	case "n":
		m.openModal(modal{Kind: modalPrompt})
	// Keystroke() spells a shifted letter with the base key in lower case, so
	// a terminal reporting the modifier gives "shift+n"; the bare "N" is
	// accepted too, because a legacy terminal cannot report shift at all. The
	// upper-case "shift+N" spelling never occurs and was dead (issue #87).
	case "shift+n", "shift+N", "N":
		m.openModal(modal{Kind: modalPrompt, Editor: lineEditor{Worktree: true}})
	default:
		return m.paneCommand(key)
	}
	return nil
}

// cursorMove is the sidebar's two axes: j/k walk sessions, ] and [ walk
// projects (#130). Split from navigate so each arm stays a one-liner under
// the statement budget. One spelling each for the brackets: they are
// unshifted on a US layout, which is why they were chosen over J/K
// (#87, #122).
func (m *Model) cursorMove(key string) (func(), bool) {
	switch key {
	case "j":
		return m.sidebar.MoveDown, true
	case "k":
		return m.sidebar.MoveUp, true
	case "]":
		return m.sidebar.NextProject, true
	case "[":
		return m.sidebar.PrevProject, true
	}
	return nil, false
}

// paneCommand runs the leader commands that act on the focused session.
func (m *Model) paneCommand(key string) tea.Cmd {
	if cmd, ok := m.lifecycleCommand(key); ok {
		return cmd
	}
	if cmd, ok := m.columnCommand(key); ok {
		return cmd
	}
	switch key {
	case "m":
		return m.toggleMouse()
	case "q":
		return tea.Quit
	}
	return m.modalCommand(key)
}

// columnCommand is the four keys that open, switch or close the review column.
// Split off paneCommand when the tracker pushed it past the statement limit,
// and the four belong together: one column, four faces (#396).
func (m *Model) columnCommand(key string) (tea.Cmd, bool) {
	switch key {
	case "d":
		return m.toggleView(ViewDiff), true
	case "f":
		return m.toggleView(ViewTree), true
	case "g":
		return m.toggleView(ViewGate), true
	case "i":
		return m.toggleTracker(), true
	case "z":
		m.toggleZoom()
		return nil, true
	}
	return nil, false
}

// lifecycleCommand is the keys that act on the focused session's own state:
// restart starts another process at once (#15), stop leaves the pane waiting
// for enter (#318), and revert puts its worktree back to the start of its last
// turn (#334). Split off paneCommand when the second pushed it past the
// statement limit, and they belong together - each one throws something away
// and asks first or says so.
func (m *Model) lifecycleCommand(key string) (tea.Cmd, bool) {
	switch key {
	case "r":
		return m.restartSelected(), true
	case "s":
		return m.stopSelected(), true
	case "u":
		return m.askRevert(), true
	}
	return nil, false
}

// modalCommand opens a surface that takes the keyboard. It is a third table
// beside navigate and paneCommand for the reason paneCommand was split off in
// the first place: M4's keys would push one switch past gocyclo's limit.
func (m *Model) modalCommand(key string) tea.Cmd {
	// Every shifted key here takes three spellings, and all three occur.
	// Keystroke() spells a shifted letter with the base key in LOWER case, so a
	// terminal reporting the modifier sends "shift+r"; a legacy one that cannot
	// report shift sends the bare "R"; and one that shifts the base key too
	// sends "shift+R".
	//
	// Matching any two of the three is how the help key and then the adoption
	// key each shipped opening nothing on a modern terminal, found by the smoke
	// test and by no unit test - they send the legacy spelling. Rename carried
	// the same gap, and a comment asserting the opposite of the one three lines
	// below it (#87, #103, #122).
	if m.renameCommand(key) {
		return nil
	}
	switch key {
	case "x":
		m.openConfirm()
	case "/":
		return m.openSwitcher()
	case "a":
		return m.openDiscovery()
	case "shift+a", "shift+A", "A":
		return m.openAdoption()
	// "?" is shift+/ on a US layout, so it takes the same three spellings.
	// Matching "?" alone left the whole keymap unreachable (#103).
	case "shift+/", "shift+?", "?":
		m.openModal(modal{Kind: modalHelp})
	}
	return nil
}

// renameCommand opens one of the two rename boxes and reports whether the key
// was one of them. Split off modalCommand for the reason that table was split
// off navigate: #151's second rename key pushed it past the length limit, and
// the two belong together - one names a session, the other names the branch it
// is on.
func (m *Model) renameCommand(key string) bool {
	switch key {
	case "shift+r", "shift+R", "R":
		// Lower-case r is restart, so a missed spelling here is silent: it
		// restarts nothing rather than failing to rename.
		m.openRename()
	case "shift+b", "shift+B", "B":
		// The branch rather than the title: a worktree's branch is a name git
		// and the filesystem hold too, so it gets a key of its own (#151).
		m.openBranchRename()
	default:
		return false
	}
	return true
}
