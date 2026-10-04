// The tracker (#396): the review column's fifth face, showing a project's open
// issues and open pull requests.
//
// It is the one view that belongs to a *project* rather than to a session. That
// is what makes it reachable on a project nobody has started a session in
// (#158), and it is why its state is keyed by project: following the sidebar
// between two sessions of one project must not throw the list away.
//
// Nothing here is stored (invariant 9). The rows are derived at render time
// from the two polls, the way a card's pull request is (#310).

package app

import (
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
)

// trackerList is the tracker's own state: whose lists are shown, and where the
// cursor is in them.
type trackerList struct {
	Project string
	listWindow
	// The open item (#397): which row is being read in full, and how far down it
	// is scrolled. Only meaningful while the view is ViewTrackerItem.
	ItemPR     bool
	ItemNumber int
	ItemOffset int
}

// trackerKind is what a drawn row is.
type trackerKind int

const (
	rowIssue trackerKind = iota
	rowPR
	rowRule // the labelled rule between the two lists
)

// trackerRow is one line of the tracker, flattened from both lists so the
// cursor, the scroll and the renderer all index the same thing.
type trackerRow struct {
	Kind    trackerKind
	Number  int
	Label   string // an issue's first label, or a pull request's CI mark
	Title   string
	Updated time.Time
	// Sigil writes a change's number its forge's way, "!" on GitLab (#449);
	// empty means "#", which is how every issue is written.
	Sigil string
	// Section is the list a rule heads, and Folded whether tab folded it
	// (#663). A folded rule is the one rule the cursor rests on: tab there is
	// what opens it again, so it is not #432's heading with nothing to act on.
	Section trackerKind
	Folded  bool
}

// stop reports whether the cursor may rest on the row: any item, and a
// folded heading.
func (r trackerRow) stop() bool { return r.Kind != rowRule || r.Folded }

// ref is the row's number as drawn: "#399", or "!400" for a GitLab change.
func (r trackerRow) ref() string {
	if r.Sigil == "" {
		return "#" + strconv.Itoa(r.Number)
	}
	return r.Sigil + strconv.Itoa(r.Number)
}

// TrackerCursor is the highlighted item's place among the items, from 0 -
// headings not counted, so a test walking items is not rewritten each time a
// heading is added (#432). Exported for tests, as ReviewCursor is.
func (m *Model) TrackerCursor() int {
	n, _ := m.trackerItemPosition()
	return n - 1
}

// toggleTracker opens the column on the tracker, switches to it, or closes it.
//
// Deliberately not toggleView: that one returns early when no session is
// selected, and this view's whole point is that it needs a project and not a
// session. Opening also asks for both lists now - a pane that showed only the
// last poll would be up to five minutes stale on the keypress that asked for
// it, which is renderGate's argument for running the gate on open (#231).
func (m *Model) toggleTracker() tea.Cmd {
	if m.review.Open && keptView(m.review.View) == ViewTracker {
		return m.closeColumn()
	}
	project := m.sidebar.CursorProject()
	if project == "" {
		return nil
	}
	wasOpen := m.review.Open
	m.review.Open, m.review.View, m.review.Focused, m.review.ColOffset = true, ViewTracker, true, 0
	m.review.Tracker = keptTracker(m.review.Tracker, project)
	m.contentChanged()
	m.moveTrackerCursor(0) // off a heading, onto a row (#432)
	return tea.Batch(m.resizeIfWidthChanged(wasOpen), m.readTrackerNow(project))
}

// keptTracker points the tracker at project, keeping the cursor only when it is
// already that project's: another project is another list, and a cursor carried
// onto it would rest on an unrelated row.
func keptTracker(t trackerList, project string) trackerList {
	if t.Project == project {
		return t
	}
	return trackerList{Project: project}
}

// readTracker asks for one project's two lists, subject to every refusal the
// polls already make - no gh, not GitHub, in flight, or asked a moment ago.
func (m *Model) readTracker(project string) tea.Cmd {
	return tea.Batch(m.pollProjectIssues(project), m.pollProjectPRs(project))
}

// readTrackerNow is readTracker asked for by the operator - r, or opening the
// tracker - so it goes past what a poll respects: the thirty-second floor, a
// stopped forge that may since have been fixed, and a forge that said it keeps
// no issues (#658). Each refused r without a word, which read as "not
// syncing". A read still in flight cannot be asked twice, and says so.
func (m *Model) readTrackerNow(project string) tea.Cmd {
	if m.issuePending[project] || m.prPending[project] {
		m.notice = "still reading " + project + "'s issues and " + m.label(project).Change + "s"
	}
	m.retryForge(project)
	delete(m.noTracker, project)
	delete(m.issueAsked, project)
	delete(m.prAsked, project)
	return m.readTracker(project)
}

// onTrackerKey is the tracker's keymap, deliberately the diff's shape - j/k to
// walk, r to read again, esc to leave - so the column's five faces do not each
// need learning.
func (m *Model) onTrackerKey(key string) tea.Cmd {
	if m.trackerCursorKey(key) || m.trackerSectionKey(key) {
		return nil
	}
	switch {
	case is(key, trackerBind.Reload):
		return m.readTrackerNow(m.review.Tracker.Project)
	case is(key, trackerBind.Read):
		return m.openItemAtCursor()
	}
	return m.trackerDefault(key)
}

// trackerCursorKey is the keys that only move or leave, reporting whether key
// was one. Split off onTrackerKey when the filter pushed it past the statement
// limit; the four are what the list does without touching the forge.
func (m *Model) trackerCursorKey(key string) bool {
	switch {
	case is(key, columnBind.Down):
		m.moveTrackerCursor(1)
	case is(key, columnBind.Up):
		m.moveTrackerCursor(-1)
	case is(key, columnBind.Back):
		m.leaveTracker()
	case is(key, columnBind.Interrupt):
		m.review.Focused = false
	case is(key, trackerBind.Filter):
		m.review.Filter.Active = true
	default:
		return false
	}
	return true
}

// trackerSectionKey is the keys that act on a whole section - jump to it
// (#662) or fold it (#663) - reporting whether key was one.
func (m *Model) trackerSectionKey(key string) bool {
	switch {
	case is(key, trackerBind.NextSection):
		m.jumpTrackerSection(1)
	case is(key, trackerBind.PrevSection):
		m.jumpTrackerSection(-1)
	case is(key, trackerBind.Fold):
		m.toggleTrackerFold()
	default:
		return false
	}
	return true
}

// trackerDefault is the pan keys and the three that act on a row (#398), in that
// order: h/l/0 are the column's and every view answers them the same way.
func (m *Model) trackerDefault(key string) tea.Cmd {
	if m.panKey(key) {
		return nil
	}
	cmd, _ := m.trackerAction(key)
	return cmd
}

// leaveTracker is esc in the list: a kept filter is lifted first, so the
// operator sees the narrowing go before the column does (#198's rule).
func (m *Model) leaveTracker() {
	if m.review.Filter.Query != "" {
		m.setTrackerFilter("")
		return
	}
	m.review.Focused = false
}

// setTrackerFilter applies query and re-clamps the cursor and the pan, since the
// visible set changed under both (#133).
func (m *Model) setTrackerFilter(query string) {
	m.review.Filter.Query = query
	m.contentChanged()
	m.moveTrackerCursor(0)
}

// moveTrackerCursor walks the rows, skipping the rule: it is a label, not an
// item, and a cursor resting on it would have nothing to act on.
func (m *Model) moveTrackerCursor(delta int) {
	rows := m.trackerRows()
	// A narrowed list can be shorter than where the cursor stood (#399);
	// move clamps it back onto the rows there are.
	m.review.Tracker.move(delta, len(rows), m.reviewRows())
	if len(rows) > 0 && !rows[m.review.Tracker.Cursor].stop() {
		m.stepOffRule(rows, delta)
	}
}

// jumpTrackerSection moves the cursor to the first item of the next section, or
// the previous one (#662): with a hundred open issues the pull requests were a
// walk past every one of them away. The diff's ] and [ between files work the
// same way - [ inside a section goes to its first item, and neither wraps.
func (m *Model) jumpTrackerSection(dir int) {
	cursor, target := m.review.Tracker.Cursor, -1
	for _, start := range sectionStarts(m.trackerRows()) {
		if dir > 0 && start > cursor {
			target = start
			break
		}
		if dir < 0 && start < cursor {
			target = start
		}
	}
	if target >= 0 {
		m.moveTrackerCursor(target - cursor)
	}
}

// sectionStarts is where each section starts for the cursor: the row after
// each heading, or a folded heading itself, which is all a folded section
// shows (#663). A list with no heading has one section and no start to jump
// to, and a list the filter emptied has no heading (#399), so it is never a
// stop.
func sectionStarts(rows []trackerRow) []int {
	var starts []int
	for i, r := range rows {
		switch {
		case r.Kind != rowRule:
		case r.Folded:
			starts = append(starts, i)
		case i+1 < len(rows) && rows[i+1].Kind != rowRule:
			starts = append(starts, i+1)
		}
	}
	return starts
}

// stepOffRule moves the cursor to the nearest row that is not a rule, the way
// it was going, or back the other way when there is none that way: g lands on
// the issues heading at the top (#432), and the nearest item is below it.
// It walks rather than jumping two, which skipped the first issue.
func (m *Model) stepOffRule(rows []trackerRow, delta int) {
	step := 1
	if delta < 0 {
		step = -1
	}
	for _, dir := range []int{step, -step} {
		for i := m.review.Tracker.Cursor + dir; i >= 0 && i < len(rows); i += dir {
			if rows[i].stop() {
				m.review.Tracker.move(i-m.review.Tracker.Cursor, len(rows), m.reviewRows())
				return
			}
		}
	}
}

// trackerItemPosition is the cursor's place among the items, not the rows:
// headings are labels, and "row 1 of 3" should not count one.
func (m *Model) trackerItemPosition() (n, total int) {
	for i, r := range m.trackerRows() {
		if r.Kind == rowRule {
			continue
		}
		total++
		if i <= m.review.Tracker.Cursor {
			n = total
		}
	}
	return n, total
}

// settleTracker puts the tracker's cursor back on a row when project's lists
// change under it: an arriving list can add the headings above the cursor,
// and a cursor resting on a heading has nothing to act on (#432).
func (m *Model) settleTracker(project string) {
	if m.review.Tracker.Project == project {
		m.moveTrackerCursor(0)
	}
}
