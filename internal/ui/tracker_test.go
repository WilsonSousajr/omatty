package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// trackerModel is a sized model with both forge lists wired and one project's
// issues and pull requests already answered.
func trackerModel(t *testing.T) (*ui.Model, *FakeIssues, *FakePRs) {
	t.Helper()
	m, fi, fp := modelWithBothLists(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	fi.Lists["/p/omatty"] = []forge.Issue{
		{Number: 399, Title: "filter the tracker", Labels: []string{"feat", "M14"}, Updated: fixedNow.Add(-48 * time.Hour)},
		{Number: 396, Title: "the tracker view", Labels: []string{"feat"}, Updated: fixedNow.Add(-2 * time.Hour)},
	}
	fp.Lists["/p/omatty"] = []forge.PR{
		{Number: 402, Title: "header counts", State: forge.Open, CI: forge.CIPassing, Updated: fixedNow},
		{Number: 391, Title: "prepare v0.4.0", State: forge.Merged},
	}
	return m, fi, fp
}

// openTracker presses ctrl+o i and lets both polls land.
func openTracker(m *ui.Model) { leader(m, key('i')) }

// pressDeliver presses a key in the pane and runs what it scheduled. A dropped
// command leaves a poll marked in flight for ever, so a test that only Updates
// asserts against a model half-way through the keypress (#43's argument).
func pressDeliver(m *ui.Model, k tea.KeyPressMsg) {
	_, cmd := m.Update(k)
	deliver(m, cmd)
}

// trackerBody is the review column's rows as one string.
func trackerBody(t *testing.T, m *ui.Model) string {
	t.Helper()
	return strings.Join(frameLines(m), "\n")
}

// ctrl+o i is the fifth face of the review column, and it takes the keys the
// way the other four do.
func TestTracker_LeaderIOpensTheTracker_issue396(t *testing.T) {
	m, _, _ := trackerModel(t)

	openTracker(m)

	if !m.ReviewOpen() || !m.ReviewFocused() {
		t.Fatalf("after ctrl+o i: open=%v focused=%v, want both true", m.ReviewOpen(), m.ReviewFocused())
	}
	if m.ReviewView() != ui.ViewTracker {
		t.Errorf("view = %v, want ViewTracker", m.ReviewView())
	}
}

// Opening it asks for both lists now. A pane that only showed the last poll
// would be up to five minutes stale on the keypress that asked for it.
func TestTracker_OpeningItReadsBothListsNow_issue396(t *testing.T) {
	m, fi, fp := trackerModel(t)

	openTracker(m)

	if len(fi.Asked) == 0 || len(fp.Asked) == 0 {
		t.Errorf("opening asked issues=%v prs=%v, want both", fi.Asked, fp.Asked)
	}
}

// The issues, then the pull requests, each row naming its number and title.
func TestTracker_ListsOpenIssuesThenOpenPullRequests_issue396(t *testing.T) {
	m, _, _ := trackerModel(t)

	openTracker(m)

	body := trackerBody(t, m)
	for _, want := range []string{"#399", "filter the tracker", "#396", "#402", "header counts"} {
		if !strings.Contains(body, want) {
			t.Errorf("the tracker does not show %q:\n%s", want, body)
		}
	}
	if i, p := strings.Index(body, "#399"), strings.Index(body, "#402"); i > p {
		t.Errorf("#402 (a pull request) is drawn before #399 (an issue); want issues first")
	}
}

// A finished pull request is not open work. The map holds it for the card that
// says "merged" (#310); the tracker answers what is open.
func TestTracker_AFinishedPullRequestIsNotListed_issue396(t *testing.T) {
	m, _, _ := trackerModel(t)

	openTracker(m)

	if body := trackerBody(t, m); strings.Contains(body, "#391") {
		t.Errorf("the tracker lists merged #391:\n%s", body)
	}
}

// The title says how many of each, so the counts are readable without the
// sidebar (#395's are 28 cells wide).
func TestTracker_TitleCarriesTheCounts_issue396(t *testing.T) {
	m, _, _ := trackerModel(t)

	openTracker(m)

	head := frameLines(m)[0]
	for _, want := range []string{"tracker", "omatty", "2 issues", "1 pr"} {
		if !strings.Contains(head, want) {
			t.Errorf("header row = %q, want it to carry %q", head, want)
		}
	}
}

// A project with no sessions at all can be read. Every other view needs a
// session; this one needs a project, and a project you registered and have not
// started yet is exactly when its issues matter (#158).
func TestTracker_OpensOnAProjectWithNoSessions_issue396(t *testing.T) {
	m, fi, _ := trackerModel(t)
	fi.Lists["/p/empty"] = []forge.Issue{{Number: 12, Title: "first thing", Updated: fixedNow}}
	for range len(withEmptyProject().Projects) { // walk round to the empty project
		if m.SelectedProject() == "empty" {
			break
		}
		leader(m, key(']'))
	}
	if got := m.SelectedProject(); got != "empty" {
		t.Fatalf("the cursor is on %q, want the empty project", got)
	}

	openTracker(m)

	if !m.ReviewOpen() {
		t.Fatal("ctrl+o i on a project with no sessions opened nothing")
	}
	if body := trackerBody(t, m); !strings.Contains(body, "first thing") {
		t.Errorf("the tracker does not show the empty project's issue:\n%s", body)
	}
}

func TestTracker_JAndKWalkTheRows_issue396(t *testing.T) {
	m, _, _ := trackerModel(t)
	openTracker(m)

	pressDeliver(m, key('j'))
	if got := m.TrackerCursor(); got != 1 {
		t.Errorf("after j the cursor is %d, want 1", got)
	}
	pressDeliver(m, key('k'))
	if got := m.TrackerCursor(); got != 0 {
		t.Errorf("after k the cursor is %d, want 0", got)
	}
	pressDeliver(m, key('k'))
	if got := m.TrackerCursor(); got != 0 {
		t.Errorf("k at the top moved the cursor to %d, want it held at 0", got)
	}
}

// r is the diff's key for the same thing: read it again now.
func TestTracker_RReadsBothListsAgain_issue396(t *testing.T) {
	m, fi, fp := trackerModel(t)
	openTracker(m)
	fi.Asked, fp.Asked = nil, nil
	fi.later()

	pressDeliver(m, key('r'))

	if len(fi.Asked) != 1 || len(fp.Asked) != 1 {
		t.Errorf("r asked issues=%v prs=%v, want one of each", fi.Asked, fp.Asked)
	}
}

// esc hands the keys back with the column still open; a second ctrl+o i closes
// it. The round trip every view has (#124).
func TestTracker_EscHandsTheKeysBackAndTheLeaderCloses_issue396(t *testing.T) {
	m, _, _ := trackerModel(t)
	openTracker(m)

	pressDeliver(m, special(tea.KeyEscape))
	if !m.ReviewOpen() || m.ReviewFocused() {
		t.Errorf("after esc: open=%v focused=%v, want open and unfocused", m.ReviewOpen(), m.ReviewFocused())
	}

	// From either focus state the leader closes a column already showing this
	// view: refocusing instead is what made esc-then-leader an endless loop
	// (#124).
	openTracker(m)
	if m.ReviewOpen() {
		t.Error("ctrl+o i on the open tracker left the column open")
	}
}

// The trap: ReviewPane is reset whenever the shown session changes, which would
// throw the list away as the cursor moves between two sessions of one project.
// The tracker is a project's, so it survives - and its cursor with it.
func TestTracker_SurvivesMovingBetweenTwoSessionsOfOneProject_issue396(t *testing.T) {
	m, _, _ := trackerModel(t)
	openTracker(m)
	pressDeliver(m, key('j'))

	leader(m, key('j')) // to the project's second session

	if m.ReviewView() != ui.ViewTracker {
		t.Fatalf("view = %v after moving session, want ViewTracker", m.ReviewView())
	}
	if got := m.TrackerCursor(); got != 1 {
		t.Errorf("the cursor moved to %d, want it held at 1", got)
	}
	if body := trackerBody(t, m); !strings.Contains(body, "#399") {
		t.Errorf("the list was thrown away moving between two sessions of one project:\n%s", body)
	}
}

// Moving to another project re-points it, the way the tree follows the sidebar
// (#24), and starts its cursor at the top: it is a different list.
func TestTracker_FollowsTheSidebarToAnotherProject_issue396(t *testing.T) {
	m, fi, _ := trackerModel(t)
	fi.Lists["/p/api-svc"] = []forge.Issue{{Number: 77, Title: "api thing", Updated: fixedNow}}
	openTracker(m)
	pressDeliver(m, key('j'))

	leader(m, key(']')) // to api-svc

	if got := m.TrackerCursor(); got != 0 {
		t.Errorf("the cursor is %d on another project's list, want 0", got)
	}
	if body := trackerBody(t, m); !strings.Contains(body, "api thing") {
		t.Errorf("the tracker did not follow to api-svc:\n%s", body)
	}
}

// Each "nothing to show" state says which one it is: they call for different
// things from the operator, which is renderGate's argument (#231).
func TestTracker_SaysWhyItHasNothingToShow_issue396(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want string
	}{
		{"gh is missing", forge.ErrNoGH, "gh"},
		{"not on GitHub", fmt.Errorf("forge: no git remotes found: %w", forge.ErrNotGitHub), "not on GitHub"},
		{"nothing open", nil, "nothing open"},
	} {
		m, fi, fp := modelWithBothLists(t)
		m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
		if tt.err != nil {
			for _, root := range []string{"/p/omatty", "/p/api-svc", "/p/empty"} {
				fi.Errs[root], fp.Errs[root] = tt.err, tt.err
			}
		}

		openTracker(m)

		if body := trackerBody(t, m); !strings.Contains(body, tt.want) {
			t.Errorf("%s: the tracker does not say %q:\n%s", tt.name, tt.want, body)
		}
	}
}

// Before either list has answered it says it is reading, rather than showing an
// empty list that reads as "nothing is open" (noDiff's argument, #21).
func TestTracker_SaysItIsReadingBeforeTheFirstAnswer_issue396(t *testing.T) {
	m, _, _ := trackerModel(t)

	press(m, ctrl('o'))
	press(m, key('i')) // no deliver: the polls are still in flight

	if body := trackerBody(t, m); !strings.Contains(body, "reading") {
		t.Errorf("the tracker does not say it is reading:\n%s", body)
	}
}

// h and l reach a title too long for the column, and 0 comes back. The clamp is
// measured from the same row builder the renderer uses, so the pan can never
// walk the text off the screen (#94, #133).
func TestTracker_HAndLPanALongTitle_issue396(t *testing.T) {
	m, fi, _ := trackerModel(t)
	fi.Lists["/p/omatty"] = append(fi.Lists["/p/omatty"], forge.Issue{
		Number: 400, Title: strings.Repeat("a very long issue title ", 12), Updated: fixedNow,
	})
	openTracker(m)

	pressDeliver(m, key('l'))
	panned := m.ReviewColOffset()
	if panned == 0 {
		t.Fatal("l did not pan the tracker")
	}
	if body := trackerBody(t, m); strings.Contains(body, "#399") {
		t.Errorf("a panned tracker still shows the left edge:\n%s", body)
	}

	pressDeliver(m, key('0'))
	if got := m.ReviewColOffset(); got != 0 {
		t.Errorf("0 left the offset at %d, want the left edge", got)
	}

	// The clamp holds: a burst past the widest row stops at it.
	for range 40 {
		pressDeliver(m, key('l'))
	}
	if got := m.ReviewColOffset(); got <= panned {
		t.Errorf("a burst of l reached %d, want past the first press's %d", got, panned)
	}
	last := m.ReviewColOffset()
	pressDeliver(m, key('l'))
	if got := m.ReviewColOffset(); got != last {
		t.Errorf("l past the widest row moved the offset to %d, want it clamped at %d", got, last)
	}
}

// The rule between the two lists does not pan: it is a label, not content, so
// it fits its own column the way a diff's file header does (#291). Panned right
// it went blank, and the two lists merged into one with nothing to say where the
// issues stopped - found by the real-PTY smoke run, not by a test.
func TestTracker_TheRuleDoesNotPanAway_issue396(t *testing.T) {
	m, _, _ := trackerModel(t)
	openTracker(m)

	for range 8 {
		pressDeliver(m, key('l'))
	}

	if m.ReviewColOffset() == 0 {
		t.Fatal("the tracker did not pan")
	}
	if body := trackerBody(t, m); !strings.Contains(body, "pull requests") {
		t.Errorf("the panned tracker lost the rule between its two lists:\n%s", body)
	}
}
