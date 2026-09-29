package ui_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// scene is one screen a person can reach, built the way they reach it: a
// fixture, a window size, then the keys (#620). The goldens, the key table
// and the message table all start from these.
type scene struct {
	name  string
	build func(t *testing.T, w, h int) *ui.Model
}

var scenes = []scene{
	{"terminal", terminalScene},
	{"sidebar_many", sidebarManyScene},
	{"diff", diffScene},
	{"diff_turn", diffTurnScene},
	{"tree", treeScene},
	{"preview", previewScene},
	{"gate", gateScene},
	{"tracker", trackerScene},
	{"tracker_item", trackerItemScene},
	{"help", func(t *testing.T, w, h int) *ui.Model { return leaderOn(terminalScene(t, w, h), '?') }},
	{"new_session", func(t *testing.T, w, h int) *ui.Model { return leaderOn(terminalScene(t, w, h), 'n') }},
	{"switcher", func(t *testing.T, w, h int) *ui.Model { return leaderOn(terminalScene(t, w, h), '/') }},
	{"zoomed_diff", func(t *testing.T, w, h int) *ui.Model { return leaderOn(diffScene(t, w, h), 'z') }},
}

// leaderOn presses ctrl+o then k and returns m.
func leaderOn(m *ui.Model, k rune) *ui.Model {
	leader(m, key(k))
	return m
}

func terminalScene(t *testing.T, w, h int) *ui.Model {
	terms, _ := fakeTerms(t)
	return sized(ui.NewModel(sceneDeps(twoProjectState(), terms)), w, h)
}

func sidebarManyScene(_ *testing.T, w, h int) *ui.Model {
	st := sevenProjectState()
	m := sized(ui.NewModel(sceneDeps(st, fakeTermsFor(st))), w, h)
	for range 4 {
		leader(m, key('j'))
	}
	return m
}

func diffScene(t *testing.T, w, h int) *ui.Model {
	terms, _ := fakeTerms(t)
	d := sceneDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn
	return leaderOn(sized(ui.NewModel(d), w, h), 'd')
}

func diffTurnScene(t *testing.T, w, h int) *ui.Model {
	terms, _ := fakeTerms(t)
	tr := &turnRecorder{}
	tr.Diff = turnDiffParsed(t)
	d := sceneDeps(twoProjectState(), terms)
	d.Diff, d.Turn = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn, tr.funcs()
	m := leaderOn(sized(ui.NewModel(d), w, h), 'd')
	pressAndSettle(m, key('t'))
	return m
}

func treeScene(t *testing.T, w, h int) *ui.Model {
	terms, _ := fakeTerms(t)
	d := sceneDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn
	d.Files = (&fileLister{Paths: []string{"go.mod", "internal/ui/model.go", "internal/ui/render.go", "new.txt"}}).fn
	d.Preview = (&previewReader{Files: map[string]string{"go.mod": "module omatty\n\ngo 1.26"}}).fn
	return leaderOn(sized(ui.NewModel(d), w, h), 'f')
}

// previewScene folds internal/ and opens go.mod, the path
// TestModel_EnterCollapsesADirectoryAndPreviewsAFile_issue24 walks.
func previewScene(t *testing.T, w, h int) *ui.Model {
	m := treeScene(t, w, h)
	press(m, special(tea.KeyEnter))
	press(m, key('j'))
	pressAndSettle(m, special(tea.KeyEnter))
	return m
}

func gateScene(t *testing.T, w, h int) *ui.Model {
	m := diffScene(t, w, h)
	m.SetGateReport("s1", gateReport(gate.Pass, gate.Pass, gate.Fail, gate.Pending))
	return leaderOn(m, 'g')
}

func trackerScene(t *testing.T, w, h int) *ui.Model {
	terms, _ := fakeTerms(t)
	fi := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{}, Now: fixedNow}
	fp := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{}, Now: fixedNow}
	fi.Lists["/p/omatty"] = trackerIssues()
	fp.Lists["/p/omatty"] = trackerPRs()
	d := sceneDeps(withEmptyProject(), terms)
	d.Issues, d.PRs = fi.List, fp.List
	return leaderOn(sized(ui.NewModel(d), w, h), 'i')
}

func trackerItemScene(t *testing.T, w, h int) *ui.Model {
	m, _ := itemModel(t)
	m = sized(m, w, h)
	pressDeliver(m, special(tea.KeyEnter))
	return m
}

func trackerIssues() []forge.Issue {
	return []forge.Issue{
		{Number: 399, Title: "filter the tracker", Labels: []string{"feat", "M14"}, Updated: fixedNow.Add(-48 * time.Hour)},
		{Number: 396, Title: "the tracker view", Labels: []string{"feat"}, Updated: fixedNow.Add(-2 * time.Hour)},
	}
}

func trackerPRs() []forge.PR {
	return []forge.PR{
		{Number: 402, Title: "header counts", State: forge.Open, CI: forge.CIPassing, Updated: fixedNow},
		{Number: 391, Title: "prepare v0.4.0", State: forge.Merged},
	}
}
