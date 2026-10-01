package app

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/service/review"
)

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

// viewTitle names the view and, room allowing, where it stands in it (#424).
// The position is the first thing a short title gives up: it is kept only when
// the title fitted to the smaller budget is the same title, so it never costs
// a name, a count or a flag.
func (m *Model) viewTitle(budget int) string {
	full, pos := m.faceTitle(budget), m.positionMark()
	if pos == "" || lipgloss.Width(full+pos) > budget || m.faceTitle(budget-lipgloss.Width(pos)) != full {
		return full
	}
	return full + pos
}

// faceTitle names the face. Only the diff's is given a budget: the other three
// are a path or a session title, which are one part each and have nothing to
// give up, so they are cut as they always were.
func (m *Model) faceTitle(budget int) string {
	switch m.review.View {
	case ViewTree:
		return m.treeTitle(budget)
	case ViewPreview:
		return previewTitle(m.review.Preview.Path, budget)
	case ViewGate:
		return m.gateTitle(budget)
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
	marker := m.changedMarker() + m.foldMarker() + m.filterMarker()
	room := budget - lipgloss.Width(head) - lipgloss.Width(marker)
	if room < minNameCells {
		if marker == "" {
			return strings.TrimSuffix(head, " · ")
		}
		return head + strings.TrimSpace(marker)
	}
	return head + elideMiddle(m.sessionTitle(m.review.SessionID), room) + marker
}

// changedMarker says the listing is cut to changed files (#430). A narrowed
// listing that did not say so would read as the whole tree, which is the
// filter marker's argument (#285), so like it this is never given up.
func (m *Model) changedMarker() string {
	if m.review.Tree == nil || !m.review.Tree.ChangedOnly() {
		return ""
	}
	return " changed"
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

// faceName is the face on show, as the column's rule names it (#426).
func (m *Model) faceName() string {
	switch m.review.View {
	case ViewTree:
		return "files"
	case ViewPreview:
		return "preview"
	case ViewGate:
		return "gate"
	case ViewTracker, ViewTrackerItem:
		return "tracker"
	}
	return "diff"
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
	if q := m.activeFilter().Query; q != "" {
		return " /" + q
	}
	return ""
}
