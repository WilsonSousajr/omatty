// Key routing: which pane owns a keystroke, and what each omatty command key
// does. Split out of model.go when the review column made a third focus
// target, so model.go stays the state and the message router (#21).

package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/tui/keys"
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
	route := m.router.Next(msg.Keystroke(), focused)
	m.disarmQuit(route == keys.Swallow)
	switch route {
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
	if m.activeFilter().Active {
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
	// ctrl+c is the unconditional escape hatch, so an open surface cannot trap
	// the operator (issue #28). Inside a modal it closes the modal, as esc
	// does, because every shell and TUI reads ctrl+c in a field as "cancel
	// this": quitting there threw away the layout and any unsent notes (#749).
	// With a session focused this is never reached: ctrl+c belongs to Claude,
	// which uses it to interrupt a turn (invariant 1).
	if msg.Keystroke() == "ctrl+c" {
		return m.escapeHatch()
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

// escapeHatch is ctrl+c on omatty's own path: it closes an open modal, and
// quits only when nothing is open and no pane is focused (#28, #749).
func (m *Model) escapeHatch() tea.Cmd {
	if m.modalOpen() {
		m.modal = modal{}
		return nil
	}
	return tea.Quit
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
	// The keys are leaderBind's, which carry every spelling a shifted letter
	// arrives in (#87, #750).
	switch {
	case is(key, leaderBind.New):
		m.openModal(modal{Kind: modalPrompt})
	case is(key, leaderBind.NewWorktree):
		m.openModal(modal{Kind: modalPrompt, Editor: lineEditor{Worktree: true}})
	case is(key, leaderBind.Fold):
		return m.toggleFold(m.sidebar.CursorProject())
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
	switch {
	case is(key, leaderBind.Down):
		return m.sidebar.MoveDown, true
	case is(key, leaderBind.Up):
		return m.sidebar.MoveUp, true
	case is(key, leaderBind.NextProject):
		return m.sidebar.NextProject, true
	case is(key, leaderBind.PrevProject):
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
	switch {
	case is(key, leaderBind.Mouse):
		return m.toggleMouse()
	case is(key, leaderBind.Quit):
		return m.requestQuit()
	}
	return m.modalCommand(key)
}

// columnCommand is the four keys that open, switch or close the review column.
// Split off paneCommand when the tracker pushed it past the statement limit,
// and the four belong together: one column, four faces (#396).
func (m *Model) columnCommand(key string) (tea.Cmd, bool) {
	switch {
	case is(key, leaderBind.Diff):
		return m.toggleView(ViewDiff), true
	case is(key, leaderBind.Files):
		return m.toggleView(ViewTree), true
	case is(key, leaderBind.Gate):
		return m.toggleView(ViewGate), true
	case is(key, leaderBind.Tracker):
		return m.toggleTracker(), true
	case is(key, leaderBind.Zoom):
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
	switch {
	case is(key, leaderBind.Restart):
		return m.restartSelected(), true
	case is(key, leaderBind.Stop):
		return m.stopSelected(), true
	case is(key, leaderBind.Revert):
		return m.askRevert(), true
	case is(key, leaderBind.Ship):
		return m.shipSelected(), true
	}
	return nil, false
}

// modalCommand opens a surface that takes the keyboard. It is a third table
// beside navigate and paneCommand for the reason paneCommand was split off in
// the first place: M4's keys would push one switch past gocyclo's limit.
func (m *Model) modalCommand(key string) tea.Cmd {
	// Every shifted key here takes three spellings, and all three occur.
	// Matching any two of the three is how the help key and then the adoption
	// key each shipped opening nothing on a modern terminal, found by the smoke
	// test and by no unit test - they send the legacy spelling. Rename carried
	// the same gap (#87, #103, #122). The spellings live in leaderBind now, one
	// list per command (#750).
	if m.renameCommand(key) || m.projectCommand(key) || m.openCommand(key) {
		return nil
	}
	switch {
	case is(key, leaderBind.Switcher):
		return m.openSwitcher()
	case is(key, leaderBind.Discover):
		return m.openDiscovery()
	case is(key, leaderBind.Adopt):
		return m.openAdoption()
	}
	m.unknownLeaderKey(key)
	return nil
}

// openCommand opens one of the two surfaces that need nothing read first -
// the archive confirmation and the keymap - and reports whether the key was
// one. Split off modalCommand when #751's fall-through pushed it past the
// length limit, as renameCommand was.
func (m *Model) openCommand(key string) bool {
	switch {
	case is(key, leaderBind.Archive):
		m.openConfirm()
	case is(key, leaderBind.Help):
		m.openModal(modal{Kind: modalHelp})
	default:
		return false
	}
	return true
}

// unknownLeaderKey is the end of the leader's tables: a key none of them
// took. It says so, rather than looking like a command that ran and showed
// nothing (#751), tmux's "Unknown key". The key is still swallowed: the
// router gave it to omatty, and it never reaches the session (invariant 1).
//
// esc is let go quietly: after a prefix it is backing out, not a typo.
func (m *Model) unknownLeaderKey(key string) {
	if key == "esc" {
		return
	}
	m.notice = m.leader + " " + key + ": no such command · " + m.leader + " ? lists them"
}

// renameCommand opens one of the two rename boxes and reports whether the key
// was one of them. Split off modalCommand for the reason that table was split
// off navigate: #151's second rename key pushed it past the length limit, and
// the two belong together - one names a session, the other names the branch it
// is on.
func (m *Model) renameCommand(key string) bool {
	switch {
	case is(key, leaderBind.Rename):
		// Lower-case r is restart, so a missed spelling here is silent: it
		// restarts nothing rather than failing to rename.
		m.openRename()
	case is(key, leaderBind.RenameBranch):
		// The branch rather than the title: a worktree's branch is a name git
		// and the filesystem hold too, so it gets a key of its own (#151).
		m.openBranchRename()
	default:
		return false
	}
	return true
}

// projectCommand opens a surface about the selected project and reports
// whether the key was one. Split off modalCommand when ctrl+o c (#524) pushed
// it past the length limit, as renameCommand was.
func (m *Model) projectCommand(key string) bool {
	if !is(key, leaderBind.Agent) {
		return false
	}
	m.openProjectAgent()
	return true
}
