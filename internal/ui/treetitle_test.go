package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// modelWithNamedTree opens the file tree on a session with the given title,
// long enough that the column has to do something about it.
func modelWithNamedTree(t *testing.T, title string) *ui.Model {
	t.Helper()
	st := registry.State{
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty"}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: title, Dir: "/p/omatty", Branch: "work"}},
	}
	deps := baseDeps(st, fakeTermsFor(st))
	deps.Files = (&fileLister{Paths: []string{"go.mod", "internal/gate/run.go", "internal/ui/render.go"}}).fn
	m := ui.NewModel(deps)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	leader(m, key('f'))
	return m
}

// filterTo types query into the tree's filter line and commits it, which is
// what puts the marker in the title.
func filterTo(m *ui.Model, query string) {
	press(m, key('/'))
	for _, r := range query {
		press(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	press(m, special(tea.KeyEnter))
}

// treeTitleOf is the review column's title alone. The row it sits on also
// carries the sidebar, which names the session in full whatever the title
// does, so asserting on the row would pass for the wrong reason.
func treeTitleOf(t *testing.T, m *ui.Model) string {
	t.Helper()
	row := stripSGR(lineWith(t, m.View().Content, "files ·"))
	return strings.TrimRight(row[strings.Index(row, "files ·"):], " ")
}

// The marker is why the listing is short. A cut one leaves a filtered tree
// indistinguishable from a complete one, and nothing else on screen says
// otherwise - which is worse than a dropped count, and the reason this title
// needs its own rule (#285).
func TestModel_theTreeTitleKeepsTheFilterMarker_issue285(t *testing.T) {
	m := modelWithNamedTree(t, "a-long-session-name")
	filterTo(m, "gate")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	title := treeTitleOf(t, m)

	if !strings.HasSuffix(title, "/gate") {
		t.Errorf("the filter marker was cut, so a filtered listing reads as a complete one:\n%s", title)
	}
	if strings.Contains(title, "a-long-session-name") {
		t.Errorf("the whole name survived a column too narrow for it:\n%s", title)
	}
}

// A name is shortened rather than dropped while there is room to recognise
// one: `a-l…ame` still identifies the session, where `0 comment` identified
// nothing, which is why #283 drops counts whole and this drops nothing yet.
func TestModel_theTreeTitleShortensTheNameBeforeDroppingIt_issue285(t *testing.T) {
	m := modelWithNamedTree(t, "a-long-session-name")
	filterTo(m, "gate")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	title := treeTitleOf(t, m)

	if !strings.Contains(title, "…") {
		t.Errorf("the name was dropped or cut rather than elided:\n%s", title)
	}
	if !strings.Contains(title, "/gate") {
		t.Errorf("the marker did not survive:\n%s", title)
	}
	if !strings.Contains(title, "a-l") {
		t.Errorf("the name's head did not survive, so it names nothing:\n%s", title)
	}
}

// Below six cells a name says nothing worth the room, so it goes entirely -
// the session is still named by the sidebar cursor and the pane title beside
// it, which is what makes this the part that can go.
func TestModel_aVeryNarrowTreeTitleDropsTheNameAndKeepsTheFilter_issue285(t *testing.T) {
	m := modelWithNamedTree(t, "a-long-session-name")
	filterTo(m, "internal")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})

	title := treeTitleOf(t, m)

	if title != "files · /internal" {
		t.Errorf("title = %q, want the view and the filter alone", title)
	}
}

// With room, nothing is given up and nothing is elided.
func TestModel_aWideTreeTitleShowsEverything_issue285(t *testing.T) {
	m := modelWithNamedTree(t, "a-long-session-name")
	filterTo(m, "gate")

	title := treeTitleOf(t, m)

	// With room, the position #424 added is not given up either.
	// Two rows since #430 compacted internal/ and gate/ into one.
	if title != "files · a-long-session-name /gate · 1/2" {
		t.Errorf("a wide column did not draw the whole title:\n%s", title)
	}
}

// With no filter the name has the rest of the column to itself, and still
// shrinks rather than vanishing.
func TestModel_anUnfilteredTreeTitleShortensTheName_issue285(t *testing.T) {
	m := modelWithNamedTree(t, "a-really-quite-long-session-name")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	title := treeTitleOf(t, m)

	if !strings.Contains(title, "…") || !strings.Contains(title, "a-re") {
		t.Errorf("the name was cut rather than elided:\n%s", title)
	}
}

// Shortening must not buy width back: the row is still exactly the frame.
func TestModel_theTreeTitleStillFitsItsColumn_issue285(t *testing.T) {
	m := modelWithNamedTree(t, "a-long-session-name")
	filterTo(m, "gate")
	for _, w := range []int{160, 120, 100, 92, 84, 76} {
		m.Update(tea.WindowSizeMsg{Width: w, Height: 30})

		for i, l := range strings.Split(m.View().Content, "\n") {
			if got := lipgloss.Width(l); got != w {
				t.Fatalf("width %d: row %d is %d cells, want %d:\n%s", w, i, got, w, l)
			}
		}
	}
}

// modelPreviewing opens the preview of a file the given path names, which is
// how the title gets a path long enough for the column to do something about.
func modelPreviewing(t *testing.T, path string) *ui.Model {
	t.Helper()
	d, err := review.ParseDiff(strings.NewReader(diffOf(map[string][]string{path: {"x := 1"}})))
	if err != nil {
		t.Fatalf("parsing the fixture: %v", err)
	}
	terms, _ := fakeTerms(t)
	deps := baseDeps(twoProjectState(), terms)
	deps.Diff = (&diffRecorder{Diff: d}).fn
	deps.Preview = (&previewReader{Files: map[string]string{path: "one line"}}).fn
	m := ui.NewModel(deps)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	leader(m, key('d'))
	down(m, 2) // the file header, the hunk header, then the added line
	pressAndSettle(m, key('o'))
	return m
}

// The table in previewtitle_internal_test.go proves the shortening; this
// proves it is wired to the column's real width, through the keys an operator
// actually presses (#287).
func TestModel_aNarrowPreviewTitleKeepsTheFilename_issue287(t *testing.T) {
	m := modelPreviewing(t, "internal/supervisor/lifecycle/restart.go")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	row := stripSGR(lineWith(t, m.View().Content, "restart.go"))
	title := strings.TrimRight(row[strings.LastIndex(row, "│")+len("│"):], " ")

	if !strings.HasSuffix(title, "restart.go") {
		t.Errorf("the filename was cut from the title:\n%q", title)
	}
	if !strings.HasPrefix(strings.TrimSpace(title), "…/") {
		t.Errorf("the title does not say the path was shortened:\n%q", title)
	}
}

// Shortening must not buy width back here either.
func TestModel_thePreviewTitleStillFitsItsColumn_issue287(t *testing.T) {
	m := modelPreviewing(t, "internal/supervisor/lifecycle/restart.go")

	for _, w := range []int{160, 120, 100, 92, 84, 76} {
		m.Update(tea.WindowSizeMsg{Width: w, Height: 30})

		for i, l := range strings.Split(m.View().Content, "\n") {
			if got := lipgloss.Width(l); got != w {
				t.Fatalf("width %d: row %d is %d cells, want %d:\n%s", w, i, got, w, l)
			}
		}
	}
}
