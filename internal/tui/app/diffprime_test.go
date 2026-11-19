package app_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	dsession "github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// IndexHighlighter colours a line with its place in the stream it was lexed
// in, so a test can tell a line lexed with its whole file («4») from one lexed
// with its hunk alone («1»).
type IndexHighlighter struct{}

func (IndexHighlighter) Lines(_ string, lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = fmt.Sprintf("«%d»%s", i, l)
	}
	return out
}

// oldFileReader is a diff's old side by "rev:path", recording what it read.
type oldFileReader struct {
	Files map[string]string
	Read  []string
}

func (o *oldFileReader) fn(_ dsession.Session, rev, path string) (dreview.Preview, error) {
	o.Read = append(o.Read, rev+":"+path)
	text, ok := o.Files[rev+":"+path]
	if !ok {
		return dreview.Preview{}, fmt.Errorf("oldFileReader: no %s at %s", path, rev)
	}
	return dreview.Preview{Path: path, Lines: strings.Split(text, "\n")}, nil
}

// primedDiff is one file whose hunk starts four lines into it, against base.
func primedDiff() dreview.Diff {
	return dreview.Diff{Base: "base", Files: []dreview.File{{Path: "p.go", Hunks: []dreview.Hunk{{
		Header: "@@ -4,4 +4,5 @@",
		Lines: []dreview.Line{
			{Kind: dreview.LineContext, Text: "first", OldNo: 4, NewNo: 4},
			{Kind: dreview.LineAdded, Text: "second", NewNo: 5},
			{Kind: dreview.LineRemoved, Text: "gone", OldNo: 5},
			{Kind: dreview.LineContext, Text: "`", OldNo: 6, NewNo: 6},
		},
	}}}}}
}

// primedModel opens the diff of primedDiff over both of p.go's sides.
func primedModel(t *testing.T) (*app.Model, *oldFileReader) {
	t.Helper()
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Highlighter = IndexHighlighter{}
	d.Diff = (&diffRecorder{Diff: primedDiff()}).fn
	d.Preview = (&previewReader{Files: map[string]string{"p.go": "package p\n\nvar s = `\nfirst\nsecond\n`"}}).fn
	old := &oldFileReader{Files: map[string]string{"base:p.go": "package p\n\nvar s = `\nfirst\ngone\n`"}}
	d.OldFile = old.fn
	m := app.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	leader(m, key('d'))
	return m, old
}

// The diff reads each changed file's two sides whole and lexes each from its
// first line, so a hunk is coloured as the file reads, not as code that starts
// at the hunk (#800): the added line is the new file's fifth, the removed one
// the old file's fifth.
func TestDiff_eachSideIsLexedFromTheFilesStart_issue800(t *testing.T) {
	m, old := primedModel(t)

	frame := stripSGR(m.View().Content)
	for _, want := range []string{"«4»second", "«4»gone"} {
		if !strings.Contains(frame, want) {
			t.Errorf("the diff does not draw %q, lexed with its whole file:\n%s", want, frame)
		}
	}
	if strings.Join(old.Read, " ") != "base:p.go" {
		t.Errorf("old sides read = %v, want p.go at the diff's base", old.Read)
	}
}

// A load's sides that arrive after a newer diff are not that diff's, and are
// dropped (#800).
func TestDiff_staleSidesAreDropped_issue800(t *testing.T) {
	m, _ := primedModel(t)
	m.Update(app.SidesLoadedMsg{SessionID: "s1", Seq: 0})

	if frame := stripSGR(m.View().Content); !strings.Contains(frame, "«4»second") {
		t.Errorf("an old load's sides replaced the current ones:\n%s", frame)
	}
}
