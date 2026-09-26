// Working from an issue (#398): the three keys that turn reading into doing.
//
//	n  a worktree session named and branched from the issue
//	a  the item's reference typed into the selected session's composer
//	b  the item in the operator's own browser
//
// None of them sends a prompt. `a` pastes without a carriage return, exactly as
// the tree's `a` does for an @path (#199, invariant 8): omatty types for you and
// never submits a turn, which is the line the roadmap's orchestrator table
// draws - "there is no mode in which omatty works while nobody is reading".

package ui

import (
	"log/slog"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/paste"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// BrowseFunc opens one item in the operator's browser. Injected so ui never runs
// gh itself (invariant 4 in spirit).
type BrowseFunc func(projectRoot string, number int) error

// noBrowse is the Deps.Browse default: it names the missing wiring rather than
// appearing to succeed, because a browser that silently never opens is the one
// failure the operator cannot see from inside the terminal.
func noBrowse(_ string, number int) error {
	return errNoBrowser(number)
}

func errNoBrowser(number int) error {
	return &browseError{number: number}
}

// browseError says no browser is wired, naming the item it would have opened.
type browseError struct{ number int }

func (e *browseError) Error() string {
	return "no browser configured, so #" + strconv.Itoa(e.number) + " cannot be opened"
}

// BrowsedMsg carries a browse attempt's outcome into Update. Exported so tests
// can send one.
type BrowsedMsg struct {
	Number int
	Err    error
}

// trackerAction runs the three keys that act on the row under the cursor, and
// says whether it consumed the key. Shared by the list and the open item, so
// reading an issue and then starting work on it is one keypress either way.
func (m *Model) trackerAction(key string) (tea.Cmd, bool) {
	row, ok := m.actionTarget()
	if !ok {
		return nil, false
	}
	switch key {
	case "n":
		return m.startSessionOnIssue(row), true
	case "a":
		return m.attachItem(row), true
	case "b":
		return m.browseItem(row), true
	}
	return nil, false
}

// actionTarget is the item the keys act on: the row under the list's cursor, or,
// from the child view, the item it has open.
//
// The child does not ask the list. It knows what it is showing, and a list that
// has gone out from under it - gh lost, the project forgotten, a poll that
// answered with nothing - would otherwise swallow the keypress in silence. Found
// by TestTrackerWork_TheKeysWorkFromAnOpenItem_issue398.
func (m *Model) actionTarget() (trackerRow, bool) {
	if m.review.View != ViewTrackerItem {
		return m.trackerRowAtCursor()
	}
	t := m.review.Tracker
	if t.ItemNumber == 0 {
		return trackerRow{}, false
	}
	kind := rowIssue
	if t.ItemPR {
		kind = rowPR
	}
	return trackerRow{Kind: kind, Number: t.ItemNumber, Title: m.openItemTitle()}, true
}

// openItemTitle is the open item's title: the read item's own, or the list row's
// while the read is still in flight, so n never names a branch after a number
// alone.
func (m *Model) openItemTitle() string {
	if item, held := m.items[m.itemKeyAtCursor()]; held {
		return item.Title
	}
	if row, ok := m.trackerRowAtCursor(); ok {
		return row.Title
	}
	return ""
}

// trackerRowAtCursor is the row under the list's cursor. The rule is not a row:
// it is a label, with nothing to act on.
func (m *Model) trackerRowAtCursor() (trackerRow, bool) {
	rows := m.trackerRows()
	if m.review.Tracker.Cursor >= len(rows) {
		return trackerRow{}, false
	}
	row := rows[m.review.Tracker.Cursor]
	return row, row.Kind != rowRule
}

// startSessionOnIssue creates a worktree session named and branched from the
// issue, and hands the keyboard to it so the next thing typed is a prompt.
//
// The branch is decided at creation because `git worktree add -b` bakes it into
// a directory and into state.json (#151), so this path passes the name rather
// than leaving a placeholder for the first prompt to rename. It is slugged for
// the reason every other branch name is: the issue's own punctuation must reach
// neither a ref nor a path (#127).
func (m *Model) startSessionOnIssue(row trackerRow) tea.Cmd {
	number := "#" + strconv.Itoa(row.Number)
	if row.Kind == rowPR {
		m.lastErr = number + " already has a branch; n starts a session on an issue"
		return nil
	}
	project := m.review.Tracker.Project
	title := number + " " + row.Title
	branch := registry.Slug(strconv.Itoa(row.Number) + " " + row.Title)
	cmd, err := m.addSession(project, title, branch, true)
	if err != nil {
		slog.Error("starting a session on an issue", "project", project, "issue", row.Number, "err", err)
		m.lastErr = err.Error()
		return nil
	}
	m.review.Focused = false
	return cmd
}

// attachItem types the item's reference into the selected session's composer -
// bracketed, and with no carriage return, so the operator finishes the prompt
// (invariant 8) - then hands the keys back, the way attachPath does (#199).
func (m *Model) attachItem(row trackerRow) tea.Cmd {
	id := m.Selected()
	term := m.terms[id]
	if term == nil {
		m.lastErr = "no session to type into in " + m.review.Tracker.Project + "; press n to start one"
		return nil
	}
	m.review.Focused = false
	return term.SendInput(paste.BracketedText(itemReference(row) + " "))
}

// itemReference names the item in words a prompt can carry: "issue #399", not a
// bare number, since a session's project holds both kinds and claude reads it as
// text either way.
func itemReference(row trackerRow) string {
	kind := "issue"
	if row.Kind == rowPR {
		kind = "pull request"
	}
	return kind + " #" + strconv.Itoa(row.Number)
}

// browseItem hands the item to the operator's browser off the Update goroutine:
// gh spawns an opener, and a slow one must not hold the frame.
func (m *Model) browseItem(row trackerRow) tea.Cmd {
	browse, root, number := m.browse, m.projectRoot(m.review.Tracker.Project), row.Number
	return func() tea.Msg {
		return BrowsedMsg{Number: number, Err: browse(root, number)}
	}
}

// onBrowsed reports a browser that would not open. A silent failure is the one
// the operator cannot see from inside the terminal.
func (m *Model) onBrowsed(msg BrowsedMsg) tea.Cmd {
	if msg.Err != nil {
		slog.Warn("opening an item in the browser", "number", msg.Number, "err", msg.Err)
		m.lastErr = msg.Err.Error()
	}
	return nil
}
