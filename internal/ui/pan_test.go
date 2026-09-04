package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// longLine is wider than the review column at the 100x30 the fixtures use
// (ReviewWidth(100, true) is 28, so its content is 26 cells), with a distinct
// marker near the end that only panning can bring into view.
const longLine = "package main // aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa END_MARKER"

// modelWithLongPreview opens the tree, previews a file whose first line is far
// wider than the column, and returns the model sitting in the preview.
func modelWithLongPreview(t *testing.T) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	reader := &previewReader{Files: map[string]string{"long.go": longLine + "\nshort"}}
	d := baseDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn
	d.Files = (&fileLister{Paths: []string{"long.go"}}).fn
	d.Preview = reader.fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('f'))
	_, cmd := m.Update(special(tea.KeyEnter))
	deliver(m, cmd)
	if m.ReviewView() != ui.ViewPreview {
		t.Fatalf("view = %v, want the preview open", m.ReviewView())
	}
	return m
}

// Regression, issue #94: a preview line wider than the pane was truncated by
// fitLine with no wrap and no scroll, so the rest of the line could not be read
// at all. l pans right.
func TestModel_PansThePreviewRight_issue94(t *testing.T) {
	m := modelWithLongPreview(t)
	if strings.Contains(m.View().Content, "END_MARKER") {
		t.Fatal("the fixture line is not wider than the column; the test proves nothing")
	}

	for range 8 {
		press(m, key('l'))
	}

	if !strings.Contains(m.View().Content, "END_MARKER") {
		t.Errorf("panning right never revealed the end of the line:\n%s", m.View().Content)
	}
}

// Panning must stop at the widest line the view can show, or l walks the text
// off the screen into blank space with no way to tell how far it went.
func TestModel_PanStopsAtTheLongestLine_issue94(t *testing.T) {
	m := modelWithLongPreview(t)

	for range 200 {
		press(m, key('l'))
	}
	atEnd := m.ReviewColOffset()
	press(m, key('l'))

	if m.ReviewColOffset() != atEnd {
		t.Errorf("offset kept growing past the longest line: %d then %d", atEnd, m.ReviewColOffset())
	}
	if !strings.Contains(m.View().Content, "END_MARKER") {
		t.Error("the clamp landed past the end of the text, showing blank space")
	}
}

func TestModel_PanLeftStopsAtTheLeftEdge_issue94(t *testing.T) {
	m := modelWithLongPreview(t)
	for range 3 {
		press(m, key('l'))
	}

	for range 20 {
		press(m, key('h'))
	}

	if got := m.ReviewColOffset(); got != 0 {
		t.Errorf("offset = %d after panning left off the edge, want 0", got)
	}
}

// 0 is the way back to the left edge without holding h.
func TestModel_ZeroReturnsToTheLeftEdge_issue94(t *testing.T) {
	m := modelWithLongPreview(t)
	for range 4 {
		press(m, key('l'))
	}
	if m.ReviewColOffset() == 0 {
		t.Fatal("the pan did not move, so 0 cannot be shown to reset it")
	}

	press(m, key('0'))

	if got := m.ReviewColOffset(); got != 0 {
		t.Errorf("offset = %d after 0, want the left edge", got)
	}
}

// A file opened while the last one was panned must start at its left edge,
// or the first thing you see of a new file is its middle.
func TestModel_OpeningAPreviewResetsThePan_issue94(t *testing.T) {
	m := modelWithLongPreview(t)
	for range 4 {
		press(m, key('l'))
	}

	press(m, special(tea.KeyEscape)) // back to the tree
	_, cmd := m.Update(special(tea.KeyEnter))
	deliver(m, cmd)

	if got := m.ReviewColOffset(); got != 0 {
		t.Errorf("offset = %d on a freshly opened preview, want the left edge", got)
	}
}

// The diff view truncates through the same fitLine call, so it pans too.
func TestModel_PansTheDiff_issue94(t *testing.T) {
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: wideDiff(t)}).fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('d'))
	if strings.Contains(m.View().Content, "DIFF_END") {
		t.Fatal("the fixture diff line is not wider than the column")
	}

	for range 8 {
		press(m, key('l'))
	}

	if !strings.Contains(m.View().Content, "DIFF_END") {
		t.Errorf("panning the diff never revealed the end of the line:\n%s", m.View().Content)
	}
}

// And so does the tree: a deep path with a long name runs off the column.
func TestModel_PansTheTree_issue94(t *testing.T) {
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn
	d.Files = (&fileLister{Paths: []string{"a/b/c/d/a-very-long-file-name-TREE_END.go"}}).fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('f'))
	if strings.Contains(m.View().Content, "TREE_END") {
		t.Fatal("the fixture path is not wider than the column")
	}

	for range 6 {
		press(m, key('l'))
	}

	if !strings.Contains(m.View().Content, "TREE_END") {
		t.Errorf("panning the tree never revealed the end of the name:\n%s", m.View().Content)
	}
}

// The pane says how far it has scrolled, so line numbers that have slid out of
// view are explained rather than just missing.
func TestModel_TheTitleMarksAPannedColumn_issue94(t *testing.T) {
	m := modelWithLongPreview(t)
	title := func() string { return lineWith(t, m.View().Content, "long.go") }
	if strings.Contains(title(), "+") {
		t.Fatalf("unpanned title already carries a marker: %q", title())
	}

	press(m, key('l'))

	if !strings.Contains(title(), "+8") {
		t.Errorf("panned title = %q, want a +8 marker", title())
	}
}

// wideDiff is a one-file diff whose added line is far wider than the column.
func wideDiff(t *testing.T) review.Diff {
	t.Helper()
	d, err := review.ParseDiff(strings.NewReader("diff --git a/w.go b/w.go\n" +
		"--- a/w.go\n+++ b/w.go\n@@ -1 +1,2 @@\n context\n" +
		"+// bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb DIFF_END\n"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}
