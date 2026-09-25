package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// filterModel is a tracker with enough rows for a filter to be worth using.
func filterModel(t *testing.T) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	fi := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{}, Now: fixedNow}
	fp := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{}, Now: fixedNow}
	fi.Lists["/p/omatty"] = []forge.Issue{
		{Number: 399, Title: "filter the tracker", Labels: []string{"feat"}, Updated: fixedNow},
		{Number: 369, Title: "brew install warns", Labels: []string{"fix"}, Updated: fixedNow},
		{Number: 315, Title: "upstream parser buffer", Updated: fixedNow},
	}
	fp.Lists["/p/omatty"] = []forge.PR{{Number: 405, Title: "work from an issue", State: forge.Open, Updated: fixedNow}}
	d := baseDeps(withEmptyProject(), terms)
	d.Issues, d.PRs = fi.List, fp.List
	d.Clock = func() time.Time { return fi.Now }
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	openTracker(m)
	return m
}

// / narrows the list as you type, over the number, the title and the labels -
// the three things a row shows.
func TestTrackerFilter_NarrowsOnNumberTitleAndLabel_issue399(t *testing.T) {
	for _, tt := range []struct {
		query string
		want  string
		gone  string
	}{
		{"brew", "#369", "#399"},
		{"315", "#315", "#369"},
		{"fix", "#369", "#315"},
	} {
		m := filterModel(t)
		pressDeliver(m, key('/'))
		typeInto(m, tt.query)

		body := trackerBody(t, m)
		if !strings.Contains(body, tt.want) || strings.Contains(body, tt.gone) {
			t.Errorf("/%s kept %v and dropped %v wrongly:\n%s", tt.query, tt.want, tt.gone, body)
		}
	}
}

// enter keeps the query and hands the keys back to the list; esc lifts it, so
// the narrowing goes before the column does (#198).
func TestTrackerFilter_EnterKeepsItAndEscLiftsIt_issue399(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, key('/'))
	typeInto(m, "brew")

	pressDeliver(m, special(tea.KeyEnter))
	if body := trackerBody(t, m); strings.Contains(body, "#399") {
		t.Errorf("enter did not keep the filter:\n%s", body)
	}
	if !m.ReviewFocused() {
		t.Error("enter handed the keys past the list")
	}

	pressDeliver(m, special(tea.KeyEscape))
	if body := trackerBody(t, m); !strings.Contains(body, "#399") {
		t.Errorf("esc did not lift the filter:\n%s", body)
	}
	if !m.ReviewOpen() || !m.ReviewFocused() {
		t.Error("the esc that lifted the filter also left the list")
	}
}

// The marker is never dropped from the title. A filtered list is short because a
// filter is in force; a missing marker leaves it indistinguishable from a
// complete one, and nothing else on screen says otherwise (#285).
func TestTrackerFilter_TheTitleAlwaysSaysAFilterIsInForce_issue399(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, key('/'))
	typeInto(m, "brew")
	pressDeliver(m, special(tea.KeyEnter))

	if head := frameLines(m)[0]; !strings.Contains(head, "/brew") {
		t.Errorf("header row = %q, want the filter marker", head)
	}
}

// A query nothing matches says so, rather than showing the empty list that means
// "nothing is open in this project".
func TestTrackerFilter_AQueryThatMatchesNothingSaysSo_issue399(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, key('/'))
	typeInto(m, "zzzz")

	if body := trackerBody(t, m); !strings.Contains(body, "match") {
		t.Errorf("an empty filter result does not say why it is empty:\n%s", body)
	}
}

// The cursor cannot be left past the end of a shorter list, and what the keys
// act on is what the filtered list shows.
func TestTrackerFilter_TheCursorFollowsTheNarrowedList_issue399(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, key('j'))
	pressDeliver(m, key('j')) // on #315, the third row

	pressDeliver(m, key('/'))
	typeInto(m, "brew")
	pressDeliver(m, special(tea.KeyEnter))

	if got := m.TrackerCursor(); got >= m.TrackerRowCount() {
		t.Errorf("the cursor is %d over %d rows, want it inside the narrowed list", got, m.TrackerRowCount())
	}
}

// A filter that hides every pull request hides their rule too: a rule with
// nothing under it says a list is there when it is not.
func TestTrackerFilter_TheRuleGoesWithItsList_issue399(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, key('/'))
	typeInto(m, "brew")

	if body := trackerBody(t, m); strings.Contains(body, "pull requests") {
		t.Errorf("the rule outlived the pull requests it labels:\n%s", body)
	}
}
