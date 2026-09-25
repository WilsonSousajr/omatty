package ui_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// FakeItems is a named fake for ui.ItemFunc: one body per kind and number, and
// a record of every call so the cache can be asserted (#397).
type FakeItems struct {
	Issues map[int]forge.Detail
	PRs    map[int]forge.Detail
	Err    error
	Asked  []string
}

func (f *FakeItems) issue(root string, n int) (forge.Detail, error) {
	f.Asked = append(f.Asked, "issue "+root+" "+strconv.Itoa(n))
	return f.Issues[n], f.Err
}

func (f *FakeItems) pr(root string, n int) (forge.Detail, error) {
	f.Asked = append(f.Asked, "pr "+root+" "+strconv.Itoa(n))
	return f.PRs[n], f.Err
}

// itemModel is a tracker with one issue and one pull request whose bodies are
// wired, already open on the list.
func itemModel(t *testing.T) (*ui.Model, *FakeItems) {
	t.Helper()
	terms, _ := fakeTerms(t)
	fi := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{}, Now: fixedNow}
	fp := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{}, Now: fixedNow}
	items := &FakeItems{Issues: map[int]forge.Detail{}, PRs: map[int]forge.Detail{}}
	fi.Lists["/p/omatty"] = []forge.Issue{{Number: 397, Title: "read one item", Updated: fixedNow}}
	fp.Lists["/p/omatty"] = []forge.PR{{Number: 400, Title: "forge.ListIssues", State: forge.Open, Updated: fixedNow}}
	items.Issues[397] = forge.Detail{
		Number: 397, Title: "read one item", Author: "WilsonSousajr",
		Body:    "A list of titles says what is open; what they are about needs the body.",
		Created: fixedNow.Add(-24 * time.Hour),
		Comments: []forge.Comment{
			{Author: "someone", Body: "and the comments under it", At: fixedNow},
		},
	}
	items.PRs[400] = forge.Detail{Number: 400, Title: "forge.ListIssues", Author: "WilsonSousajr", Body: "the first slice"}
	d := baseDeps(withEmptyProject(), terms)
	d.Issues, d.PRs = fi.List, fp.List
	d.Item = ui.ForgeItemFuncs{Issue: items.issue, PR: items.pr}
	d.Clock = func() time.Time { return fi.Now }
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	openTracker(m)
	return m, items
}

// enter on a row reads that item and shows its body and its comments.
func TestTrackerItem_EnterOpensTheRowUnderTheCursor_issue397(t *testing.T) {
	m, items := itemModel(t)

	pressDeliver(m, special(tea.KeyEnter))

	if m.ReviewView() != ui.ViewTrackerItem {
		t.Fatalf("view = %v after enter, want ViewTrackerItem", m.ReviewView())
	}
	if len(items.Asked) != 1 || !strings.HasPrefix(items.Asked[0], "issue /p/omatty 397") {
		t.Errorf("asked %v, want the issue under the cursor in its project's root", items.Asked)
	}
	body := trackerBody(t, m)
	for _, want := range []string{"#397", "needs the body", "someone", "and the comments under it"} {
		if !strings.Contains(body, want) {
			t.Errorf("the item view does not show %q:\n%s", want, body)
		}
	}
}

// A pull request row is the other call. Nothing guesses from the number.
func TestTrackerItem_APullRequestRowReadsThePullRequest_issue397(t *testing.T) {
	m, items := itemModel(t)
	pressDeliver(m, key('j')) // over the rule, onto the pull request
	pressDeliver(m, key('j'))

	pressDeliver(m, special(tea.KeyEnter))

	if len(items.Asked) != 1 || !strings.HasPrefix(items.Asked[0], "pr /p/omatty 400") {
		t.Errorf("asked %v, want the pull request", items.Asked)
	}
}

// esc goes back to the list, and the cursor is where it was.
func TestTrackerItem_EscReturnsToTheList_issue397(t *testing.T) {
	m, _ := itemModel(t)
	pressDeliver(m, special(tea.KeyEnter))

	pressDeliver(m, special(tea.KeyEscape))

	if m.ReviewView() != ui.ViewTracker {
		t.Errorf("view = %v after esc, want the list back", m.ReviewView())
	}
	if !m.ReviewFocused() {
		t.Error("esc from an item handed the keys back to claude; want the list to keep them")
	}
}

// The item is the list's child, so ctrl+o i over it closes the column - the
// round trip a preview has over the tree (#124).
func TestTrackerItem_TheLeaderClosesTheColumnFromAnItem_issue397(t *testing.T) {
	m, _ := itemModel(t)
	pressDeliver(m, special(tea.KeyEnter))

	openTracker(m)

	if m.ReviewOpen() {
		t.Error("ctrl+o i over an open item left the column open")
	}
}

// Read once. A per-item call on every keypress is what #358 is about.
func TestTrackerItem_ASecondLookAsksNothing_issue397(t *testing.T) {
	m, items := itemModel(t)
	pressDeliver(m, special(tea.KeyEnter))
	pressDeliver(m, special(tea.KeyEscape))

	pressDeliver(m, special(tea.KeyEnter))

	if len(items.Asked) != 1 {
		t.Errorf("asked %v, want one call for one item", items.Asked)
	}
}

// r asks again, for an item whose comments have moved on since.
func TestTrackerItem_RReadsItAgain_issue397(t *testing.T) {
	m, items := itemModel(t)
	pressDeliver(m, special(tea.KeyEnter))

	pressDeliver(m, key('r'))

	if len(items.Asked) != 2 {
		t.Errorf("asked %v, want r to have read it again", items.Asked)
	}
}

// The body is wrapped to the column, not cut at it: an issue body is prose, and
// prose read through a 35-cell window is unreadable one line at a time.
func TestTrackerItem_TheBodyIsWrappedToTheColumn_issue397(t *testing.T) {
	m, items := itemModel(t)
	items.Issues[397] = forge.Detail{
		Number: 397, Title: "wrapping", Body: strings.Repeat("word ", 80),
	}

	pressDeliver(m, special(tea.KeyEnter))

	lines := frameLines(m)
	found := 0
	for _, line := range lines {
		if strings.Count(line, "word") > 1 {
			found++
		}
	}
	if found < 3 {
		t.Errorf("the body is on %d lines, want it wrapped over several:\n%s", found, strings.Join(lines, "\n"))
	}
}

// j and k scroll a body taller than the pane.
func TestTrackerItem_JAndKScrollALongBody_issue397(t *testing.T) {
	m, items := itemModel(t)
	items.Issues[397] = forge.Detail{Number: 397, Title: "long", Body: strings.Repeat("a line of it\n", 200)}
	pressDeliver(m, special(tea.KeyEnter))
	first := frameLines(m)[3]

	for range 5 {
		pressDeliver(m, key('j'))
	}

	if got := frameLines(m)[3]; got == first {
		t.Errorf("j did not scroll the item: row is still %q", got)
	}
}

// Reading and failing are distinct states, and neither is an empty body: an
// empty pane reads as "this issue says nothing".
func TestTrackerItem_SaysWhenItIsReadingAndWhenItFailed_issue397(t *testing.T) {
	m, items := itemModel(t)

	press(m, special(tea.KeyEnter)) // no deliver: the read is in flight
	if body := trackerBody(t, m); !strings.Contains(body, "reading") {
		t.Errorf("the item view does not say it is reading:\n%s", body)
	}

	m2, items2 := itemModel(t)
	items2.Err = errors.New("HTTP 401: Bad credentials")
	pressDeliver(m2, special(tea.KeyEnter))
	if body := trackerBody(t, m2); !strings.Contains(body, "could not be read") {
		t.Errorf("a failed read does not say so:\n%s", body)
	}
	_ = items
}

// A bounded item says the rest was not read, rather than being quietly short.
func TestTrackerItem_ATruncatedItemSaysSo_issue397(t *testing.T) {
	m, items := itemModel(t)
	items.Issues[397] = forge.Detail{Number: 397, Title: "big", Body: "the start of it", Truncated: true}

	pressDeliver(m, special(tea.KeyEnter))

	// The needle is a phrase the wrap cannot break in the middle: every line of
	// this view is wrapped to the column, so "not read" straddles two of them.
	if body := trackerBody(t, m); !strings.Contains(body, "the rest of this item") {
		t.Errorf("a truncated item does not say so:\n%s", body)
	}
}
