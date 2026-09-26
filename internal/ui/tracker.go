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

package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/WilsonSousajr/omatty/internal/fuzzy"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
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
}

// TrackerCursor is the index of the highlighted tracker row. Exported for
// tests, as ReviewCursor is.
func (m *Model) TrackerCursor() int { return m.review.Tracker.Cursor }

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
	return tea.Batch(m.resizeIfWidthChanged(wasOpen), m.readTracker(project))
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

// onTrackerKey is the tracker's keymap, deliberately the diff's shape - j/k to
// walk, r to read again, esc to leave - so the column's five faces do not each
// need learning.
func (m *Model) onTrackerKey(key string) tea.Cmd {
	if m.trackerCursorKey(key) {
		return nil
	}
	switch key {
	case "r":
		return m.readTracker(m.review.Tracker.Project)
	case "enter":
		return m.openItemAtCursor()
	}
	return m.trackerDefault(key)
}

// trackerCursorKey is the keys that only move or leave, reporting whether key
// was one. Split off onTrackerKey when the filter pushed it past the statement
// limit; the four are what the list does without touching the forge.
func (m *Model) trackerCursorKey(key string) bool {
	switch key {
	case "j", "down":
		m.moveTrackerCursor(1)
	case "k", "up":
		m.moveTrackerCursor(-1)
	case "esc":
		m.leaveTracker()
	case "ctrl+c":
		m.review.Focused = false
	case "/":
		m.review.Filter.Active = true
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
	if len(rows) > 0 && rows[m.review.Tracker.Cursor].Kind == rowRule {
		m.stepOffRule(rows, delta)
	}
}

// stepOffRule moves the cursor one row off the rule, the way it was going, or
// back the other way when that is an end: g lands on the rule when there are
// no issues above it, and a step past it that clamps would leave it there.
func (m *Model) stepOffRule(rows []trackerRow, delta int) {
	step := 1
	if delta < 0 {
		step = -1
	}
	m.review.Tracker.move(step, len(rows), m.reviewRows())
	if rows[m.review.Tracker.Cursor].Kind == rowRule {
		m.review.Tracker.move(-2*step, len(rows), m.reviewRows())
	}
}

// trackerRows is the project's open issues, then its open pull requests under a
// labelled rule. One flat slice, so the cursor and the window index the rows a
// person sees rather than two lists and an offset between them.
func (m *Model) trackerRows() []trackerRow {
	project := m.review.Tracker.Project
	rows := make([]trackerRow, 0, len(m.issues[project]))
	for _, is := range m.issues[project] {
		row := trackerRow{
			Kind: rowIssue, Number: is.Number, Label: firstLabel(is.Labels),
			Title: is.Title, Updated: is.Updated,
		}
		if m.matchesFilter(row, is.Labels) {
			rows = append(rows, row)
		}
	}
	prs := m.openPRs(project)
	if len(prs) == 0 {
		return rows
	}
	// The rule goes with its list: one with nothing under it says a list is
	// there when it is not (#399).
	if len(rows) > 0 {
		rows = append(rows, trackerRow{Kind: rowRule, Title: "pull requests"})
	}
	return append(rows, prs...)
}

// matchesFilter reports whether a row survives the filter line. The haystack is
// what the row shows - its number, its title and its labels - because a filter
// over text the operator cannot see is a filter they cannot predict. All of the
// labels, not just the drawn one: `M14` is how a milestone is asked for.
func (m *Model) matchesFilter(row trackerRow, labels []string) bool {
	query := m.review.Filter.Query
	if query == "" || m.review.View == ViewDiff {
		return true
	}
	hay := strconv.Itoa(row.Number) + " " + row.Title + " " + strings.Join(labels, " ")
	_, ok := fuzzy.Match(query, hay)
	return ok
}

// openPRs is the project's open pull requests as rows, narrowed by the filter.
// Only the open ones: the map holds the finished for the card that says "merged"
// (#310), and the tracker answers what is open.
func (m *Model) openPRs(project string) []trackerRow {
	var rows []trackerRow
	for _, pr := range m.prs[project] {
		if pr.State != forge.Open {
			continue
		}
		row := trackerRow{
			Kind: rowPR, Number: pr.Number, Label: prMark(pr),
			Title: pr.Title, Updated: pr.Updated,
		}
		if m.matchesFilter(row, nil) {
			rows = append(rows, row)
		}
	}
	return rows
}

// prMark is a pull request's one cell in the label column: its CI, or "draft"
// for one not offered as work yet.
func prMark(pr forge.PR) string {
	if pr.Draft {
		return "draft"
	}
	return ciMark(pr)
}

func firstLabel(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	return labels[0]
}

// trackerTitleParts is the tracker's title in reading order. The counts are
// here as well as on the sidebar header because the header has 28 cells and
// this line has the column (#395).
func (m *Model) trackerTitleParts() []titlePart {
	project := m.review.Tracker.Project
	parts := []titlePart{
		{text: "tracker", priority: keepAlways},
		{text: project, priority: keepAlways},
	}
	if issues, polled := m.issues[project]; polled {
		parts = append(parts, titlePart{text: plural(len(issues), "issue"), priority: dropFiles})
	}
	if n, polled := m.openPRCount(project); polled {
		parts = append(parts, titlePart{text: plural(n, "pr"), priority: dropFirst})
	}
	if q := m.review.Filter.Query; q != "" {
		// keepAlways, not keepFlag: a filtered list is short *because* a filter
		// is in force, and a dropped marker leaves it indistinguishable from a
		// complete one (#285).
		parts = append(parts, titlePart{text: "/" + q, priority: keepAlways})
	}
	if m.issueFailed[project] || m.prFailed[project] {
		// keepFlag, as the card's "?" is: a stale list shown as current is this
		// view's worst failure (Orca #18484).
		parts = append(parts, titlePart{text: "?", priority: keepFlag})
	}
	return parts
}

// plural is "1 issue", "2 issues" - a count that reads as English, since the
// title is prose and not a table.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// renderTracker draws the rows, or says why there are none.
//
// The width is unused for the same reason renderGate's is: fitBlock cuts every
// view to the column, and a title truncated at the edge is more useful than one
// wrapped onto a row the cursor would then have to account for.
func (m *Model) renderTracker(_, h int) []string {
	if note := m.trackerNote(); note != nil {
		return m.withFilterLine(note, reviewContentWidth(m.width), h)
	}
	rows := m.trackerRows()
	lines := make([]string, 0, len(rows))
	for i, r := range rows {
		lines = append(lines, m.trackerLine(r, i == m.review.Tracker.Cursor))
	}
	return m.withFilterLine(window(lines, m.review.Tracker.Offset, h), reviewContentWidth(m.width), h)
}

// trackerNote is the "nothing to show" state, or nil when there are rows. Each
// state is distinct because each calls for something different from the
// operator, which is renderGate's argument (#231).
func (m *Model) trackerNote() []string {
	project := m.review.Tracker.Project
	_, issuesPolled := m.issues[project]
	_, prsPolled := m.prs[project]
	switch {
	case m.ghMissing:
		return []string{"gh is not installed, so omatty cannot read this project's", "issues or pull requests."}
	case m.notGitHub[project]:
		return []string{"this project is not on GitHub."}
	case !issuesPolled && !prsPolled:
		return []string{"reading " + project + "'s issues and pull requests..."}
	case len(m.trackerRows()) == 0 && m.review.Filter.Query != "":
		return []string{"nothing here matches /" + m.review.Filter.Query}
	case len(m.trackerRows()) == 0:
		return []string{"nothing open in " + project + "."}
	}
	return nil
}

// trackerLine is one row: the number, one label or CI mark, the title, and how
// long since it last moved. The cursor row is drawn in the accent, the way a
// selected card's rail is (#174).
func (m *Model) trackerLine(r trackerRow, selected bool) string {
	w := reviewContentWidth(m.width)
	if r.Kind == rowRule {
		// The rule does not pan. It is a label rather than content, so it fits
		// its own column the way a diff's file header does (#291): panned right
		// it went blank, and the smoke run showed the two lists merging into
		// one with nothing to say where the issues stopped.
		return fitLine(labelledRule(r.Title, w), w)
	}
	// Only the lead pans. The age is pinned to the right edge, so a title
	// longer than the column is what gets cut - not the age after it (#423).
	lead, age := trackerParts(r, m.clock(), w)
	text := m.fitContent(lead, w-len(trackerAgeGap)-trackerAgeCols) + trackerAgeGap + age
	if selected {
		return cursorStyle.Render(text)
	}
	return text
}

// trackerNumberCols and trackerLabelCols are the two fixed columns; the title
// takes what is left and the age sits at the right edge, the card's layout
// (#176) applied to a wider row.
const (
	trackerNumberCols = 6
	trackerLabelCols  = 10
	trackerAgeCols    = 4
	trackerAgeGap     = "  "
	// trackerMinTitle is the least a title keeps before the label gives way:
	// about a word. Pinning the age at 80 columns left one cell for it (#423).
	trackerMinTitle = 12
)

// trackerText is a row's plain text, measured by the pan clamp. It comes from
// the renderer's own builder so the two can never disagree (#133): the age is
// a fixed trackerAgeCols wide, so the clamp's widest-minus-column is exactly
// how far the lead must pan for the end of the longest title to show beside
// the pinned age (#423).
func trackerText(r trackerRow, now time.Time, w int) string {
	lead, age := trackerParts(r, now, w)
	return lead + trackerAgeGap + age
}

// trackerParts is a row split where the renderer splits it: the lead - number,
// label and title - which pans, and the age, which stays at the right edge.
// In a column too narrow for all four the label is left out, since it is the
// least of them and the title is what a row is read for.
func trackerParts(r trackerRow, now time.Time, w int) (lead, age string) {
	number := padRight("#"+strconv.Itoa(r.Number), trackerNumberCols)
	age = padLeft(clip(AgeString(now, r.Updated), trackerAgeCols), trackerAgeCols)
	if w < trackerNumberCols+trackerLabelCols+trackerMinTitle+len(trackerAgeGap)+trackerAgeCols {
		return number + r.Title, age
	}
	label := padRight(clip(r.Label, trackerLabelCols-1), trackerLabelCols)
	return number + label + r.Title, age
}

// labelledRule is "── pull requests ─────": a rule that says what is under it.
func labelledRule(label string, w int) string {
	head := "── " + label + " "
	if fill := w - lipgloss.Width(head); fill > 0 {
		return head + strings.Repeat("─", fill)
	}
	return head
}

// trackerMaxWidth is how far the tracker can pan: its widest row. The rule is
// skipped - it fits its column by construction, so it can never set the clamp,
// which is diffMaxWidth's argument for skipping a file header (#291).
func (m *Model) trackerMaxWidth() int {
	widest := 0
	for _, r := range m.trackerRows() {
		if r.Kind == rowRule {
			continue
		}
		widest = max(widest, lipgloss.Width(trackerText(r, m.clock(), reviewContentWidth(m.width))))
	}
	return widest
}

// trackerTitle is the header row's text for this view.
func (m *Model) trackerTitle(budget int) string {
	return joinTitle(m.trackerTitleParts(), budget)
}
