package ui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

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
		parts = append(parts, titlePart{text: plural(n, strings.ToLower(m.label(project).Short)), priority: dropFirst})
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
	// Wrapped, not cut: a note drawn as fixed lines lost its tail at the
	// column's edge, and the tail was the part that said what to do (#572).
	if note := m.trackerNote(); note != "" {
		return m.withFilterLine(wrapBlock(note, m.columnWidth()), m.columnWidth(), h)
	}
	rows := m.trackerRows()
	numW := trackerNumberWidth(rows)
	lines := make([]string, 0, len(rows))
	for i, r := range rows {
		lines = append(lines, m.trackerLine(r, i == m.review.Tracker.Cursor, numW))
	}
	list := m.withFilterLine(window(lines, m.review.Tracker.Offset, h), m.trackerListWidth(), h)
	return m.withPreview(list, h)
}

// trackerNote is the "nothing to show" state, or "" when there are rows. Each
// state is distinct because each calls for something different from the
// operator, which is renderGate's argument (#231).
func (m *Model) trackerNote() string {
	project := m.review.Tracker.Project
	_, issuesPolled := m.issues[project]
	_, prsPolled := m.prs[project]
	switch {
	case m.forgeStopped[project] != nil:
		return m.stoppedNote(project, m.forgeStopped[project])
	case !issuesPolled && !prsPolled:
		return "reading " + project + "'s issues and " + m.label(project).Change + "s..."
	case len(m.trackerRows()) == 0 && m.review.Filter.Query != "":
		return "nothing here matches /" + m.review.Filter.Query
	case len(m.trackerRows()) == 0:
		return "nothing open in " + project + "."
	}
	return ""
}

// trackerLine is one row: the number, one label or CI mark, the title, and how
// long since it last moved. The cursor row is drawn in the accent, the way a
// selected card's rail is (#174).
func (m *Model) trackerLine(r trackerRow, selected bool, numW int) string {
	w := m.trackerListWidth()
	if r.Kind == rowRule {
		// The rule does not pan. It is a label rather than content, so it fits
		// its own column the way a diff's file header does (#291): panned right
		// it went blank, and the smoke run showed the two lists merging into
		// one with nothing to say where the issues stopped.
		return fitLine(labelledRule(r.Title, w), w)
	}
	// Only the lead pans. The age is pinned to the right edge, so a title
	// longer than the column is what gets cut - not the age after it (#423).
	lead, age := trackerParts(r, m.clock(), w, numW)
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
func trackerText(r trackerRow, now time.Time, w, numW int) string {
	lead, age := trackerParts(r, now, w, numW)
	return lead + trackerAgeGap + age
}

// trackerParts is a row split where the renderer splits it: the lead - number,
// label and title - which pans, and the age, which stays at the right edge.
// In a column too narrow for all four the label is left out, since it is the
// least of them and the title is what a row is read for.
func trackerParts(r trackerRow, now time.Time, w, numW int) (lead, age string) {
	number := padRight(r.ref(), numW)
	age = padLeft(clip(AgeString(now, r.Updated), trackerAgeCols), trackerAgeCols)
	if r.Kind == rowPR {
		// Three glyph cells, never given up: they fit where a label does not,
		// and they are the row's state (#432).
		return number + r.Label + " " + r.Title, age
	}
	if w < numW+trackerLabelCols+trackerMinTitle+len(trackerAgeGap)+trackerAgeCols {
		return number + r.Title, age
	}
	label := padRight(clip(r.Label, trackerLabelCols-1), trackerLabelCols)
	return number + label + r.Title, age
}

// trackerNumberWidth is the number column: trackerNumberCols, or the widest
// number and a gap. A fixed six cells ran "#14588" into its label (#590).
func trackerNumberWidth(rows []trackerRow) int {
	w := trackerNumberCols
	for _, r := range rows {
		if r.Kind != rowRule {
			w = max(w, len(r.ref())+1)
		}
	}
	return w
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
	widest, rows := 0, m.trackerRows()
	numW := trackerNumberWidth(rows)
	for _, r := range rows {
		if r.Kind == rowRule {
			continue
		}
		widest = max(widest, lipgloss.Width(trackerText(r, m.clock(), m.trackerListWidth(), numW)))
	}
	return widest
}

// trackerTitle is the header row's text for this view.
func (m *Model) trackerTitle(budget int) string {
	return joinTitle(m.trackerTitleParts(), budget)
}
