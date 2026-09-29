package ui_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/infra/forge"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// scene is one screen a person can reach, built the way they reach it: a
// fixture, a window size, then the keys (#620). The goldens, the key table
// and the message table all start from these.
type scene struct {
	name  string
	build func(t *testing.T, w, h int) *ui.Model
}

// sceneIn builds a scene over terms the caller holds, so the key table can
// count what reached a session's PTY (invariant 1).
type sceneIn func(t *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model

// fresh adapts a sceneIn to a scene over new fakes, for the goldens.
func fresh(in sceneIn) func(t *testing.T, w, h int) *ui.Model {
	return func(t *testing.T, w, h int) *ui.Model {
		terms, _ := fakeTerms(t)
		return in(t, terms, w, h)
	}
}

// then runs in and presses ctrl+o k on the result.
func then(in sceneIn, k rune) sceneIn {
	return func(t *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model {
		return leaderOn(in(t, terms, w, h), k)
	}
}

var scenes = []scene{
	{"terminal", fresh(terminalScene)},
	{"sidebar_many", sidebarManyScene},
	{"diff", fresh(diffScene)},
	{"diff_turn", fresh(diffTurnScene)},
	{"tree", fresh(treeScene)},
	{"preview", fresh(previewScene)},
	{"gate", fresh(gateScene)},
	{"tracker", fresh(trackerScene)},
	{"tracker_item", fresh(trackerItemScene)},
	{"help", fresh(then(terminalScene, '?'))},
	{"new_session", fresh(then(terminalScene, 'n'))},
	{"switcher", fresh(then(terminalScene, '/'))},
	{"zoomed_diff", fresh(then(diffScene, 'z'))},
}

// leaderOn presses ctrl+o then k and returns m.
func leaderOn(m *ui.Model, k rune) *ui.Model {
	leader(m, key(k))
	return m
}

func terminalScene(_ *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model {
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

func diffScene(t *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model {
	d := sceneDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn
	return leaderOn(sized(ui.NewModel(d), w, h), 'd')
}

func diffTurnScene(t *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model {
	tr := &turnRecorder{}
	tr.Diff = turnDiffParsed(t)
	d := sceneDeps(twoProjectState(), terms)
	d.Diff, d.Turn = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn, tr.funcs()
	m := leaderOn(sized(ui.NewModel(d), w, h), 'd')
	pressAndSettle(m, key('t'))
	return m
}

func treeScene(t *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model {
	d := sceneDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn
	d.Files = (&fileLister{Paths: []string{"go.mod", "internal/ui/model.go", "internal/ui/render.go", "new.txt"}}).fn
	d.Preview = (&previewReader{Files: map[string]string{"go.mod": "module omatty\n\ngo 1.26"}}).fn
	return leaderOn(sized(ui.NewModel(d), w, h), 'f')
}

// previewScene folds internal/ and opens go.mod, the path
// TestModel_EnterCollapsesADirectoryAndPreviewsAFile_issue24 walks.
func previewScene(t *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model {
	m := treeScene(t, terms, w, h)
	press(m, special(tea.KeyEnter))
	press(m, key('j'))
	pressAndSettle(m, special(tea.KeyEnter))
	return m
}

func gateScene(t *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model {
	m := diffScene(t, terms, w, h)
	m.SetGateReport("s1", gateReport(gate.Pass, gate.Pass, gate.Fail, gate.Pending))
	return leaderOn(m, 'g')
}

func trackerScene(_ *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model {
	fi := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{}, Now: fixedNow}
	fp := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{}, Now: fixedNow}
	fi.Lists["/p/omatty"] = trackerIssues()
	fp.Lists["/p/omatty"] = trackerPRs()
	d := sceneDeps(withEmptyProject(), terms)
	d.Issues, d.PRs = fi.List, fp.List
	return leaderOn(sized(ui.NewModel(d), w, h), 'i')
}

// trackerItemScene is itemModel's fixture over the caller's terms: issue #397
// open, with its body and one comment.
func trackerItemScene(_ *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model {
	fi := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{}, Now: fixedNow}
	fp := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{}, Now: fixedNow}
	fi.Lists["/p/omatty"] = []forge.Issue{{Number: 397, Title: "read one item", Updated: fixedNow}}
	fp.Lists["/p/omatty"] = []forge.PR{{Number: 400, Title: "forge.ListIssues", State: forge.Open, Updated: fixedNow}}
	d := sceneDeps(withEmptyProject(), terms)
	d.Issues, d.PRs = fi.List, fp.List
	d.Item = ui.ForgeItemFuncs{Issue: sceneItems().issue, PR: sceneItems().pr}
	m := leaderOn(sized(ui.NewModel(d), w, h), 'i')
	pressDeliver(m, special(tea.KeyEnter))
	return m
}

func sceneItems() *FakeItems {
	items := &FakeItems{Issues: map[int]forge.Detail{}, PRs: map[int]forge.Detail{}}
	items.Issues[397] = forge.Detail{
		Number: 397, Title: "read one item", Author: "WilsonSousajr",
		Body:     "A list of titles says what is open; what they are about needs the body.",
		Created:  fixedNow.Add(-24 * time.Hour),
		Comments: []forge.Comment{{Author: "someone", Body: "and the comments under it", At: fixedNow}},
	}
	items.PRs[400] = forge.Detail{Number: 400, Title: "forge.ListIssues", Author: "WilsonSousajr", Body: "the first slice"}
	return items
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
