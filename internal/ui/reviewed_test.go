package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// treeOnModelGo opens the tree and puts the cursor on internal/ui/model.go,
// the fixture's modified file. The listing opens with internal/ first (#194),
// so the rows are internal/, internal/ui/, model.go.
func treeOnModelGo(t *testing.T) (*ui.Model, *diffRecorder) {
	t.Helper()
	terms, _ := fakeTerms(t)
	rec := &diffRecorder{Diff: sampleDiffParsed(t)}
	lister := &fileLister{Paths: []string{"go.mod", "internal/ui/model.go", "internal/ui/render.go", "new.txt"}}
	d := baseDeps(twoProjectState(), terms)
	d.Diff, d.Files = rec.fn, lister.fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('f'))
	press(m, key('j')) // model.go, under the compacted internal/ui/ row (#430)
	return m, rec
}

// A reviewer on a long session re-reads what they have already read, because
// the tree remembers which files changed and not which ones they have seen
// (#337). v is that memory.
func TestModel_vMarksTheRowUnderTheCursorReviewed_issue337(t *testing.T) {
	m, _ := treeOnModelGo(t)

	if row := lineWith(t, m.View().Content, "M model.go"); strings.Contains(row, "✓") {
		t.Fatalf("model.go reads as reviewed before v was pressed: %q", row)
	}

	press(m, key('v'))

	row := lineWith(t, m.View().Content, "M model.go")
	if !strings.Contains(row, "✓") {
		t.Errorf("after v the row is %q, want a ✓ on it", row)
	}
	if other := lineWith(t, m.View().Content, "render.go"); strings.Contains(other, "✓") {
		t.Errorf("v marked more than the row under the cursor: %q", other)
	}
}

// Pressing it again is how a mark made by mistake is taken back.
func TestModel_vAgainTakesTheMarkBack_issue337(t *testing.T) {
	m, _ := treeOnModelGo(t)

	press(m, key('v'))
	press(m, key('v'))

	if row := lineWith(t, m.View().Content, "M model.go"); strings.Contains(row, "✓") {
		t.Errorf("the mark survived a second v: %q", row)
	}
}

// The half that earns the feature: a file read once and then changed by the
// next turn must say so, or the mark is a lie. The identity compared is the
// file's diff content, never its mtime (#337, invariant 7).
func TestModel_aReviewedFileThatChangesSaysChangedSince_issue337(t *testing.T) {
	m, rec := treeOnModelGo(t)
	press(m, key('v'))

	rec.Diff = parseDiff(t, strings.Replace(sampleDiff, "+	b := 3", "+	b := 99", 1))
	// The turn ending is what reloads the diff in real use (#21, #195); the
	// tree's own r only re-lists the worktree.
	statusDeliver(m, "s1", watcher.TurnEnded, time.Now())

	row := columnPart(lineWith(t, m.View().Content, "M model.go"))
	if strings.Contains(row, "✓") {
		t.Errorf("the row still reads reviewed after its diff changed: %q", row)
	}
	if !strings.Contains(row, "~") {
		t.Errorf("row = %q, want the changed-since-reviewed mark", row)
	}
}

// A file whose content is untouched keeps its mark. Without this the feature
// is noise: every reload would reset every mark.
func TestModel_aReviewedFileThatDidNotChangeKeepsItsMark_issue337(t *testing.T) {
	m, _ := treeOnModelGo(t)
	press(m, key('v'))

	statusDeliver(m, "s1", watcher.TurnEnded, time.Now())

	if row := lineWith(t, m.View().Content, "M model.go"); !strings.Contains(row, "✓") {
		t.Errorf("row = %q, want the mark to survive a reload that changed nothing", row)
	}
}

// "Changed since reviewed" has no meaning for a file the session did not
// change, so v refuses it - and says so, rather than doing nothing visible.
func TestModel_vRefusesAFileTheSessionDidNotChange_issue337(t *testing.T) {
	m, _ := treeOnModelGo(t)
	press(m, key('j')) // render.go, in the listing but not in the diff

	press(m, key('v'))

	if row := lineWith(t, m.View().Content, "render.go"); strings.Contains(row, "✓") {
		t.Errorf("an unchanged file was marked reviewed: %q", row)
	}
	if footer := m.View().Content; !strings.Contains(footer, "changed") {
		t.Error("v on an unchanged file said nothing; a key that appears to do nothing reads as a bug")
	}
}

// v on a directory row is not a review of everything under it.
func TestModel_vOnADirectoryDoesNothing_issue337(t *testing.T) {
	m, _ := treeOnModelGo(t)
	press(m, key('k'))
	press(m, key('k')) // back to internal/

	press(m, key('v'))

	if row := lineWith(t, m.View().Content, "▾ internal/"); strings.Contains(row, "✓") {
		t.Errorf("a directory was marked reviewed: %q", row)
	}
}

// The marks are one session's, and the reason to hold them per session rather
// than on the Tree: the column keeps one tree, so a mark stored there would be
// dropped the moment the operator looked at another session.
func TestModel_marksSurviveALookAtAnotherSession_issue337(t *testing.T) {
	m, _ := treeOnModelGo(t)
	press(m, key('v'))

	leader(m, key('j'))
	leader(m, key('k'))

	if row := lineWith(t, m.View().Content, "M model.go"); !strings.Contains(row, "✓") {
		t.Errorf("row = %q, want the mark still there after looking away and back", row)
	}
}

// parseDiff is sampleDiffParsed for a diff written by the caller.
func parseDiff(t *testing.T, src string) review.Diff {
	t.Helper()
	d, err := review.ParseDiff(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// The mark is not only a glyph: a file read and unchanged goes quiet, so the
// eye stops returning to it. A changed-since row keeps its change colour,
// because it wants attention again. README says both (#337, #196's one-hue
// one-meaning rule).
func TestModel_aReviewedRowGoesQuietAndAChangedOneDoesNot_issue337(t *testing.T) {
	const sgrMuted = "\x1b[38;5;245m"
	m, rec := treeOnModelGo(t)
	press(m, key('j')) // off model.go, so the cursor's reverse does not mask the hue
	press(m, key('k'))
	press(m, key('v'))
	press(m, key('j')) // render.go, leaving model.go to its own style

	if row := columnPart(lineWith(t, m.View().Content, "✓M model.go")); !strings.Contains(row, sgrMuted) {
		t.Errorf("a reviewed row is not muted: %q", row)
	}

	rec.Diff = parseDiff(t, strings.Replace(sampleDiff, "+	b := 3", "+	b := 99", 1))
	statusDeliver(m, "s1", watcher.TurnEnded, time.Now())

	row := columnPart(lineWith(t, m.View().Content, "~M model.go"))
	if strings.Contains(row, sgrMuted) {
		t.Errorf("a changed-since-reviewed row is muted, so the change reads as read: %q", row)
	}
	if !strings.Contains(row, sgrAmber) {
		t.Errorf("row = %q, want a modified file's amber back", row)
	}
}

// columnPart is the review column's share of a frame line: past its last
// hairline. A whole line also carries the sidebar, whose status glyph (✓ once
// a turn ends) and muted age would answer these tests' questions for it - which
// is what happened when #430 moved model.go up onto the card's row.
func columnPart(line string) string { return line[strings.LastIndex(line, "│"):] }
