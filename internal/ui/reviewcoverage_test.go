package ui_test

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// The sample diff adds `b := 3` at new line 11 and `c := 4` at new line 12 of
// internal/ui/model.go, and every line of new.txt. This profile says the first
// of those never ran, the second did, and says nothing at all about new.txt -
// the three cases the markers have to tell apart.
const overlayProfile = "mode: set\n" +
	"example.com/m/internal/ui/model.go:11.1,11.20 1 0\n" +
	"example.com/m/internal/ui/model.go:12.1,12.20 1 1\n"

// modelWithOverlay is a one-session model showing the sample diff with the
// given profile already loaded by a finished gate - the real path, through
// GateMsg, rather than an overlay poked into the model.
func modelWithOverlay(t *testing.T, profile string) *ui.Model {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/m\n")
	if profile != "" {
		write(t, filepath.Join(dir, "cover.out"), profile)
	}
	st := registry.State{
		Projects: []registry.Project{{Name: "omatty", Root: dir, Gate: []gate.Step{
			{Name: "cov", Run: "./cov.sh", Kind: gate.KindCoverage, Profile: "cover.out"},
		}}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "one", Dir: dir, Branch: "main"}},
	}
	deps := baseDeps(st, fakeTermsFor(st))
	rec := &diffRecorder{Diff: sampleDiffParsed(t)}
	deps.Diff = rec.fn
	m := ui.NewModel(deps)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	settle(m, second(m.Update(passingReport())))
	leader(m, key('d'))
	return m
}

// The milestone in one glance: of the lines this session added, which are not
// exercised by anything? An added line the profile says never ran is marked in
// the gutter; one that ran keeps its ordinary +.
func TestModel_anUncoveredAddedLineIsMarked_issue255(t *testing.T) {
	m := modelWithOverlay(t, overlayProfile)

	view := m.View().Content

	if got := plainLineWith(t, view, "b := 3"); !strings.Contains(got, "!    b := 3") {
		t.Errorf("the uncovered added line is not marked:\n%s", got)
	}
	if got := plainLineWith(t, view, "c := 4"); !strings.Contains(got, "+    c := 4") {
		t.Errorf("a covered added line was marked:\n%s", got)
	}
}

// Three states, and the third is the one that matters. A line in no block is
// not a statement - a declaration, a brace, a comment - so it has no verdict
// and gets no marker. Marking those would be noise that trains the eye to
// ignore the marker, which costs more than it ever gave.
func TestModel_aLineWithNoVerdictIsUnmarked_issue255(t *testing.T) {
	// Only line 12 has a block, so line 11 is a line the profile is silent on.
	m := modelWithOverlay(t, "mode: set\nexample.com/m/internal/ui/model.go:12.1,12.20 1 1\n")

	if got := plainLineWith(t, m.View().Content, "b := 3"); !strings.Contains(got, "+    b := 3") {
		t.Errorf("a line the profile says nothing about was marked:\n%s", got)
	}
}

// Only added lines are marked. A context line's coverage is not this change's
// business, and a removed line is not in the tree the profile describes.
func TestModel_contextAndRemovedLinesAreNeverMarked_issue255(t *testing.T) {
	// Every line of the hunk is uncovered, so only the rule stops the markers.
	m := modelWithOverlay(t, "mode: set\nexample.com/m/internal/ui/model.go:1.1,40.20 30 0\n")

	view := m.View().Content
	for _, c := range []struct{ needle, want string }{
		{"a := 1", "     a := 1"},
		{"b := 2", "-    b := 2"},
		{"return", "     return"},
	} {
		if got := plainLineWith(t, view, c.needle); !strings.Contains(got, c.want) {
			t.Errorf("line %q = %q, want it drawn as %q", c.needle, got, c.want)
		}
	}
	if got := plainLineWith(t, view, "b := 3"); !strings.Contains(got, "!    b := 3") {
		t.Errorf("the added line in the same hunk is not marked:\n%s", got)
	}
}

// A long diff says where to look without being scrolled: the file header
// carries the count, and it matches the markers drawn under it.
func TestModel_theFileHeaderCountsUncoveredAddedLines_issue255(t *testing.T) {
	m := modelWithOverlay(t, overlayProfile)
	// Wide enough that the header is not cut: the count is what is under test,
	// not how a narrow column truncates it.
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})

	view := m.View().Content

	if got := plainLineWith(t, view, "internal/ui/model.go"); !strings.Contains(got, "+2 -1  1 uncovered") {
		t.Errorf("file header = %q, want the uncovered count beside the diffstat", got)
	}
	if got := plainLineWith(t, view, "new.txt"); strings.Contains(got, "uncovered") {
		t.Errorf("a file the profile does not mention carries a count: %q", got)
	}
}

// A file the profile has no entry for draws exactly as it does today, to the
// cell. An overlay that shifted every row of every file it knew nothing about
// would be a redesign of the diff rather than a remark on it.
func TestModel_aFileTheProfileNeverMentions_drawsAsItDidBefore_issue255(t *testing.T) {
	with := rowsOf(t, modelWithOverlay(t, overlayProfile), "fresh", "file")
	without := rowsOf(t, modelWithOverlay(t, ""), "fresh", "file")

	if with != without {
		t.Errorf("new.txt drew differently under an overlay:\nwith    %q\nwithout %q", with, without)
	}
}

// rowsOf is the drawn rows holding each needle, joined, so a comparison is
// about what was drawn rather than about where it sat on screen.
func rowsOf(t *testing.T, m *ui.Model, needles ...string) string {
	t.Helper()
	view := m.View().Content
	out := make([]string, 0, len(needles))
	for _, n := range needles {
		out = append(out, strings.TrimRight(plainLineWith(t, view, n), " "))
	}
	return strings.Join(out, "\n")
}

// The count is the finding, so it is the last part of the header to be given
// up - and it used to be the first. At 120 columns the row ended "+2 -1  1"
// and at 140 "1 uncover", a fragment that reads as part of the diffstat
// rather than as a count of anything (#291).
func TestModel_aNarrowColumnKeepsTheUncoveredCountWhole_issue291(t *testing.T) {
	m := modelWithOverlay(t, overlayProfile)

	for _, w := range []int{160, 140, 120, 100, 92, 84, 80} {
		m.Update(tea.WindowSizeMsg{Width: w, Height: 30})
		got := plainLineWith(t, m.View().Content, "model.go")
		if !strings.Contains(got, "1 uncovered") {
			t.Errorf("at %d columns the file header is %q, want the count present and whole", w, got)
		}
	}
}

// The path is what shortens instead, from the front, so the filename - the
// part that says which file this is - survives every width (#287, #291).
func TestModel_aNarrowFileHeaderShortensThePathFromTheFront_issue291(t *testing.T) {
	m := modelWithOverlay(t, overlayProfile)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})

	got := plainLineWith(t, m.View().Content, "model.go")
	if !strings.Contains(got, "…/model.go") {
		t.Errorf("file header = %q, want the path shortened from the front with the marker", got)
	}
	if strings.Contains(got, "+2 -1") {
		t.Errorf("file header = %q, want the diffstat given up before the count", got)
	}
}

// Every drawn row is still exactly the window at every width the header has to
// fit itself in. A rule that keeps a part by growing the row would trade one
// bug for a worse one (#174, #291).
func TestModel_theFileHeaderNeverRunsPastItsColumn_issue291(t *testing.T) {
	m := modelWithOverlay(t, overlayProfile)

	for _, w := range []int{160, 140, 120, 100, 92, 84, 80} {
		m.Update(tea.WindowSizeMsg{Width: w, Height: 30})
		assertFrameIs(t, m, w, 30)
	}
}

// The header fits itself, so it no longer pans with the body: panning the
// column leaves it where it is, whole, instead of sliding the count off the
// left-hand edge (#291).
func TestModel_panningTheDiffLeavesTheFileHeaderInPlace_issue291(t *testing.T) {
	m := modelWithOverlay(t, overlayProfile)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	viewBefore := m.View().Content
	before := plainLineWith(t, viewBefore, "model.go")

	for range 4 {
		press(m, key('l'))
	}

	if m.View().Content == viewBefore {
		t.Fatal("the column never panned, so the header staying put proves nothing")
	}
	if after := plainLineWith(t, m.View().Content, "model.go"); after != before {
		t.Errorf("the file header moved when the column panned:\nbefore %q\nafter  %q", before, after)
	}
}
