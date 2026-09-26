package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// generatedDetector is a named GeneratedFunc fake recording what it was asked
// to classify.
type generatedDetector struct {
	Gen   map[string]bool
	Err   error
	Asked [][]string
}

func (g *generatedDetector) fn(_ registry.Session, paths []string) (map[string]bool, error) {
	g.Asked = append(g.Asked, paths)
	return g.Gen, g.Err
}

// modelWithGenerated opens the tree on a listing that holds a lockfile and a
// coverage directory beside source, with the detection already wired.
func modelWithGenerated(t *testing.T, gen map[string]bool) (*ui.Model, *generatedDetector) {
	t.Helper()
	terms, _ := fakeTerms(t)
	det := &generatedDetector{Gen: gen}
	lister := &fileLister{Paths: []string{
		"go.mod", "go.sum", "coverage/lcov.info", "internal/ui/model.go", "new.txt",
	}}
	d := baseDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn
	d.Files, d.Generated = lister.fn, det.fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('f'))
	return m, det
}

// The gap #338 names: a lockfile and a coverage directory sit in the review
// queue beside source, with equal weight.
func TestModel_theTreeFoldsGeneratedFilesAway_issue338(t *testing.T) {
	m, det := modelWithGenerated(t, map[string]bool{"go.sum": true, "coverage/lcov.info": true})

	view := m.View().Content

	if strings.Contains(view, "go.sum") {
		t.Errorf("go.sum is in the listing:\n%s", view)
	}
	if strings.Contains(view, "lcov.info") || strings.Contains(view, "coverage/") {
		t.Errorf("the coverage directory is still listed:\n%s", view)
	}
	lineWith(t, view, "go.mod")   // a lockfile's neighbour stays
	lineWith(t, view, "model.go") // and so does source
	if len(det.Asked) == 0 {
		t.Fatal("nothing was asked to classify")
	}
}

// Still listed, still reviewable: g is what makes the fold a fold rather than a
// deletion.
func TestModel_gShowsTheFoldedFilesAndFoldsThemAgain_issue338(t *testing.T) {
	m, _ := modelWithGenerated(t, map[string]bool{"go.sum": true})

	press(m, key('g'))

	if !strings.Contains(m.View().Content, "go.sum") {
		t.Errorf("g did not bring the folded files back:\n%s", m.View().Content)
	}

	press(m, key('g'))

	if strings.Contains(m.View().Content, "go.sum") {
		t.Error("a second g did not fold them away again")
	}
}

// A listing that is short because rows were withheld reads exactly like a
// complete one. The title has to say so - the same argument the filter marker
// makes (#285).
func TestModel_theTitleSaysHowManyAreFolded_issue338(t *testing.T) {
	m, _ := modelWithGenerated(t, map[string]bool{"go.sum": true, "coverage/lcov.info": true})

	if title := lineWith(t, m.View().Content, "files ·"); !strings.Contains(title, "⊞2") {
		t.Errorf("the title does not say two rows are withheld: %q", title)
	}

	press(m, key('g'))

	if title := lineWith(t, m.View().Content, "files ·"); strings.Contains(title, "⊞") {
		t.Errorf("the marker survived showing them: %q", title)
	}
}

// With nothing detected the tree is what it always was. The unwired default
// must be invisible.
func TestModel_nothingGeneratedChangesNothing_issue338(t *testing.T) {
	m, _ := modelWithGenerated(t, nil)

	view := m.View().Content

	lineWith(t, view, "go.sum")
	if title := lineWith(t, view, "files ·"); strings.Contains(title, "⊞") {
		t.Errorf("the fold marker is shown with nothing folded: %q", title)
	}
}

// The second half of #338: M10's markers make a generated file look worse than
// it is. It has no test and never will, so an uncovered marker on it says
// something true about the file and nothing about the change.
func TestModel_aGeneratedFileGetsNoCoverageMarkers_issue338(t *testing.T) {
	m := modelWithOverlay(t, overlayProfile)

	// Before: the marker is there, which is what #255 shipped.
	if got := lineWith(t, m.View().Content, "b := 3"); !strings.Contains(got, "!    b := 3") {
		t.Fatalf("the fixture is not marking the uncovered line at all:\n%s", got)
	}

	m.Update(ui.GeneratedMsgFor("s1", map[string]bool{"internal/ui/model.go": true}))

	view := m.View().Content
	if got := lineWith(t, view, "b := 3"); strings.Contains(got, "!") {
		t.Errorf("a generated file still carries an uncovered marker:\n%s", got)
	}
	if strings.Contains(view, "uncovered") {
		t.Errorf("a generated file still carries the uncovered count:\n%s", view)
	}
}
