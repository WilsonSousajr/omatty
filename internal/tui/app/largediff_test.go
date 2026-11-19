package app_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// CountingHighlighter counts the lexes it is asked for, and colours nothing.
type CountingHighlighter struct{ Calls int }

func (c *CountingHighlighter) Lines(_ string, lines []string) []string {
	c.Calls++
	return lines
}

// A comment changes no hunk, so adding one re-lexes nothing: it cleared
// every hunk's colours, and the next frame lexed every visible hunk again -
// on a large diff, the moment you are trying to comment (#773).
func TestReview_aCommentReLexesNothing_issue773(t *testing.T) {
	terms, _ := fakeTerms(t)
	hl := &CountingHighlighter{}
	d := baseDeps(twoProjectState(), terms)
	d.Diff, d.Highlighter = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn, hl
	m := app.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('d'))
	for range 5 {
		press(m, key('j'))
	}
	m.View()
	before := hl.Calls

	press(m, key('c'))
	typeIn(m, "why this")
	m.View()

	if m.PendingComments() != 1 {
		t.Fatalf("PendingComments() = %d, want 1", m.PendingComments())
	}
	if hl.Calls != before {
		t.Errorf("adding a comment lexed %d hunks again, want 0", hl.Calls-before)
	}
}

// bigDiff is one file with n added lines, and a small one after it.
func bigDiff(n int) dreview.Diff {
	lines := make([]dreview.Line, n)
	for i := range lines {
		lines[i] = dreview.Line{Kind: dreview.LineAdded, Text: fmt.Sprintf("row %d", i), NewNo: i + 1}
	}
	small := []dreview.Line{{Kind: dreview.LineAdded, Text: "small change", NewNo: 1}}
	return dreview.Diff{Files: []dreview.File{
		{Path: "fixture.json", Status: dreview.FileAdded, Hunks: []dreview.Hunk{{Header: "@@ -0,0 +1 @@", Lines: lines}}},
		{Path: "main.go", Status: dreview.FileAdded, Hunks: []dreview.Hunk{{Header: "@@ -0,0 +1 @@", Lines: small}}},
	}}
}

// bigModel opens the diff of bigDiff(n).
func bigModel(t *testing.T, n int) (*app.Model, *diffRecorder) {
	t.Helper()
	terms, _ := fakeTerms(t)
	rec := &diffRecorder{Diff: bigDiff(n)}
	d := baseDeps(twoProjectState(), terms)
	d.Diff = rec.fn
	m := app.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	leader(m, key('d'))
	return m, rec
}

// A file over the cap arrives folded to its header, which says how large it
// is and how to open it, as GitHub's "Large diffs are not rendered by
// default" does; the files after it are on screen (#773).
func TestReview_aLargeFileArrivesFolded_issue773(t *testing.T) {
	m, _ := bigModel(t, 20000)

	frame := stripSGR(m.View().Content)
	if !strings.Contains(frame, "20000 lines · enter to load") {
		t.Errorf("the large file's header does not say it is folded and how large:\n%s", frame)
	}
	if strings.Contains(frame, "row 0") || !strings.Contains(frame, "small change") {
		t.Errorf("the large file is drawn, or the file after it is not:\n%s", frame)
	}
}

// enter on its header loads it, and a reload of the same diff keeps it open:
// the operator decided (#773).
func TestReview_anOpenedLargeFileStaysOpen_issue773(t *testing.T) {
	m, _ := bigModel(t, 3000)
	press(m, special(tea.KeyEnter)) // the cursor starts on the first header

	leader(m, key('d'))
	leader(m, key('d')) // close and open: the diff loads again

	if frame := stripSGR(m.View().Content); !strings.Contains(frame, "row 0") {
		t.Errorf("the large file the operator opened folded again on a reload:\n%s", frame)
	}
}

// A file under the cap arrives open, as every file did.
func TestReview_aSmallFileArrivesOpen_issue773(t *testing.T) {
	m, _ := bigModel(t, 50)

	if frame := stripSGR(m.View().Content); !strings.Contains(frame, "row 0") || strings.Contains(frame, "enter to load") {
		t.Errorf("a small file arrived folded:\n%s", frame)
	}
}

// A generated file - a lockfile, a generated client - arrives folded in the
// diff too, as #338 folds it in the tree (#773).
func TestReview_aGeneratedFileArrivesFolded_issue773(t *testing.T) {
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: bigDiff(10)}).fn
	d.Generated = (&generatedDetector{Gen: map[string]bool{"fixture.json": true}}).fn
	m := app.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	leader(m, key('d'))

	frame := stripSGR(m.View().Content)
	if strings.Contains(frame, "row 0") || !strings.Contains(frame, "generated · enter to load") {
		t.Errorf("the generated file arrived open, or its header does not say why it is folded:\n%s", frame)
	}
}

// BenchmarkReview_commentOnLargeDiff is a comment added and drawn on a diff
// that holds a 20000-line file (#773). TestReview_aCommentReLexesNothing
// holds the work; this measures the time.
func BenchmarkReview_commentOnLargeDiff(b *testing.B) {
	t := &testing.T{}
	m, _ := bigModel(t, 20000)
	press(m, key('j'))
	press(m, key('j')) // the small file's line
	for b.Loop() {
		press(m, key('c'))
		typeIn(m, "x")
		m.View()
	}
}
