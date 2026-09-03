package ui_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// errListing is what a broken git listing looks like to the model.
var errListing = errors.New("no files today")

// fileLister is a named ListFilesFunc fake recording the directories it was
// asked to list.
type fileLister struct {
	Paths []string
	Err   error
	Asked []string
}

func (l *fileLister) fn(dir string) ([]string, error) {
	l.Asked = append(l.Asked, dir)
	return l.Paths, l.Err
}

// previewReader is a named PreviewFunc fake serving canned file contents.
type previewReader struct {
	Files map[string]string
	Err   error
	Read  []string
}

func (p *previewReader) fn(_ string, rel string) (review.Preview, error) {
	p.Read = append(p.Read, rel)
	if p.Err != nil {
		return review.Preview{}, p.Err
	}
	return review.Preview{Path: rel, Lines: strings.Split(p.Files[rel], "\n")}, nil
}

func modelWithTree(t *testing.T) (*ui.Model, map[string]*termwrap.Fake, *fileLister, *previewReader) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	lister := &fileLister{Paths: []string{"go.mod", "internal/ui/model.go", "internal/ui/render.go", "new.txt"}}
	reader := &previewReader{Files: map[string]string{"go.mod": "module omatty\n\ngo 1.26"}}
	d := baseDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn
	d.Files = lister.fn
	d.Preview = reader.fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m, fakes, lister, reader
}

func TestModel_LeaderFOpensTheTreeWithTouchedFilesMarked_issue24(t *testing.T) {
	m, _, lister, _ := modelWithTree(t)

	leader(m, key('f'))

	if m.ReviewView() != ui.ViewTree || !m.ReviewFocused() {
		t.Fatalf("view = %v focused = %v, want the tree focused", m.ReviewView(), m.ReviewFocused())
	}
	if len(lister.Asked) != 1 || lister.Asked[0] != "" {
		t.Errorf("files listed for %v, want the session dir (empty in the fixture state)", lister.Asked)
	}
	view := m.View().Content
	lineWith(t, view, "* model.go")
	if strings.Contains(lineWith(t, view, "render.go"), "*") {
		t.Error("render.go is marked touched but the diff does not change it")
	}
	lineWith(t, view, "▾ internal/")
}

func TestModel_EnterCollapsesADirectoryAndPreviewsAFile_issue24(t *testing.T) {
	m, _, _, reader := modelWithTree(t)
	leader(m, key('f'))

	press(m, key('j')) // internal/
	press(m, special(tea.KeyEnter))
	if strings.Contains(m.View().Content, "model.go") {
		t.Error("enter on a directory did not collapse it")
	}
	press(m, key('k')) // go.mod
	_, cmd := m.Update(special(tea.KeyEnter))
	deliver(m, cmd)

	if m.ReviewView() != ui.ViewPreview || len(reader.Read) != 1 || reader.Read[0] != "go.mod" {
		t.Fatalf("view = %v, read %v; want a preview of go.mod", m.ReviewView(), reader.Read)
	}
	if !strings.Contains(m.View().Content, "module omatty") {
		t.Errorf("preview content missing:\n%s", m.View().Content)
	}
	press(m, special(tea.KeyEscape))
	if m.ReviewView() != ui.ViewTree || !m.ReviewFocused() {
		t.Error("esc from the preview should return to the tree, still focused")
	}
	press(m, special(tea.KeyEscape))
	if m.ReviewFocused() {
		t.Error("esc from the tree should return focus to the terminal")
	}
}

func TestModel_LeaderFFromTheDiffSwitchesViewAndAgainCloses_issue24(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	leader(m, key('d'))

	leader(m, key('f'))
	if m.ReviewView() != ui.ViewTree || !m.ReviewOpen() {
		t.Fatal("ctrl+o f from the diff did not switch to the tree")
	}
	leader(m, key('d'))
	if m.ReviewView() != ui.ViewDiff {
		t.Fatal("ctrl+o d from the tree did not switch back to the diff")
	}
	leader(m, key('f'))
	leader(m, key('f'))
	if m.ReviewOpen() {
		t.Error("ctrl+o f from the tree did not close the column")
	}
}

// The tree footer names the keys that work there; the diff's keymap would be
// a lie, since c and S do nothing in the tree.
func TestModel_TreeFooterReplacesTheDiffKeymap_issue24(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})

	leader(m, key('f'))

	foot := lineWith(t, m.View().Content, "enter open")
	if strings.Contains(foot, "submit") {
		t.Errorf("tree footer %q offers the diff's submit key", foot)
	}
}

// The column follows the sidebar, so ctrl+o j with the tree open lists the
// newly focused session rather than leaving the previous one's files up.
func TestModel_MovingSessionsRelistsTheTree_issue24(t *testing.T) {
	m, _, lister, _ := modelWithTree(t)
	leader(m, key('f'))

	leader(m, key('j'))

	if len(lister.Asked) != 2 {
		t.Errorf("listed %d times, want a second listing for the new session", len(lister.Asked))
	}
}

// A lister that fails must say so in the pane rather than showing an empty
// worktree, which reads as "this session has no files".
func TestModel_AFailedListingIsReported_issue24(t *testing.T) {
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn
	d.Files = (&fileLister{Err: errListing}).fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	leader(m, key('f'))

	lineWith(t, m.View().Content, "no files today")
}

// Without a lister wired in, the pane names the missing wiring rather than
// drawing an empty tree (issue #76's rule, applied to Deps.Files).
func TestModel_NoListerConfiguredNamesTheMissingWiring_issue24(t *testing.T) {
	terms, _ := fakeTerms(t)
	m := ui.NewModel(baseDeps(twoProjectState(), terms))
	// Wide enough for the whole message: the column truncates to its width.
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})

	leader(m, key('f'))

	lineWith(t, m.View().Content, "no file lister configured")
}

// `git ls-files` returns before `git diff`, so the listing usually lands
// first and the marks have to be applied to the tree already on screen.
func TestModel_ADiffArrivingAfterTheListingStillMarksTheTree_issue24(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	leader(m, key('f'))

	m.Update(ui.FilesLoadedMsg{SessionID: "s1", Paths: []string{"internal/ui/model.go", "new.txt"}})
	m.Update(ui.DiffLoadedMsg{SessionID: "s1", Diff: sampleDiffParsed(t)})

	lineWith(t, m.View().Content, "* model.go")
}

func TestModel_PreviewScrollsAndStopsAtTheEnds_issue24(t *testing.T) {
	m, _, _, reader := modelWithTree(t)
	reader.Files["go.mod"] = strings.Repeat("line\n", 200)
	leader(m, key('f'))
	_, cmd := m.Update(special(tea.KeyEnter)) // go.mod is the first row
	deliver(m, cmd)

	press(m, key('k')) // already at the top
	first := lineWith(t, m.View().Content, "line")
	if !strings.Contains(first, "   1  line") {
		t.Errorf("first row = %q, want line 1: k at the top must not scroll past it", first)
	}
	for range 5 {
		press(m, key('j'))
	}
	if !strings.Contains(m.View().Content, "   6  line") {
		t.Errorf("after five j the preview does not start at line 6:\n%s", m.View().Content)
	}
}
