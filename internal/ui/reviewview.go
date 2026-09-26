package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/WilsonSousajr/omatty/internal/coverage"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// tabWidth is what a tab becomes before a row is measured. Tabs are expanded
// here rather than left to the renderer: lipgloss.Width counts a tab as one
// cell but draws it as several, so an unexpanded tab made the column wider
// than the width the layout had reserved for it, and the frame tore (#21).
const tabWidth = 4

// renderReview boxes the review column: a title, then whichever view is
// showing - diff rows with their comments and the note editor, the worktree
// tree, or one file's preview (#21, #24).
func (m *Model) renderReview(w, h int) string {
	// The title is in the rule, so every view draws h rows (#128).
	var lines []string
	switch m.review.View {
	case ViewTree:
		lines = m.renderTree(w, h)
	case ViewPreview:
		lines = m.renderPreview(w, h)
	case ViewGate:
		lines = m.renderGate(w, h)
	case ViewTracker:
		lines = m.renderTracker(w, h)
	case ViewTrackerItem:
		lines = m.renderTrackerItem(w, h)
	default:
		lines = m.reviewBody(w, h)
	}
	// Which column owns the keys - and so wears the accent hairline - is
	// decided once, in keyboardEdge (#174); the title is the header row's.
	return fitBlock(lines, w, h)
}

// reviewTitle names what the column is showing, so a glance at the top row
// says which of the three views has the keys.
//
// width is the column's, so the diff title can fit itself rather than be cut
// (#283). The budget is one cell less because headerRow draws every title
// behind a space, and the pan marker comes off the top of it: a title that
// gave up a part and then had the marker appended would be back over budget.
func (m *Model) reviewTitle(width int) string {
	marker := m.panMarker()
	return m.viewTitle(width-1-lipgloss.Width(marker)) + marker
}

// viewTitle names the view. Only the diff's is given a budget: the other three
// are a path or a session title, which are one part each and have nothing to
// give up, so they are cut as they always were.
func (m *Model) viewTitle(budget int) string {
	switch m.review.View {
	case ViewTree:
		return m.treeTitle(budget)
	case ViewPreview:
		return previewTitle(m.review.Preview.Path, budget)
	case ViewGate:
		return "gate · " + m.sessionTitle(m.review.SessionID)
	case ViewTracker:
		return m.trackerTitle(budget)
	case ViewTrackerItem:
		return m.trackerItemTitle(budget)
	}
	return joinTitle(m.diffTitleParts(), budget)
}

// treeTitle is "files · <session> /query", fitted by shortening the session
// name and, past the point where a name says anything, dropping it (#285).
//
// The filter marker never goes, and that is the whole point of the rule. The
// listing under it is short *because* a filter is in force; a cut marker leaves
// a filtered tree indistinguishable from a complete one, and nothing else on
// screen says otherwise. A dropped count is missing information, which the diff
// title can afford; a dropped marker is misleading, which no title can.
//
// The name shrinks rather than being dropped outright because it is a name: a
// shortened one still identifies the session, where a shortened count says
// nothing - which is why the diff title gives its parts up whole and this one
// does not. Deliberately not the same machinery: two different rules, and one
// mechanism over both would hide which applies where.
func (m *Model) treeTitle(budget int) string {
	const head = "files · "
	marker := m.foldMarker() + m.filterMarker()
	room := budget - lipgloss.Width(head) - lipgloss.Width(marker)
	if room < minNameCells {
		if marker == "" {
			return strings.TrimSuffix(head, " · ")
		}
		return head + strings.TrimSpace(marker)
	}
	return head + elideMiddle(m.sessionTitle(m.review.SessionID), room) + marker
}

// foldMarker says how many generated files the tree is keeping out of the
// listing, or nothing when it is keeping none (#338).
//
// It is a marker rather than a count that may be dropped, for the reason the
// filter marker is: a listing that is short because rows were withheld reads
// exactly like a complete one, and nothing else on screen says otherwise. The
// number is small and so is the marker.
func (m *Model) foldMarker() string {
	if m.review.Tree == nil {
		return ""
	}
	if n := m.review.Tree.GeneratedHidden(); n > 0 {
		return fmt.Sprintf(" ⊞%d", n)
	}
	return ""
}

// previewTitle is the file's path, shortened from the *front* (#287).
//
// The third rule these titles need, and it is a third because neither of the
// others fits. A path has no parts to rank the way the diff title's counts are
// ranked (#283), and shortening it from the middle the way a session name is
// shortened (#285) would be wrong: both ends of a *name* distinguish it, while
// the whole left-hand side of a path is the least valuable part of it. The
// filename is what says which file is on screen - two previews in one package
// otherwise draw the same title - so the directories above it go first.
//
// The "…/" is not decoration. "ui/reviewview.go" alone reads as a complete
// repo-relative path, which is a lie about where the file is; the marker says
// there was more above it. Same failure the filter marker guards against.
//
// When even the filename will not fit, elideMiddle takes it: a name's start
// and its extension both carry meaning, and the middle is what can go.
func previewTitle(path string, budget int) string {
	if lipgloss.Width(path) <= budget {
		return path
	}
	segments := strings.Split(path, "/")
	for i := 1; i < len(segments); i++ {
		if short := "…/" + strings.Join(segments[i:], "/"); lipgloss.Width(short) <= budget {
			return short
		}
	}
	return elideMiddle(segments[len(segments)-1], budget)
}

// minNameCells is the room below which a shortened session name is not worth
// the cells: two characters and an ellipsis identify nothing. The session is
// named by the sidebar cursor and by the pane title beside it either way,
// which is what makes the name the part that can go.
const minNameCells = 6

// titlePart is one piece of the diff title and how long it survives a column
// too narrow to hold everything. Higher survives longer.
type titlePart struct {
	text     string
	priority int
}

// The priorities, and the whole of the argument for them. `diff` names the
// view and never goes. The flag is the only remark M10 makes about the diff as
// a whole, and it went first under a plain right-hand cut, which is what #283
// is: a flag that disappears exactly when the column is busiest cannot be
// relied on. Unsent comments are work the operator still owes; a file count is
// context; a zero comment count is the absence of news, and goes first.
const (
	dropFirst = iota // a zero comment count
	dropFiles
	dropComments // only when there are some
	keepFlag
	keepAlways
)

// diffTitleParts is the diff title in reading order, each part carrying how
// readily it is given up.
func (m *Model) diffTitleParts() []titlePart {
	comments := m.commentsFor(m.review.SessionID).PendingLen()
	priority := dropComments
	if comments == 0 {
		priority = dropFirst
	}
	d := m.shownDiff()
	parts := []titlePart{{text: "diff", priority: keepAlways}}
	if m.review.Scope == scopeTurn {
		// keepFlag, as ⚠ no tests: a turn view that reads as the whole diff
		// is this feature's worst failure (#311).
		parts = append(parts, titlePart{text: "this turn", priority: keepFlag})
	}
	parts = append(parts,
		titlePart{text: fmt.Sprintf("%d files", len(d.Files)), priority: dropFiles},
		titlePart{text: fmt.Sprintf("%d comments", comments), priority: priority})
	if note := pairingNote(d); note != "" {
		parts = append(parts, titlePart{text: note, priority: keepFlag})
	}
	return parts
}

// joinTitle renders the parts that fit, giving up the least valuable one at a
// time rather than cutting the line from the right. `0 comment` says nothing
// and reads like a bug; `diff · 2 files` says something.
//
// When only the parts that are never given up remain, the title is returned
// whatever its width and fitLine cuts it as before - there is nothing left to
// give, and a column that narrow has bigger problems than its label.
func joinTitle(parts []titlePart, budget int) string {
	for {
		title := renderTitle(parts)
		if lipgloss.Width(title) <= budget || !droppable(parts) {
			return title
		}
		parts = dropWeakest(parts)
	}
}

func renderTitle(parts []titlePart) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, p.text)
	}
	return strings.Join(out, " · ")
}

// droppable reports whether any part is still worth giving up.
func droppable(parts []titlePart) bool {
	for _, p := range parts {
		if p.priority < keepFlag {
			return true
		}
	}
	return false
}

// dropWeakest removes the lowest-priority part. Ties cannot happen: every
// priority is held by exactly one part.
func dropWeakest(parts []titlePart) []titlePart {
	weakest := 0
	for i, p := range parts {
		if p.priority < parts[weakest].priority {
			weakest = i
		}
	}
	return append(parts[:weakest:weakest], parts[weakest+1:]...)
}

// pairingNote is the word a diff that changed source and no tests is worth
// (#257), and nothing at all for the other three outcomes: a remark that
// appeared on most diffs would be decoration, and the eye would learn to skip
// it.
//
// It is a remark, not a gate. Nothing is blocked, nothing turns red, and S
// sends no more than it did. M9's line holds - omatty reports, the operator
// decides - and this is the cheapest possible place to test that line, because
// a flag is exactly the kind of thing that grows teeth later.
//
// Computed per frame rather than cached: Pair reads paths, and only a Rust
// file's hunks, so it costs less than laying out the rows underneath it.
func pairingNote(d review.Diff) string {
	if review.Pair(d) != review.PairingUnpaired {
		return ""
	}
	return "⚠ no tests"
}

// filterMarker names the filter in force, so a short listing says why.
func (m *Model) filterMarker() string {
	if m.review.Filter.Query == "" {
		return ""
	}
	return " /" + m.review.Filter.Query
}

// reviewBody is the error, the empty-state line, or the scrolled rows with the
// editor beneath them.
func (m *Model) reviewBody(w, rows int) []string {
	if m.review.Err != "" {
		return noticeLines([]string{"loading the diff failed:", m.review.Err, logHint}, true, w)
	}
	if m.review.Scope == scopeTurn {
		if lines, isErr := m.turnNotice(); lines != nil {
			return noticeLines(lines, isErr, w)
		}
	}
	if len(m.review.Entries) == 0 {
		return []string{mutedStyle.Render(m.noChanges())}
	}
	if !m.review.Note.Active {
		return m.renderEntries(w, rows)
	}
	out := m.renderEntries(w, rows-1)
	return append(out, editLine(noteLabel(m.review.Note), m.review.Note.Buffer, w))
}

// renderEntries draws the rows-high window around the cursor. The offset is
// recomputed here rather than trusted, because a resize can shrink rows after
// the cursor last moved.
func (m *Model) renderEntries(w, rows int) []string {
	off := ScrollOffset(m.review.Cursor, m.review.Offset, rows)
	end := min(off+rows, len(m.review.Entries))
	comments := m.commentsFor(m.review.SessionID).All()
	out := make([]string, 0, rows)
	for i := off; i < end; i++ {
		e := m.review.Entries[i]
		out = append(out, m.renderEntry(e, i == m.review.Cursor, w, comments))
	}
	return out
}

// renderEntry draws one row; the cursor row is reversed.
func (m *Model) renderEntry(e review.Entry, cursor bool, w int, comments []review.Comment) string {
	text := m.fitRow(e, comments, w)
	if cursor {
		return cursorStyle.Render(text)
	}
	if e.Kind == review.EntryComment && !comments[e.Comment].Sent.IsZero() {
		return mutedStyle.Render(text) // sent: context for this turn, not a to-do (#335)
	}
	return entryStyle(e, m.shownDiff()).Render(text)
}

// fitRow draws one row into the column. A file header fits itself and stays
// put; every other row is panned and then cut (#291).
//
// The header does not pan because it is a header: pan.go says why the titles
// keep a plain fitLine, and a header that slides sideways with the body reads
// as a broken frame. Once it fits itself there is nothing behind it to reveal,
// and panning would slide the count off the left instead of the right.
func (m *Model) fitRow(e review.Entry, comments []review.Comment, w int) string {
	if e.Kind == review.EntryFile {
		fi := e.Pos.File
		return fitLine(fileHeading(m.shownDiff().Files[fi], m.uncoveredNote(fi), w), w)
	}
	return m.fitContent(m.entryText(e, comments), w)
}

// entryText is the plain text of a panned row before styling. A file header is
// not one of them: it fits itself, in fileheader.go.
//
// A method since #255: what a row says now depends on the coverage overlay the
// session's last gate loaded, and the overlay is the model's.
func (m *Model) entryText(e review.Entry, comments []review.Comment) string {
	switch e.Kind {
	case review.EntryHunk:
		return expandTabs(e.Text)
	case review.EntryComment:
		if c := comments[e.Comment]; !c.Sent.IsZero() {
			return "  >> (sent " + c.Sent.Format("15:04") + ") " + c.Note
		}
		return "  >> " + comments[e.Comment].Note
	case review.EntryOrphan:
		return "  >> (moved) " + comments[e.Comment].Note
	}
	return m.linePrefix(e) + expandTabs(e.Text)
}

// linePrefix is the row's first cell: its sign, or the uncovered marker where
// the overlay says an added line never ran (#255).
//
// The marker takes the sign's cell rather than adding a gutter column of its
// own, deliberately. A new column would shift every row of every diff,
// including the files a profile says nothing about, and an overlay is a remark
// on the diff rather than a redesign of it. The line keeps its added colour, so
// the row still reads as an addition - one nothing exercised.
func (m *Model) linePrefix(e review.Entry) string {
	if m.uncovered(e) {
		return uncoveredMark
	}
	return signPrefix(m.shownDiff().LineAt(e.Pos).Kind)
}

// uncoveredMark is what an added line no test covers draws instead of its +.
const uncoveredMark = "!"

// uncovered reports whether e is an added line the overlay has a verdict for
// and that verdict is "never ran".
//
// Three states, and the third is the one that earns the check its shape: a
// line the profile does not mention has no verdict at all - it is a
// declaration, a brace, a comment - and silence is never a claim. Only added
// lines are asked about: a context line's coverage is not this change's
// business, and a removed line is not in the tree the profile describes.
func (m *Model) uncovered(e review.Entry) bool {
	if e.Kind != review.EntryLine {
		return false
	}
	line := m.shownDiff().LineAt(e.Pos)
	if line.Kind != review.LineAdded {
		return false
	}
	path := m.shownDiff().Files[e.Pos.File].Path
	// A generated file has no test and never will, so an uncovered marker on
	// it says something true about the file and nothing at all about the
	// change - it only makes the file look worse than it is (#338).
	if m.isGenerated(path) {
		return false
	}
	covered, known := m.overlay().Files[path].Lines[line.NewNo]
	return known && !covered
}

// overlay is the coverage the session under review last loaded, or the zero
// profile - which answers "no verdict" for every line, so no caller needs a
// nil check.
func (m *Model) overlay() coverage.Profile {
	return m.covers[m.review.SessionID]
}

func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", strings.Repeat(" ", tabWidth))
}

func signPrefix(k review.LineKind) string {
	switch k {
	case review.LineAdded:
		return "+"
	case review.LineRemoved:
		return "-"
	}
	return " "
}

// uncoveredNote counts what the markers under this header will say, and says
// nothing at all when there is nothing to report - a note on every file would
// be decoration rather than a finding.
func (m *Model) uncoveredNote(fi int) string {
	if m.isGenerated(m.shownDiff().Files[fi].Path) {
		return "" // #338, the same argument uncovered makes
	}
	lines := m.overlay().Files[m.shownDiff().Files[fi].Path].Lines
	if len(lines) == 0 {
		return ""
	}
	n := 0
	for _, h := range m.shownDiff().Files[fi].Hunks {
		n += uncoveredInHunk(h, lines)
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("  %d uncovered", n)
}

// uncoveredInHunk is how many of a hunk's added lines the overlay says never
// ran, counted the same way linePrefix marks them.
func uncoveredInHunk(h review.Hunk, lines map[int]bool) int {
	n := 0
	for _, l := range h.Lines {
		if covered, known := lines[l.NewNo]; l.Kind == review.LineAdded && known && !covered {
			n++
		}
	}
	return n
}

// noChanges is the empty diff's line, which says which diff was empty.
func (m *Model) noChanges() string {
	if m.review.Scope == scopeTurn {
		return "no changes this turn"
	}
	return "no changes"
}

// noticeLines draws a notice wrapped to the column rather than cut at its
// edge: a cut line lost the part of an error that says what went wrong
// (#351), and git's stderr can be several lines of its own.
func noticeLines(lines []string, isErr bool, w int) []string {
	style := mutedStyle
	if isErr {
		style = errorStyle
	}
	var out []string
	for _, l := range lines {
		for _, part := range strings.Split(ansi.Wrap(l, w, " "), "\n") {
			out = append(out, style.Render(fitLine(part, w)))
		}
	}
	return out
}

// noteLabel says which of the note editor's two prompts is showing (#339).
func noteLabel(n noteEditor) string {
	if n.Stage == stageFragment {
		return "fragment"
	}
	return "note"
}
