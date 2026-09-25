// One item in full (#397): the tracker's child view, showing an issue's or a
// pull request's body and comments wrapped to the column.
//
// The read happens on enter and nowhere else. A list poll that also fetched
// every body would be a per-item call inside a list, which is what tripped
// GitHub's secondary rate limit before (#358), so this is one call for the one
// item the operator asked for, cached until r asks again.

package ui

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// ItemFunc reads one issue or pull request in full. Injected so ui never runs gh
// itself; the arguments are the project's root and the item's number.
type ItemFunc func(projectRoot string, number int) (forge.Detail, error)

// ForgeItemFuncs is the pair of readers the tracker needs. Two functions rather
// than one with a flag, because gh has two subcommands and the tracker always
// knows which list its row came from.
type ForgeItemFuncs struct {
	Issue ItemFunc
	PR    ItemFunc
}

// noItem is the Deps.Item default for either half: with nothing wired there is
// no gh to ask.
func noItem(string, int) (forge.Detail, error) { return forge.Detail{}, forge.ErrNoGH }

// itemKey identifies one read item. A struct rather than a joined string so the
// three parts cannot be confused, and so archive's reflection guard skips the
// map without needing to be told (it looks for string keys).
type itemKey struct {
	Project string
	PR      bool
	Number  int
}

// ItemLoadedMsg carries one item into Update. Exported so tests can send one.
type ItemLoadedMsg struct {
	Key    itemKey
	Detail forge.Detail
	Err    error
}

// openItemAtCursor reads the row under the tracker's cursor and shows it. The
// rule is not a row: its Number is zero and there is nothing to open.
func (m *Model) openItemAtCursor() tea.Cmd {
	rows := m.trackerRows()
	if m.review.Tracker.Cursor >= len(rows) {
		return nil
	}
	r := rows[m.review.Tracker.Cursor]
	if r.Kind == rowRule {
		return nil
	}
	m.review.View, m.review.ColOffset = ViewTrackerItem, 0
	m.review.Tracker.ItemPR, m.review.Tracker.ItemNumber = r.Kind == rowPR, r.Number
	m.review.Tracker.ItemOffset = 0
	m.contentChanged()
	return m.readItem(m.itemKeyAtCursor(), false)
}

// itemKeyAtCursor is the open item's key.
func (m *Model) itemKeyAtCursor() itemKey {
	t := m.review.Tracker
	return itemKey{Project: t.Project, PR: t.ItemPR, Number: t.ItemNumber}
}

// readItem asks for one item unless it is already held or in flight. again
// forces the call, which is what r is for: an item's comments move on.
func (m *Model) readItem(key itemKey, again bool) tea.Cmd {
	if m.ghMissing || m.notGitHub[key.Project] || m.itemPending[key] {
		return nil
	}
	if _, held := m.items[key]; held && !again {
		return nil
	}
	m.itemPending[key] = true
	read, root := m.itemReader(key.PR), m.projectRoot(key.Project)
	return func() tea.Msg {
		item, err := read(root, key.Number)
		return ItemLoadedMsg{Key: key, Detail: item, Err: err}
	}
}

func (m *Model) itemReader(pr bool) ItemFunc {
	if pr {
		return m.itemFuncs.PR
	}
	return m.itemFuncs.Issue
}

// onItem stores one read. gh missing stops every poll, as it does for the lists;
// any other failure is held against that item so the view can say the read
// failed rather than showing an empty body.
func (m *Model) onItem(msg ItemLoadedMsg) tea.Cmd {
	delete(m.itemPending, msg.Key)
	if msg.Err == nil {
		delete(m.itemFailed, msg.Key)
		m.items[msg.Key] = msg.Detail
		m.contentChanged()
		return nil
	}
	if errors.Is(msg.Err, forge.ErrNoGH) {
		m.loseGH()
		return nil
	}
	if !m.itemFailed[msg.Key] {
		slog.Warn("reading an item", "project", msg.Key.Project, "number", msg.Key.Number, "err", msg.Err)
	}
	m.itemFailed[msg.Key] = true
	m.contentChanged()
	return nil
}

// onTrackerItemKey is the item view's keymap: scroll it, read it again, or go
// back to the list - the preview's shape over the tree (#24).
func (m *Model) onTrackerItemKey(key string) tea.Cmd {
	switch key {
	case "j", "down":
		m.scrollItem(1)
	case "k", "up":
		m.scrollItem(-1)
	case "esc":
		m.review.View, m.review.ColOffset = ViewTracker, 0
		m.contentChanged()
	case "ctrl+c":
		m.review.Focused = false
	case "r":
		return m.readItem(m.itemKeyAtCursor(), true)
	default:
		m.panKey(key)
	}
	return nil
}

func (m *Model) scrollItem(delta int) {
	lines := len(m.itemLines())
	_, h := PaneSize(m.width, m.height, true)
	m.review.Tracker.ItemOffset = min(max(m.review.Tracker.ItemOffset+delta, 0), max(lines-h, 0))
}

// renderTrackerItem draws the open item, or says why it cannot.
func (m *Model) renderTrackerItem(_, h int) []string {
	return window(m.itemLines(), m.review.Tracker.ItemOffset, h)
}

// itemLines is the item as rows: a heading, the body, then each comment under
// its own rule, all wrapped to the column. Wrapped rather than cut, because an
// issue body is prose and prose read through a 35-cell window one line at a time
// is not read at all.
func (m *Model) itemLines() []string {
	key := m.itemKeyAtCursor()
	item, held := m.items[key]
	if !held {
		return m.itemNote(key)
	}
	w := reviewContentWidth(m.width)
	lines := append(m.itemHead(item, w), wrapBlock(item.Body, w)...)
	for _, c := range item.Comments {
		lines = append(lines, "", fitLine(labelledRule(c.Author+" · "+AgeString(m.clock(), c.At), w), w))
		lines = append(lines, wrapBlock(c.Body, w)...)
	}
	if item.Truncated {
		notice := "… the rest of this item was not read (over " + strconv.Itoa(forge.DetailMax>>10) + " KiB)."
		lines = append(lines, "")
		lines = append(lines, wrapBlock(notice, w)...)
	}
	return lines
}

// itemHead is the item's own heading: what it is, and who opened it when. Every
// line the view draws is wrapped, including these: a line that is prose in a
// wrapped view but cut at the edge reads as a rendering bug, which is how the
// truncation notice was found.
func (m *Model) itemHead(item forge.Detail, w int) []string {
	by := "opened by " + item.Author
	if age := AgeString(m.clock(), item.Created); age != "" {
		by += " · " + age + " ago"
	}
	head := wrapBlock("#"+strconv.Itoa(item.Number)+"  "+item.Title, w)
	return append(append(head, wrapBlock(by, w)...), "")
}

// itemNote is the state before an item is held: reading, failed, or gh gone.
// Distinct states, because an empty pane reads as "this issue says nothing".
func (m *Model) itemNote(key itemKey) []string {
	number := "#" + strconv.Itoa(key.Number)
	w := reviewContentWidth(m.width)
	switch {
	case m.ghMissing:
		return wrapBlock("gh is not installed, so "+number+" cannot be read.", w)
	case m.itemFailed[key]:
		return wrapBlock(number+" could not be read. Press r to try again.", w)
	}
	return wrapBlock("reading "+number+"...", w)
}

// wrapBlock wraps one block of text to w, keeping its own line breaks: a body is
// markdown, and its paragraphs and lists are part of what it says.
func wrapBlock(text string, w int) []string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		out = append(out, strings.Split(ansi.Wrap(expandTabs(line), w, " "), "\n")...)
	}
	return out
}

// trackerItemMaxWidth is how far the item can pan. Wrapped rows fit the column
// by construction, so this is all but always the column's width; it is measured
// anyway, because a run of wide runes can land a row one cell over.
func (m *Model) trackerItemMaxWidth() int {
	widest := 0
	for _, line := range m.itemLines() {
		widest = max(widest, lipgloss.Width(line))
	}
	return widest
}

// trackerItemTitle names the item. The project stays in it: with several
// registered, "#397" alone does not say whose.
func (m *Model) trackerItemTitle(budget int) string {
	t := m.review.Tracker
	parts := []titlePart{
		{text: "#" + strconv.Itoa(t.ItemNumber), priority: keepAlways},
		{text: t.Project, priority: keepAlways},
	}
	if item, held := m.items[m.itemKeyAtCursor()]; held && item.Truncated {
		parts = append(parts, titlePart{text: "part read", priority: keepFlag})
	}
	return joinTitle(parts, budget)
}

// withItemMaps allocates the item state (#397). Keyed by itemKey, so archive's
// reflection guard passes it over and forgetProject clears it by project.
func (m *Model) withItemMaps() *Model {
	m.items = map[itemKey]forge.Detail{}
	m.itemPending = map[itemKey]bool{}
	m.itemFailed = map[itemKey]bool{}
	return m
}

// forgetProjectItems drops one project's read items.
func (m *Model) forgetProjectItems(project string) {
	for key := range m.items {
		if key.Project == project {
			delete(m.items, key)
			delete(m.itemPending, key)
			delete(m.itemFailed, key)
		}
	}
}
